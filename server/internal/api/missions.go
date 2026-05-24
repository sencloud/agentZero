package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/agentzero/server/internal/agent"
	"github.com/agentzero/server/internal/db"
	"github.com/agentzero/server/internal/model"
	"github.com/agentzero/server/internal/tools"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// missionAPI 持有派遣任务所需的所有依赖。
type missionAPI struct {
	*Handlers
	runner   *agent.Runner
	broker   *agent.Broker
	registry *tools.Registry
}

// ---- POST /missions ----

type dispatchReq struct {
	Codename string   `json:"codename"`
	Brief    string   `json:"brief"`
	Tier     string   `json:"tier"`
	Loadout  []string `json:"loadout"`
}

func (m *missionAPI) dispatch(w http.ResponseWriter, r *http.Request) {
	uid, _ := userIDFrom(r)
	var req dispatchReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	req.Brief = strings.TrimSpace(req.Brief)
	req.Codename = strings.TrimSpace(req.Codename)
	if req.Brief == "" {
		writeError(w, http.StatusBadRequest, "brief_required")
		return
	}
	if req.Codename == "" {
		req.Codename = "未命名行动"
	}
	tier := normalizeTier(req.Tier)
	loadout := m.sanitizeLoadout(req.Loadout)
	if len(loadout) == 0 {
		writeError(w, http.StatusBadRequest, "loadout_required")
		return
	}

	missionID := uuid.NewString()
	mission := &model.Mission{
		ID:           missionID,
		UserID:       uid,
		Codename:     req.Codename,
		Brief:        req.Brief,
		Tier:         tier,
		Status:       model.StatusPending,
		Loadout:      loadout,
		WorkspaceDir: filepath.Join(m.runner.MissionWorkspace(missionID)),
		CreatedAt:    time.Now().UTC(),
	}
	if err := db.CreateMission(r.Context(), m.db, mission); err != nil {
		m.logger.Error("create mission failed", "err", err)
		writeError(w, http.StatusInternalServerError, "db_error")
		return
	}
	if err := m.runner.Start(mission); err != nil {
		m.logger.Error("start runner failed", "err", err)
		writeError(w, http.StatusInternalServerError, "start_failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"mission": mission})
}

// ---- GET /missions ----

func (m *missionAPI) listMine(w http.ResponseWriter, r *http.Request) {
	uid, _ := userIDFrom(r)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	items, err := db.ListMissions(r.Context(), m.db, uid, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// ---- GET /missions/:id ----

func (m *missionAPI) detail(w http.ResponseWriter, r *http.Request) {
	uid, _ := userIDFrom(r)
	id := chi.URLParam(r, "id")
	mi, err := db.GetMission(r.Context(), m.db, id, uid)
	if err != nil {
		writeError(w, http.StatusNotFound, "mission_not_found")
		return
	}
	steps, err := db.ListSteps(r.Context(), m.db, mi.ID, 0, 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error")
		return
	}
	arts, err := db.ListArtifacts(r.Context(), m.db, mi.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"mission":   mi,
		"steps":     steps,
		"artifacts": arts,
		"running":   m.runner.IsRunning(mi.ID),
	})
}

// ---- DELETE /missions/:id ----

func (m *missionAPI) deleteMission(w http.ResponseWriter, r *http.Request) {
	uid, _ := userIDFrom(r)
	id := chi.URLParam(r, "id")
	// 先确认归属
	mi, err := db.GetMission(r.Context(), m.db, id, uid)
	if err != nil {
		writeError(w, http.StatusNotFound, "mission_not_found")
		return
	}
	// 正在跑的话先撤离，停掉 goroutine
	if m.runner.IsRunning(id) {
		m.runner.Abort(id)
	}
	// 数据库
	if err := db.DeleteMission(r.Context(), m.db, id, uid); err != nil {
		writeError(w, http.StatusInternalServerError, "db_error")
		return
	}
	// workspace 目录（best-effort，失败不致命）
	if mi.WorkspaceDir != "" {
		_ = os.RemoveAll(mi.WorkspaceDir)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---- POST /missions/:id/follow_up ----
//
// 在某个已完成 mission 的基础上「继续安排」一份新任务。
// 新 mission 进入同一卷宗（series），series_seq 自增；并把上一行动的简报
// 与报告.html 作为上下文注入新一轮 LLM 调用（由 runner 在 ParentID 不空时处理）。

type followUpReq struct {
	Brief    string   `json:"brief"`
	Codename string   `json:"codename"`
	Tier     string   `json:"tier"`    // 可选，留空沿用 parent
	Loadout  []string `json:"loadout"` // 可选，留空沿用 parent
}

func (m *missionAPI) followUp(w http.ResponseWriter, r *http.Request) {
	uid, _ := userIDFrom(r)
	parentID := chi.URLParam(r, "id")
	parent, err := db.GetMission(r.Context(), m.db, parentID, uid)
	if err != nil {
		writeError(w, http.StatusNotFound, "mission_not_found")
		return
	}
	if !isTerminal(parent.Status) {
		writeError(w, http.StatusBadRequest, "parent_not_finished")
		return
	}
	var req followUpReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	req.Brief = strings.TrimSpace(req.Brief)
	req.Codename = strings.TrimSpace(req.Codename)
	if req.Brief == "" {
		writeError(w, http.StatusBadRequest, "brief_required")
		return
	}
	if req.Codename == "" {
		req.Codename = parent.Codename + " · 续"
	}
	tier := normalizeTier(req.Tier)
	if req.Tier == "" {
		tier = parent.Tier
	}
	var loadout []string
	if len(req.Loadout) > 0 {
		loadout = m.sanitizeLoadout(req.Loadout)
	} else {
		loadout = append(loadout, parent.Loadout...)
	}
	if len(loadout) == 0 {
		writeError(w, http.StatusBadRequest, "loadout_required")
		return
	}

	nextSeq, err := db.NextSeriesSeq(r.Context(), m.db, parent.SeriesID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error")
		return
	}

	missionID := uuid.NewString()
	parentRef := parent.ID
	mission := &model.Mission{
		ID:           missionID,
		UserID:       uid,
		Codename:     req.Codename,
		Brief:        req.Brief,
		Tier:         tier,
		Status:       model.StatusPending,
		Loadout:      loadout,
		WorkspaceDir: m.runner.MissionWorkspace(missionID),
		SeriesID:     parent.SeriesID,
		SeriesSeq:    nextSeq,
		ParentID:     &parentRef,
		CreatedAt:    time.Now().UTC(),
	}
	if err := db.CreateMission(r.Context(), m.db, mission); err != nil {
		m.logger.Error("create follow-up failed", "err", err)
		writeError(w, http.StatusInternalServerError, "db_error")
		return
	}
	if err := m.runner.Start(mission); err != nil {
		m.logger.Error("start follow-up failed", "err", err)
		writeError(w, http.StatusInternalServerError, "start_failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"mission": mission})
}

// ---- GET /missions/:id/series ----
// 取该 mission 所在卷宗下的全部 mission（含自己），按序号升序。
func (m *missionAPI) series(w http.ResponseWriter, r *http.Request) {
	uid, _ := userIDFrom(r)
	id := chi.URLParam(r, "id")
	mi, err := db.GetMission(r.Context(), m.db, id, uid)
	if err != nil {
		writeError(w, http.StatusNotFound, "mission_not_found")
		return
	}
	items, err := db.ListMissionsBySeries(r.Context(), m.db, mi.SeriesID, uid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "series_id": mi.SeriesID})
}

// ---- POST /missions/:id/abort ----

func (m *missionAPI) abort(w http.ResponseWriter, r *http.Request) {
	uid, _ := userIDFrom(r)
	id := chi.URLParam(r, "id")
	if _, err := db.GetMission(r.Context(), m.db, id, uid); err != nil {
		writeError(w, http.StatusNotFound, "mission_not_found")
		return
	}
	m.runner.Abort(id)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---- GET /missions/:id/stream  (SSE) ----

func (m *missionAPI) stream(w http.ResponseWriter, r *http.Request) {
	uid, _ := userIDFrom(r)
	id := chi.URLParam(r, "id")
	if _, err := db.GetMission(r.Context(), m.db, id, uid); err != nil {
		writeError(w, http.StatusNotFound, "mission_not_found")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming_unsupported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // 让 nginx/caddy 不缓冲
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	// 先订阅 broker（防止订阅前发生的事件被漏）
	sub := m.broker.Subscribe(id)
	defer m.broker.Unsubscribe(id, sub)

	// 然后从 DB 拉历史，按 seq 回放
	afterSeq, _ := strconv.Atoi(r.URL.Query().Get("after_seq"))
	hist, err := db.ListSteps(r.Context(), m.db, id, afterSeq, 0)
	if err == nil {
		for _, s := range hist {
			if !writeSSEStep(w, s) {
				return
			}
			afterSeq = s.Seq
		}
		flusher.Flush()
	}

	// 心跳 ticker，防代理 idle 超时
	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case step, ok := <-sub:
			if !ok {
				// mission 收尾，broker 已关闭，发完一条 done 后退出
				fmt.Fprintf(w, "event: closed\ndata: {}\n\n")
				flusher.Flush()
				return
			}
			// broker 可能把订阅前的历史事件也推过来；去重以 seq 为准
			if step.Seq <= afterSeq {
				continue
			}
			if !writeSSEStep(w, step) {
				return
			}
			afterSeq = step.Seq
			flusher.Flush()
		case <-heartbeat.C:
			fmt.Fprint(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}

// writeSSEStep 序列化一条 Step 并写到 SSE。
// 返回 false 表示写入失败（连接断开），调用方应停止循环。
func writeSSEStep(w http.ResponseWriter, s *model.Step) bool {
	raw, err := json.Marshal(s)
	if err != nil {
		return true // 单条失败不致命
	}
	_, err = fmt.Fprintf(w, "event: step\nid: %d\ndata: %s\n\n", s.Seq, raw)
	return err == nil
}

// ---- GET /missions/:id/artifacts/:aid/content  (返回工件 raw 内容) ----

func (m *missionAPI) artifactContent(w http.ResponseWriter, r *http.Request) {
	uid, _ := userIDFrom(r)
	missionID := chi.URLParam(r, "id")
	aidStr := chi.URLParam(r, "aid")
	aid, err := strconv.ParseInt(aidStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_artifact_id")
		return
	}
	if _, err := db.GetMission(r.Context(), m.db, missionID, uid); err != nil {
		writeError(w, http.StatusNotFound, "mission_not_found")
		return
	}
	art, err := db.GetArtifact(r.Context(), m.db, missionID, aid)
	if err != nil {
		writeError(w, http.StatusNotFound, "artifact_not_found")
		return
	}

	workspaceDir := m.runner.MissionWorkspace(missionID)
	full := art.Path
	if !filepath.IsAbs(full) {
		full = filepath.Join(workspaceDir, full)
	}
	rel, err := filepath.Rel(workspaceDir, full)
	if err != nil || strings.HasPrefix(rel, "..") {
		writeError(w, http.StatusForbidden, "forbidden_path")
		return
	}

	data, err := os.ReadFile(full)
	if err != nil {
		writeError(w, http.StatusNotFound, "file_not_found")
		return
	}
	ctype := art.Mime
	if ctype == "" {
		ctype = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Content-Disposition", `inline; filename="`+art.Name+`"`)
	w.Header().Set("X-Artifact-Name", art.Name)
	w.Header().Set("X-Artifact-Kind", art.Kind)
	_, _ = w.Write(data)
}

// ---- GET /tools  (列出注册的装备，用于派遣页) ----

type toolDescDTO struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
}

func (m *missionAPI) listTools(w http.ResponseWriter, _ *http.Request) {
	all := m.registry.All()
	out := make([]toolDescDTO, 0, len(all))
	for _, t := range all {
		out = append(out, toolDescDTO{
			Name:        t.Name(),
			DisplayName: t.DisplayName(),
			Description: t.Description(),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

// ---- helpers ----

func normalizeTier(t string) model.MissionTier {
	switch model.MissionTier(t) {
	case model.TierFlash, model.TierStandard, model.TierPro:
		return model.MissionTier(t)
	default:
		return model.TierStandard
	}
}

func (m *missionAPI) sanitizeLoadout(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, name := range in {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, dup := seen[name]; dup {
			continue
		}
		if _, ok := m.registry.Get(name); !ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}
