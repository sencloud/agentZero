package feed

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/agentzero/server/internal/db"
	"github.com/agentzero/server/internal/llm"
	"github.com/agentzero/server/internal/model"
)

// Service 是事件流的协调器，统一调度 fetcher / matcher / extractor / pruner。
//
// 节拍：
//   - 启动时立刻跑一次 fetch + match（不抽实体，避免冷启动 LLM 风暴）
//   - 之后每 FetchInterval 抓一遍 + match + extract（限制每轮最多 ExtractPerTick 个事件）
//   - 每 PruneInterval 跑一次裁剪
//
// 关心的失败模式：单源失败不传染、LLM 抽取失败不阻塞、整体 cron 不会卡住。
type Service struct {
	db     *sql.DB
	logger *slog.Logger

	fetcher   *Fetcher
	matcher   *Matcher
	extractor *Extractor
	pruner    *Pruner

	FetchInterval  time.Duration
	PruneInterval  time.Duration
	ExtractPerTick int

	mu          sync.RWMutex
	running     bool
	lastFetchAt time.Time
	lastPruneAt time.Time
	lastError   string
	cancel      context.CancelFunc
}

// Config 控制 Service 启动参数；零值用合理默认。
type Config struct {
	FetchInterval  time.Duration
	PruneInterval  time.Duration
	ExtractPerTick int
	ExtractModel   string
}

func NewService(database *sql.DB, llmClient *llm.Client, logger *slog.Logger, cfg Config) *Service {
	if cfg.FetchInterval <= 0 {
		cfg.FetchInterval = 30 * time.Minute
	}
	if cfg.PruneInterval <= 0 {
		cfg.PruneInterval = 6 * time.Hour
	}
	if cfg.ExtractPerTick <= 0 {
		cfg.ExtractPerTick = 8
	}
	s := &Service{
		db:             database,
		logger:         logger,
		FetchInterval:  cfg.FetchInterval,
		PruneInterval:  cfg.PruneInterval,
		ExtractPerTick: cfg.ExtractPerTick,
	}
	s.fetcher = NewFetcher(database, logger)
	s.matcher = NewMatcher(database, logger)
	s.extractor = NewExtractor(database, llmClient, logger, cfg.ExtractModel)
	s.pruner = NewPruner(database, logger)
	return s
}

// Start 启动协调 goroutine。重复调用是 no-op。
func (s *Service) Start(parent context.Context) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(parent)
	s.cancel = cancel
	s.running = true
	s.mu.Unlock()

	go s.loop(ctx)
}

// Stop 优雅停止。
func (s *Service) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return
	}
	s.running = false
	if s.cancel != nil {
		s.cancel()
	}
}

func (s *Service) loop(ctx context.Context) {
	s.logger.Info("feed service started",
		"fetch_interval", s.FetchInterval, "prune_interval", s.PruneInterval)

	// 启动后第一次马上跑（仅抓 + 匹配，避免上来就大量调 LLM）
	s.runFetchTick(ctx, false)

	fetchTicker := time.NewTicker(s.FetchInterval)
	defer fetchTicker.Stop()
	pruneTicker := time.NewTicker(s.PruneInterval)
	defer pruneTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("feed service stopped")
			return
		case <-fetchTicker.C:
			s.runFetchTick(ctx, true)
		case <-pruneTicker.C:
			s.runPruneTick(ctx)
		}
	}
}

func (s *Service) runFetchTick(ctx context.Context, doExtract bool) {
	tStart := time.Now()
	inserted, perErrs, err := s.fetcher.FetchAll(ctx)
	if err != nil {
		s.setLastError(fmt.Sprintf("fetch: %v", err))
		s.logger.Warn("fetch all failed", "err", err)
		return
	}
	if matched, err := s.matcher.MatchPending(ctx, 200); err != nil {
		s.logger.Warn("match pending failed", "err", err)
	} else {
		s.logger.Info("feed fetched", "new", inserted, "matched_events", matched, "took", time.Since(tStart))
	}
	if doExtract {
		if n, err := s.extractor.ExtractBatch(ctx, s.ExtractPerTick); err != nil {
			s.logger.Warn("extract batch failed", "err", err)
		} else if n > 0 {
			s.logger.Info("feed extracted", "events", n)
		}
	}
	if len(perErrs) > 0 {
		s.setLastError(fmt.Sprintf("%d source(s) failed", len(perErrs)))
	} else {
		s.setLastError("")
	}
	s.mu.Lock()
	s.lastFetchAt = time.Now()
	s.mu.Unlock()
	_ = db.SetFeedStateValue(ctx, s.db, "last_fetch_at", time.Now().UTC().Format(time.RFC3339))
}

func (s *Service) runPruneTick(ctx context.Context) {
	rels, ents, err := s.pruner.Run(ctx)
	if err != nil {
		s.logger.Warn("prune failed", "err", err)
		return
	}
	s.logger.Info("feed pruned", "relations_removed", rels, "entities_removed", ents)
	s.mu.Lock()
	s.lastPruneAt = time.Now()
	s.mu.Unlock()
	_ = db.SetFeedStateValue(ctx, s.db, "last_prune_at", time.Now().UTC().Format(time.RFC3339))
}

func (s *Service) setLastError(msg string) {
	s.mu.Lock()
	s.lastError = msg
	s.mu.Unlock()
}

// Status 给 /feed/status API 用，返回 worker 与数据库聚合的快照。
func (s *Service) Status(ctx context.Context) (*model.FeedStatus, error) {
	agg, err := db.CountFeedAggregates(ctx, s.db)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	st := &model.FeedStatus{
		Running:        s.running,
		SourcesTotal:   agg.SourcesTotal,
		SourcesActive:  agg.SourcesActive,
		Events24h:      agg.Events24h,
		EntitiesTotal:  agg.EntitiesTotal,
		RelationsTotal: agg.RelationsTotal,
		LastError:      s.lastError,
	}
	if !s.lastFetchAt.IsZero() {
		t := s.lastFetchAt
		st.LastFetchAt = &t
	}
	if !s.lastPruneAt.IsZero() {
		t := s.lastPruneAt
		st.LastPruneAt = &t
	}
	return st, nil
}

// TriggerFetchNow 给 API 用，手动触发一次抓取（异步执行，不阻塞调用方）。
func (s *Service) TriggerFetchNow() {
	go s.runFetchTick(context.Background(), true)
}
