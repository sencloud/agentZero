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

// Extractor 用 LLM 把一条事件抽取成 entities + relations，
// 写到 entities / event_entities / entity_relations。
//
// 设计目标：尽可能省 token，所以：
//   - 只对至少匹配上一个用户 topic 的事件做抽取
//   - 用 deepseek-flash 不思考模式
//   - 一次只塞 title + summary（不上 content 全文）
//   - 输出强制 JSON 结构
type Extractor struct {
	db     *sql.DB
	llm    *llm.Client
	logger *slog.Logger
	model  string
}

func NewExtractor(database *sql.DB, llmClient *llm.Client, logger *slog.Logger, modelID string) *Extractor {
	if modelID == "" {
		modelID = "deepseek-chat" // 占位：实际 v4 flash 的 model id 由配置注入
	}
	return &Extractor{db: database, llm: llmClient, logger: logger, model: modelID}
}

const extractSystemPrompt = `你是一个新闻图谱抽取器。读完用户给的标题和摘要后，
抽出最关键的实体和它们之间的两两关系，只输出严格的 JSON，不带任何解释或 markdown。

要求：
- 实体至多 8 个；类型只能从 person / org / place / concept / event 中选一个
- 实体 name 用最简洁的官方名（公司用全称或常用简称，人物用全名）
- 关系尽量给一个 1-6 字的中文动词或名词短语（例如：表态、合作、收购、提议、关注、出席）
- 关系数量 = 实体数量的 0.5~2 倍，不要全连
- salience: 主角实体 1.0，配角 0.4 ~ 0.8
- weight: 关系强度 0.3 ~ 1.0

输出 JSON 结构：
{
  "entities": [{"name": "...", "type": "...", "salience": 0.9}, ...],
  "relations": [{"src": "...", "dst": "...", "label": "...", "weight": 0.8}, ...]
}
`

type extractResponse struct {
	Entities []struct {
		Name     string  `json:"name"`
		Type     string  `json:"type"`
		Salience float64 `json:"salience"`
	} `json:"entities"`
	Relations []struct {
		Src    string  `json:"src"`
		Dst    string  `json:"dst"`
		Label  string  `json:"label"`
		Weight float64 `json:"weight"`
	} `json:"relations"`
}

// ExtractOne 对一条事件做抽取，结果写库 + 标记 extracted。
func (x *Extractor) ExtractOne(ctx context.Context, e *model.NewsEvent) error {
	userText := fmt.Sprintf("【标题】%s\n【摘要】%s", e.Title, e.Summary)
	req := &llm.ChatRequest{
		Model: x.model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: extractSystemPrompt},
			{Role: llm.RoleUser, Content: userText},
		},
		ResponseFormat: &llm.ResponseFormat{Type: "json_object"},
	}
	resp, err := x.llm.Chat(ctx, req)
	if err != nil {
		return fmt.Errorf("llm chat: %w", err)
	}
	if len(resp.Choices) == 0 {
		return fmt.Errorf("empty llm response")
	}
	raw := strings.TrimSpace(resp.Choices[0].Message.Content)
	raw = trimJSONFence(raw)

	var parsed extractResponse
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return fmt.Errorf("decode extract: %w (raw=%q)", err, truncateString(raw, 200))
	}

	// 入库：先 upsert entity，记录 name → id
	nameToID := map[string]int64{}
	for _, en := range parsed.Entities {
		name := strings.TrimSpace(en.Name)
		typ := normalizeEntityType(en.Type)
		if name == "" || typ == "" {
			continue
		}
		ent := &model.Entity{Type: typ, Name: name, Weight: clampPos(en.Salience, 1.0)}
		if err := db.UpsertEntity(ctx, x.db, ent); err != nil {
			x.logger.Warn("upsert entity failed", "name", name, "err", err)
			continue
		}
		nameToID[name] = ent.ID
		// event ↔ entity
		if err := db.LinkEventEntity(ctx, x.db, e.ID, ent.ID, clampPos(en.Salience, 1.0)); err != nil {
			x.logger.Warn("link event_entity failed", "name", name, "err", err)
		}
	}

	for _, rel := range parsed.Relations {
		srcID, ok1 := nameToID[strings.TrimSpace(rel.Src)]
		dstID, ok2 := nameToID[strings.TrimSpace(rel.Dst)]
		if !ok1 || !ok2 || srcID == dstID {
			continue
		}
		r := &model.EntityRelation{
			SrcID:  srcID,
			DstID:  dstID,
			Label:  strings.TrimSpace(rel.Label),
			Weight: clampPos(rel.Weight, 1.0),
		}
		if err := db.UpsertRelation(ctx, x.db, r); err != nil {
			x.logger.Warn("upsert relation failed", "src", rel.Src, "dst", rel.Dst, "err", err)
		}
	}

	return db.MarkEventExtracted(ctx, x.db, e.ID)
}

// ExtractProgress 是 ExtractBatchWithProgress 的进度回调入参。
type ExtractProgress struct {
	Index int
	Total int
	Event *model.NewsEvent
	Err   error
}

// ExtractBatch 对一批未抽取的事件依次抽取，限定上限。
// 优先抽取「被任意用户关心」的事件，省 token。
func (x *Extractor) ExtractBatch(ctx context.Context, maxEvents int) (int, error) {
	return x.ExtractBatchWithProgress(ctx, maxEvents, nil)
}

// ExtractBatchWithProgress 在每一条抽取完成后调一次 onProgress。
func (x *Extractor) ExtractBatchWithProgress(
	ctx context.Context,
	maxEvents int,
	onProgress func(p ExtractProgress) bool,
) (int, error) {
	if maxEvents <= 0 {
		maxEvents = 10
	}
	rows, err := x.db.QueryContext(ctx, `
		SELECT e.id, e.source_id, e.url, e.title, e.summary, e.content, e.lang
		FROM news_events e
		WHERE e.extracted = 0
		  AND EXISTS (SELECT 1 FROM user_event_subs u WHERE u.event_id = e.id)
		ORDER BY COALESCE(e.published_at, e.fetched_at) DESC
		LIMIT ?
	`, maxEvents)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var batch []*model.NewsEvent
	for rows.Next() {
		var e model.NewsEvent
		if err := rows.Scan(&e.ID, &e.SourceID, &e.URL, &e.Title, &e.Summary, &e.Content, &e.Lang); err != nil {
			return 0, err
		}
		batch = append(batch, &e)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	done := 0
	total := len(batch)
	for i, e := range batch {
		if err := ctx.Err(); err != nil {
			return done, err
		}
		extractErr := x.ExtractOne(ctx, e)
		if extractErr != nil {
			x.logger.Warn("extract failed", "ev", e.ID, "err", extractErr)
		} else {
			done++
		}
		if onProgress != nil {
			if !onProgress(ExtractProgress{Index: i + 1, Total: total, Event: e, Err: extractErr}) {
				return done, nil
			}
		}
	}
	return done, nil
}

func normalizeEntityType(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "person", "org", "place", "concept", "event":
		return s
	case "people":
		return "person"
	case "organization", "company":
		return "org"
	case "location", "city", "country":
		return "place"
	default:
		return "concept"
	}
}

func clampPos(v, def float64) float64 {
	if v <= 0 {
		return def
	}
	if v > 1.5 {
		return 1.5
	}
	return v
}

func trimJSONFence(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimSuffix(s, "```")
	}
	return strings.TrimSpace(s)
}

func truncateString(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
