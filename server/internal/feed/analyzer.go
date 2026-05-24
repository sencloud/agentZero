package feed

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/agentzero/server/internal/db"
	"github.com/agentzero/server/internal/llm"
	"github.com/agentzero/server/internal/model"
)

// Analyzer 跑「情报简报」三阶段分析：
//
//	阶段 A · cluster      把窗口内事件按主题聚类
//	阶段 B · correlate    跨簇关联挖掘 + 信号识别
//	阶段 C · briefing     综合写一份 HTML 富媒体简报，落盘 + 入库
//
// 三阶段总耗 token 较高但产出质量也高，符合「不计成本、深度分析」的设定。
type Analyzer struct {
	db           *sql.DB
	llm          *llm.Client
	logger       *slog.Logger
	model        string
	briefingsDir string
}

func NewAnalyzer(database *sql.DB, llmClient *llm.Client, logger *slog.Logger, modelID, dir string) *Analyzer {
	if modelID == "" {
		modelID = "deepseek-chat"
	}
	if dir == "" {
		dir = "/var/lib/agentzero/briefings"
	}
	_ = os.MkdirAll(dir, 0o755)
	return &Analyzer{db: database, llm: llmClient, logger: logger, model: modelID, briefingsDir: dir}
}

// AnalyzeProgress 是阶段化的进度回调入参。
type AnalyzeProgress struct {
	Phase   string         // cluster / correlate / write / save
	Message string         // 人类可读的提示
	Data    map[string]any // 附加结构化数据
}

// Generate 跑一轮简报生成。
//
//	window: 时间窗口字符串，"1h" / "24h" / "7d"，决定取多久的事件
//	onProgress: 可选阶段进度回调，nil 表示不上报
//
// 返回生成的 briefing；如果窗口内事件 < 3 条会直接退出（无意义）。
func (a *Analyzer) Generate(
	ctx context.Context,
	userID int64,
	window string,
	onProgress func(p AnalyzeProgress),
) (*model.Briefing, error) {
	if window == "" {
		window = "1h"
	}
	tStart := time.Now()

	events, topics, err := a.gatherInputs(ctx, userID, window)
	if err != nil {
		return nil, err
	}
	if len(events) < 3 {
		return nil, fmt.Errorf("not_enough_events: 仅 %d 条，建议先拉取更多源", len(events))
	}
	a.emit(onProgress, AnalyzeProgress{
		Phase:   "gather",
		Message: fmt.Sprintf("窗口 %s · 命中 %d 条事件 · %d 个话题", window, len(events), len(topics)),
		Data: map[string]any{
			"events":     len(events),
			"topics":     len(topics),
			"window":     window,
		},
	})

	// ===== 阶段 A：事件聚类 =====
	a.emit(onProgress, AnalyzeProgress{Phase: "cluster", Message: "LLM 阶段 A：按主题聚类"})
	clusters, err := a.cluster(ctx, events, topics)
	if err != nil {
		return nil, fmt.Errorf("cluster: %w", err)
	}
	a.emit(onProgress, AnalyzeProgress{
		Phase:   "cluster_done",
		Message: fmt.Sprintf("识别出 %d 个主题簇", len(clusters)),
		Data:    map[string]any{"clusters": len(clusters)},
	})

	// ===== 阶段 B：跨簇关联挖掘 =====
	a.emit(onProgress, AnalyzeProgress{Phase: "correlate", Message: "LLM 阶段 B：跨主题关联与信号"})
	insights, err := a.correlate(ctx, clusters, topics)
	if err != nil {
		return nil, fmt.Errorf("correlate: %w", err)
	}
	a.emit(onProgress, AnalyzeProgress{
		Phase:   "correlate_done",
		Message: fmt.Sprintf("生成 %d 条洞察 + %d 条潜在信号", len(insights.Insights), len(insights.Signals)),
	})

	// ===== 阶段 C：综合写简报 =====
	a.emit(onProgress, AnalyzeProgress{Phase: "write", Message: "LLM 阶段 C：综合写 HTML 简报"})
	brief, err := a.writeBriefing(ctx, window, topics, clusters, insights)
	if err != nil {
		return nil, fmt.Errorf("write_briefing: %w", err)
	}
	a.emit(onProgress, AnalyzeProgress{
		Phase:   "write_done",
		Message: fmt.Sprintf("简报正文 %d 字 · 标题：%s", len([]rune(brief.HTML)), brief.Title),
	})

	// ===== 落盘 + 入库 =====
	htmlPath, err := a.persistHTML(userID, brief)
	if err != nil {
		return nil, fmt.Errorf("persist_html: %w", err)
	}

	reasoning, _ := json.Marshal(map[string]any{
		"clusters": clusters,
		"insights": insights,
	})

	row := &model.Briefing{
		UserID:        userID,
		Window:        window,
		Title:         brief.Title,
		Summary:       brief.Summary,
		HTMLPath:      htmlPath,
		Model:         a.model,
		EventCount:    len(events),
		ClusterCount:  len(clusters),
		ReasoningJSON: string(reasoning),
	}
	if err := db.CreateBriefing(ctx, a.db, row); err != nil {
		return nil, fmt.Errorf("create_briefing: %w", err)
	}
	a.emit(onProgress, AnalyzeProgress{
		Phase:   "save",
		Message: fmt.Sprintf("简报 #%d 已落地，总耗时 %s", row.ID, time.Since(tStart).Round(time.Millisecond)),
		Data:    map[string]any{"briefing_id": row.ID},
	})
	return row, nil
}

func (a *Analyzer) emit(cb func(AnalyzeProgress), p AnalyzeProgress) {
	if cb != nil {
		cb(p)
	}
}

// gatherInputs 取窗口内的事件 + 用户的话题。
func (a *Analyzer) gatherInputs(ctx context.Context, userID int64, window string) ([]*db.UserFeedEvent, []*model.Topic, error) {
	limit := 80
	switch window {
	case "1h":
		limit = 40
	case "24h":
		limit = 80
	case "7d":
		limit = 160
	}
	events, err := db.ListUserFeedEvents(ctx, a.db, userID, limit)
	if err != nil {
		return nil, nil, err
	}
	// 客户端过滤：按 window 截掉太旧的
	cutoff := time.Now()
	switch window {
	case "1h":
		cutoff = cutoff.Add(-1 * time.Hour)
	case "24h":
		cutoff = cutoff.Add(-24 * time.Hour)
	case "7d":
		cutoff = cutoff.Add(-7 * 24 * time.Hour)
	}
	filtered := make([]*db.UserFeedEvent, 0, len(events))
	for _, e := range events {
		t := e.Event.FetchedAt
		if e.Event.PublishedAt != nil {
			t = *e.Event.PublishedAt
		}
		if t.Before(cutoff) {
			continue
		}
		filtered = append(filtered, e)
	}
	topics, err := db.ListTopics(ctx, a.db, userID)
	if err != nil {
		return nil, nil, err
	}
	return filtered, topics, nil
}

// ----- 阶段 A：聚类 ----------------------------------------------------------

const clusterSystemPrompt = `你是情报聚类官。
我会给你一批新闻事件，请按"主题 / 议题"维度做聚类，要求：

- 同一议题的事件归到同一个 cluster（不要按话题/source 分簇）
- 每个 cluster 命名一个 6-12 字的中文标签（如：英伟达供应链动态 / 美国对华芯片限制升级）
- 每个 cluster 给一句话主题概述（30 字内）
- 列出该 cluster 包含的事件 id 列表
- cluster 数量上限 8 个；单事件可不归簇（noise）

严格输出 JSON，不要 markdown 包裹：
{
  "clusters": [
    {"label":"...", "summary":"...", "event_ids":[...]},
    ...
  ]
}`

type clusterItem struct {
	Label    string  `json:"label"`
	Summary  string  `json:"summary"`
	EventIDs []int64 `json:"event_ids"`
}

func (a *Analyzer) cluster(ctx context.Context, events []*db.UserFeedEvent, topics []*model.Topic) ([]clusterItem, error) {
	var b strings.Builder
	b.WriteString("用户关心的话题：")
	for i, t := range topics {
		if i > 0 {
			b.WriteString("、")
		}
		b.WriteString(t.Name)
	}
	b.WriteString("\n\n事件列表（id · 标题 · 摘要）：\n")
	for _, e := range events {
		fmt.Fprintf(&b, "id=%d  [%s]  %s\n    %s\n",
			e.Event.ID, e.Source.Name, e.Event.Title,
			truncateUTF8(stripWhitespace(e.Event.Summary), 80))
	}
	resp, err := a.llm.Chat(ctx, &llm.ChatRequest{
		Model: a.model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: clusterSystemPrompt},
			{Role: llm.RoleUser, Content: b.String()},
		},
		ResponseFormat: &llm.ResponseFormat{Type: "json_object"},
	})
	if err != nil {
		return nil, err
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("empty cluster response")
	}
	raw := trimJSONFence(strings.TrimSpace(resp.Choices[0].Message.Content))
	var parsed struct {
		Clusters []clusterItem `json:"clusters"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, fmt.Errorf("decode cluster: %w (raw=%q)", err, truncateString(raw, 200))
	}
	return parsed.Clusters, nil
}

// ----- 阶段 B：跨簇关联挖掘 -------------------------------------------------

const correlateSystemPrompt = `你是高级情报分析师。
基于我给你的「主题簇」清单，请：

1. 找出 3-6 条跨簇的「隐含关联 / 因果链」（即看起来不相关、但存在内在关联的）
2. 找出 1-3 条值得长期跟踪的「演进趋势」
3. 找出 0-2 条容易被忽视但重要的「弱信号」（含信号方向：看涨/看跌/观望）
4. 结合用户话题，提一条具体可执行的「行动建议」

严格输出 JSON，不要 markdown：
{
  "insights": [{"chain":"A→B→C", "reasoning":"为什么"}, ...],
  "trends":   [{"name":"趋势名", "evidence":"证据 1-2 句"}, ...],
  "signals":  [{"signal":"弱信号", "direction":"up|down|watch", "why":"理由"}, ...],
  "actions":  [{"action":"行动建议", "rationale":"理由"}]
}`

type insightItem struct {
	Chain     string `json:"chain"`
	Reasoning string `json:"reasoning"`
}
type trendItem struct {
	Name     string `json:"name"`
	Evidence string `json:"evidence"`
}
type signalItem struct {
	Signal    string `json:"signal"`
	Direction string `json:"direction"`
	Why       string `json:"why"`
}
type actionItem struct {
	Action    string `json:"action"`
	Rationale string `json:"rationale"`
}
type insightSet struct {
	Insights []insightItem `json:"insights"`
	Trends   []trendItem   `json:"trends"`
	Signals  []signalItem  `json:"signals"`
	Actions  []actionItem  `json:"actions"`
}

func (a *Analyzer) correlate(ctx context.Context, clusters []clusterItem, topics []*model.Topic) (*insightSet, error) {
	var b strings.Builder
	b.WriteString("用户关心的话题：")
	for i, t := range topics {
		if i > 0 {
			b.WriteString("、")
		}
		b.WriteString(t.Name)
	}
	b.WriteString("\n\n主题簇清单：\n")
	for i, c := range clusters {
		fmt.Fprintf(&b, "%d. 【%s】%s  （包含 %d 条事件）\n", i+1, c.Label, c.Summary, len(c.EventIDs))
	}
	resp, err := a.llm.Chat(ctx, &llm.ChatRequest{
		Model: a.model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: correlateSystemPrompt},
			{Role: llm.RoleUser, Content: b.String()},
		},
		ResponseFormat: &llm.ResponseFormat{Type: "json_object"},
	})
	if err != nil {
		return nil, err
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("empty correlate response")
	}
	raw := trimJSONFence(strings.TrimSpace(resp.Choices[0].Message.Content))
	var out insightSet
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("decode correlate: %w (raw=%q)", err, truncateString(raw, 200))
	}
	return &out, nil
}

// ----- 阶段 C：综合写 HTML 简报 --------------------------------------------

const writeSystemPrompt = `你是首席编辑，正在为「特工：代号零」用户写每小时情报简报。

要求：
- 标题：6-14 字，要有钩子（如「英伟达供应链异动 + 上海政策连发」）
- 摘要：一句话点睛（25 字内），最有价值的信息
- 正文：严格 HTML，使用以下结构（不要 <html><body>，只输出片段内容）：

<section class="key-events">
  <h3>关键事件</h3>
  <article>
    <h4>事件标题</h4>
    <p class="meta">来源 · 主题簇 · 关键实体</p>
    <p>事件 1-2 句话深度复盘（不要复述原标题）</p>
  </article>
</section>

<section class="trends">
  <h3>演进趋势</h3>
  <ul><li><b>趋势名</b>：证据与方向</li></ul>
</section>

<section class="signals">
  <h3>弱信号</h3>
  <ul><li><span class="dir up|down|watch">↑/↓/●</span> <b>信号</b>：理由</li></ul>
</section>

<section class="insights">
  <h3>隐含关联</h3>
  <ol><li><b>A → B → C</b>：因果链推演</li></ol>
</section>

<section class="actions">
  <h3>给你的建议</h3>
  <ul><li>具体可执行建议 1</li></ul>
</section>

只输出 JSON：{"title":"...", "summary":"...", "html":"..."}`

type briefingContent struct {
	Title   string `json:"title"`
	Summary string `json:"summary"`
	HTML    string `json:"html"`
}

func (a *Analyzer) writeBriefing(ctx context.Context, window string,
	topics []*model.Topic, clusters []clusterItem, ins *insightSet) (*briefingContent, error) {
	var b strings.Builder
	b.WriteString("窗口：")
	b.WriteString(window)
	b.WriteString("\n用户话题：")
	for i, t := range topics {
		if i > 0 {
			b.WriteString("、")
		}
		b.WriteString(t.Name)
	}
	b.WriteString("\n\n主题簇：\n")
	for i, c := range clusters {
		fmt.Fprintf(&b, "%d. 【%s】%s\n", i+1, c.Label, c.Summary)
	}
	b.WriteString("\n隐含关联：\n")
	for i, it := range ins.Insights {
		fmt.Fprintf(&b, "%d. %s — %s\n", i+1, it.Chain, it.Reasoning)
	}
	b.WriteString("\n趋势：\n")
	for i, tr := range ins.Trends {
		fmt.Fprintf(&b, "%d. %s — %s\n", i+1, tr.Name, tr.Evidence)
	}
	b.WriteString("\n弱信号：\n")
	for i, s := range ins.Signals {
		fmt.Fprintf(&b, "%d. [%s] %s — %s\n", i+1, s.Direction, s.Signal, s.Why)
	}
	b.WriteString("\n行动建议候选：\n")
	for i, act := range ins.Actions {
		fmt.Fprintf(&b, "%d. %s — %s\n", i+1, act.Action, act.Rationale)
	}

	resp, err := a.llm.Chat(ctx, &llm.ChatRequest{
		Model: a.model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: writeSystemPrompt},
			{Role: llm.RoleUser, Content: b.String()},
		},
		ResponseFormat: &llm.ResponseFormat{Type: "json_object"},
	})
	if err != nil {
		return nil, err
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("empty briefing response")
	}
	raw := trimJSONFence(strings.TrimSpace(resp.Choices[0].Message.Content))
	var out briefingContent
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("decode briefing: %w (raw=%q)", err, truncateString(raw, 200))
	}
	return &out, nil
}

// persistHTML 把简报包装成完整 HTML 文档落到 briefingsDir。
//
// 路径规则：<dir>/<userID>/<YYYYMMDD>/<unix>.html
func (a *Analyzer) persistHTML(userID int64, brief *briefingContent) (string, error) {
	day := time.Now().Format("20060102")
	subdir := filepath.Join(a.briefingsDir, fmt.Sprintf("%d", userID), day)
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		return "", err
	}
	name := fmt.Sprintf("%d.html", time.Now().Unix())
	path := filepath.Join(subdir, name)
	full := wrapBriefingHTML(brief)
	if err := os.WriteFile(path, []byte(full), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// wrapBriefingHTML 给 LLM 返回的 HTML 片段套上完整文档外壳与样式表。
func wrapBriefingHTML(b *briefingContent) string {
	return `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>` + htmlEscape(b.Title) + `</title>
<style>
:root {
  --ink: #0f1014;
  --carbon: #18191e;
  --pen: #c8cbd1;
  --paper: #f4f1ea;
  --muted: #8c8f97;
  --amber: #d9a23a;
  --redline: #c44a3c;
  --green: #67d17b;
}
* { box-sizing: border-box; }
html, body {
  margin: 0; padding: 0;
  background: var(--ink);
  color: var(--paper);
  font-family: -apple-system, "PingFang SC", "Microsoft YaHei", sans-serif;
  line-height: 1.7;
  font-size: 15px;
}
body { padding: 22px 18px 60px; max-width: 720px; margin: 0 auto; }
h1 { font-size: 22px; letter-spacing: 2px; margin: 0 0 6px; }
h3 { font-size: 13px; letter-spacing: 4px; color: var(--amber);
     border-bottom: 1px solid #2b2c33; padding-bottom: 6px; margin: 28px 0 12px;
     text-transform: uppercase; }
h4 { font-size: 15px; margin: 14px 0 4px; color: var(--paper); }
p, li { color: var(--pen); }
.meta { color: var(--muted); font-size: 11px; letter-spacing: 1px; margin: 0 0 6px; }
section { margin-bottom: 8px; }
article { padding: 10px 0; border-bottom: 1px dashed #2b2c33; }
.summary {
  padding: 14px 16px; border-left: 3px solid var(--redline);
  background: var(--carbon); color: var(--paper);
  font-size: 14px; margin: 12px 0 8px;
}
.dir.up   { color: var(--green); }
.dir.down { color: var(--redline); }
.dir.watch{ color: var(--amber); }
b { color: var(--paper); }
ol, ul { padding-left: 22px; }
li { margin-bottom: 6px; }
.footer { color: var(--muted); font-size: 10px; letter-spacing: 2px;
          margin-top: 28px; text-align: center; }
</style>
</head>
<body>
  <h1>` + htmlEscape(b.Title) + `</h1>
  <div class="summary">` + htmlEscape(b.Summary) + `</div>
  ` + b.HTML + `
  <div class="footer">AGENTZERO · ` + time.Now().Format("2006-01-02 15:04") + `</div>
</body>
</html>`
}

func htmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;")
	return r.Replace(s)
}

func stripWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
