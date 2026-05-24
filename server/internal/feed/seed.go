package feed

import (
	"context"
	"database/sql"

	"github.com/agentzero/server/internal/db"
	"github.com/agentzero/server/internal/model"
)

// SeedSources 把内置的 RSS 源库 upsert 到 news_sources。
//
// 源库构成：
//   - 原生 RSS（约 25 条）：站点自带 feed，稳定但覆盖有限
//   - RSSHub 路由（约 55 条）：通过自托管 RSSHub 实例把微信公众号、知乎、
//     Twitter、Bilibili、微博、雪球、HN、GitHub Trending、arXiv 等
//     "RSS 化"，让覆盖度跃升一个数量级
//
// 默认只启用 4 个稳源；其他源都 enabled=false，等 LLM 推荐 / 用户勾选。
func SeedSources(ctx context.Context, database *sql.DB) error {
	seeds := append([]model.NewsSource{}, nativeRSSSeeds()...)
	seeds = append(seeds, rsshubSeeds()...)
	for i := range seeds {
		if err := db.UpsertNewsSource(ctx, database, &seeds[i]); err != nil {
			return err
		}
	}
	return nil
}

func nativeRSSSeeds() []model.NewsSource {
	return []model.NewsSource{
		// ===== 科技 =====
		{Name: "36 氪", URL: "https://36kr.com/feed", Category: "tech", Region: "cn", Lang: "zh", Kind: "rss", Enabled: true,
			Description: "中文科技创投媒体，覆盖创业公司、融资、产品"},
		{Name: "少数派", URL: "https://sspai.com/feed", Category: "tech", Region: "cn", Lang: "zh", Kind: "rss", Enabled: true,
			Description: "数字生活效率工具、软硬件玩法、App 评测"},
		{Name: "InfoQ 中文", URL: "https://www.infoq.cn/feed", Category: "dev", Region: "cn", Lang: "zh", Kind: "rss", Enabled: true,
			Description: "面向工程师的技术架构、企业级软件、AI 实践"},
		{Name: "雷锋网", URL: "https://www.leiphone.com/feed", Category: "tech", Region: "cn", Lang: "zh", Kind: "rss", Enabled: true,
			Description: "中文科技媒体，硬科技 / 智能驾驶 / 行业研究"},
		{Name: "Solidot", URL: "https://www.solidot.org/index.rss", Category: "tech", Region: "cn", Lang: "zh", Kind: "rss",
			Description: "极客资讯，开源、隐私、信息安全、互联网政策"},
		{Name: "IT 之家", URL: "https://www.ithome.com/rss/", Category: "tech", Region: "cn", Lang: "zh", Kind: "rss",
			Description: "数码硬件、消费电子、Windows / Apple 生态新闻"},
		{Name: "阿里云开发者", URL: "https://developer.aliyun.com/rss/all", Category: "dev", Region: "cn", Lang: "zh", Kind: "rss",
			Description: "云计算 / 数据库 / 大数据 / Java / Go 实践"},
		{Name: "阮一峰科技爱好者周刊", URL: "http://www.ruanyifeng.com/blog/atom.xml", Category: "dev", Region: "cn", Lang: "zh", Kind: "rss",
			Description: "周刊体裁的技术 / 工具 / 创业精选，质量高"},

		// ===== AI =====
		{Name: "机器之心", URL: "https://www.jiqizhixin.com/rss", Category: "ai", Region: "cn", Lang: "zh", Kind: "rss",
			Description: "AI 学术与产业新闻，论文解读、模型发布、行业进展"},
		{Name: "量子位", URL: "https://www.qbitai.com/feed", Category: "ai", Region: "cn", Lang: "zh", Kind: "rss",
			Description: "AI 新闻、大模型、机器人、芯片，更新频繁"},
		{Name: "PaperWeekly", URL: "https://www.paperweekly.site/rss", Category: "ai", Region: "cn", Lang: "zh", Kind: "rss",
			Description: "前沿 AI 论文解读，NLP / CV / LLM 研究方向"},
		{Name: "OpenAI Blog", URL: "https://openai.com/blog/rss.xml", Category: "ai", Region: "intl_en", Lang: "en", Kind: "rss",
			Description: "OpenAI 官方发布与研究公告"},
		{Name: "Anthropic News", URL: "https://www.anthropic.com/news/rss.xml", Category: "ai", Region: "intl_en", Lang: "en", Kind: "rss",
			Description: "Anthropic 官方动态、Claude 模型更新"},

		// ===== 财经 =====
		{Name: "华尔街见闻", URL: "https://wallstreetcn.com/feed/", Category: "finance", Region: "cn", Lang: "zh", Kind: "rss",
			Description: "宏观经济、A 股 / 美股、外汇大宗、政策解读"},
		{Name: "钛媒体", URL: "https://www.tmtpost.com/rss.xml", Category: "finance", Region: "cn", Lang: "zh", Kind: "rss",
			Description: "TMT 行业、商业模式、企业战略、上市公司动态"},

		// ===== 国际新闻 =====
		{Name: "联合早报", URL: "https://www.zaobao.com/realtime/rss.xml", Category: "intl", Region: "intl_zh", Lang: "zh", Kind: "rss",
			Description: "新加坡中文媒体，第三方视角看中美 / 亚太 / 国际"},
		{Name: "BBC 中文", URL: "http://feeds.bbci.co.uk/zhongwen/simp/rss.xml", Category: "intl", Region: "intl_zh", Lang: "zh", Kind: "rss",
			Description: "BBC 中文部，国际时政 / 中港台 / 全球热点"},
		{Name: "DW 中文", URL: "https://rss.dw.com/atom/rss-zh-all", Category: "intl", Region: "intl_zh", Lang: "zh", Kind: "rss",
			Description: "德国之声中文版，欧洲视角的国际新闻"},
		{Name: "RFI 中文", URL: "https://www.rfi.fr/cn/rss", Category: "intl", Region: "intl_zh", Lang: "zh", Kind: "rss",
			Description: "法广中文，法语区视角，中东 / 非洲 / 国际突发"},
		{Name: "Reuters Top News", URL: "https://www.reutersagency.com/feed/?best-topics=top-news&post_type=best", Category: "intl", Region: "intl_en", Lang: "en", Kind: "rss",
			Description: "路透社英文要闻，国际政治 / 商业"},

		// ===== 体育 / 文化 / 设计 / 健康 / 科学 =====
		{Name: "虎扑步行街头条", URL: "https://voice.hupu.com/rss-all", Category: "sports", Region: "cn", Lang: "zh", Kind: "rss",
			Description: "篮球 / 足球 / 中国体育 / 评论员热议话题"},
		{Name: "懒熊体育", URL: "https://www.lanxiongsports.com/rss", Category: "sports", Region: "cn", Lang: "zh", Kind: "rss",
			Description: "体育商业、赛事运营、品牌赞助行业新闻"},
		{Name: "果壳网", URL: "https://www.guokr.com/rss/", Category: "science", Region: "cn", Lang: "zh", Kind: "rss",
			Description: "科普读物、自然 / 医学 / 心理 / 物理热点科普"},
		{Name: "豆瓣一刻", URL: "https://moment.douban.com/rss", Category: "culture", Region: "cn", Lang: "zh", Kind: "rss",
			Description: "电影 / 图书 / 音乐评论、年轻人文化议题"},
		{Name: "V2EX 最新主题", URL: "https://www.v2ex.com/index.xml", Category: "dev", Region: "cn", Lang: "zh", Kind: "rss",
			Description: "国内开发者社区热门话题"},
		{Name: "优设网", URL: "https://www.uisdc.com/feed", Category: "design", Region: "cn", Lang: "zh", Kind: "rss",
			Description: "UI / UX / 平面设计行业新闻、设计师作品"},
		{Name: "丁香园", URL: "https://www.dxy.cn/rss.xml", Category: "health", Region: "cn", Lang: "zh", Kind: "rss",
			Description: "医学 / 临床 / 药品 / 公共卫生新闻"},
	}
}

// rsshubSeeds 通过自托管 RSSHub 实例代理的源。
// URL 字段保留站点首页（仅作展示），实际拉取走 rsshub_route。
func rsshubSeeds() []model.NewsSource {
	rh := func(name, route, category, region, lang, desc string) model.NewsSource {
		return model.NewsSource{
			Name: name, URL: "rsshub://" + route, RSSHubRoute: route,
			Kind: "rss", Region: region, Lang: lang,
			Category: category, Description: desc,
		}
	}
	return []model.NewsSource{
		// ===== AI / 科技 中文 =====
		rh("智东西", "/zhidx", "ai", "cn", "zh", "智能硬件 / 自动驾驶 / 大模型行业深度"),
		rh("新智元", "/aifrontiers", "ai", "cn", "zh", "AI 学术与产业头条"),
		rh("Hugging Face Daily Papers", "/huggingface/daily-papers", "ai", "intl_en", "en", "Hugging Face 每日精选论文榜"),
		rh("Google AI Blog", "/google/blog/ai", "ai", "intl_en", "en", "Google 研究院 AI 与基础设施博客"),
		rh("DeepMind Blog", "/deepmind/blog", "ai", "intl_en", "en", "DeepMind 官方研究博客"),
		rh("Meta AI Research", "/facebook/research", "ai", "intl_en", "en", "Meta AI 研究院发布"),
		rh("MIT Technology Review 中文", "/mittrchina/news", "tech", "intl_zh", "zh", "MIT 科技评论中文版"),
		rh("机器之心精选", "/jiqizhixin/highlights", "ai", "cn", "zh", "机器之心精选热门"),

		// ===== 综合中文 =====
		rh("知乎热榜", "/zhihu/hotlist", "culture", "cn", "zh", "知乎全站热门话题"),
		rh("知乎日报", "/zhihu/daily", "culture", "cn", "zh", "知乎日报精选"),
		rh("微博热搜", "/weibo/search/hot", "culture", "cn", "zh", "微博实时热搜榜"),
		rh("百度热搜", "/baidu/realtime", "general", "cn", "zh", "百度实时热搜"),
		rh("澎湃新闻头条", "/thepaper/featured", "general", "cn", "zh", "澎湃新闻头条"),
		rh("界面新闻", "/jiemian/lists/24", "finance", "cn", "zh", "界面新闻头条"),
		rh("第一财经头条", "/yicai/brief", "finance", "cn", "zh", "第一财经实时财经资讯"),
		rh("财新网最新", "/caixin/latest", "finance", "cn", "zh", "财新网最新报道"),
		rh("经济观察网", "/eeo", "finance", "cn", "zh", "经济观察网首页"),
		rh("证券时报头条", "/stcn/headline", "finance", "cn", "zh", "证券时报头条新闻"),
		rh("央视新闻头条", "/cctv/world/news", "general", "cn", "zh", "央视新闻国际频道头条"),
		rh("人民日报评论", "/people/opinions", "general", "cn", "zh", "人民日报评论文章"),
		rh("光明日报头条", "/guangming/topnews", "general", "cn", "zh", "光明日报头条"),

		// ===== 雪球 / 行情 =====
		rh("雪球热门", "/xueqiu/hots", "finance", "cn", "zh", "雪球热门帖子"),
		rh("雪球今日话题", "/xueqiu/today_topic", "finance", "cn", "zh", "雪球今日话题"),

		// ===== 国际中文 =====
		rh("端传媒", "/initium/latest", "intl", "intl_zh", "zh", "香港独立中文媒体，深度报道"),
		rh("纽约时报中文", "/nytimes/home", "intl", "intl_zh", "zh", "纽约时报中文网"),
		rh("FT 中文网", "/ftchinese/story", "finance", "intl_zh", "zh", "英国金融时报中文版"),
		rh("BBC News 全球", "/bbc/world/asia", "intl", "intl_en", "en", "BBC 亚洲国际频道"),
		rh("CNN Top Stories", "/cnn/topstories", "intl", "intl_en", "en", "CNN 头条要闻"),

		// ===== 极客 / 开发 国际 =====
		rh("Hacker News Best", "/hackernews/best", "dev", "intl_en", "en", "Hacker News 高分热门"),
		rh("GitHub Trending Daily", "/github/trending/daily/any", "dev", "intl_en", "en", "GitHub 每日热门项目"),
		rh("Product Hunt 今日", "/producthunt/today", "tech", "intl_en", "en", "今日 Product Hunt 上榜产品"),
		rh("TechCrunch", "/techcrunch/news", "tech", "intl_en", "en", "TechCrunch 全球科技创投"),
		rh("The Verge", "/theverge", "tech", "intl_en", "en", "The Verge 科技 / 文化"),
		rh("Ars Technica", "/arstechnica/index", "tech", "intl_en", "en", "Ars Technica 深度技术报道"),
		rh("Wired", "/wired/feed", "tech", "intl_en", "en", "Wired 科技与文化"),

		// ===== 学术 / 研究 =====
		rh("arXiv cs.AI", "/journals/arxiv/cs.AI", "science", "intl_en", "en", "arXiv 人工智能新论文"),
		rh("arXiv cs.CL", "/journals/arxiv/cs.CL", "science", "intl_en", "en", "arXiv 计算语言学新论文"),
		rh("arXiv cs.LG", "/journals/arxiv/cs.LG", "science", "intl_en", "en", "arXiv 机器学习新论文"),
		rh("Nature 头条", "/nature/news", "science", "intl_en", "en", "Nature 杂志最新研究新闻"),
		rh("Science Magazine", "/sciencemag/twis", "science", "intl_en", "en", "Science 周刊最新研究综述"),

		// ===== 体育 / 文化 / 视频 =====
		rh("懂球帝头条", "/dongqiudi/main", "sports", "cn", "zh", "懂球帝足球头条"),
		rh("虎扑 NBA", "/hupu/nba", "sports", "cn", "zh", "虎扑 NBA 资讯"),
		rh("豆瓣电影热门", "/douban/movie/showing", "culture", "cn", "zh", "豆瓣正在热映电影"),
		rh("豆瓣读书热门", "/douban/book/latest", "culture", "cn", "zh", "豆瓣读书新书"),
		rh("Bilibili 热门排行", "/bilibili/ranking/0/3", "culture", "cn", "zh", "B 站全站热门视频排行"),
		rh("YouTube 趋势", "/youtube/trending", "culture", "intl_en", "en", "YouTube 全球趋势视频"),

		// ===== 政策 / 政府公告 =====
		rh("中国证监会公告", "/csrc/news", "finance", "cn", "zh", "证监会最新公告与发布"),
		rh("国务院政策文件", "/gov/news/policy", "general", "cn", "zh", "国务院最新政策文件"),

		// ===== 健康 / 医疗 =====
		rh("WHO 新闻", "/who/news", "health", "intl_en", "en", "世界卫生组织官方公告"),
		rh("国家卫健委", "/nhc/news", "health", "cn", "zh", "国家卫生健康委员会最新公告"),

		// ===== 设计 / 产品 =====
		rh("Dribbble Popular", "/dribbble/popular/now", "design", "intl_en", "en", "Dribbble 当前热门设计稿"),
		rh("UI Movement", "/uimovement", "design", "intl_en", "en", "UI Movement 最新优秀界面设计"),

		// ===== 科技博主 =====
		rh("月光博客", "/blogs/williamlong", "tech", "cn", "zh", "月光博客技术与互联网评论"),
		rh("36 氪快讯", "/36kr/newsflashes", "tech", "cn", "zh", "36 氪实时商业快讯"),
	}
}
