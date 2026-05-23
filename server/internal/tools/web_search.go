package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// WebSearch 是基于博查 (https://open.bochaai.com) 的中文友好网络搜索装备。
//
// API: POST https://api.bochaai.com/v1/web-search
// 响应格式兼容 Bing Search API（webPages.value[]）。
type WebSearch struct {
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
}

// NewWebSearch 构造装备。apiKey 为空时装备仍然会被 Register，但 Run 时返回明确错误。
func NewWebSearch(apiKey string) *WebSearch {
	return &WebSearch{
		APIKey:     apiKey,
		BaseURL:    "https://api.bochaai.com/v1/web-search",
		HTTPClient: &http.Client{Timeout: 20 * time.Second},
	}
}

func (*WebSearch) Name() string        { return "web_search" }
func (*WebSearch) DisplayName() string { return "情报检索" }
func (*WebSearch) Description() string {
	return "在中文友好的全网搜索引擎里检索关键词，返回前 N 条网页结果（标题/URL/摘要/发布时间）。适合调研、查找资料、核实信息。后续可对感兴趣的 URL 用 fetch_url 抓全文。"
}

var webSearchSchema = json.RawMessage(`{
  "type": "object",
  "properties": {
    "query": {"type":"string","description":"搜索关键词，直接用自然语言即可"},
    "count": {"type":"integer","description":"返回结果条数 1-20，默认 8","default":8,"minimum":1,"maximum":20},
    "freshness": {"type":"string","enum":["noLimit","oneDay","oneWeek","oneMonth","oneYear"],"default":"noLimit","description":"时效范围；推荐 noLimit，让搜索引擎自行决定。"}
  },
  "required": ["query"]
}`)

func (*WebSearch) Parameters() json.RawMessage { return webSearchSchema }

type bochaReq struct {
	Query     string `json:"query"`
	Freshness string `json:"freshness,omitempty"`
	Summary   bool   `json:"summary"`
	Count     int    `json:"count"`
}

type bochaWebPage struct {
	Name          string `json:"name"`
	URL           string `json:"url"`
	SiteName      string `json:"siteName"`
	Snippet       string `json:"snippet"`
	Summary       string `json:"summary"`
	DatePublished string `json:"datePublished"`
}

type bochaResp struct {
	Code int `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		WebPages struct {
			Value []bochaWebPage `json:"value"`
		} `json:"webPages"`
	} `json:"data"`
}

func (w *WebSearch) Run(ctx context.Context, raw json.RawMessage, _ *Env) (*Result, error) {
	if w.APIKey == "" {
		return nil, fmt.Errorf("BOCHA_API_KEY 未配置，情报检索装备不可用")
	}
	var args struct {
		Query     string `json:"query"`
		Count     int    `json:"count"`
		Freshness string `json:"freshness"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %w", err)
	}
	args.Query = strings.TrimSpace(args.Query)
	if args.Query == "" {
		return nil, fmt.Errorf("query is required")
	}
	if args.Count <= 0 || args.Count > 20 {
		args.Count = 8
	}
	if args.Freshness == "" {
		args.Freshness = "noLimit"
	}

	body, _ := json.Marshal(bochaReq{
		Query:     args.Query,
		Freshness: args.Freshness,
		Summary:   true,
		Count:     args.Count,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.BaseURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+w.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bocha: %w", err)
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bocha http %d: %s", resp.StatusCode, truncateAt(string(bodyBytes), 300))
	}
	var br bochaResp
	if err := json.Unmarshal(bodyBytes, &br); err != nil {
		return nil, fmt.Errorf("decode bocha resp: %w", err)
	}
	if br.Code != 0 && br.Code != 200 {
		return nil, fmt.Errorf("bocha code=%d msg=%s", br.Code, br.Msg)
	}
	pages := br.Data.WebPages.Value
	if len(pages) == 0 {
		return &Result{Content: fmt.Sprintf("「%s」未检索到结果。可以改写关键词或放宽时效范围。", args.Query)}, nil
	}
	return &Result{Content: formatSearchResults(args.Query, pages)}, nil
}

func formatSearchResults(query string, pages []bochaWebPage) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "「%s」共找到 %d 条候选：\n\n", query, len(pages))
	for i, p := range pages {
		fmt.Fprintf(&sb, "[%d] %s\n", i+1, strings.TrimSpace(p.Name))
		fmt.Fprintf(&sb, "    URL: %s\n", p.URL)
		if p.SiteName != "" || p.DatePublished != "" {
			fmt.Fprintf(&sb, "    来源: %s  日期: %s\n", p.SiteName, p.DatePublished)
		}
		summary := strings.TrimSpace(p.Summary)
		if summary == "" {
			summary = strings.TrimSpace(p.Snippet)
		}
		if summary != "" {
			fmt.Fprintf(&sb, "    摘要: %s\n", truncateAt(summary, 400))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("如需查看某条的完整内容，调用 fetch_url 抓取对应 URL。")
	return sb.String()
}

func truncateAt(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
