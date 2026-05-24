package feed

import (
	"context"
	"database/sql"

	"github.com/agentzero/server/internal/db"
	"github.com/agentzero/server/internal/model"
)

// SeedSources 把内置的 RSS 源 upsert 到 news_sources。
// 选源原则：从中国境内服务器能直连、内容质量稳定、有公开 RSS 接口。
//
// 注：境外大媒体（BBC / Reuters / NYT）的 RSS 在部分网络下无法直连；
// 这里默认只放国内必通源，运营侧后续可在数据库里直接补充。
func SeedSources(ctx context.Context, database *sql.DB) error {
	seeds := []model.NewsSource{
		{Name: "36 氪", URL: "https://36kr.com/feed", Kind: "rss", Region: "cn", Lang: "zh", Enabled: true},
		{Name: "虎嗅网", URL: "https://www.huxiu.com/rss/0.xml", Kind: "rss", Region: "cn", Lang: "zh", Enabled: true},
		{Name: "少数派", URL: "https://sspai.com/feed", Kind: "rss", Region: "cn", Lang: "zh", Enabled: true},
		{Name: "InfoQ 中文", URL: "https://www.infoq.cn/feed", Kind: "rss", Region: "cn", Lang: "zh", Enabled: true},
		{Name: "机器之心", URL: "https://www.jiqizhixin.com/rss", Kind: "rss", Region: "cn", Lang: "zh", Enabled: true},
		{Name: "雷锋网", URL: "https://www.leiphone.com/feed", Kind: "rss", Region: "cn", Lang: "zh", Enabled: true},
		{Name: "极客公园", URL: "https://www.geekpark.net/rss", Kind: "rss", Region: "cn", Lang: "zh", Enabled: true},
		{Name: "知乎日报", URL: "https://daily.zhihu.com/feed.xml", Kind: "rss", Region: "cn", Lang: "zh", Enabled: true},
	}
	for i := range seeds {
		if err := db.UpsertNewsSource(ctx, database, &seeds[i]); err != nil {
			return err
		}
	}
	return nil
}
