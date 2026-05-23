// Package tools 是特工的装备系统。
//
// 每件装备实现 Tool 接口；Registry 负责按名字派发调用。
// Agent loop 把模型给出的 tool_call 转给 Registry.Invoke 执行后，把结果
// 作为 role=tool 的消息塞回 DeepSeek。
//
// 第一版只有内置装备（fetch_url / read_file / write_file / web_search）。
// 后续可以再挂 MCP 外部装备到同一个 Registry 上。
package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// Env 是装备执行时拿得到的上下文。每个 Mission 一份。
type Env struct {
	MissionID    string
	UserID       int64
	WorkspaceDir string // 绝对路径，装备里所有相对路径都基于这里
}

// Result 是装备执行后的结果。
//
//   - Content 是回给模型的文本（会包到 role=tool 的消息里）。
//     不要太长，超过 ~8KB 建议自行截断 + 提示模型继续追问。
//   - Artifact 非空意味着同时入柜一件工件。Path 必须是 WorkspaceDir
//     的相对路径，由装备自己负责落盘。
type Result struct {
	Content  string
	Artifact *ArtifactSpec
}

// ArtifactSpec 描述要入柜的工件。
type ArtifactSpec struct {
	Kind string // file / image / code / url / chart
	Name string
	Path string // 相对 Env.WorkspaceDir
	Mime string
	Size int64
}

// Tool 是装备的统一接口。
//
// Parameters() 返回 JSON Schema，会原样塞到 DeepSeek 请求的 tools[i].function.parameters。
type Tool interface {
	Name() string
	DisplayName() string
	Description() string
	Parameters() json.RawMessage
	Run(ctx context.Context, args json.RawMessage, env *Env) (*Result, error)
}

// Registry 持有所有可用装备。线程安全只发生在初始化阶段。
type Registry struct {
	tools map[string]Tool
	order []string // 保持注册顺序，方便 UI 展示
}

func NewRegistry() *Registry {
	return &Registry{tools: map[string]Tool{}}
}

// Register 注册一件装备。重名直接覆盖（让调用方在 main 启动时控制顺序）。
func (r *Registry) Register(t Tool) {
	if _, exists := r.tools[t.Name()]; !exists {
		r.order = append(r.order, t.Name())
	}
	r.tools[t.Name()] = t
}

// Get 按名取一件装备。
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// All 返回按注册顺序的所有装备。
func (r *Registry) All() []Tool {
	out := make([]Tool, 0, len(r.order))
	for _, n := range r.order {
		if t, ok := r.tools[n]; ok {
			out = append(out, t)
		}
	}
	return out
}

// Invoke 执行一次调用。
//
// 即便装备返回 error，agent loop 也应当把错误内容作为
// tool_result 喂回模型（让模型自己决定怎么补救），这里不要 panic。
func (r *Registry) Invoke(ctx context.Context, name string, args json.RawMessage, env *Env) (*Result, error) {
	t, ok := r.tools[name]
	if !ok {
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
	return t.Run(ctx, args, env)
}
