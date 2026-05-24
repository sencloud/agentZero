package feed

import (
	"context"
	"database/sql"
	"log/slog"

	"github.com/agentzero/server/internal/db"
)

// Pruner 跑自动裁剪逻辑。参数从配置注入，避免到处魔术数字。
type Pruner struct {
	db        *sql.DB
	logger    *slog.Logger
	OlderDays int
	Decay     float64
	MinWeight float64
}

func NewPruner(database *sql.DB, logger *slog.Logger) *Pruner {
	return &Pruner{db: database, logger: logger, OlderDays: 7, Decay: 0.6, MinWeight: 0.4}
}

func (p *Pruner) Run(ctx context.Context) (relationsRemoved, entitiesRemoved int, err error) {
	return db.PruneFeed(ctx, p.db, p.OlderDays, p.Decay, p.MinWeight)
}
