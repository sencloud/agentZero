package llm

import "strings"

// StreamAggregator 把流式 delta 聚合成一条完整的 assistant 消息。
//
// 典型用法：
//
//	agg := llm.NewAggregator()
//	for ev := range ch {
//	    agg.Apply(ev)
//	    // 同时可以把 ev 直接推给前端 SSE
//	}
//	msg := agg.AssistantMessage()
//
// 注意 V4 thinking 模式：若 msg.ToolCalls 非空，下一轮请求时
// msg 必须整条（包含 ReasoningContent + Content + ToolCalls）回传给 DeepSeek。
type StreamAggregator struct {
	reasoning strings.Builder
	content   strings.Builder
	tools     map[int]*toolBuilder // 按 delta 上的 index 维护
	maxIndex  int
	finish    string
	usage     *UsageInfo
}

type toolBuilder struct {
	id   string
	name string
	args strings.Builder
}

func NewAggregator() *StreamAggregator {
	return &StreamAggregator{tools: map[int]*toolBuilder{}, maxIndex: -1}
}

// Apply 吸收一个事件。EvtError 不影响累加；调用方负责自己处理 Err。
func (a *StreamAggregator) Apply(ev StreamEvent) {
	switch ev.Kind {
	case EvtReasoningDelta:
		a.reasoning.WriteString(ev.ReasoningDelta)
	case EvtContentDelta:
		a.content.WriteString(ev.ContentDelta)
	case EvtToolCallDelta:
		if ev.ToolCall == nil {
			return
		}
		b, ok := a.tools[ev.ToolCall.Index]
		if !ok {
			b = &toolBuilder{}
			a.tools[ev.ToolCall.Index] = b
		}
		if ev.ToolCall.ID != "" {
			b.id = ev.ToolCall.ID
		}
		if ev.ToolCall.Name != "" {
			b.name = ev.ToolCall.Name
		}
		if ev.ToolCall.ArgsFrag != "" {
			b.args.WriteString(ev.ToolCall.ArgsFrag)
		}
		if ev.ToolCall.Index > a.maxIndex {
			a.maxIndex = ev.ToolCall.Index
		}
	case EvtFinish:
		a.finish = ev.FinishReason
	case EvtUsage:
		a.usage = ev.Usage
	}
}

// AssistantMessage 返回累加结果。
func (a *StreamAggregator) AssistantMessage() Message {
	msg := Message{
		Role:             RoleAssistant,
		ReasoningContent: a.reasoning.String(),
		Content:          a.content.String(),
	}
	if a.maxIndex >= 0 {
		msg.ToolCalls = make([]ToolCall, 0, a.maxIndex+1)
		for i := 0; i <= a.maxIndex; i++ {
			b, ok := a.tools[i]
			if !ok {
				continue
			}
			msg.ToolCalls = append(msg.ToolCalls, ToolCall{
				ID:   b.id,
				Type: "function",
				Function: ToolFunction{
					Name:      b.name,
					Arguments: b.args.String(),
				},
			})
		}
	}
	return msg
}

// FinishReason 返回 OpenAI 兼容 finish_reason: "stop" / "tool_calls" / "length" 等。
func (a *StreamAggregator) FinishReason() string { return a.finish }

// Usage 返回流末尾的 usage 信息（若服务器有发送）。
func (a *StreamAggregator) Usage() *UsageInfo { return a.usage }
