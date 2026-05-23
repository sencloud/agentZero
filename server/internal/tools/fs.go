package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// safeJoin 把用户给的相对路径拼到 mission workspace 内，禁止越权。
//
// 规则：
//   - 不接受绝对路径
//   - 拼接后必须仍在 workspaceDir 之下（防 ../../ 逃逸）
//   - workspaceDir 必须事先存在且是绝对路径
func safeJoin(workspaceDir, rel string) (string, error) {
	if !filepath.IsAbs(workspaceDir) {
		return "", fmt.Errorf("workspace dir must be absolute: %s", workspaceDir)
	}
	if rel == "" {
		return "", fmt.Errorf("path required")
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("absolute path not allowed: %s", rel)
	}
	cleaned := filepath.Clean(rel)
	if strings.HasPrefix(cleaned, "..") || strings.Contains(cleaned, string(filepath.Separator)+".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes workspace: %s", rel)
	}
	full := filepath.Join(workspaceDir, cleaned)
	if !strings.HasPrefix(full, workspaceDir+string(filepath.Separator)) && full != workspaceDir {
		return "", fmt.Errorf("path escapes workspace: %s", rel)
	}
	return full, nil
}

// ---- write_file ----

type WriteFile struct{}

func (WriteFile) Name() string        { return "write_file" }
func (WriteFile) DisplayName() string { return "笔录" }
func (WriteFile) Description() string {
	return "把一段文本写入任务工作区里的文件（相对路径）。常用于把搜集到的资料整理成报告、笔记、代码草稿等并入柜。"
}

var writeFileSchema = json.RawMessage(`{
  "type": "object",
  "properties": {
    "path": {"type":"string","description":"工作区内的相对路径，例如 'report.md' 或 'notes/draft.md'"},
    "content": {"type":"string","description":"要写入的完整文本内容（UTF-8）"},
    "kind": {"type":"string","enum":["file","code","chart","url","image"],"default":"file","description":"工件类别，用于在工件柜里分组展示"},
    "mime": {"type":"string","description":"MIME 类型，例如 'text/markdown'，留空则按扩展名推断"}
  },
  "required": ["path","content"]
}`)

func (WriteFile) Parameters() json.RawMessage { return writeFileSchema }

func (WriteFile) Run(_ context.Context, raw json.RawMessage, env *Env) (*Result, error) {
	var args struct {
		Path    string `json:"path"`
		Content string `json:"content"`
		Kind    string `json:"kind"`
		Mime    string `json:"mime"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %w", err)
	}
	full, err := safeJoin(env.WorkspaceDir, args.Path)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}
	if err := os.WriteFile(full, []byte(args.Content), 0o644); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}
	info, err := os.Stat(full)
	if err != nil {
		return nil, err
	}
	mime := args.Mime
	if mime == "" {
		mime = guessMime(args.Path)
	}
	kind := args.Kind
	if kind == "" {
		kind = "file"
	}
	rel, _ := filepath.Rel(env.WorkspaceDir, full)
	return &Result{
		Content: fmt.Sprintf("已写入工件：%s（%d 字节）", rel, info.Size()),
		Artifact: &ArtifactSpec{
			Kind: kind,
			Name: filepath.Base(rel),
			Path: rel,
			Mime: mime,
			Size: info.Size(),
		},
	}, nil
}

// ---- read_file ----

type ReadFile struct{}

func (ReadFile) Name() string        { return "read_file" }
func (ReadFile) DisplayName() string { return "调阅" }
func (ReadFile) Description() string {
	return "读取任务工作区里之前写入的文件内容。常用于回看自己刚整理的资料、读取之前的报告草稿。"
}

var readFileSchema = json.RawMessage(`{
  "type": "object",
  "properties": {
    "path": {"type":"string","description":"工作区内的相对路径"},
    "max_bytes": {"type":"integer","description":"最多读取的字节数，默认 65536","default":65536}
  },
  "required": ["path"]
}`)

func (ReadFile) Parameters() json.RawMessage { return readFileSchema }

func (ReadFile) Run(_ context.Context, raw json.RawMessage, env *Env) (*Result, error) {
	var args struct {
		Path     string `json:"path"`
		MaxBytes int64  `json:"max_bytes"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %w", err)
	}
	if args.MaxBytes <= 0 || args.MaxBytes > 1<<20 {
		args.MaxBytes = 65536
	}
	full, err := safeJoin(env.WorkspaceDir, args.Path)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(full)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	defer f.Close()
	buf := make([]byte, args.MaxBytes+1)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, fmt.Errorf("read: %w", err)
	}
	truncated := int64(n) > args.MaxBytes
	if truncated {
		n = int(args.MaxBytes)
	}
	content := string(buf[:n])
	if truncated {
		content += "\n\n…（文件被截断，请使用更小的 max_bytes 或按需要再次调用）"
	}
	return &Result{Content: content}, nil
}

func guessMime(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md", ".markdown":
		return "text/markdown"
	case ".txt":
		return "text/plain"
	case ".html", ".htm":
		return "text/html"
	case ".json":
		return "application/json"
	case ".csv":
		return "text/csv"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".pdf":
		return "application/pdf"
	}
	return "application/octet-stream"
}
