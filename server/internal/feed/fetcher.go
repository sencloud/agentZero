// Package feed 是「事件流图谱」的后端核心：
//
//	fetcher  - 定时拉 RSS / atom 源
//	extractor- 调 LLM 抽取实体 + 关系
//	matcher  - 用关键词把事件配到用户的话题
//	pruner   - 周期性裁剪关系/孤立实体
//	service  - cron 协调器（启动后跑 30min fetch + 1h prune 节拍）
package feed

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/agentzero/server/internal/db"
	"github.com/agentzero/server/internal/model"
	"github.com/mmcdole/gofeed"
)

// Fetcher 负责拉 RSS 源、入库新事件、刷新 source 状态。
type Fetcher struct {
	db     *sql.DB
	logger *slog.Logger
	parser *gofeed.Parser
	client *http.Client
}

func NewFetcher(database *sql.DB, logger *slog.Logger) *Fetcher {
	p := gofeed.NewParser()
	c := &http.Client{Timeout: 20 * time.Second}
	p.Client = c
	return &Fetcher{db: database, logger: logger, parser: p, client: c}
}

// SourceResult 表示单个源的拉取结果，给 FetchAllWithProgress 的回调用。
type SourceResult struct {
	Source *model.NewsSource
	Added  int
	Err    error
}

// FetchAll 拉一遍所有 enabled 源。返回 (newEvents, perSourceErrors)。
func (f *Fetcher) FetchAll(ctx context.Context) (int, map[int64]string, error) {
	return f.FetchAllWithProgress(ctx, nil)
}

// FetchAllWithProgress 拉一遍所有 enabled 源；每完成一个源调用一次 onSource。
// onSource 返回 false 时立即停止后续抓取（用于上层取消）。
func (f *Fetcher) FetchAllWithProgress(
	ctx context.Context,
	onSource func(r SourceResult) bool,
) (int, map[int64]string, error) {
	sources, err := db.ListNewsSources(ctx, f.db, true)
	if err != nil {
		return 0, nil, fmt.Errorf("list sources: %w", err)
	}
	perErrs := map[int64]string{}
	total := 0
	for _, s := range sources {
		n, err := f.fetchOne(ctx, s)
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
			f.logger.Warn("fetch source failed", "source", s.Name, "url", s.URL, "err", err)
			perErrs[s.ID] = errMsg
		}
		if mErr := db.MarkNewsSourceFetched(ctx, f.db, s.ID, errMsg); mErr != nil {
			f.logger.Warn("mark source fetched failed", "err", mErr)
		}
		total += n
		if onSource != nil {
			if !onSource(SourceResult{Source: s, Added: n, Err: err}) {
				return total, perErrs, nil
			}
		}
	}
	return total, perErrs, nil
}

func (f *Fetcher) fetchOne(ctx context.Context, s *model.NewsSource) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "AgentZeroFeed/0.2 (+https://agentzero.me)")
	req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml;q=0.9, */*;q=0.5")
	resp, err := f.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return 0, fmt.Errorf("http %d", resp.StatusCode)
	}
	parsed, err := f.parser.Parse(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("parse feed: %w", err)
	}

	inserted := 0
	for _, item := range parsed.Items {
		if item == nil || item.Link == "" || item.Title == "" {
			continue
		}
		ev := &model.NewsEvent{
			SourceID: s.ID,
			URL:      item.Link,
			Title:    cleanText(item.Title),
			Summary:  trimSummary(cleanText(firstNonEmpty(item.Description, item.Content))),
			Content:  cleanText(item.Content),
			Lang:     s.Lang,
		}
		if t := pickPublishedAt(item); t != nil {
			ev.PublishedAt = t
		}
		ok, err := db.UpsertNewsEvent(ctx, f.db, ev)
		if err != nil {
			f.logger.Warn("upsert event failed", "source", s.Name, "title", ev.Title, "err", err)
			continue
		}
		if ok {
			inserted++
		}
	}
	return inserted, nil
}

func cleanText(s string) string {
	// 简单的 HTML 标签清除，避免直接把网页片段塞进数据库
	s = strings.TrimSpace(s)
	// 砍掉常见的 CDATA 包裹
	s = strings.TrimPrefix(s, "<![CDATA[")
	s = strings.TrimSuffix(s, "]]>")
	return s
}

func trimSummary(s string) string {
	const maxLen = 400
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "…"
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

func pickPublishedAt(item *gofeed.Item) *time.Time {
	if item.PublishedParsed != nil {
		t := item.PublishedParsed.UTC()
		return &t
	}
	if item.UpdatedParsed != nil {
		t := item.UpdatedParsed.UTC()
		return &t
	}
	return nil
}
