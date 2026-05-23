package model

import "time"

type User struct {
	ID         int64     `json:"id"`
	AppleSub   string    `json:"-"`
	Email      string    `json:"email,omitempty"`
	Nickname   string    `json:"nickname"`
	AvatarURL  string    `json:"avatar_url,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type Category struct {
	ID    int64  `json:"id"`
	Slug  string `json:"slug"`
	Name  string `json:"name"`
	Icon  string `json:"icon"`
	Color string `json:"color"`
}

type Agent struct {
	ID            int64     `json:"id"`
	Slug          string    `json:"slug"`
	Name          string    `json:"name"`
	Tagline       string    `json:"tagline"`
	Description   string    `json:"description"`
	IconURL       string    `json:"icon_url"`
	CoverURL      string    `json:"cover_url"`
	Screenshots   []string  `json:"screenshots"`
	CategoryID    int64     `json:"category_id"`
	CategoryName  string    `json:"category_name,omitempty"`
	CategorySlug  string    `json:"category_slug,omitempty"`
	Developer     string    `json:"developer"`
	Version       string    `json:"version"`
	SizeBytes     int64     `json:"size_bytes"`
	Rating        float64   `json:"rating"`
	RatingCount   int64     `json:"rating_count"`
	InstallCount  int64     `json:"install_count"`
	IsFree        bool      `json:"is_free"`
	PriceCents    int64     `json:"price_cents"`
	IsFeatured    bool      `json:"is_featured"`
	FeatureBadge  string    `json:"feature_badge,omitempty"`
	Capabilities  []string  `json:"capabilities"`
	UpdatedNotes  string    `json:"updated_notes,omitempty"`
	ReleasedAt    time.Time `json:"released_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	Installed     bool      `json:"installed"`
}

type Review struct {
	ID        int64     `json:"id"`
	AgentID   int64     `json:"agent_id"`
	UserID    int64     `json:"user_id"`
	Nickname  string    `json:"nickname"`
	Avatar    string    `json:"avatar,omitempty"`
	Rating    int       `json:"rating"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

type TodayCard struct {
	ID        int64  `json:"id"`
	Kind      string `json:"kind"`
	Eyebrow   string `json:"eyebrow"`
	Title     string `json:"title"`
	Subtitle  string `json:"subtitle"`
	CoverURL  string `json:"cover_url"`
	AgentID   int64  `json:"agent_id,omitempty"`
	AgentSlug string `json:"agent_slug,omitempty"`
	SortOrder int    `json:"sort_order"`
}

type ChatSession struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	AgentID   int64     `json:"agent_id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
}

type ChatMessage struct {
	ID        int64     `json:"id"`
	SessionID int64     `json:"session_id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}
