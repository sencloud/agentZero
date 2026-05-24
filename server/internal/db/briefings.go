package db

import (
	"context"
	"database/sql"
	"errors"

	"github.com/agentzero/server/internal/model"
)

// CreateBriefing 插入一份新生成的简报，返回带 ID。
func CreateBriefing(ctx context.Context, db *sql.DB, b *model.Briefing) error {
	if b.UserID == 0 {
		return errors.New("user_id required")
	}
	if b.Window == "" {
		b.Window = "1h"
	}
	res, err := db.ExecContext(ctx, `
		INSERT INTO briefings (user_id, window, title, summary, html_path, model,
		                      event_count, cluster_count, reasoning_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, b.UserID, b.Window, b.Title, b.Summary, b.HTMLPath, b.Model,
		b.EventCount, b.ClusterCount, b.ReasoningJSON)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	b.ID = id
	return nil
}

// ListBriefings 按时间倒序列出某用户的简报。
func ListBriefings(ctx context.Context, db *sql.DB, userID int64, limit int) ([]*model.Briefing, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	rows, err := db.QueryContext(ctx, `
		SELECT id, user_id, window, title, summary, html_path, model,
		       event_count, cluster_count, generated_at
		FROM briefings
		WHERE user_id = ?
		ORDER BY generated_at DESC
		LIMIT ?
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Briefing
	for rows.Next() {
		var b model.Briefing
		if err := rows.Scan(&b.ID, &b.UserID, &b.Window, &b.Title, &b.Summary, &b.HTMLPath,
			&b.Model, &b.EventCount, &b.ClusterCount, &b.GeneratedAt); err != nil {
			return nil, err
		}
		out = append(out, &b)
	}
	return out, rows.Err()
}

// GetBriefing 按 (id, userID) 拉单份简报（含 reasoning_json）。
func GetBriefing(ctx context.Context, db *sql.DB, id, userID int64) (*model.Briefing, error) {
	var b model.Briefing
	row := db.QueryRowContext(ctx, `
		SELECT id, user_id, window, title, summary, html_path, model,
		       event_count, cluster_count, reasoning_json, generated_at
		FROM briefings WHERE id = ? AND user_id = ?
	`, id, userID)
	if err := row.Scan(&b.ID, &b.UserID, &b.Window, &b.Title, &b.Summary, &b.HTMLPath,
		&b.Model, &b.EventCount, &b.ClusterCount, &b.ReasoningJSON, &b.GeneratedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &b, nil
}

// ListUsersWithTopics 列出至少有一个 topic 的用户 ID，给定时简报使用。
func ListUsersWithTopics(ctx context.Context, db *sql.DB) ([]int64, error) {
	rows, err := db.QueryContext(ctx, `SELECT DISTINCT user_id FROM topics`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
