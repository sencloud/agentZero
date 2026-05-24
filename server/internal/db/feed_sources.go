package db

import (
	"context"
	"database/sql"
	"errors"

	"github.com/agentzero/server/internal/model"
)

// UpsertNewsSource 插入或更新一个新闻源。按 url 唯一。
// 注意：upsert 时不覆盖 enabled，避免每次 seed 把用户手动启用的源关掉。
func UpsertNewsSource(ctx context.Context, db *sql.DB, s *model.NewsSource) error {
	enabled := 0
	if s.Enabled {
		enabled = 1
	}
	res, err := db.ExecContext(ctx, `
		INSERT INTO news_sources (name, url, kind, region, lang, category, description, rsshub_route, enabled)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(url) DO UPDATE SET
		  name = excluded.name,
		  kind = excluded.kind,
		  region = excluded.region,
		  lang = excluded.lang,
		  category = excluded.category,
		  description = excluded.description,
		  rsshub_route = excluded.rsshub_route
	`, s.Name, s.URL, s.Kind, s.Region, s.Lang, s.Category, s.Description, s.RSSHubRoute, enabled)
	if err != nil {
		return err
	}
	if id, err := res.LastInsertId(); err == nil && s.ID == 0 {
		s.ID = id
	}
	return nil
}

// SetSourceEnabled 单独切换 enabled 位。
func SetSourceEnabled(ctx context.Context, db *sql.DB, ids []int64, enabled bool) error {
	if len(ids) == 0 {
		return nil
	}
	val := 0
	if enabled {
		val = 1
	}
	q := `UPDATE news_sources SET enabled = ? WHERE id IN (` + intsPlaceholder(len(ids)) + `)`
	args := []any{val}
	for _, id := range ids {
		args = append(args, id)
	}
	_, err := db.ExecContext(ctx, q, args...)
	return err
}

// ListNewsSources 列出全部新闻源；onlyEnabled=true 时只返回 enabled=1 的。
func ListNewsSources(ctx context.Context, db *sql.DB, onlyEnabled bool) ([]*model.NewsSource, error) {
	q := `SELECT id, name, url, kind, region, lang, category, description, rsshub_route, enabled, last_fetch_at, last_error, created_at FROM news_sources`
	if onlyEnabled {
		q += ` WHERE enabled = 1`
	}
	q += ` ORDER BY category, id ASC`
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.NewsSource
	for rows.Next() {
		s, err := scanNewsSource(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func intsPlaceholder(n int) string {
	if n <= 0 {
		return ""
	}
	out := make([]byte, 0, n*2-1)
	for i := 0; i < n; i++ {
		if i > 0 {
			out = append(out, ',')
		}
		out = append(out, '?')
	}
	return string(out)
}

// MarkNewsSourceFetched 记录最后一次抓取结果（成功或带错误）。
func MarkNewsSourceFetched(ctx context.Context, db *sql.DB, sourceID int64, errMsg string) error {
	_, err := db.ExecContext(ctx, `
		UPDATE news_sources SET last_fetch_at = CURRENT_TIMESTAMP, last_error = ? WHERE id = ?
	`, errMsg, sourceID)
	return err
}

func scanNewsSource(rows *sql.Rows) (*model.NewsSource, error) {
	var s model.NewsSource
	var enabled int
	var lastFetch sql.NullTime
	if err := rows.Scan(&s.ID, &s.Name, &s.URL, &s.Kind, &s.Region, &s.Lang,
		&s.Category, &s.Description, &s.RSSHubRoute, &enabled, &lastFetch, &s.LastError, &s.CreatedAt); err != nil {
		return nil, err
	}
	s.Enabled = enabled != 0
	if lastFetch.Valid {
		t := lastFetch.Time
		s.LastFetchAt = &t
	}
	return &s, nil
}

// CountFeedAggregates 给 /feed/status 用：源总数、激活数、24h 事件数、节点 / 关系总数。
type FeedAggregate struct {
	SourcesTotal   int
	SourcesActive  int
	Events24h      int
	EntitiesTotal  int
	RelationsTotal int
}

func CountFeedAggregates(ctx context.Context, db *sql.DB) (*FeedAggregate, error) {
	a := &FeedAggregate{}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(enabled),0) FROM news_sources`).
		Scan(&a.SourcesTotal, &a.SourcesActive); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
	}
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM news_events WHERE fetched_at >= datetime('now', '-1 day')`).
		Scan(&a.Events24h)
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM entities`).Scan(&a.EntitiesTotal)
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM entity_relations`).Scan(&a.RelationsTotal)
	return a, nil
}

// GetFeedStateValue 取 feed_state 单值。不存在返回 ""。
func GetFeedStateValue(ctx context.Context, db *sql.DB, key string) (string, error) {
	var v string
	row := db.QueryRowContext(ctx, `SELECT v FROM feed_state WHERE k = ?`, key)
	if err := row.Scan(&v); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return v, nil
}

// SetFeedStateValue upsert 一个键值。
func SetFeedStateValue(ctx context.Context, db *sql.DB, key, value string) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO feed_state (k, v) VALUES (?, ?)
		ON CONFLICT(k) DO UPDATE SET v = excluded.v, updated_at = CURRENT_TIMESTAMP
	`, key, value)
	return err
}
