package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/agentzero/server/internal/db"
	"github.com/agentzero/server/internal/llm"
	"github.com/agentzero/server/internal/model"
	"github.com/agentzero/server/internal/tools"
)

// Config 是 Runner 的运行时配置。
type Config struct {
	// WorkspaceRoot 是所有 mission workspace 的父目录绝对路径，
	// 每个 mission 的隔离目录在 <WorkspaceRoot>/<mission_id>/ 下。
	WorkspaceRoot string

	// MaxIterations 是 think→act→observe 循环的最大轮数（防失控）。
	MaxIterations int

	// SystemPrompt 是注入到每次 LLM 调用最前面的"特工身份"提示。
	SystemPrompt string
}

// DefaultSystemPrompt 是默认的特工 system prompt。
const DefaultSystemPrompt = `你是「代号零」（Agent Zero），一名隶属于 AgentZero 行动局的特工。

你的工作风格：
- 接到任务后用中文思考、用中文给出最终回答。
- 优先调用装备（tool）获取一手信息，而不是凭记忆作答。
- 每一步动作之前先用 1-2 句解释打算做什么；做完后用 1 句话总结观察。

【最终交付格式·硬性要求】
任务完成前的"最后一步"，必须调用 write_file 装备产出一份完整的 HTML 报告：
- path 固定为 "报告.html"
- mime 用 "text/html"
- content 必须是一份独立、自包含的 HTML 文档（不允许只有片段），包括：
    <!DOCTYPE html>
    <html lang="zh-CN">
    <head>
      <meta charset="utf-8">
      <meta name="viewport" content="width=device-width,initial-scale=1">
      <title>……</title>
      <style>
        /* 内联 CSS：在手机上打开就好看。建议浅底深字，标题分级清晰，
           段落 line-height ~1.7，正文字号 16-17px；表格、引用块、代码块
           都要有合适的样式；移动端宽度自适应。 */
      </style>
    </head>
    <body>
      <!-- 报告正文：用 h1/h2/h3 分章节；正文段落、有序/无序列表、
           适当的强调、表格、引用块按需。最后一节 <h2>参考来源</h2>，
           把所有引用的资料用 <a href="…" target="_blank"> 列出来。 -->
    </body>
    </html>
- HTML 入柜后，用 1-2 句中文给用户一个非常简短的总览（不要复述全文，
  不要再贴 markdown 报告）。汇报本体以工件柜里的 HTML 为准。

你正在执行的当前任务（行动代号与简报）由用户在下一条 user 消息中给出。`

// Runner 跑 Agent loop。
//
// 它本身是无状态的（除 active 表外）；可以被 API 层用同一个实例并发跑多个 mission。
type Runner struct {
	cfg      Config
	db       *sql.DB
	llm      *llm.Client
	registry *tools.Registry
	broker   *Broker
	logger   *slog.Logger

	activeMu sync.Mutex
	active   map[string]context.CancelFunc
}

// New 创建一个 Runner。WorkspaceRoot/SystemPrompt 留空会取默认值。
func New(cfg Config, database *sql.DB, llmClient *llm.Client, reg *tools.Registry, broker *Broker, logger *slog.Logger) *Runner {
	if cfg.WorkspaceRoot == "" {
		cfg.WorkspaceRoot = "/var/lib/agentzero/workspaces"
	}
	if cfg.MaxIterations <= 0 {
		cfg.MaxIterations = 16
	}
	if cfg.SystemPrompt == "" {
		cfg.SystemPrompt = DefaultSystemPrompt
	}
	return &Runner{
		cfg:      cfg,
		db:       database,
		llm:      llmClient,
		registry: reg,
		broker:   broker,
		logger:   logger,
		active:   map[string]context.CancelFunc{},
	}
}

// Start 启动一个 mission。它会立即在后台启一个 goroutine 跑 loop，并返回。
// 调用方可以用 Abort 中止。
//
// 注意：ctx 必须用 context.Background 派生，不能用 HTTP request 的 r.Context()。
// 否则 handler 一返回 request ctx 就被 cancel，整个 mission 立刻被取消。
func (r *Runner) Start(m *model.Mission) error {
	if err := os.MkdirAll(m.WorkspaceDir, 0o755); err != nil {
		return fmt.Errorf("prepare workspace: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.activeMu.Lock()
	r.active[m.ID] = cancel
	r.activeMu.Unlock()

	go func() {
		defer func() {
			r.activeMu.Lock()
			delete(r.active, m.ID)
			r.activeMu.Unlock()
			cancel()
			r.broker.FinishMission(m.ID)
		}()
		r.runLoop(ctx, m)
	}()
	return nil
}

// Abort 取消一个正在跑的 mission。可重复调用，幂等。
func (r *Runner) Abort(missionID string) {
	r.activeMu.Lock()
	cancel, ok := r.active[missionID]
	r.activeMu.Unlock()
	if !ok {
		return
	}
	cancel()
}

// IsRunning 判断 mission 是否在跑（在 active map 里）。
func (r *Runner) IsRunning(missionID string) bool {
	r.activeMu.Lock()
	defer r.activeMu.Unlock()
	_, ok := r.active[missionID]
	return ok
}

// runLoop 是 Agent 主循环。在自己 goroutine 里独立跑。
func (r *Runner) runLoop(ctx context.Context, m *model.Mission) {
	if err := db.UpdateMissionStatus(ctx, r.db, m.ID, model.StatusRunning); err != nil {
		r.logger.Error("mark mission running failed", "mission", m.ID, "err", err)
	}
	r.emit(ctx, m.ID, model.StepSystem, jsonObj{"kind": "dispatched", "text": "任务已派遣，特工就位。"}, "")

	modelID, _ := resolveModel(m.Tier)
	thinking := resolveThinking(m.Tier)
	effort := resolveEffort(m.Tier)

	toolsForModel := r.buildToolDefs(m.Loadout)
	r.logger.Info("mission start", "mission", m.ID, "model", modelID, "tools", len(toolsForModel))

	userText := fmt.Sprintf("【行动代号】%s\n【任务简报】%s", m.Codename, m.Brief)
	if chain := r.buildChainContext(ctx, m); chain != "" {
		userText = chain + "\n\n--- 本次行动 ---\n" + userText
	}
	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: r.cfg.SystemPrompt},
		{Role: llm.RoleUser, Content: userText},
	}

	env := &tools.Env{
		MissionID:    m.ID,
		UserID:       m.UserID,
		WorkspaceDir: m.WorkspaceDir,
	}

	for iter := 0; iter < r.cfg.MaxIterations; iter++ {
		if ctx.Err() != nil {
			r.finish(ctx, m.ID, model.MissionStatus(""), "aborted") //nolint:staticcheck // 后续 finish 内部判断
			return
		}

		req := &llm.ChatRequest{
			Model:           modelID,
			Messages:        messages,
			Tools:           toolsForModel,
			Thinking:        thinking,
			ReasoningEffort: effort,
			Stream:          true,
		}

		stream, err := r.llm.Stream(ctx, req)
		if err != nil {
			r.emit(ctx, m.ID, model.StepSystem, jsonObj{"kind": "error", "text": "调用 DeepSeek 失败：" + err.Error()}, "")
			r.finish(ctx, m.ID, model.MissionStatus(""), "error")
			return
		}

		aggregator := llm.NewAggregator()
		// 同一次 LLM 调用里，所有 reasoning/content 增量按到达顺序逐条发到前端
		// （由前端做合并），但 DB 里也按条落库以便重连重放。
		for ev := range stream {
			aggregator.Apply(ev)
			switch ev.Kind {
			case llm.EvtReasoningDelta:
				r.emit(ctx, m.ID, model.StepThought, jsonObj{"text": ev.ReasoningDelta}, "")
			case llm.EvtContentDelta:
				r.emit(ctx, m.ID, model.StepMessage, jsonObj{"text": ev.ContentDelta}, "")
			case llm.EvtToolCallDelta:
				// 工具调用的 args 是流式拼接的，等聚合完整后再 emit 完整 tool_call。
			case llm.EvtUsage:
				if ev.Usage != nil {
					_ = db.AddMissionUsage(ctx, r.db, m.ID, int64(ev.Usage.PromptTokens), int64(ev.Usage.CompletionTokens))
					r.emit(ctx, m.ID, model.StepUsage, jsonObj{
						"input_tokens":         ev.Usage.PromptTokens,
						"output_tokens":        ev.Usage.CompletionTokens,
						"cache_hit_tokens":     ev.Usage.PromptCacheHitTokens,
						"cache_miss_tokens":    ev.Usage.PromptCacheMissTokens,
					}, "")
				}
			case llm.EvtError:
				r.emit(ctx, m.ID, model.StepSystem, jsonObj{"kind": "error", "text": "流式解析失败：" + ev.Err.Error()}, "")
				r.finish(ctx, m.ID, model.MissionStatus(""), "error")
				return
			}
		}

		assistantMsg := aggregator.AssistantMessage()
		finishReason := aggregator.FinishReason()

		// 把完整 assistant 消息（含 reasoning_content + content + tool_calls）回灌到 messages，
		// 这是 V4 thinking 模式 + tool_call 的硬性要求。
		messages = append(messages, assistantMsg)

		if len(assistantMsg.ToolCalls) == 0 {
			r.emit(ctx, m.ID, model.StepSystem, jsonObj{"kind": "task_done", "text": "任务完成。", "finish_reason": finishReason}, "")
			r.finish(ctx, m.ID, model.StatusDone, "")
			return
		}

		// 把每个 tool_call 立即 emit 一条完整 step，然后执行
		for _, tc := range assistantMsg.ToolCalls {
			r.emit(ctx, m.ID, model.StepToolCall, jsonObj{
				"id":             tc.ID,
				"name":           tc.Function.Name,
				"arguments_json": tc.Function.Arguments,
			}, assistantMsg.ReasoningContent)

			toolMsg := r.invokeTool(ctx, env, &tc)
			messages = append(messages, toolMsg)
		}
	}

	r.emit(ctx, m.ID, model.StepSystem, jsonObj{"kind": "max_iter", "text": "达到最大行动轮数，任务终止。"}, "")
	r.finish(ctx, m.ID, model.StatusError, "")
}

// invokeTool 执行一次工具调用，并把"装备结果"事件落库 + 广播。
// 返回的 llm.Message 是 role=tool 那条，需要追加到下一次请求的 messages 里。
func (r *Runner) invokeTool(ctx context.Context, env *tools.Env, tc *llm.ToolCall) llm.Message {
	tool, ok := r.registry.Get(tc.Function.Name)
	if !ok {
		errMsg := fmt.Sprintf("未知装备：%s", tc.Function.Name)
		r.emit(ctx, env.MissionID, model.StepToolResult, jsonObj{
			"id":      tc.ID,
			"name":    tc.Function.Name,
			"ok":      false,
			"content": errMsg,
		}, "")
		return llm.Message{Role: llm.RoleTool, ToolCallID: tc.ID, Content: errMsg}
	}
	if !r.isAllowed(env.MissionID, tc.Function.Name) {
		errMsg := fmt.Sprintf("装备 %s 不在本任务允许范围", tc.Function.Name)
		r.emit(ctx, env.MissionID, model.StepToolResult, jsonObj{
			"id":      tc.ID,
			"name":    tc.Function.Name,
			"ok":      false,
			"content": errMsg,
		}, "")
		return llm.Message{Role: llm.RoleTool, ToolCallID: tc.ID, Content: errMsg}
	}

	result, err := tool.Run(ctx, json.RawMessage(tc.Function.Arguments), env)
	if err != nil {
		errMsg := err.Error()
		r.emit(ctx, env.MissionID, model.StepToolResult, jsonObj{
			"id":      tc.ID,
			"name":    tc.Function.Name,
			"ok":      false,
			"content": errMsg,
		}, "")
		return llm.Message{Role: llm.RoleTool, ToolCallID: tc.ID, Content: "装备执行失败：" + errMsg}
	}

	if result.Artifact != nil {
		art := &model.Artifact{
			MissionID: env.MissionID,
			Kind:      result.Artifact.Kind,
			Name:      result.Artifact.Name,
			Path:      result.Artifact.Path,
			Mime:      result.Artifact.Mime,
			Size:      result.Artifact.Size,
		}
		if err := db.AddArtifact(ctx, r.db, art); err == nil {
			r.emit(ctx, env.MissionID, model.StepArtifact, jsonObj{
				"artifact_id": art.ID,
				"name":        art.Name,
				"kind":        art.Kind,
				"path":        art.Path,
				"mime":        art.Mime,
				"size":        art.Size,
			}, "")
		}
	}

	r.emit(ctx, env.MissionID, model.StepToolResult, jsonObj{
		"id":      tc.ID,
		"name":    tc.Function.Name,
		"ok":      true,
		"content": result.Content,
	}, "")
	return llm.Message{Role: llm.RoleTool, ToolCallID: tc.ID, Content: result.Content}
}

// isAllowed 判断本 mission 的 loadout 里是否允许该装备。
// 第一版我们直接读 mission.loadout 字段；为了不每次查 DB，我们在 Runner 启动时
// 把 loadout 缓存到一个轻 map，但目前实现简化为每次查 DB 一次。
//
// 性能上，每个 tool_call 一次单行 SELECT 没什么压力；后续可加内存缓存。
func (r *Runner) isAllowed(missionID, name string) bool {
	row := r.db.QueryRow(`SELECT loadout_json FROM missions WHERE id = ?`, missionID)
	var raw string
	if err := row.Scan(&raw); err != nil {
		return false
	}
	var loadout []string
	if err := json.Unmarshal([]byte(raw), &loadout); err != nil {
		return false
	}
	for _, l := range loadout {
		if l == name {
			return true
		}
	}
	return false
}

// buildToolDefs 把 registry 里的工具转成 DeepSeek tools 数组，仅包含 loadout 允许的那些。
func (r *Runner) buildToolDefs(loadout []string) []llm.ToolDef {
	allow := map[string]struct{}{}
	for _, n := range loadout {
		allow[n] = struct{}{}
	}
	var out []llm.ToolDef
	for _, t := range r.registry.All() {
		if _, ok := allow[t.Name()]; !ok {
			continue
		}
		out = append(out, llm.ToolDef{
			Type: "function",
			Function: llm.ToolDefFunc{
				Name:        t.Name(),
				Description: t.Description(),
				Parameters:  t.Parameters(),
			},
		})
	}
	return out
}

// emit 把一条 step 写入 DB 并广播给订阅者。
func (r *Runner) emit(ctx context.Context, missionID string, stepType model.StepType, payload jsonObj, reasoning string) {
	raw, err := json.Marshal(payload)
	if err != nil {
		r.logger.Error("marshal step payload", "err", err)
		return
	}
	step := &model.Step{
		MissionID:        missionID,
		Type:             stepType,
		Payload:          raw,
		ReasoningContent: reasoning,
	}
	if err := db.AppendStep(ctx, r.db, step); err != nil {
		r.logger.Error("append step failed", "err", err, "mission", missionID, "type", stepType)
		return
	}
	r.broker.Publish(missionID, step)
}

// finish 推进 mission 终态，并 emit 一条 system 收尾事件。
// 如果 ctx 已经被取消（abort），强制把 status 设为 aborted。
func (r *Runner) finish(ctx context.Context, missionID string, status model.MissionStatus, override string) {
	// 一旦 ctx 被取消，无论 caller 传了什么，都按 aborted 终态走。
	finalStatus := status
	if errors.Is(ctx.Err(), context.Canceled) {
		finalStatus = model.StatusAborted
	}
	if override == "error" {
		finalStatus = model.StatusError
	}
	if override == "aborted" {
		finalStatus = model.StatusAborted
	}

	// 落库用一个独立 background ctx，避免 ctx 已被取消时写不进去。
	persistCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.UpdateMissionStatus(persistCtx, r.db, missionID, finalStatus); err != nil {
		r.logger.Error("finalize mission failed", "err", err, "mission", missionID)
	}
}

// jsonObj 是 emit 时构造 payload 的内联辅助类型。
type jsonObj map[string]any

// resolveModel 把档位映射到具体的 DeepSeek 模型 ID。
func resolveModel(tier model.MissionTier) (string, model.MissionTier) {
	switch tier {
	case model.TierPro:
		return llm.ModelV4Pro, tier
	case model.TierFlash, model.TierStandard:
		return llm.ModelV4Flash, tier
	default:
		return llm.ModelV4Flash, model.TierStandard
	}
}

func resolveThinking(tier model.MissionTier) *llm.ThinkingMode {
	switch tier {
	case model.TierFlash:
		return &llm.ThinkingMode{Type: llm.ThinkingDisabled}
	case model.TierPro, model.TierStandard:
		return &llm.ThinkingMode{Type: llm.ThinkingEnabled}
	default:
		return &llm.ThinkingMode{Type: llm.ThinkingEnabled}
	}
}

func resolveEffort(tier model.MissionTier) string {
	switch tier {
	case model.TierPro:
		return llm.EffortMax
	case model.TierStandard:
		return llm.EffortHigh
	case model.TierFlash:
		return ""
	default:
		return llm.EffortHigh
	}
}

// MissionWorkspace 给外部调用方一个统一拼路径的入口。
func (r *Runner) MissionWorkspace(missionID string) string {
	return filepath.Join(r.cfg.WorkspaceRoot, missionID)
}

// buildChainContext 在「继续安排」场景下，把上一行动的简报与最终报告摘要
// 拼成一段前置 user 上下文。无父任务或读取失败时返回空串（不影响主流程）。
func (r *Runner) buildChainContext(ctx context.Context, m *model.Mission) string {
	if m.ParentID == nil || *m.ParentID == "" {
		return ""
	}
	parent, err := db.GetMission(ctx, r.db, *m.ParentID, m.UserID)
	if err != nil {
		r.logger.Warn("chain ctx: load parent failed", "parent", *m.ParentID, "err", err)
		return ""
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "【行动卷宗·第 %d 次出勤·上次代号：%s】\n", parent.SeriesSeq, parent.Codename)
	fmt.Fprintf(&sb, "上次简报：%s\n", parent.Brief)

	arts, err := db.ListArtifacts(ctx, r.db, parent.ID)
	if err == nil {
		for _, a := range arts {
			lo := strings.ToLower(a.Name)
			if a.Name != "报告.html" && !strings.HasSuffix(lo, ".html") && !strings.HasSuffix(lo, ".htm") {
				continue
			}
			full := a.Path
			if !filepath.IsAbs(full) {
				full = filepath.Join(parent.WorkspaceDir, full)
			}
			b, err := os.ReadFile(full)
			if err != nil {
				continue
			}
			plain := htmlToPlain(b)
			if len(plain) > 4000 {
				plain = plain[:4000] + "…"
			}
			sb.WriteString("上次行动报告（节选）：\n")
			sb.WriteString(plain)
			sb.WriteString("\n")
			break
		}
	}
	sb.WriteString("\n请基于上次行动的结论与产出继续推进下文的新指令。")
	return sb.String()
}

var (
	stripScriptRe = regexp.MustCompile(`(?is)<(script|style)[^>]*>.*?</(?:script|style)>`)
	stripTagRe    = regexp.MustCompile(`<[^>]+>`)
)

// htmlToPlain 简化版去标签：把 HTML 转成可塞进 prompt 的纯文本。
func htmlToPlain(b []byte) string {
	s := string(b)
	s = stripScriptRe.ReplaceAllString(s, " ")
	s = stripTagRe.ReplaceAllString(s, " ")
	return strings.Join(strings.Fields(s), " ")
}
