package feed

import (
	"context"
	"database/sql"

	"github.com/agentzero/server/internal/db"
	"github.com/agentzero/server/internal/model"
)

// SeedSources 把内置的 RSS 源库 upsert 到 news_sources。
//
// 选源原则：
//   - 中国境内服务器尽量直连
//   - 中文为主，国际事件优先用中文化媒体
//   - 标 category，给 LLM 做智能选源用
//
// upsert 不会覆盖 enabled 字段；只有 4 个稳源默认 enabled=true，其他源进库后
// 由「LLM 推荐」或「用户手动勾选」启用。
func SeedSources(ctx context.Context, database *sql.DB) error {
	seeds := []model.NewsSource{
		// ===== 科技综合 =====
		{Name: "36 氪", URL: "https://36kr.com/feed", Category: "tech",
			Description: "中文科技创投媒体，覆盖创业公司、融资、产品",
			Region:      "cn", Lang: "zh", Kind: "rss", Enabled: true},
		{Name: "少数派", URL: "https://sspai.com/feed", Category: "tech",
			Description: "数字生活效率工具、软硬件玩法、App 评测",
			Region:      "cn", Lang: "zh", Kind: "rss", Enabled: true},
		{Name: "InfoQ 中文", URL: "https://www.infoq.cn/feed", Category: "dev",
			Description: "面向工程师的技术架构、企业级软件、AI 实践",
			Region:      "cn", Lang: "zh", Kind: "rss", Enabled: true},
		{Name: "雷锋网", URL: "https://www.leiphone.com/feed", Category: "tech",
			Description: "中文科技媒体，硬科技 / 智能驾驶 / 行业研究",
			Region:      "cn", Lang: "zh", Kind: "rss", Enabled: true},
		{Name: "Solidot", URL: "https://www.solidot.org/index.rss", Category: "tech",
			Description: "极客资讯，开源、隐私、信息安全、互联网政策",
			Region:      "cn", Lang: "zh", Kind: "rss"},
		{Name: "IT 之家", URL: "https://www.ithome.com/rss/", Category: "tech",
			Description: "数码硬件、消费电子、Windows / Apple 生态新闻",
			Region:      "cn", Lang: "zh", Kind: "rss"},
		{Name: "阿里云开发者", URL: "https://developer.aliyun.com/rss/all", Category: "dev",
			Description: "云计算 / 数据库 / 大数据 / Java / Go 实践",
			Region:      "cn", Lang: "zh", Kind: "rss"},
		{Name: "阮一峰科技爱好者周刊", URL: "http://www.ruanyifeng.com/blog/atom.xml", Category: "dev",
			Description: "周刊体裁的技术 / 工具 / 创业精选，质量高",
			Region:      "cn", Lang: "zh", Kind: "rss"},

		// ===== AI =====
		{Name: "机器之心", URL: "https://www.jiqizhixin.com/rss", Category: "ai",
			Description: "AI 学术与产业新闻，论文解读、模型发布、行业进展",
			Region:      "cn", Lang: "zh", Kind: "rss"},
		{Name: "量子位", URL: "https://www.qbitai.com/feed", Category: "ai",
			Description: "AI 新闻、大模型、机器人、芯片，更新频繁",
			Region:      "cn", Lang: "zh", Kind: "rss"},
		{Name: "PaperWeekly", URL: "https://www.paperweekly.site/rss", Category: "ai",
			Description: "前沿 AI 论文解读，NLP / CV / LLM 研究方向",
			Region:      "cn", Lang: "zh", Kind: "rss"},

		// ===== 财经商业 =====
		{Name: "华尔街见闻", URL: "https://wallstreetcn.com/feed/", Category: "finance",
			Description: "宏观经济、A 股 / 美股、外汇大宗、政策解读",
			Region:      "cn", Lang: "zh", Kind: "rss"},
		{Name: "钛媒体", URL: "https://www.tmtpost.com/rss.xml", Category: "finance",
			Description: "TMT 行业、商业模式、企业战略、上市公司动态",
			Region:      "cn", Lang: "zh", Kind: "rss"},

		// ===== 国际新闻（中文化）=====
		{Name: "联合早报", URL: "https://www.zaobao.com/realtime/rss.xml", Category: "intl",
			Description: "新加坡中文媒体，第三方视角看中美 / 亚太 / 国际",
			Region:      "intl_zh", Lang: "zh", Kind: "rss"},
		{Name: "BBC 中文", URL: "http://feeds.bbci.co.uk/zhongwen/simp/rss.xml", Category: "intl",
			Description: "BBC 中文部，国际时政 / 中港台 / 全球热点",
			Region:      "intl_zh", Lang: "zh", Kind: "rss"},
		{Name: "DW 中文", URL: "https://rss.dw.com/atom/rss-zh-all", Category: "intl",
			Description: "德国之声中文版，欧洲视角的国际新闻 / 政治分析",
			Region:      "intl_zh", Lang: "zh", Kind: "rss"},
		{Name: "RFI 中文", URL: "https://www.rfi.fr/cn/rss", Category: "intl",
			Description: "法广中文，法语区视角，中东 / 非洲 / 国际突发事件",
			Region:      "intl_zh", Lang: "zh", Kind: "rss"},

		// ===== 体育 =====
		{Name: "虎扑步行街头条", URL: "https://voice.hupu.com/rss-all", Category: "sports",
			Description: "篮球 / 足球 / 中国体育 / 评论员热议话题",
			Region:      "cn", Lang: "zh", Kind: "rss"},
		{Name: "懒熊体育", URL: "https://www.lanxiongsports.com/rss", Category: "sports",
			Description: "体育商业、赛事运营、品牌赞助行业新闻",
			Region:      "cn", Lang: "zh", Kind: "rss"},

		// ===== 文化 / 生活 =====
		{Name: "果壳网", URL: "https://www.guokr.com/rss/", Category: "science",
			Description: "科普读物、自然 / 医学 / 心理 / 物理热点科普",
			Region:      "cn", Lang: "zh", Kind: "rss"},
		{Name: "豆瓣一刻", URL: "https://moment.douban.com/rss", Category: "culture",
			Description: "电影 / 图书 / 音乐评论、年轻人文化议题",
			Region:      "cn", Lang: "zh", Kind: "rss"},

		// ===== 极客 / 开发者 =====
		{Name: "V2EX 最新主题", URL: "https://www.v2ex.com/index.xml", Category: "dev",
			Description: "国内开发者社区热门话题，技术 / 远程办公 / 职场",
			Region:      "cn", Lang: "zh", Kind: "rss"},

		// ===== 设计 =====
		{Name: "优设网", URL: "https://www.uisdc.com/feed", Category: "design",
			Description: "UI / UX / 平面设计行业新闻、设计师作品、教程",
			Region:      "cn", Lang: "zh", Kind: "rss"},

		// ===== 学术 / 政策 =====
		{Name: "知社学术圈", URL: "https://www.zhishesci.com/rss", Category: "science",
			Description: "学术圈新闻、科研政策、Nature / Science 解读",
			Region:      "cn", Lang: "zh", Kind: "rss"},

		// ===== 健康 / 医疗 =====
		{Name: "丁香园", URL: "https://www.dxy.cn/rss.xml", Category: "health",
			Description: "医学 / 临床 / 药品 / 公共卫生新闻",
			Region:      "cn", Lang: "zh", Kind: "rss"},
	}
	for i := range seeds {
		if err := db.UpsertNewsSource(ctx, database, &seeds[i]); err != nil {
			return err
		}
	}
	return nil
}
