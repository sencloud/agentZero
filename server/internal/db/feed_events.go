package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/agentzero/server/internal/model"
)

// UpsertNewsEvent 插入事件；按 url 唯一，已存在则忽略并返回原 id 与 inserted=false。
func UpsertNewsEvent(ctx context.Context, db *sql.DB, e *model.NewsEvent) (inserted bool, err error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	var existing int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM news_events WHERE url = ?`, e.URL).Scan(&existing); err == nil {
		e.ID = existing
		return false, tx.Commit()
	} else if !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}

	res, err := tx.ExecContext(ctx, `
		INSERT INTO news_events (source_id, url, title, summary, content, lang, published_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, e.SourceID, e.URL, e.Title, e.Summary, e.Content, e.Lang, e.PublishedAt)
	if err != nil {
		return false, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	e.ID = id
	if e.FetchedAt.IsZero() {
		e.FetchedAt = time.Now().UTC()
	}
	return true, nil
}

// ListUserFeedEvents 按用户取最新匹配到的事件，可加 since / limit。
// 返回事件本体 + 命中的话题 id 列表 + relevance。
type UserFeedEvent struct {
	Event         *model.NewsEvent
	Source        *model.NewsSource
	MatchedTopics []int64
	Relevance     float64
}

func ListUserFeedEvents(ctx context.Context, db *sql.DB, userID int64, limit int) ([]*UserFeedEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := db.QueryContext(ctx, `
		SELECT e.id, e.source_id, e.url, e.title, e.summary, e.content, e.lang,
		       e.published_at, e.fetched_at, e.extracted,
		       s.id, s.name, s.url, s.kind, s.region, s.lang, s.enabled, s.last_fetch_at, s.last_error, s.created_at,
		       u.relevance, u.matched_topics
		FROM user_event_subs u
		JOIN news_events e ON e.id = u.event_id
		JOIN news_sources s ON s.id = e.source_id
		WHERE u.user_id = ?
		ORDER BY u.created_at DESC
		LIMIT ?
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*UserFeedEvent
	for rows.Next() {
		var (
			e         model.NewsEvent
			s         model.NewsSource
			extracted int
			enabled   int
			lastFetch sql.NullTime
			published sql.NullTime
			matched   string
			rel       float64
		)
		if err := rows.Scan(&e.ID, &e.SourceID, &e.URL, &e.Title, &e.Summary, &e.Content, &e.Lang,
			&published, &e.FetchedAt, &extracted,
			&s.ID, &s.Name, &s.URL, &s.Kind, &s.Region, &s.Lang, &enabled, &lastFetch, &s.LastError, &s.CreatedAt,
			&rel, &matched); err != nil {
			return nil, err
		}
		e.Extracted = extracted != 0
		if published.Valid {
			t := published.Time
			e.PublishedAt = &t
		}
		s.Enabled = enabled != 0
		if lastFetch.Valid {
			t := lastFetch.Time
			s.LastFetchAt = &t
		}
		var topicIDs []int64
		_ = json.Unmarshal([]byte(matched), &topicIDs)
		out = append(out, &UserFeedEvent{
			Event:         &e,
			Source:        &s,
			MatchedTopics: topicIDs,
			Relevance:     rel,
		})
	}
	return out, rows.Err()
}

// UpsertUserEventSub 落「用户命中事件」。已存在则取较大的 relevance + merge matched_topics。
func UpsertUserEventSub(ctx context.Context, db *sql.DB, sub *model.UserEventSub) error {
	mt, err := json.Marshal(sub.MatchedTopics)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `
		INSERT INTO user_event_subs (user_id, event_id, relevance, matched_topics)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(user_id, event_id) DO UPDATE SET
		  relevance = MAX(relevance, excluded.relevance),
		  matched_topics = excluded.matched_topics
	`, sub.UserID, sub.EventID, sub.Relevance, string(mt))
	return err
}

// ListUnextractedEvents 取尚未做实体抽取的事件，按发布时间倒序。
func ListUnextractedEvents(ctx context.Context, db *sql.DB, limit int) ([]*model.NewsEvent, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := db.QueryContext(ctx, `
		SELECT id, source_id, url, title, summary, content, lang, published_at, fetched_at, extracted
		FROM news_events
		WHERE extracted = 0
		ORDER BY COALESCE(published_at, fetched_at) DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.NewsEvent
	for rows.Next() {
		var e model.NewsEvent
		var extracted int
		var published sql.NullTime
		if err := rows.Scan(&e.ID, &e.SourceID, &e.URL, &e.Title, &e.Summary, &e.Content, &e.Lang,
			&published, &e.FetchedAt, &extracted); err != nil {
			return nil, err
		}
		e.Extracted = extracted != 0
		if published.Valid {
			t := published.Time
			e.PublishedAt = &t
		}
		out = append(out, &e)
	}
	return out, rows.Err()
}

// MarkEventExtracted 把事件标记为已抽取。
func MarkEventExtracted(ctx context.Context, db *sql.DB, eventID int64) error {
	_, err := db.ExecContext(ctx, `UPDATE news_events SET extracted = 1 WHERE id = ?`, eventID)
	return err
}
