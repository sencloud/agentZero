package model

import (
	"encoding/json"
	"time"
)

// MissionTier 代表派遣任务时选择的"档位"，对应不同的 DeepSeek 模型与思考强度。
type MissionTier string

const (
	TierFlash    MissionTier = "flash"    // deepseek-v4-flash + thinking disabled
	TierStandard MissionTier = "standard" // deepseek-v4-flash + thinking high  (默认)
	TierPro      MissionTier = "pro"      // deepseek-v4-pro   + thinking max
)

// MissionStatus 是任务的生命周期状态。
type MissionStatus string

const (
	StatusPending MissionStatus = "pending"
	StatusRunning MissionStatus = "running"
	StatusDone    MissionStatus = "done"
	StatusAborted MissionStatus = "aborted"
	StatusError   MissionStatus = "error"
)

// Mission 是一次完整的任务派遣（特工出勤）。
type Mission struct {
	ID           string        `json:"id"`
	UserID       int64         `json:"user_id"`
	Codename     string        `json:"codename"`
	Brief        string        `json:"brief"`
	Tier         MissionTier   `json:"tier"`
	Status       MissionStatus `json:"status"`
	Loadout      []string      `json:"loadout"`       // 允许动用的装备名列表
	WorkspaceDir string        `json:"workspace_dir"` // 任务隔离目录绝对路径（不直接暴露给前端）
	InputTokens  int64         `json:"input_tokens"`
	OutputTokens int64         `json:"output_tokens"`
	// SeriesID 行动卷宗 ID：一次性派遣的 mission，SeriesID = ID；
	// 通过「继续安排」生成的后续 mission，SeriesID = 首次 mission 的 ID。
	SeriesID  string  `json:"series_id"`
	SeriesSeq int     `json:"series_seq"`           // 在该卷宗内的序号，1 = 首次
	ParentID  *string `json:"parent_id,omitempty"`  // 上一个 mission 的 ID（首次为 nil）
	StartedAt *time.Time `json:"started_at,omitempty"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// StepType 是行动现场事件流上每一条记录的类型。
//
// 它与 Step.Payload 的具体结构对应，约定如下：
//
//	thought       { "text": string }              ← 模型 reasoning_content 增量段
//	message       { "text": string }              ← 模型 content 输出段（可能多段拼装）
//	tool_call     { "id","name","arguments_json" }← 模型决定调装备
//	tool_result   { "id","name","ok","content","error" } ← 装备执行结果
//	artifact      { "artifact_id","name","kind" } ← 入柜事件
//	system        { "kind","text" }               ← 派遣/撤离/错误提示
//	usage         { "input_tokens","output_tokens","cache_hit_tokens" } ← 计费快照
type StepType string

const (
	StepThought    StepType = "thought"
	StepMessage    StepType = "message"
	StepToolCall   StepType = "tool_call"
	StepToolResult StepType = "tool_result"
	StepArtifact   StepType = "artifact"
	StepSystem     StepType = "system"
	StepUsage      StepType = "usage"
)

// Step 是任务事件流的一条不可变记录。所有事件都按 (mission_id, seq) 单调递增。
type Step struct {
	ID        int64           `json:"id"`
	MissionID string          `json:"mission_id"`
	Seq       int             `json:"seq"`
	Ts        time.Time       `json:"ts"`
	Type      StepType        `json:"type"`
	Payload   json.RawMessage `json:"payload"`

	// ReasoningContent 仅对"导致 tool_call 的那一轮 assistant 消息"非空。
	// DeepSeek V4 thinking 模式要求：tool_call 所在轮的 reasoning_content
	// 必须在下一次请求中完整回传，否则 HTTP 400。
	// 我们把它和发起 tool_call 的 step 一起持久化，以便重连/重放/继续。
	ReasoningContent string `json:"-"`
}

// Artifact 是任务产出的工件（入柜物）。
type Artifact struct {
	ID        int64     `json:"id"`
	MissionID string    `json:"mission_id"`
	Kind      string    `json:"kind"` // file / image / code / url / chart
	Name      string    `json:"name"`
	Path      string    `json:"path"` // 相对 mission workspace 的路径
	Mime      string    `json:"mime,omitempty"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
}

// Review 是用户对一次行动的点评：1-5 星 + 可选评语。
// 一个 mission 最多一条 review，重复提交即 upsert。
type Review struct {
	MissionID string    `json:"mission_id"`
	Rating    int       `json:"rating"` // 1..5
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Skill 是用户从一次高分行动里"沉淀"下来的可复用方法论。
// 之后派遣相关任务时，prompt_template 可以作为隐式系统提示参与。
type Skill struct {
	ID              int64     `json:"id"`
	UserID          int64     `json:"user_id"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	TriggerHint     string    `json:"trigger_hint"`    // 什么时候启用这项技能的一句话提示
	PromptTemplate  string    `json:"prompt_template"` // 实际注入的 prompt 片段
	SourceMissionID *string   `json:"source_mission_id,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}
