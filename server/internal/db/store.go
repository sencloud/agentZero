package db

import (
	"context"
	"database/sql"
	"errors"

	"github.com/agentzero/server/internal/model"
)

// ErrNotFound 是 store 层对外的统一未找到错误。
var ErrNotFound = errors.New("not found")

// UpsertUserByApple 按 Apple subject 维度落库或更新一个用户。
// 调用方一般来自 Apple Sign In 完成后的回调。
func UpsertUserByApple(ctx context.Context, db *sql.DB, appleSub, email, nickname string) (*model.User, error) {
	if _, err := db.ExecContext(ctx, `
		INSERT INTO users (apple_sub, email, nickname)
		VALUES (?, ?, ?)
		ON CONFLICT(apple_sub) DO UPDATE SET
		  email    = COALESCE(NULLIF(excluded.email, ''), users.email),
		  nickname = CASE WHEN users.nickname = '' THEN excluded.nickname ELSE users.nickname END
	`, appleSub, email, nickname); err != nil {
		return nil, err
	}
	row := db.QueryRowContext(ctx, `SELECT id, apple_sub, email, nickname, avatar_url, created_at FROM users WHERE apple_sub = ?`, appleSub)
	return scanUser(row)
}

// GetUserByID 按 ID 取用户。
func GetUserByID(ctx context.Context, db *sql.DB, id int64) (*model.User, error) {
	row := db.QueryRowContext(ctx, `SELECT id, apple_sub, email, nickname, avatar_url, created_at FROM users WHERE id = ?`, id)
	u, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

func scanUser(row *sql.Row) (*model.User, error) {
	var u model.User
	var email, avatar sql.NullString
	if err := row.Scan(&u.ID, &u.AppleSub, &email, &u.Nickname, &avatar, &u.CreatedAt); err != nil {
		return nil, err
	}
	u.Email = email.String
	u.AvatarURL = avatar.String
	return &u, nil
}
