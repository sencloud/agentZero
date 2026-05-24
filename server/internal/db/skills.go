package db

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/agentzero/server/internal/model"
)

// CreateSkill 新建一项技能；source_mission_id 可为空。
func CreateSkill(ctx context.Context, db *sql.DB, s *model.Skill) error {
	if s.UserID == 0 || s.Name == "" {
		return errors.New("user_id and name required")
	}
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now().UTC()
	}
	var src sql.NullString
	if s.SourceMissionID != nil {
		src = sql.NullString{String: *s.SourceMissionID, Valid: true}
	}
	res, err := db.ExecContext(ctx, `
		INSERT INTO skills (user_id, name, description, trigger_hint, prompt_template, source_mission_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, s.UserID, s.Name, s.Description, s.TriggerHint, s.PromptTemplate, src, s.CreatedAt)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	s.ID = id
	return nil
}

// ListSkills 列出用户的全部技能，按创建时间倒序。
func ListSkills(ctx context.Context, db *sql.DB, userID int64) ([]*model.Skill, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, user_id, name, description, trigger_hint, prompt_template, source_mission_id, created_at
		FROM skills WHERE user_id = ?
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Skill
	for rows.Next() {
		var s model.Skill
		var src sql.NullString
		if err := rows.Scan(&s.ID, &s.UserID, &s.Name, &s.Description, &s.TriggerHint,
			&s.PromptTemplate, &src, &s.CreatedAt); err != nil {
			return nil, err
		}
		if src.Valid {
			v := src.String
			s.SourceMissionID = &v
		}
		out = append(out, &s)
	}
	return out, rows.Err()
}
