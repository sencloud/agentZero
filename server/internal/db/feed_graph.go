package db

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/agentzero/server/internal/model"
)

// UpsertEntity 按 (type, name) 去重；已存在则刷新 last_seen_at 与累加 weight。
func UpsertEntity(ctx context.Context, db *sql.DB, e *model.Entity) error {
	now := time.Now().UTC()
	if e.Weight <= 0 {
		e.Weight = 1.0
	}
	res, err := db.ExecContext(ctx, `
		INSERT INTO entities (type, name, weight, first_seen_at, last_seen_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(type, name) DO UPDATE SET
		  weight = entities.weight + excluded.weight,
		  last_seen_at = excluded.last_seen_at
	`, e.Type, e.Name, e.Weight, now, now)
	if err != nil {
		return err
	}
	if id, _ := res.LastInsertId(); id != 0 && e.ID == 0 {
		e.ID = id
	}
	if e.ID == 0 {
		// 冲突分支没拿到 LastInsertId，回查一次
		row := db.QueryRowContext(ctx, `SELECT id FROM entities WHERE type = ? AND name = ?`, e.Type, e.Name)
		if err := row.Scan(&e.ID); err != nil {
			return err
		}
	}
	e.LastSeenAt = now
	return nil
}

// LinkEventEntity 把事件 ↔ 实体的关系入库（同 PK 已存在则取较大 salience）。
func LinkEventEntity(ctx context.Context, db *sql.DB, eventID, entityID int64, salience float64) error {
	if salience <= 0 {
		salience = 1.0
	}
	_, err := db.ExecContext(ctx, `
		INSERT INTO event_entities (event_id, entity_id, salience)
		VALUES (?, ?, ?)
		ON CONFLICT(event_id, entity_id) DO UPDATE SET
		  salience = MAX(event_entities.salience, excluded.salience)
	`, eventID, entityID, salience)
	return err
}

// UpsertRelation 按 (src, dst, label) 唯一；已存在则累加 weight + 刷新 last_seen_at。
func UpsertRelation(ctx context.Context, db *sql.DB, r *model.EntityRelation) error {
	now := time.Now().UTC()
	if r.Weight <= 0 {
		r.Weight = 1.0
	}
	if r.Label == "" {
		r.Label = "related"
	}
	_, err := db.ExecContext(ctx, `
		INSERT INTO entity_relations (src_id, dst_id, label, weight, last_seen_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(src_id, dst_id, label) DO UPDATE SET
		  weight = entity_relations.weight + excluded.weight,
		  last_seen_at = excluded.last_seen_at
	`, r.SrcID, r.DstID, r.Label, r.Weight, now)
	return err
}

// ListGraphForUser 给 /feed/graph 用：取该用户最近订阅事件涉及的实体子图。
// limitEntities 控制最大节点数（按 weight 降序截断）。
type GraphSnapshot struct {
	Nodes []*model.Entity
	Edges []*model.EntityRelation
}

func ListGraphForUser(ctx context.Context, db *sql.DB, userID int64, limitEntities int) (*GraphSnapshot, error) {
	if limitEntities <= 0 || limitEntities > 200 {
		limitEntities = 80
	}
	// 1) 收集该用户事件涉及到的 entity，按累计 salience * relevance 加权
	rows, err := db.QueryContext(ctx, `
		SELECT en.id, en.type, en.name, en.weight, en.first_seen_at, en.last_seen_at,
		       SUM(ee.salience * u.relevance) AS score
		FROM user_event_subs u
		JOIN event_entities ee ON ee.event_id = u.event_id
		JOIN entities en ON en.id = ee.entity_id
		WHERE u.user_id = ?
		GROUP BY en.id
		ORDER BY score DESC, en.last_seen_at DESC
		LIMIT ?
	`, userID, limitEntities)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	nodes := make([]*model.Entity, 0, limitEntities)
	ids := make([]int64, 0, limitEntities)
	idSet := map[int64]struct{}{}
	for rows.Next() {
		var e model.Entity
		var score float64
		if err := rows.Scan(&e.ID, &e.Type, &e.Name, &e.Weight, &e.FirstSeenAt, &e.LastSeenAt, &score); err != nil {
			return nil, err
		}
		nodes = append(nodes, &e)
		ids = append(ids, e.ID)
		idSet[e.ID] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return &GraphSnapshot{Nodes: nil, Edges: nil}, nil
	}

	// 2) 取这些节点之间的关系
	placeholders := "?"
	args := []any{ids[0]}
	for i := 1; i < len(ids); i++ {
		placeholders += ",?"
		args = append(args, ids[i])
	}
	// src 和 dst 都在 ids 集里的关系
	q := `
		SELECT id, src_id, dst_id, label, weight, last_seen_at
		FROM entity_relations
		WHERE src_id IN (` + placeholders + `) AND dst_id IN (` + placeholders + `)
		ORDER BY weight DESC
		LIMIT 400
	`
	// 复用 args 两次
	dupArgs := make([]any, 0, len(args)*2)
	dupArgs = append(dupArgs, args...)
	dupArgs = append(dupArgs, args...)

	edgeRows, err := db.QueryContext(ctx, q, dupArgs...)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &GraphSnapshot{Nodes: nodes, Edges: nil}, nil
		}
		return nil, err
	}
	defer edgeRows.Close()
	var edges []*model.EntityRelation
	for edgeRows.Next() {
		var r model.EntityRelation
		if err := edgeRows.Scan(&r.ID, &r.SrcID, &r.DstID, &r.Label, &r.Weight, &r.LastSeenAt); err != nil {
			return nil, err
		}
		edges = append(edges, &r)
	}
	return &GraphSnapshot{Nodes: nodes, Edges: edges}, edgeRows.Err()
}

// PruneFeed 自动裁剪：
// - 关系：last_seen_at > olderDays 天的，weight *= decay；weight < minWeight 删除
// - 实体：没有任何关系且 last_seen_at > olderDays 天的删除
// 返回 (relationsRemoved, entitiesRemoved)。
func PruneFeed(ctx context.Context, db *sql.DB, olderDays int, decay, minWeight float64) (int, int, error) {
	if olderDays <= 0 {
		olderDays = 7
	}
	if decay <= 0 || decay >= 1 {
		decay = 0.6
	}
	if minWeight <= 0 {
		minWeight = 0.4
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		UPDATE entity_relations
		SET weight = weight * ?
		WHERE last_seen_at < datetime('now', ?)
	`, decay, "-"+itoa(olderDays)+" day"); err != nil {
		return 0, 0, err
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM entity_relations WHERE weight < ?`, minWeight)
	if err != nil {
		return 0, 0, err
	}
	relRemoved, _ := res.RowsAffected()

	res2, err := tx.ExecContext(ctx, `
		DELETE FROM entities WHERE id IN (
		  SELECT en.id FROM entities en
		  LEFT JOIN entity_relations r1 ON r1.src_id = en.id
		  LEFT JOIN entity_relations r2 ON r2.dst_id = en.id
		  WHERE r1.id IS NULL AND r2.id IS NULL
		    AND en.last_seen_at < datetime('now', ?)
		)
	`, "-"+itoa(olderDays)+" day")
	if err != nil {
		return 0, 0, err
	}
	entRemoved, _ := res2.RowsAffected()

	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}
	return int(relRemoved), int(entRemoved), nil
}

// 小辅助：避免引入 strconv 干扰可读性
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	buf := make([]byte, 0, 8)
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	if neg {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}
