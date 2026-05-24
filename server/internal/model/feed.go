package model

import "time"

// Topic 是用户关心的话题（关键词/事项）。
// 同一个用户下 (user_id, name) 唯一；weight 用作匹配时的权重放大。
type Topic struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Name      string    `json:"name"`
	Weight    float64   `json:"weight"`
	CreatedAt time.Time `json:"created_at"`
}

// NewsSource 是一个新闻数据源（RSS / atom feed）。
type NewsSource struct {
	ID          int64      `json:"id"`
	Name        string     `json:"name"`
	URL         string     `json:"url"`
	Kind        string     `json:"kind"`   // rss
	Region      string     `json:"region"` // cn / intl_zh / intl_en
	Lang        string     `json:"lang"`   // zh / en
	Category    string     `json:"category"`    // tech / ai / finance / intl / sports / culture / dev …
	Description string     `json:"description"` // 1-2 句源简介，给 LLM 推荐用
	Enabled     bool       `json:"enabled"`
	LastFetchAt *time.Time `json:"last_fetch_at,omitempty"`
	LastError   string     `json:"last_error,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// NewsEvent 是一条入库的新闻事件。
type NewsEvent struct {
	ID          int64      `json:"id"`
	SourceID    int64      `json:"source_id"`
	URL         string     `json:"url"`
	Title       string     `json:"title"`
	Summary     string     `json:"summary,omitempty"`
	Content     string     `json:"content,omitempty"`
	Lang        string     `json:"lang"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
	FetchedAt   time.Time  `json:"fetched_at"`
	Extracted   bool       `json:"extracted"`
}

// Entity 是关系图谱里的节点：人物 / 机构 / 地点 / 概念 / 事件。
type Entity struct {
	ID          int64     `json:"id"`
	Type        string    `json:"type"`
	Name        string    `json:"name"`
	Weight      float64   `json:"weight"`
	FirstSeenAt time.Time `json:"first_seen_at"`
	LastSeenAt  time.Time `json:"last_seen_at"`
}

// EventEntity 把一条事件和它涉及的实体连起来，salience 用作显著度。
type EventEntity struct {
	EventID  int64   `json:"event_id"`
	EntityID int64   `json:"entity_id"`
	Salience float64 `json:"salience"`
}

// EntityRelation 是图谱里的边：src→dst，label 描述关系语义。
// 同 (src, dst, label) 多次出现会累加 weight + 刷新 last_seen_at。
type EntityRelation struct {
	ID         int64     `json:"id"`
	SrcID      int64     `json:"src_id"`
	DstID      int64     `json:"dst_id"`
	Label      string    `json:"label"`
	Weight     float64   `json:"weight"`
	LastSeenAt time.Time `json:"last_seen_at"`
}

// UserEventSub 是「这个用户因为某些 topic 命中了这条事件」的记录。
type UserEventSub struct {
	UserID        int64     `json:"user_id"`
	EventID       int64     `json:"event_id"`
	Relevance     float64   `json:"relevance"`
	MatchedTopics []int64   `json:"matched_topics"`
	CreatedAt     time.Time `json:"created_at"`
}

// FeedStatus 是事件流 worker 的运行态快照。
type FeedStatus struct {
	Running       bool       `json:"running"`
	LastFetchAt   *time.Time `json:"last_fetch_at,omitempty"`
	LastPruneAt   *time.Time `json:"last_prune_at,omitempty"`
	SourcesTotal  int        `json:"sources_total"`
	SourcesActive int        `json:"sources_active"`
	Events24h     int        `json:"events_24h"`
	EntitiesTotal int        `json:"entities_total"`
	RelationsTotal int       `json:"relations_total"`
	LastError     string     `json:"last_error,omitempty"`
}
