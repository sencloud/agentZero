package api

import (
	"encoding/json"
	"fmt"
	"net/http"
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
