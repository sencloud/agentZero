package db

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/agentzero/server/internal/model"
)

// UpsertReview 写入或更新一次行动点评。rating 必须 1..5，否则报错。
// 调用方先做 user 归属校验（评分关联的 mission 必须属于当前用户）。
func UpsertReview(ctx context.Context, db *sql.DB, r *model.Review) error {
	if r.MissionID == "" {
		return errors.New("mission_id required")
	}
	if r.Rating < 1 || r.Rating > 5 {
		return errors.New("rating must be 1..5")
	}
	now := time.Now().UTC()
	_, err := db.ExecContext(ctx, `
		INSERT INTO mission_reviews (mission_id, rating, comment, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(mission_id) DO UPDATE SET
		  rating = excluded.rating,
		  comment = excluded.comment,
		  updated_at = excluded.updated_at
	`, r.MissionID, r.Rating, r.Comment, now, now)
	if err == nil {
		if r.CreatedAt.IsZero() {
			r.CreatedAt = now
		}
		r.UpdatedAt = now
	}
	return err
}

// GetReview 取一次任务的点评。无评则返回 (nil, ErrNotFound)。
func GetReview(ctx context.Context, db *sql.DB, missionID string) (*model.Review, error) {
	row := db.QueryRowContext(ctx, `
		SELECT mission_id, rating, comment, created_at, updated_at
		FROM mission_reviews WHERE mission_id = ?
	`, missionID)
	var r model.Review
	if err := row.Scan(&r.MissionID, &r.Rating, &r.Comment, &r.CreatedAt, &r.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &r, nil
}
