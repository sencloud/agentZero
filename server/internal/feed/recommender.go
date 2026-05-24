package feed

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/agentzero/server/internal/db"
	"github.com/agentzero/server/internal/llm"
	"github.com/agentzero/server/internal/model"
)

// Recommender 用 LLM 看一眼源元数据 + 用户话题，推荐应该启用哪些源。
//
// 设计目标：
//   - 调用一次 LLM 一次性返回 IDs 列表，省 token
//   - 保留用户已启用的源（合并集合），从不主动 disable
//   - 默认上限 ~12 个源，避免一次开太多源造成抓取风暴
type Recommender struct {
	db     *sql.DB
	llm    *llm.Client
	logger *slog.Logger
	model  string
}

func NewRecommender(database *sql.DB, llmClient *llm.Client, logger *slog.Logger, modelID string) *Recommender {
	if modelID == "" {
		modelID = "deepseek-chat"
	}
	return &Recommender{db: database, llm: llmClient, logger: logger, model: modelID}
}

const recommendSystemPrompt = `你是一名情报选源官。
我会给你一份用户关心的「话题」清单，以及一份「候选 RSS 源」清单（含 id / name / category / description）。

请挑选最相关的 RSS 源，返回它们的 id。
要求：
- 严格输出 JSON：{"ids":[...], "reason":"<25 字以内总结理由>"}
- ids 是数组，元素是源 id 的整数，按相关性从高到低排
- 总数不超过 12 个
- 跨多个话题应当覆盖多个 category（科技/AI/财经/国际/体育/文化/科学/健康等），避免单一
- 若用户话题宽泛或无清晰主题，挑科技、综合、国际几个高频源即可
`

type recommendResp struct {
	IDs    []int64 `json:"ids"`
	Reason string  `json:"reason"`
}

// RecommendResult 是一次推荐的产物。
type RecommendResult struct {
	NewlyEnabled []*model.NewsSource // 这次新启用的源
	AlreadyOn    []*model.NewsSource // 推荐里已经是 enabled 的源（保留）
	Reason       string              // LLM 给出的简短理由
}

// Recommend 拉用户所有话题 + 全部 sources，调 LLM 决定启用哪些，并落库。
func (r *Recommender) Recommend(ctx context.Context, userID int64) (*RecommendResult, error) {
	topics, err := db.ListTopics(ctx, r.db, userID)
	if err != nil {
		return nil, fmt.Errorf("list topics: %w", err)
	}
	if len(topics) == 0 {
		return &RecommendResult{Reason: "用户尚无话题，跳过推荐"}, nil
	}

	allSources, err := db.ListNewsSources(ctx, r.db, false)
	if err != nil {
		return nil, fmt.Errorf("list sources: %w", err)
	}
	if len(allSources) == 0 {
		return &RecommendResult{Reason: "源库为空"}, nil
	}

	userPrompt := buildRecommendUserPrompt(topics, allSources)
	req := &llm.ChatRequest{
		Model: r.model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: recommendSystemPrompt},
			{Role: llm.RoleUser, Content: userPrompt},
		},
		ResponseFormat: &llm.ResponseFormat{Type: "json_object"},
	}
	resp, err := r.llm.Chat(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("llm chat: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("empty llm response")
	}
	raw := strings.TrimSpace(resp.Choices[0].Message.Content)
	raw = trimJSONFence(raw)
	var parsed recommendResp
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, fmt.Errorf("decode recommend json: %w (raw=%q)", err, truncateString(raw, 200))
	}

	byID := map[int64]*model.NewsSource{}
	for _, s := range allSources {
		byID[s.ID] = s
	}

	var toEnable []int64
	var newlyEnabled []*model.NewsSource
	var alreadyOn []*model.NewsSource
	for _, id := range parsed.IDs {
		s, ok := byID[id]
		if !ok {
			continue
		}
		if s.Enabled {
			alreadyOn = append(alreadyOn, s)
		} else {
			toEnable = append(toEnable, id)
			s.Enabled = true
			newlyEnabled = append(newlyEnabled, s)
		}
	}
	if len(toEnable) > 0 {
		if err := db.SetSourceEnabled(ctx, r.db, toEnable, true); err != nil {
			return nil, fmt.Errorf("enable sources: %w", err)
		}
	}
	r.logger.Info("feed recommend done",
		"topics", len(topics), "newly_enabled", len(newlyEnabled),
		"already_on", len(alreadyOn), "reason", parsed.Reason)
	return &RecommendResult{
		NewlyEnabled: newlyEnabled,
		AlreadyOn:    alreadyOn,
		Reason:       parsed.Reason,
	}, nil
}

func buildRecommendUserPrompt(topics []*model.Topic, sources []*model.NewsSource) string {
	var b strings.Builder
	b.WriteString("【用户关心的话题】\n")
	for _, t := range topics {
		b.WriteString("- ")
		b.WriteString(t.Name)
		b.WriteString("\n")
	}
	b.WriteString("\n【候选 RSS 源】\n")
	for _, s := range sources {
		fmt.Fprintf(&b, "id=%d  name=%s  category=%s  status=%s\n  desc: %s\n",
			s.ID, s.Name, s.Category, enabledLabel(s.Enabled), s.Description)
	}
	b.WriteString("\n请按系统提示挑选合适的源 id 列表。")
	return b.String()
}

func enabledLabel(b bool) string {
	if b {
		return "已启用"
	}
	return "未启用"
}
