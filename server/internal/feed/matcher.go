package feed

import (
	"context"
	"database/sql"
	"log/slog"
	"strings"

	"github.com/agentzero/server/internal/db"
	"github.com/agentzero/server/internal/model"
)

// Matcher 用关键词匹配把新事件落到 user_event_subs。
// 简化策略：title 命中 +2 分，summary 命中 +1 分，weight 作为放大系数。
type Matcher struct {
	db     *sql.DB
	logger *slog.Logger
}

func NewMatcher(database *sql.DB, logger *slog.Logger) *Matcher {
	return &Matcher{db: database, logger: logger}
}

// MatchEventToUsers 把单条事件 vs 所有用户的话题打一遍分；
// 命中（relevance > 0）的话题会写入 user_event_subs。
// 返回 (uniqueUsersMatched)。
func (m *Matcher) MatchEventToUsers(ctx context.Context, e *model.NewsEvent) (int, error) {
	rows, err := m.db.QueryContext(ctx, `SELECT id, user_id, name, weight FROM topics`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	titleLower := strings.ToLower(e.Title)
	summaryLower := strings.ToLower(e.Summary)
	contentLower := strings.ToLower(e.Content)
	// userID -> (relevance, matchedTopicIDs)
	type acc struct {
		score  float64
		topics []int64
	}
	byUser := map[int64]*acc{}

	for rows.Next() {
		var (
			id, uid int64
			name    string
			weight  float64
		)
		if err := rows.Scan(&id, &uid, &name, &weight); err != nil {
			return 0, err
		}
		needle := strings.ToLower(strings.TrimSpace(name))
		if needle == "" {
			continue
		}
		score := 0.0
		if strings.Contains(titleLower, needle) {
			score += 2.0
		}
		if strings.Contains(summaryLower, needle) {
			score += 1.0
		}
		if strings.Contains(contentLower, needle) {
			score += 0.3
		}
		if score == 0 {
			continue
		}
		score *= weight
		a, ok := byUser[uid]
		if !ok {
			a = &acc{}
			byUser[uid] = a
		}
		a.score += score
		a.topics = append(a.topics, id)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	for uid, a := range byUser {
		sub := &model.UserEventSub{
			UserID:        uid,
			EventID:       e.ID,
			Relevance:     a.score,
			MatchedTopics: a.topics,
		}
		if err := db.UpsertUserEventSub(ctx, m.db, sub); err != nil {
			m.logger.Warn("upsert user_event_sub failed", "uid", uid, "ev", e.ID, "err", err)
		}
	}
	return len(byUser), nil
}

// MatchPending 对最近若干条事件做匹配。返回 (eventsProcessed)。
func (m *Matcher) MatchPending(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, source_id, url, title, summary, content, lang
		FROM news_events
		ORDER BY COALESCE(published_at, fetched_at) DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var batch []*model.NewsEvent
	for rows.Next() {
		var e model.NewsEvent
		if err := rows.Scan(&e.ID, &e.SourceID, &e.URL, &e.Title, &e.Summary, &e.Content, &e.Lang); err != nil {
			return 0, err
		}
		batch = append(batch, &e)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	for _, e := range batch {
		if _, err := m.MatchEventToUsers(ctx, e); err != nil {
			m.logger.Warn("match event failed", "ev", e.ID, "err", err)
		}
	}
	return len(batch), nil
}
