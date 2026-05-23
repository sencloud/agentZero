package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/agentzero/server/internal/model"
)

var ErrNotFound = errors.New("not found")

func ListCategories(ctx context.Context, db *sql.DB) ([]model.Category, error) {
	rows, err := db.QueryContext(ctx, `SELECT id,slug,name,icon,color FROM categories ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Category{}
	for rows.Next() {
		var c model.Category
		if err := rows.Scan(&c.ID, &c.Slug, &c.Name, &c.Icon, &c.Color); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

type AgentFilter struct {
	CategorySlug string
	Query        string
	Featured     bool
	Sort         string
	Limit        int
	Offset       int
	UserID       int64
}

func ListAgents(ctx context.Context, db *sql.DB, f AgentFilter) ([]model.Agent, error) {
	var where []string
	var args []any
	if f.CategorySlug != "" {
		where = append(where, `c.slug = ?`)
		args = append(args, f.CategorySlug)
	}
	if f.Query != "" {
		where = append(where, `(a.name LIKE ? OR a.tagline LIKE ? OR a.description LIKE ?)`)
		q := "%" + f.Query + "%"
		args = append(args, q, q, q)
	}
	if f.Featured {
		where = append(where, `a.is_featured = 1`)
	}
	clause := ""
	if len(where) > 0 {
		clause = " WHERE " + strings.Join(where, " AND ")
	}
	order := " ORDER BY a.install_count DESC, a.rating DESC"
	switch f.Sort {
	case "new":
		order = " ORDER BY a.released_at DESC"
	case "updated":
		order = " ORDER BY a.updated_at DESC"
	case "top":
		order = " ORDER BY a.rating DESC, a.rating_count DESC"
	}
	limit := f.Limit
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	q := `SELECT a.id,a.slug,a.name,a.tagline,a.description,a.icon_url,a.cover_url,a.screenshots,a.category_id,c.name,c.slug,a.developer,a.version,a.size_bytes,a.rating,a.rating_count,a.install_count,a.is_free,a.price_cents,a.is_featured,COALESCE(a.feature_badge,''),a.capabilities,COALESCE(a.updated_notes,''),a.released_at,a.updated_at,
	(SELECT 1 FROM installs i WHERE i.agent_id=a.id AND i.user_id=?) AS installed
	FROM agents a JOIN categories c ON c.id = a.category_id` + clause + order + fmt.Sprintf(" LIMIT %d OFFSET %d", limit, f.Offset)
	args = append([]any{f.UserID}, args...)
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Agent{}
	for rows.Next() {
		a, err := scanAgent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func GetAgent(ctx context.Context, db *sql.DB, slug string, userID int64) (*model.Agent, error) {
	row := db.QueryRowContext(ctx, `SELECT a.id,a.slug,a.name,a.tagline,a.description,a.icon_url,a.cover_url,a.screenshots,a.category_id,c.name,c.slug,a.developer,a.version,a.size_bytes,a.rating,a.rating_count,a.install_count,a.is_free,a.price_cents,a.is_featured,COALESCE(a.feature_badge,''),a.capabilities,COALESCE(a.updated_notes,''),a.released_at,a.updated_at,
	(SELECT 1 FROM installs i WHERE i.agent_id=a.id AND i.user_id=?) AS installed
	FROM agents a JOIN categories c ON c.id=a.category_id WHERE a.slug=?`, userID, slug)
	a, err := scanAgent(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanAgent(r rowScanner) (model.Agent, error) {
	var a model.Agent
	var shotsRaw, capsRaw string
	var installed sql.NullInt64
	var isFree, isFeatured int
	if err := r.Scan(
		&a.ID, &a.Slug, &a.Name, &a.Tagline, &a.Description, &a.IconURL, &a.CoverURL, &shotsRaw,
		&a.CategoryID, &a.CategoryName, &a.CategorySlug, &a.Developer, &a.Version, &a.SizeBytes,
		&a.Rating, &a.RatingCount, &a.InstallCount, &isFree, &a.PriceCents, &isFeatured,
		&a.FeatureBadge, &capsRaw, &a.UpdatedNotes, &a.ReleasedAt, &a.UpdatedAt, &installed,
	); err != nil {
		return a, err
	}
	a.IsFree = isFree == 1
	a.IsFeatured = isFeatured == 1
	a.Installed = installed.Valid && installed.Int64 == 1
	_ = json.Unmarshal([]byte(shotsRaw), &a.Screenshots)
	_ = json.Unmarshal([]byte(capsRaw), &a.Capabilities)
	if a.Screenshots == nil {
		a.Screenshots = []string{}
	}
	if a.Capabilities == nil {
		a.Capabilities = []string{}
	}
	return a, nil
}

func ListTodayCards(ctx context.Context, db *sql.DB) ([]model.TodayCard, error) {
	rows, err := db.QueryContext(ctx, `SELECT t.id,t.kind,t.eyebrow,t.title,t.subtitle,t.cover_url,COALESCE(t.agent_id,0),COALESCE(a.slug,''),t.sort_order
		FROM today_cards t LEFT JOIN agents a ON a.id=t.agent_id ORDER BY t.sort_order`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.TodayCard{}
	for rows.Next() {
		var c model.TodayCard
		if err := rows.Scan(&c.ID, &c.Kind, &c.Eyebrow, &c.Title, &c.Subtitle, &c.CoverURL, &c.AgentID, &c.AgentSlug, &c.SortOrder); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func UpsertUserByApple(ctx context.Context, db *sql.DB, sub, email, nickname string) (*model.User, error) {
	if _, err := db.ExecContext(ctx, `INSERT INTO users(apple_sub,email,nickname) VALUES(?,?,?) ON CONFLICT(apple_sub) DO UPDATE SET email=COALESCE(NULLIF(excluded.email,''),users.email), nickname=COALESCE(NULLIF(excluded.nickname,''),users.nickname)`, sub, email, nickname); err != nil {
		return nil, err
	}
	return GetUserByApple(ctx, db, sub)
}

func GetUserByApple(ctx context.Context, db *sql.DB, sub string) (*model.User, error) {
	row := db.QueryRowContext(ctx, `SELECT id,apple_sub,COALESCE(email,''),nickname,COALESCE(avatar_url,''),created_at FROM users WHERE apple_sub=?`, sub)
	var u model.User
	if err := row.Scan(&u.ID, &u.AppleSub, &u.Email, &u.Nickname, &u.AvatarURL, &u.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

func GetUserByID(ctx context.Context, db *sql.DB, id int64) (*model.User, error) {
	row := db.QueryRowContext(ctx, `SELECT id,apple_sub,COALESCE(email,''),nickname,COALESCE(avatar_url,''),created_at FROM users WHERE id=?`, id)
	var u model.User
	if err := row.Scan(&u.ID, &u.AppleSub, &u.Email, &u.Nickname, &u.AvatarURL, &u.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

func Install(ctx context.Context, db *sql.DB, userID, agentID int64) error {
	_, err := db.ExecContext(ctx, `INSERT OR IGNORE INTO installs(user_id,agent_id) VALUES(?,?)`, userID, agentID)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `UPDATE agents SET install_count = install_count + 1 WHERE id=? AND NOT EXISTS(SELECT 1 FROM installs WHERE user_id=? AND agent_id=? AND installed_at < CURRENT_TIMESTAMP)`, agentID, userID, agentID)
	return err
}

func Uninstall(ctx context.Context, db *sql.DB, userID, agentID int64) error {
	_, err := db.ExecContext(ctx, `DELETE FROM installs WHERE user_id=? AND agent_id=?`, userID, agentID)
	return err
}

func ListInstalled(ctx context.Context, db *sql.DB, userID int64) ([]model.Agent, error) {
	rows, err := db.QueryContext(ctx, `SELECT a.id,a.slug,a.name,a.tagline,a.description,a.icon_url,a.cover_url,a.screenshots,a.category_id,c.name,c.slug,a.developer,a.version,a.size_bytes,a.rating,a.rating_count,a.install_count,a.is_free,a.price_cents,a.is_featured,COALESCE(a.feature_badge,''),a.capabilities,COALESCE(a.updated_notes,''),a.released_at,a.updated_at, 1 AS installed
		FROM installs i JOIN agents a ON a.id=i.agent_id JOIN categories c ON c.id=a.category_id WHERE i.user_id=? ORDER BY i.installed_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Agent{}
	for rows.Next() {
		a, err := scanAgent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func ListReviews(ctx context.Context, db *sql.DB, agentID int64, limit, offset int) ([]model.Review, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := db.QueryContext(ctx, fmt.Sprintf(`SELECT r.id,r.agent_id,r.user_id,u.nickname,COALESCE(u.avatar_url,''),r.rating,r.title,r.body,r.created_at FROM reviews r JOIN users u ON u.id=r.user_id WHERE r.agent_id=? ORDER BY r.created_at DESC LIMIT %d OFFSET %d`, limit, offset), agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Review{}
	for rows.Next() {
		var r model.Review
		if err := rows.Scan(&r.ID, &r.AgentID, &r.UserID, &r.Nickname, &r.Avatar, &r.Rating, &r.Title, &r.Body, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func UpsertReview(ctx context.Context, db *sql.DB, agentID, userID int64, rating int, title, body string) error {
	_, err := db.ExecContext(ctx, `INSERT INTO reviews(agent_id,user_id,rating,title,body) VALUES(?,?,?,?,?) ON CONFLICT(agent_id,user_id) DO UPDATE SET rating=excluded.rating, title=excluded.title, body=excluded.body, created_at=CURRENT_TIMESTAMP`, agentID, userID, rating, title, body)
	if err != nil {
		return err
	}
	return refreshAgentRating(ctx, db, agentID)
}

func refreshAgentRating(ctx context.Context, db *sql.DB, agentID int64) error {
	_, err := db.ExecContext(ctx, `UPDATE agents SET rating = COALESCE((SELECT ROUND(AVG(rating)*10)/10.0 FROM reviews WHERE agent_id=?),0), rating_count = (SELECT COUNT(*) FROM reviews WHERE agent_id=?) WHERE id=?`, agentID, agentID, agentID)
	return err
}

func CreateSession(ctx context.Context, db *sql.DB, userID, agentID int64, title string) (*model.ChatSession, error) {
	res, err := db.ExecContext(ctx, `INSERT INTO chat_sessions(user_id,agent_id,title) VALUES(?,?,?)`, userID, agentID, title)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &model.ChatSession{ID: id, UserID: userID, AgentID: agentID, Title: title, CreatedAt: time.Now()}, nil
}

func ListSessions(ctx context.Context, db *sql.DB, userID int64) ([]model.ChatSession, error) {
	rows, err := db.QueryContext(ctx, `SELECT id,user_id,agent_id,title,created_at FROM chat_sessions WHERE user_id=? ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.ChatSession{}
	for rows.Next() {
		var s model.ChatSession
		if err := rows.Scan(&s.ID, &s.UserID, &s.AgentID, &s.Title, &s.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func AddMessage(ctx context.Context, db *sql.DB, sessionID int64, role, content string) (*model.ChatMessage, error) {
	res, err := db.ExecContext(ctx, `INSERT INTO chat_messages(session_id,role,content) VALUES(?,?,?)`, sessionID, role, content)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &model.ChatMessage{ID: id, SessionID: sessionID, Role: role, Content: content, CreatedAt: time.Now()}, nil
}

func ListMessages(ctx context.Context, db *sql.DB, sessionID, userID int64) ([]model.ChatMessage, error) {
	var owner int64
	if err := db.QueryRowContext(ctx, `SELECT user_id FROM chat_sessions WHERE id=?`, sessionID).Scan(&owner); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if owner != userID {
		return nil, ErrNotFound
	}
	rows, err := db.QueryContext(ctx, `SELECT id,session_id,role,content,created_at FROM chat_messages WHERE session_id=? ORDER BY id`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.ChatMessage{}
	for rows.Next() {
		var m model.ChatMessage
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func GetAgentByID(ctx context.Context, db *sql.DB, id int64) (*model.Agent, error) {
	row := db.QueryRowContext(ctx, `SELECT a.id,a.slug,a.name,a.tagline,a.description,a.icon_url,a.cover_url,a.screenshots,a.category_id,c.name,c.slug,a.developer,a.version,a.size_bytes,a.rating,a.rating_count,a.install_count,a.is_free,a.price_cents,a.is_featured,COALESCE(a.feature_badge,''),a.capabilities,COALESCE(a.updated_notes,''),a.released_at,a.updated_at, 0 AS installed
		FROM agents a JOIN categories c ON c.id=a.category_id WHERE a.id=?`, id)
	a, err := scanAgent(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}
