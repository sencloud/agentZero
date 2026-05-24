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

	fetcher     *Fetcher
	matcher     *Matcher
	extractor   *Extractor
	pruner      *Pruner
	recommender *Recommender

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
	s.recommender = NewRecommender(database, llmClient, logger, cfg.ExtractModel)
	return s
}

// Recommend 让 LLM 看一眼用户的话题，自动启用最相关的源。
// 不带 SSE，用于添加话题等触发场景。
func (s *Service) Recommend(ctx context.Context, userID int64) (*RecommendResult, error) {
	return s.recommender.Recommend(ctx, userID)
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
		s.logger.Info("feed fetched", "new", inserted,
			"matched_events", matched, "failed_sources", len(perErrs), "took", time.Since(tStart))
	}
	if doExtract {
		if n, err := s.extractor.ExtractBatch(ctx, s.ExtractPerTick); err != nil {
			s.logger.Warn("extract batch failed", "err", err)
		} else if n > 0 {
			s.logger.Info("feed extracted", "events", n)
		}
	}
	// 仅当所有源都失败（彻底掉线）才算 ERR；部分源失败属正常波动。
	total, _ := s.countEnabledSources(ctx)
	if total > 0 && len(perErrs) >= total {
		s.setLastError(fmt.Sprintf("all %d source(s) failed", total))
	} else {
		s.setLastError("")
	}
	s.mu.Lock()
	s.lastFetchAt = time.Now()
	s.mu.Unlock()
	_ = db.SetFeedStateValue(ctx, s.db, "last_fetch_at", time.Now().UTC().Format(time.RFC3339))
}

// countEnabledSources 返回当前 enabled=1 的 source 数量。
func (s *Service) countEnabledSources(ctx context.Context) (int, error) {
	sources, err := db.ListNewsSources(ctx, s.db, true)
	if err != nil {
		return 0, err
	}
	return len(sources), nil
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

// RefreshEvent 是 RunRefreshStream 推到上层 SSE 的单条事件。
//
// Phase 取值：
//
//	"start"          - 整轮刷新开始
//	"fetch_source"   - 单源抓取完成（含成功/失败/新增条数）
//	"fetch_done"     - 所有源抓取完毕
//	"match_done"     - 关键词匹配完成
//	"extract_event"  - 单条 LLM 抽取完成
//	"extract_done"   - 抽取阶段结束
//	"done"           - 整轮刷新结束
//	"error"          - 致命错误
type RefreshEvent struct {
	Phase   string         `json:"phase"`
	Message string         `json:"message,omitempty"`
	Data    map[string]any `json:"data,omitempty"`
}

// RunRefreshStream 同步驱动一轮「推荐源 + 抓取 + 匹配 + 抽取」，
// 在每个关键节点把 RefreshEvent 推给 emit 回调。emit 返回 false 时立即中止。
//
// 该方法不操作 s.lastFetchAt 等共享态（避免和后台 cron 冲突）。
func (s *Service) RunRefreshStream(ctx context.Context, userID int64, emit func(ev RefreshEvent) bool) {
	emitOK := func(ev RefreshEvent) bool {
		if emit == nil {
			return true
		}
		return emit(ev)
	}

	if !emitOK(RefreshEvent{Phase: "start", Message: "开始刷新事件流"}) {
		return
	}

	tStart := time.Now()

	// ===== 阶段 0：LLM 智能选源（仅在用户有话题时跑） =====
	if userID > 0 {
		topics, _ := db.ListTopics(ctx, s.db, userID)
		if len(topics) > 0 {
			emitOK(RefreshEvent{
				Phase:   "recommend_start",
				Message: fmt.Sprintf("LLM 按 %d 个话题智能选源", len(topics)),
				Data:    map[string]any{"topics": len(topics)},
			})
			rec, err := s.recommender.Recommend(ctx, userID)
			if err != nil {
				emitOK(RefreshEvent{Phase: "recommend_error",
					Message: "选源失败：" + err.Error()})
			} else {
				names := make([]string, 0, len(rec.NewlyEnabled))
				for _, n := range rec.NewlyEnabled {
					names = append(names, n.Name)
				}
				msg := fmt.Sprintf("新启用 %d 个源", len(rec.NewlyEnabled))
				if rec.Reason != "" {
					msg += " · " + rec.Reason
				}
				emitOK(RefreshEvent{
					Phase:   "recommend_done",
					Message: msg,
					Data: map[string]any{
						"newly_enabled": names,
						"already_on":    len(rec.AlreadyOn),
						"reason":        rec.Reason,
					},
				})
			}
		}
	}

	totalSources, _ := s.countEnabledSources(ctx)
	emitOK(RefreshEvent{
		Phase:   "fetch_start",
		Message: fmt.Sprintf("拉取 %d 个新闻源", totalSources),
		Data:    map[string]any{"total_sources": totalSources},
	})

	totalNew, perErrs, err := s.fetcher.FetchAllWithProgress(ctx, func(r SourceResult) bool {
		ev := RefreshEvent{
			Phase: "fetch_source",
			Data: map[string]any{
				"source": r.Source.Name,
				"added":  r.Added,
			},
		}
		if r.Err != nil {
			ev.Data["error"] = r.Err.Error()
			ev.Message = fmt.Sprintf("× %s 失败", r.Source.Name)
		} else {
			ev.Message = fmt.Sprintf("✓ %s 新增 %d 条", r.Source.Name, r.Added)
		}
		return emitOK(ev)
	})
	if err != nil {
		emitOK(RefreshEvent{Phase: "error", Message: "fetch_all_failed: " + err.Error()})
		return
	}
	emitOK(RefreshEvent{
		Phase:   "fetch_done",
		Message: fmt.Sprintf("抓取完毕：新增 %d 条，失败 %d 个源", totalNew, len(perErrs)),
		Data: map[string]any{
			"total_new":      totalNew,
			"failed_sources": len(perErrs),
		},
	})

	// 匹配阶段（一次性，不细分进度）
	emitOK(RefreshEvent{Phase: "match_start", Message: "按话题匹配命中事件"})
	matched, mErr := s.matcher.MatchPending(ctx, 200)
	if mErr != nil {
		emitOK(RefreshEvent{Phase: "error", Message: "match_failed: " + mErr.Error()})
	}
	emitOK(RefreshEvent{
		Phase:   "match_done",
		Message: fmt.Sprintf("匹配完成：%d 条事件参与匹配", matched),
		Data:    map[string]any{"matched": matched},
	})

	// 抽取阶段
	emitOK(RefreshEvent{
		Phase:   "extract_start",
		Message: fmt.Sprintf("调用 LLM 抽取实体（上限 %d 条）", s.ExtractPerTick),
		Data:    map[string]any{"limit": s.ExtractPerTick},
	})
	extracted, xErr := s.extractor.ExtractBatchWithProgress(ctx, s.ExtractPerTick, func(p ExtractProgress) bool {
		ev := RefreshEvent{
			Phase: "extract_event",
			Data: map[string]any{
				"index": p.Index,
				"total": p.Total,
				"title": p.Event.Title,
			},
		}
		if p.Err != nil {
			ev.Data["error"] = p.Err.Error()
			ev.Message = fmt.Sprintf("× [%d/%d] %s", p.Index, p.Total, truncateUTF8(p.Event.Title, 28))
		} else {
			ev.Message = fmt.Sprintf("✓ [%d/%d] %s", p.Index, p.Total, truncateUTF8(p.Event.Title, 28))
		}
		return emitOK(ev)
	})
	if xErr != nil {
		s.logger.Warn("refresh extract failed", "err", xErr)
	}
	emitOK(RefreshEvent{
		Phase:   "extract_done",
		Message: fmt.Sprintf("抽取完成：%d 条", extracted),
		Data:    map[string]any{"extracted": extracted},
	})

	// 更新 last_fetch_at / last_error
	if totalSources > 0 && len(perErrs) >= totalSources {
		s.setLastError(fmt.Sprintf("all %d source(s) failed", totalSources))
	} else {
		s.setLastError("")
	}
	s.mu.Lock()
	s.lastFetchAt = time.Now()
	s.mu.Unlock()
	_ = db.SetFeedStateValue(ctx, s.db, "last_fetch_at", time.Now().UTC().Format(time.RFC3339))

	emitOK(RefreshEvent{
		Phase:   "done",
		Message: fmt.Sprintf("刷新完成，总耗时 %s", time.Since(tStart).Round(time.Millisecond)),
		Data: map[string]any{
			"took_ms":   time.Since(tStart).Milliseconds(),
			"new":       totalNew,
			"matched":   matched,
			"extracted": extracted,
		},
	})
}

// truncateUTF8 把中文标题按 rune 截断，避免拆出半个字符。
func truncateUTF8(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}
