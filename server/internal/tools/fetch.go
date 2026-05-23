package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// FetchURL 是「网页抓取」装备。
//
// 目标：给一个 URL，返回**清洗过的纯文本**给模型阅读，避免把整个 HTML 灌给模型。
// 当前实现做最朴素的去标签 + 去脚本/样式 + 折行处理，够用即可；
// 后续可以替换为 goquery + readability 这种更专业的方案。
type FetchURL struct {
	HTTPClient *http.Client
	UserAgent  string
}

func NewFetchURL() *FetchURL {
	return &FetchURL{
		HTTPClient: &http.Client{Timeout: 20 * time.Second},
		UserAgent:  "AgentZero/1.0 (+https://agentzero.me)",
	}
}

func (*FetchURL) Name() string        { return "fetch_url" }
func (*FetchURL) DisplayName() string { return "网页抓取" }
func (*FetchURL) Description() string {
	return "抓取一个网页并清洗成纯文本（去除 HTML 标签、脚本、样式）。用于读取参考资料、调研网页内容。最多返回约 12KB 文本；超长会被截断。"
}

var fetchURLSchema = json.RawMessage(`{
  "type": "object",
  "properties": {
    "url": {"type":"string","description":"完整的 http/https URL"},
    "max_chars": {"type":"integer","description":"返回的最大字符数，默认 12000","default":12000}
  },
  "required": ["url"]
}`)

func (*FetchURL) Parameters() json.RawMessage { return fetchURLSchema }

func (f *FetchURL) Run(ctx context.Context, raw json.RawMessage, _ *Env) (*Result, error) {
	var args struct {
		URL      string `json:"url"`
		MaxChars int    `json:"max_chars"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %w", err)
	}
	if args.MaxChars <= 0 || args.MaxChars > 200000 {
		args.MaxChars = 12000
	}
	u, err := url.Parse(args.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("invalid url: must be http(s)")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, args.URL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", f.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,*/*;q=0.8")

	resp, err := f.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20)) // 最多 2MB
	if err != nil {
		return nil, err
	}

	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	var text string
	switch {
	case strings.Contains(ct, "text/html") || strings.Contains(ct, "application/xhtml"):
		text = htmlToText(string(body))
	case strings.HasPrefix(ct, "text/"), strings.Contains(ct, "json"), strings.Contains(ct, "xml"):
		text = string(body)
	default:
		return nil, fmt.Errorf("unsupported content-type: %s", ct)
	}

	text = strings.TrimSpace(text)
	header := fmt.Sprintf("URL: %s\nFinal-URL: %s\nContent-Type: %s\n\n", args.URL, resp.Request.URL.String(), ct)
	if len(text) > args.MaxChars {
		text = text[:args.MaxChars] + "\n\n…（已截断，原文更长。可指定更大的 max_chars 再次抓取）"
	}
	return &Result{Content: header + text}, nil
}

// htmlToText 把 HTML 转成"够给模型看"的纯文本。
//
// 没用任何外部库；只对 <script>/<style>/<noscript> 整段剔除，
// 把 <br>/<p>/<div> 之类块级标签换行，剩余标签直接删。
// 文档密度高的页面（新闻、博客、文档）效果不错；SPA 类页面拿不到正文。
func htmlToText(html string) string {
	html = reBlockClear.ReplaceAllString(html, "")
	html = reBlockBreak.ReplaceAllString(html, "\n")
	html = reTag.ReplaceAllString(html, "")
	html = strings.ReplaceAll(html, "&nbsp;", " ")
	html = strings.ReplaceAll(html, "&amp;", "&")
	html = strings.ReplaceAll(html, "&lt;", "<")
	html = strings.ReplaceAll(html, "&gt;", ">")
	html = strings.ReplaceAll(html, "&quot;", "\"")
	html = strings.ReplaceAll(html, "&#39;", "'")
	html = reBlankLines.ReplaceAllString(html, "\n\n")
	html = reTrailSpace.ReplaceAllString(html, " ")
	return html
}

var (
	// RE2 不支持反向引用，所以三种"段落式擦除"标签各写一条。
	reBlockClear = regexp.MustCompile(`(?is)<script[^>]*>.*?</\s*script\s*>|<style[^>]*>.*?</\s*style\s*>|<noscript[^>]*>.*?</\s*noscript\s*>`)
	reBlockBreak = regexp.MustCompile(`(?i)<\s*(br|/p|/div|/li|/h[1-6]|/section|/article|/header|/footer)\s*[^>]*>`)
	reTag        = regexp.MustCompile(`<[^>]+>`)
	reBlankLines = regexp.MustCompile(`\n\s*\n\s*\n+`)
	reTrailSpace = regexp.MustCompile(`[ \t]+`)
)
