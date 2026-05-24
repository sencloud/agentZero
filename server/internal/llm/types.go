// Package llm 是 DeepSeek V4 (OpenAI 兼容) 客户端。
//
// 关键事实：
//   - base_url: https://api.deepseek.com
//   - 模型: deepseek-v4-flash / deepseek-v4-pro
//   - thinking 模式是请求参数，不再是独立模型；默认 enabled
//   - 流式: SSE，每行 `data: <json>`，结束 `data: [DONE]`
//   - 流式 delta 上同时有三种字段:
//       reasoning_content  (CoT, V4 thinking 模式)
//       content            (最终答复正文)
//       tool_calls[]       (含 index + function.arguments 增量字符串)
//   - 含 tool_call 的 assistant 消息，下一轮请求 MUST 回传 reasoning_content。
//     缺失时 API 返 HTTP 400。
//   - usage 在最后一个 chunk 携带，含 prompt_cache_hit_tokens / cache_miss_tokens。
package llm

import "encoding/json"

// Role 是 OpenAI 兼容的消息角色。
const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

// Message 是 chat.completions 请求里的一条消息。
//
// 注意 V4 thinking 模式的回传规则：
//   - assistant 消息若含 ToolCalls，下一次请求时必须带上 ReasoningContent。
//   - role=tool 的消息必须带 ToolCallID（与 assistant 发起的某个 ToolCall.ID 配对）。
type Message struct {
	Role             string     `json:"role"`
	Content          string     `json:"content,omitempty"`
	ReasoningContent string     `json:"reasoning_content,omitempty"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string     `json:"tool_call_id,omitempty"`
	Name             string     `json:"name,omitempty"`
}

// ToolCall 是模型决定调一个工具的描述。
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"` // 总是 "function"
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON 字符串（流式时按 delta 拼接）
}

// ToolDef 是请求里给模型的工具定义。
type ToolDef struct {
	Type     string         `json:"type"` // "function"
	Function ToolDefFunc    `json:"function"`
}

type ToolDefFunc struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters"` // JSON Schema
	Strict      bool            `json:"strict,omitempty"`
}

// ThinkingMode 控制 V4 thinking。
type ThinkingMode struct {
	Type string `json:"type"` // "enabled" / "disabled"
}

// ChatRequest 对应 POST /chat/completions 的请求体。
type ChatRequest struct {
	Model           string          `json:"model"`
	Messages        []Message       `json:"messages"`
	Tools           []ToolDef       `json:"tools,omitempty"`
	ToolChoice      string          `json:"tool_choice,omitempty"` // "auto" / "none" / "required"
	Thinking        *ThinkingMode   `json:"thinking,omitempty"`
	ReasoningEffort string          `json:"reasoning_effort,omitempty"` // "high" / "max"
	Stream          bool            `json:"stream"`
	StreamOptions   *StreamOpts     `json:"stream_options,omitempty"`
	Temperature     *float64        `json:"temperature,omitempty"`
	MaxTokens       *int            `json:"max_tokens,omitempty"`
	ResponseFormat  *ResponseFormat `json:"response_format,omitempty"`
}

// ResponseFormat 让模型强制按某种格式输出，最常用是 {"type":"json_object"}。
type ResponseFormat struct {
	Type string `json:"type"`
}

// ChatResponse 是非流式 chat.completions 的响应体（OpenAI 兼容）。
type ChatResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Model   string         `json:"model"`
	Choices []ChatChoice   `json:"choices"`
	Usage   *UsageInfo     `json:"usage,omitempty"`
}

// ChatChoice 是非流式响应里的一个 choice。
type ChatChoice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason,omitempty"`
}

// StreamOpts 让流式响应在最后一个 chunk 里带上 usage。
type StreamOpts struct {
	IncludeUsage bool `json:"include_usage"`
}

// StreamChunk 是 SSE 一行 `data: <json>` 解码后的体。
// 注意 OpenAI 兼容协议里 choices 是数组但实际单流只有一项。
type StreamChunk struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Model   string         `json:"model"`
	Choices []ChunkChoice  `json:"choices"`
	Usage   *UsageInfo     `json:"usage,omitempty"`
}

type ChunkChoice struct {
	Index        int          `json:"index"`
	Delta        DeltaMessage `json:"delta"`
	FinishReason *string      `json:"finish_reason,omitempty"`
}

// DeltaMessage 是流式增量。三种字段都可能在同一 chunk 出现，也可能各自单独出现。
type DeltaMessage struct {
	Role             string          `json:"role,omitempty"`
	Content          string          `json:"content,omitempty"`
	ReasoningContent string          `json:"reasoning_content,omitempty"`
	ToolCalls        []DeltaToolCall `json:"tool_calls,omitempty"`
}

// DeltaToolCall 的 Function.Arguments 是按字符增量到达的，跨 chunk 拼接后才能 JSON.Unmarshal。
type DeltaToolCall struct {
	Index    int            `json:"index"`
	ID       string         `json:"id,omitempty"`
	Type     string         `json:"type,omitempty"`
	Function DeltaToolFunc  `json:"function,omitempty"`
}

type DeltaToolFunc struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

// UsageInfo 含 DeepSeek prefix-cache 计费所需的关键字段。
type UsageInfo struct {
	PromptTokens         int `json:"prompt_tokens"`
	CompletionTokens     int `json:"completion_tokens"`
	TotalTokens          int `json:"total_tokens"`
	PromptCacheHitTokens int `json:"prompt_cache_hit_tokens,omitempty"`
	PromptCacheMissTokens int `json:"prompt_cache_miss_tokens,omitempty"`
}

// APIError 是 DeepSeek 错误体。
type APIError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}
