package model

import "time"

// User 代表通过 Apple Sign In 登录的用户。
// 第一版只保留用户身份，后续 Mission/Step/Artifact 等领域模型在 M1 阶段单独建表。
type User struct {
	ID        int64     `json:"id"`
	AppleSub  string    `json:"-"`
	Email     string    `json:"email,omitempty"`
	Nickname  string    `json:"nickname"`
	AvatarURL string    `json:"avatar_url,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
