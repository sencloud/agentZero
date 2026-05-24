package db

import (
	"context"
	"database/sql"
	"errors"

	"github.com/agentzero/server/internal/model"
)

// CreateTopic 新建话题；同用户下重名报错。
func CreateTopic(ctx context.Context, db *sql.DB, t *model.Topic) error {
	if t.UserID == 0 || t.Name == "" {
		return errors.New("user_id and name required")
	}
	if t.Weight <= 0 {
		t.Weight = 1.0
	}
	res, err := db.ExecContext(ctx, `
		INSERT INTO topics (user_id, name, weight) VALUES (?, ?, ?)
	`, t.UserID, t.Name, t.Weight)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	t.ID = id
	return nil
}

// ListTopics 列用户全部话题。
func ListTopics(ctx context.Context, db *sql.DB, userID int64) ([]*model.Topic, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, user_id, name, weight, created_at
		FROM topics WHERE user_id = ? ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Topic
	for rows.Next() {
		var t model.Topic
		if err := rows.Scan(&t.ID, &t.UserID, &t.Name, &t.Weight, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &t)
	}
	return out, rows.Err()
}

// DeleteTopic 按 id + user_id 删除（防跨用户）。
func DeleteTopic(ctx context.Context, db *sql.DB, topicID, userID int64) error {
	res, err := db.ExecContext(ctx, `DELETE FROM topics WHERE id = ? AND user_id = ?`, topicID, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
