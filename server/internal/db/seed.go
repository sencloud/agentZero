package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type seedAgent struct {
	Slug         string
	Name         string
	Tagline      string
	Description  string
	CategorySlug string
	Developer    string
	Version      string
	SizeMB       int64
	Rating       float64
	RatingCount  int64
	InstallCount int64
	IsFeatured   bool
	FeatureBadge string
	Capabilities []string
	UpdatedNotes string
	Color        string
	Icon         string
}

type seedCategory struct {
	Slug, Name, Icon, Color string
}

type seedTodayCard struct {
	Kind, Eyebrow, Title, Subtitle, Cover, AgentSlug string
	Sort                                             int
}

var seedCategories = []seedCategory{
	{"productivity", "效率", "bolt.fill", "#FF9F0A"},
	{"creativity", "创作", "paintpalette.fill", "#FF375F"},
	{"developer", "开发", "chevron.left.forwardslash.chevron.right", "#0A84FF"},
	{"learning", "学习", "books.vertical.fill", "#30D158"},
	{"lifestyle", "生活", "leaf.fill", "#64D2FF"},
	{"entertainment", "娱乐", "gamecontroller.fill", "#BF5AF2"},
}

var seedAgents = []seedAgent{
	{
		Slug: "zero-write", Name: "Zero Write", Tagline: "把灵感写成完整的文章",
		Description: "Zero Write 是一位常驻你身边的写作伙伴。无论是公众号长文、产品介绍还是邮件草稿，它都能根据你的几行提纲，扩展成有结构、有节奏、有情绪的内容。\n\n• 支持中英文混排，自动调整语气\n• 一键生成多个开头/结尾候选\n• 内置抄写体、口语体、商务体等 12 种风格",
		CategorySlug: "productivity", Developer: "AgentZero Labs", Version: "1.4.2", SizeMB: 38,
		Rating: 4.8, RatingCount: 2134, InstallCount: 86200, IsFeatured: true, FeatureBadge: "本周推荐",
		Capabilities: []string{"文章扩写", "标题生成", "语气改写", "排版建议"},
		UpdatedNotes: "新增「商务体」与「小红书体」两种风格；修复了长文中段落断行偶尔丢失的问题。",
		Color:        "#FF9F0A", Icon: "pencil.tip.crop.circle.badge.plus",
	},
	{
		Slug: "code-pilot", Name: "CodePilot", Tagline: "你的私人结对编程师",
		Description: "把粗糙的需求描述粘进来，CodePilot 会先帮你拆解任务、列出方案权衡，再给出可直接运行的代码。支持 12 种主流语言，重点优化 Go、Python、TypeScript。",
		CategorySlug: "developer", Developer: "AgentZero Labs", Version: "2.0.1", SizeMB: 64,
		Rating: 4.9, RatingCount: 5821, InstallCount: 152300, IsFeatured: true, FeatureBadge: "编辑精选",
		Capabilities: []string{"需求拆解", "代码生成", "Bug 定位", "Code Review"},
		UpdatedNotes: "全新「Plan → Code → Verify」三段式对话模板。",
		Color:        "#0A84FF", Icon: "chevron.left.forwardslash.chevron.right",
	},
	{
		Slug: "moodbook", Name: "心情手账", Tagline: "每天 3 分钟，整理一天的情绪",
		Description: "通过简单的对话，帮你把今天的烦恼、欣喜、焦虑梳理成手账卡片，自动归类到对应的情绪标签，并生成可复盘的周报。",
		CategorySlug: "lifestyle", Developer: "暖光工作室", Version: "1.1.0", SizeMB: 22,
		Rating: 4.7, RatingCount: 982, InstallCount: 31400, IsFeatured: true, FeatureBadge: "新上线",
		Capabilities: []string{"情绪记录", "周报生成", "正念引导"},
		UpdatedNotes: "新增正念呼吸引导音频。",
		Color:        "#64D2FF", Icon: "heart.text.square.fill",
	},
	{
		Slug: "studybuddy", Name: "学伴 StudyBuddy", Tagline: "从一道题，到一整章",
		Description: "拍照或粘贴题目，学伴会一步步引导你思考，先讲解原理，再给出标准解法。支持中学到大学常见学科。",
		CategorySlug: "learning", Developer: "Lantern Edu", Version: "3.2.0", SizeMB: 48,
		Rating: 4.6, RatingCount: 12034, InstallCount: 412300,
		Capabilities: []string{"题目讲解", "知识点串联", "错题本"},
		UpdatedNotes: "新增高中物理力学专题。",
		Color:        "#30D158", Icon: "graduationcap.fill",
	},
	{
		Slug: "designer-muse", Name: "Designer Muse", Tagline: "设计师的灵感发动机",
		Description: "输入一个主题或一张参考图，Muse 会生成配色方案、字体组合、版式建议，并给出 Figma 可粘贴的设计 Token。",
		CategorySlug: "creativity", Developer: "Prism Studio", Version: "1.5.6", SizeMB: 55,
		Rating: 4.7, RatingCount: 3120, InstallCount: 92100, IsFeatured: true, FeatureBadge: "今日发现",
		Capabilities: []string{"配色生成", "版式建议", "Design Token"},
		UpdatedNotes: "支持导出到 Figma Variables。",
		Color:        "#FF375F", Icon: "paintpalette.fill",
	},
	{
		Slug: "shellsage", Name: "ShellSage", Tagline: "说人话，敲命令",
		Description: "把「我想把这个文件夹里所有大于 100MB 的视频压成一半码率」这样的需求翻译成正确的 shell 命令，并解释每一段含义。",
		CategorySlug: "developer", Developer: "AgentZero Labs", Version: "1.0.3", SizeMB: 18,
		Rating: 4.5, RatingCount: 1542, InstallCount: 44200,
		Capabilities: []string{"自然语言转命令", "命令解释", "危险操作提示"},
		UpdatedNotes: "支持 fish/zsh 别名识别。",
		Color:        "#5E5CE6", Icon: "terminal.fill",
	},
	{
		Slug: "polyglot", Name: "Polyglot", Tagline: "随身翻译与口语教练",
		Description: "Polyglot 不止翻译，还会指出你句子里的语病，给出母语者会用的表达，并陪你做角色扮演练习。",
		CategorySlug: "learning", Developer: "Lantern Edu", Version: "2.4.0", SizeMB: 35,
		Rating: 4.8, RatingCount: 7820, InstallCount: 220100,
		Capabilities: []string{"中英互译", "口语纠错", "情景对话"},
		UpdatedNotes: "新增「机场出行」「医院问诊」两个情景。",
		Color:        "#FF9F0A", Icon: "globe.asia.australia.fill",
	},
	{
		Slug: "chef-zero", Name: "Chef Zero", Tagline: "看冰箱有什么，决定今天吃什么",
		Description: "把冰箱里的食材列给 Chef Zero，它会给出 3 套时间≤30 分钟的菜谱，并自动整理购物清单。",
		CategorySlug: "lifestyle", Developer: "Bistro AI", Version: "1.2.1", SizeMB: 28,
		Rating: 4.6, RatingCount: 2103, InstallCount: 64200,
		Capabilities: []string{"食材识别", "菜谱推荐", "购物清单"},
		UpdatedNotes: "新增「低 GI 饮食」筛选。",
		Color:        "#FF6B35", Icon: "fork.knife",
	},
	{
		Slug: "dungeon", Name: "迷宫物语", Tagline: "口袋里的文字 RPG",
		Description: "一个由 AI 主持的开放世界文字冒险。你的每一个选择都会真正改变剧情走向。",
		CategorySlug: "entertainment", Developer: "Lantern Game", Version: "0.9.5", SizeMB: 72,
		Rating: 4.4, RatingCount: 921, InstallCount: 18900,
		Capabilities: []string{"剧情生成", "角色养成", "存档分支"},
		UpdatedNotes: "新章节「群星之下」上线。",
		Color:        "#BF5AF2", Icon: "scroll.fill",
	},
	{
		Slug: "meeting-mind", Name: "Meeting Mind", Tagline: "把一小时会议变成一页纪要",
		Description: "上传会议录音或文字记录，自动生成结构化纪要：决策项、待办、风险、争议焦点。",
		CategorySlug: "productivity", Developer: "AgentZero Labs", Version: "1.3.0", SizeMB: 42,
		Rating: 4.7, RatingCount: 1820, InstallCount: 58200,
		Capabilities: []string{"语音转写", "会议纪要", "待办抽取"},
		UpdatedNotes: "新增飞书/钉钉/Notion 导出。",
		Color:        "#30D158", Icon: "doc.text.magnifyingglass",
	},
	{
		Slug: "story-painter", Name: "故事绘师", Tagline: "把睡前故事画出来",
		Description: "为孩子量身定制睡前故事，并配以柔和的插画。可选择以孩子的名字作为主角。",
		CategorySlug: "creativity", Developer: "暖光工作室", Version: "1.0.4", SizeMB: 52,
		Rating: 4.9, RatingCount: 4501, InstallCount: 88100,
		Capabilities: []string{"故事生成", "插画生成", "亲子配音"},
		UpdatedNotes: "新增「水墨画」绘画风格。",
		Color:        "#FF375F", Icon: "book.fill",
	},
	{
		Slug: "fit-coach", Name: "Fit Coach", Tagline: "在家也有的私教",
		Description: "根据你的身体状况和目标，生成每周训练计划。每个动作都有 3D 演示与口令引导。",
		CategorySlug: "lifestyle", Developer: "Pulse Health", Version: "2.1.0", SizeMB: 88,
		Rating: 4.5, RatingCount: 3240, InstallCount: 102300,
		Capabilities: []string{"训练计划", "动作示范", "饮食建议"},
		UpdatedNotes: "新增「办公室碎片训练」5 分钟系列。",
		Color:        "#FF6B35", Icon: "figure.run",
	},
}

var seedToday = []seedTodayCard{
	{Kind: "spotlight", Eyebrow: "今日精选", Title: "把灵感写成完整的文章", Subtitle: "Zero Write · 写作助手", AgentSlug: "zero-write", Sort: 1},
	{Kind: "feature", Eyebrow: "编辑精选", Title: "为开发者准备的结对伙伴", Subtitle: "CodePilot 2.0 — 全新三段式对话", AgentSlug: "code-pilot", Sort: 2},
	{Kind: "story", Eyebrow: "幕后故事", Title: "暖光工作室：让 AI 更柔软", Subtitle: "心情手账与故事绘师背后的小团队", AgentSlug: "moodbook", Sort: 3},
	{Kind: "feature", Eyebrow: "今日发现", Title: "设计师的灵感发动机", Subtitle: "Designer Muse · 一键导出 Figma Token", AgentSlug: "designer-muse", Sort: 4},
	{Kind: "story", Eyebrow: "新上线", Title: "把睡前故事画出来", Subtitle: "故事绘师 · 让晚安多一点温度", AgentSlug: "story-painter", Sort: 5},
}

func iconURL(color, name string) string {
	return fmt.Sprintf("ag-icon://%s/%s", color[1:], name)
}

func coverURL(slug string) string {
	return fmt.Sprintf("https://picsum.photos/seed/agentzero-cover-%s/1200/900", slug)
}

func screenshotURL(slug string, i int) string {
	return fmt.Sprintf("https://picsum.photos/seed/agentzero-%s-%d/900/1900", slug, i)
}

func SeedIfEmpty(db *sql.DB) error {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM agents`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	catID := map[string]int64{}
	for _, c := range seedCategories {
		res, err := tx.Exec(`INSERT INTO categories(slug,name,icon,color) VALUES(?,?,?,?)`, c.Slug, c.Name, c.Icon, c.Color)
		if err != nil {
			return err
		}
		id, _ := res.LastInsertId()
		catID[c.Slug] = id
	}

	agentID := map[string]int64{}
	now := time.Now()
	for i, a := range seedAgents {
		shots := []string{
			screenshotURL(a.Slug, 1),
			screenshotURL(a.Slug, 2),
			screenshotURL(a.Slug, 3),
		}
		shotsJSON, _ := json.Marshal(shots)
		capsJSON, _ := json.Marshal(a.Capabilities)
		featured := 0
		if a.IsFeatured {
			featured = 1
		}
		released := now.Add(-time.Duration(180-i*7) * 24 * time.Hour)
		updated := now.Add(-time.Duration(i*3) * 24 * time.Hour)
		res, err := tx.Exec(`INSERT INTO agents(slug,name,tagline,description,icon_url,cover_url,screenshots,category_id,developer,version,size_bytes,rating,rating_count,install_count,is_free,price_cents,is_featured,feature_badge,capabilities,updated_notes,released_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			a.Slug, a.Name, a.Tagline, a.Description,
			iconURL(a.Color, a.Icon), coverURL(a.Slug), string(shotsJSON),
			catID[a.CategorySlug], a.Developer, a.Version, a.SizeMB*1024*1024,
			a.Rating, a.RatingCount, a.InstallCount,
			1, 0, featured, a.FeatureBadge, string(capsJSON), a.UpdatedNotes,
			released, updated,
		)
		if err != nil {
			return err
		}
		id, _ := res.LastInsertId()
		agentID[a.Slug] = id
	}

	for _, t := range seedToday {
		aid := agentID[t.AgentSlug]
		_, err := tx.Exec(`INSERT INTO today_cards(kind,eyebrow,title,subtitle,cover_url,agent_id,sort_order) VALUES(?,?,?,?,?,?,?)`,
			t.Kind, t.Eyebrow, t.Title, t.Subtitle, coverURL(t.AgentSlug+"-today"), aid, t.Sort)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}
