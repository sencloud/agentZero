package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	DefaultBaseURL = "https://api.deepseek.com"

	ModelV4Flash = "deepseek-v4-flash"
	ModelV4Pro   = "deepseek-v4-pro"

	ThinkingEnabled  = "enabled"
	ThinkingDisabled = "disabled"

	EffortHigh = "high"
	EffortMax  = "max"
)

// Client 是 DeepSeek HTTP 客户端。线程安全，可被多个 mission 共享。
type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

// NewClient 创建带默认超时的客户端。
//
// 默认 HTTP timeout 设得很长（10 分钟），是因为长 thinking 任务可能跑很久；
// 真实的 per-request 取消由调用方通过 context 控制。
func NewClient(apiKey string) *Client {
	return &Client{
		BaseURL: DefaultBaseURL,
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Minute,
		},
	}
}

// StreamEventKind 用于上层 select 时区分本次到达的是什么。
type StreamEventKind string

const (
	EvtReasoningDelta StreamEventKind = "reasoning_delta"
	EvtContentDelta   StreamEventKind = "content_delta"
	EvtToolCallDelta  StreamEventKind = "tool_call_delta"
	EvtFinish         StreamEventKind = "finish"
	EvtUsage          StreamEventKind = "usage"
	EvtError          StreamEventKind = "error"
)

// ToolCallDelta 是 tool_calls 增量。每条 delta 里 Args 是当前片段，
// 上层需按 Index 维护一个 builder，跨 delta 累加。
type ToolCallDelta struct {
	Index    int
	ID       string
	Name     string
	ArgsFrag string
}

// StreamEvent 上层 agent loop 消费的事件。
type StreamEvent struct {
	Kind           StreamEventKind
	ReasoningDelta string
	ContentDelta   string
	ToolCall       *ToolCallDelta
	FinishReason   string
	Usage          *UsageInfo
	Err            error
}

// Stream 发起一次流式 chat.completions 调用，把解析后的事件推到返回的 chan。
//
// 该 chan 会在以下任一情况关闭：
//   - 服务器发送 `data: [DONE]`
//   - context 被取消
//   - 出现网络/解析错误（先发一条 EvtError，再关闭 chan）
//
// 调用方必须把 chan 读到关闭，否则会泄漏一个 goroutine。
func (c *Client) Stream(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error) {
	if !req.Stream {
		req.Stream = true
	}
	if req.StreamOptions == nil {
		req.StreamOptions = &StreamOpts{IncludeUsage: true}
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	url := strings.TrimRight(c.BaseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 32*1024))
		var apiErr APIError
		_ = json.Unmarshal(raw, &apiErr)
		msg := apiErr.Error.Message
		if msg == "" {
			msg = string(raw)
		}
		return nil, fmt.Errorf("deepseek http %d: %s", resp.StatusCode, msg)
	}

	out := make(chan StreamEvent, 64)
	go parseSSE(resp.Body, out)
	return out, nil
}

// parseSSE 读 SSE body 行，逐行解析 `data: <json>` 并产出事件。
// 完成后关闭 chan 并 close body。
func parseSSE(body io.ReadCloser, out chan<- StreamEvent) {
	defer close(out)
	defer body.Close()

	scanner := bufio.NewScanner(body)
	// SSE 单行可能很长（含一段 reasoning_content），默认 64KB 缓冲不够。
	scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			return
		}
		var chunk StreamChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			out <- StreamEvent{Kind: EvtError, Err: fmt.Errorf("decode chunk: %w; raw=%s", err, truncate(payload, 200))}
			return
		}
		dispatchChunk(&chunk, out)
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		out <- StreamEvent{Kind: EvtError, Err: fmt.Errorf("sse scan: %w", err)}
	}
}

// dispatchChunk 把一个 chunk 拆成若干事件推到 chan。
// V4 的 chunk 里可能同时含 reasoning_content / content / tool_calls；
// 按字段顺序逐项分发。
func dispatchChunk(c *StreamChunk, out chan<- StreamEvent) {
	if c.Usage != nil {
		out <- StreamEvent{Kind: EvtUsage, Usage: c.Usage}
	}
	for _, choice := range c.Choices {
		d := choice.Delta
		if d.ReasoningContent != "" {
			out <- StreamEvent{Kind: EvtReasoningDelta, ReasoningDelta: d.ReasoningContent}
		}
		if d.Content != "" {
			out <- StreamEvent{Kind: EvtContentDelta, ContentDelta: d.Content}
		}
		for _, tc := range d.ToolCalls {
			out <- StreamEvent{
				Kind: EvtToolCallDelta,
				ToolCall: &ToolCallDelta{
					Index:    tc.Index,
					ID:       tc.ID,
					Name:     tc.Function.Name,
					ArgsFrag: tc.Function.Arguments,
				},
			}
		}
		if choice.FinishReason != nil && *choice.FinishReason != "" {
			out <- StreamEvent{Kind: EvtFinish, FinishReason: *choice.FinishReason}
		}
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
