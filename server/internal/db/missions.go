package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/agentzero/server/internal/model"
)

// CreateMission 在派遣任务时落库。状态默认为 pending，由 agent loop 推进。
// SeriesID 留空时默认设为自身 ID，SeriesSeq 留 0 时默认设为 1，
// 这样不走「继续安排」的散单任务也是一组合法的卷宗（只是只有一卷）。
func CreateMission(ctx context.Context, db *sql.DB, m *model.Mission) error {
	if m.ID == "" {
		return fmt.Errorf("mission id required")
	}
	loadout, err := json.Marshal(m.Loadout)
	if err != nil {
		return fmt.Errorf("marshal loadout: %w", err)
	}
	if m.Status == "" {
		m.Status = model.StatusPending
	}
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now().UTC()
	}
	if m.SeriesID == "" {
		m.SeriesID = m.ID
	}
	if m.SeriesSeq <= 0 {
		m.SeriesSeq = 1
	}
	_, err = db.ExecContext(ctx, `
		INSERT INTO missions (id, user_id, codename, brief, tier, status, loadout_json, workspace_dir,
		                     series_id, series_seq, parent_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, m.ID, m.UserID, m.Codename, m.Brief, string(m.Tier), string(m.Status), string(loadout), m.WorkspaceDir,
		m.SeriesID, m.SeriesSeq, m.ParentID, m.CreatedAt)
	return err
}

const missionSelect = `
	SELECT id, user_id, codename, brief, tier, status, loadout_json, workspace_dir,
	       input_tokens, output_tokens, series_id, series_seq, parent_id,
	       started_at, ended_at, created_at
	FROM missions`

// GetMission 取一个属于该 user 的 mission。跨用户访问会返回 ErrNotFound。
func GetMission(ctx context.Context, db *sql.DB, missionID string, userID int64) (*model.Mission, error) {
	row := db.QueryRowContext(ctx, missionSelect+` WHERE id = ? AND user_id = ?`, missionID, userID)
	m, err := scanMission(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return m, err
}

// ListMissions 列出该用户的任务，按 created_at desc。
func ListMissions(ctx context.Context, db *sql.DB, userID int64, limit, offset int) ([]*model.Mission, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	rows, err := db.QueryContext(ctx, missionSelect+`
		WHERE user_id = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?`, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Mission
	for rows.Next() {
		m, err := scanMissionRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ListMissionsBySeries 取同一卷宗下的全部 mission（含跨用户校验）。
// 按 series_seq 升序，用于「行动卷宗」视图。
func ListMissionsBySeries(ctx context.Context, db *sql.DB, seriesID string, userID int64) ([]*model.Mission, error) {
	rows, err := db.QueryContext(ctx, missionSelect+`
		WHERE series_id = ? AND user_id = ?
		ORDER BY series_seq ASC, created_at ASC`, seriesID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Mission
	for rows.Next() {
		m, err := scanMissionRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// NextSeriesSeq 算下一个 series_seq（同一 series_id 内）。
func NextSeriesSeq(ctx context.Context, db *sql.DB, seriesID string) (int, error) {
	var maxSeq int
	if err := db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(series_seq), 0) FROM missions WHERE series_id = ?`,
		seriesID).Scan(&maxSeq); err != nil {
		return 0, err
	}
	return maxSeq + 1, nil
}

// DeleteMission 物理删除一个任务及其相关 steps / artifacts。
// 注意：调用方应负责 abort + 清理 workspace 目录。
// 做了 user_id 归属校验，跨用户删除返回 ErrNotFound。
func DeleteMission(ctx context.Context, db *sql.DB, missionID string, userID int64) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 先确认归属
	var owner int64
	if err := tx.QueryRowContext(ctx,
		`SELECT user_id FROM missions WHERE id = ?`, missionID).Scan(&owner); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if owner != userID {
		return ErrNotFound
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM artifacts WHERE mission_id = ?`, missionID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM steps WHERE mission_id = ?`, missionID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM missions WHERE id = ?`, missionID); err != nil {
		return err
	}
	return tx.Commit()
}

// UpdateMissionStatus 推进任务生命周期。started_at/ended_at 根据新状态自动写入。
func UpdateMissionStatus(ctx context.Context, db *sql.DB, missionID string, status model.MissionStatus) error {
	now := time.Now().UTC()
	switch status {
	case model.StatusRunning:
		_, err := db.ExecContext(ctx, `
			UPDATE missions SET status = ?, started_at = COALESCE(started_at, ?) WHERE id = ?
		`, string(status), now, missionID)
		return err
	case model.StatusDone, model.StatusAborted, model.StatusError:
		_, err := db.ExecContext(ctx, `
			UPDATE missions SET status = ?, ended_at = ? WHERE id = ?
		`, string(status), now, missionID)
		return err
	default:
		_, err := db.ExecContext(ctx, `UPDATE missions SET status = ? WHERE id = ?`, string(status), missionID)
		return err
	}
}

// AddMissionUsage 累加 token 计费。多次调用累加。
func AddMissionUsage(ctx context.Context, db *sql.DB, missionID string, inputTokens, outputTokens int64) error {
	_, err := db.ExecContext(ctx, `
		UPDATE missions SET input_tokens = input_tokens + ?, output_tokens = output_tokens + ?
		WHERE id = ?
	`, inputTokens, outputTokens, missionID)
	return err
}

// AppendStep 追加一条事件，自动分配 seq。
// 返回赋好 ID/Seq/Ts 的 step 副本，方便上层立即推送给 SSE 客户端。
func AppendStep(ctx context.Context, db *sql.DB, s *model.Step) error {
	if s.MissionID == "" || s.Type == "" {
		return fmt.Errorf("mission_id and type required")
	}
	if len(s.Payload) == 0 {
		s.Payload = []byte("{}")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var nextSeq int
	if err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(seq), 0) + 1 FROM steps WHERE mission_id = ?`,
		s.MissionID).Scan(&nextSeq); err != nil {
		return err
	}
	now := time.Now().UTC()
	res, err := tx.ExecContext(ctx, `
		INSERT INTO steps (mission_id, seq, ts, type, payload_json, reasoning_content)
		VALUES (?, ?, ?, ?, ?, ?)
	`, s.MissionID, nextSeq, now, string(s.Type), string(s.Payload), s.ReasoningContent)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.ID = id
	s.Seq = nextSeq
	s.Ts = now
	return nil
}

// ListSteps 按 seq 升序拉一段事件流。afterSeq=0 表示从头开始。
func ListSteps(ctx context.Context, db *sql.DB, missionID string, afterSeq, limit int) ([]*model.Step, error) {
	if limit <= 0 || limit > 1000 {
		limit = 500
	}
	rows, err := db.QueryContext(ctx, `
		SELECT id, mission_id, seq, ts, type, payload_json, reasoning_content
		FROM steps
		WHERE mission_id = ? AND seq > ?
		ORDER BY seq ASC
		LIMIT ?
	`, missionID, afterSeq, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Step
	for rows.Next() {
		var s model.Step
		var payload string
		if err := rows.Scan(&s.ID, &s.MissionID, &s.Seq, &s.Ts, &s.Type, &payload, &s.ReasoningContent); err != nil {
			return nil, err
		}
		s.Payload = json.RawMessage(payload)
		out = append(out, &s)
	}
	return out, rows.Err()
}

// AddArtifact 入柜。
func AddArtifact(ctx context.Context, db *sql.DB, a *model.Artifact) error {
	now := time.Now().UTC()
	res, err := db.ExecContext(ctx, `
		INSERT INTO artifacts (mission_id, kind, name, path, mime, size_bytes, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, a.MissionID, a.Kind, a.Name, a.Path, a.Mime, a.Size, now)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	a.ID = id
	a.CreatedAt = now
	return nil
}

// GetArtifact 取一个 mission 下的单个工件，做了 mission_id 归属校验。
func GetArtifact(ctx context.Context, db *sql.DB, missionID string, artifactID int64) (*model.Artifact, error) {
	row := db.QueryRowContext(ctx, `
		SELECT id, mission_id, kind, name, path, mime, size_bytes, created_at
		FROM artifacts WHERE id = ? AND mission_id = ?
	`, artifactID, missionID)
	var a model.Artifact
	if err := row.Scan(&a.ID, &a.MissionID, &a.Kind, &a.Name, &a.Path, &a.Mime, &a.Size, &a.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}

// ListArtifacts 按时间倒序拉某 mission 的工件。
func ListArtifacts(ctx context.Context, db *sql.DB, missionID string) ([]*model.Artifact, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, mission_id, kind, name, path, mime, size_bytes, created_at
		FROM artifacts WHERE mission_id = ?
		ORDER BY created_at DESC
	`, missionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Artifact
	for rows.Next() {
		var a model.Artifact
		if err := rows.Scan(&a.ID, &a.MissionID, &a.Kind, &a.Name, &a.Path, &a.Mime, &a.Size, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &a)
	}
	return out, rows.Err()
}

// missionScanner 抽象出 *sql.Row 与 *sql.Rows 的公共扫描接口。
type missionScanner interface {
	Scan(dest ...any) error
}

func scanMission(row *sql.Row) (*model.Mission, error) {
	return scanMissionRow(row)
}

func scanMissionRows(rows *sql.Rows) (*model.Mission, error) {
	return scanMissionRow(rows)
}

func scanMissionRow(s missionScanner) (*model.Mission, error) {
	var m model.Mission
	var loadoutJSON string
	var tier, status string
	var parentID sql.NullString
	var startedAt, endedAt sql.NullTime
	if err := s.Scan(&m.ID, &m.UserID, &m.Codename, &m.Brief, &tier, &status,
		&loadoutJSON, &m.WorkspaceDir, &m.InputTokens, &m.OutputTokens,
		&m.SeriesID, &m.SeriesSeq, &parentID,
		&startedAt, &endedAt, &m.CreatedAt); err != nil {
		return nil, err
	}
	m.Tier = model.MissionTier(tier)
	m.Status = model.MissionStatus(status)
	if parentID.Valid {
		pid := parentID.String
		m.ParentID = &pid
	}
	if startedAt.Valid {
		t := startedAt.Time
		m.StartedAt = &t
	}
	if endedAt.Valid {
		t := endedAt.Time
		m.EndedAt = &t
	}
	if loadoutJSON == "" {
		m.Loadout = []string{}
	} else if err := json.Unmarshal([]byte(loadoutJSON), &m.Loadout); err != nil {
		return nil, fmt.Errorf("decode loadout: %w", err)
	}
	return &m, nil
}
