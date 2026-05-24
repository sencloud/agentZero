package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/agentzero/server/internal/db"
	"github.com/agentzero/server/internal/feed"
	"github.com/agentzero/server/internal/model"
	"github.com/go-chi/chi/v5"
)

// feedAPI 聚合事件流相关的 HTTP 处理器。
type feedAPI struct {
	*Handlers
	svc *feed.Service
}

// GET /api/v1/feed/status
func (a *feedAPI) status(w http.ResponseWriter, r *http.Request) {
	st, err := a.svc.Status(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "status_failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// POST /api/v1/feed/refresh
// 异步触发一次抓取（不阻塞调用方），主要给客户端"立刻刷新"按钮用。
func (a *feedAPI) refresh(w http.ResponseWriter, _ *http.Request) {
	a.svc.TriggerFetchNow()
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "queued"})
}

// GET /api/v1/feed/refresh/stream
// 同步驱动一轮刷新并以 SSE 的方式把每个阶段的进度推给前端。
func (a *feedAPI) refreshStream(w http.ResponseWriter, r *http.Request) {
	uid, _ := userIDFrom(r)
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming_unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	a.svc.RunRefreshStream(r.Context(), uid, func(ev feed.RefreshEvent) bool {
		buf, err := json.Marshal(ev)
		if err != nil {
			return false
		}
		if _, err := w.Write([]byte("data: ")); err != nil {
			return false
		}
		if _, err := w.Write(buf); err != nil {
			return false
		}
		if _, err := w.Write([]byte("\n\n")); err != nil {
			return false
		}
		flusher.Flush()
		return r.Context().Err() == nil
	})
}

// GET /api/v1/feed/topics
func (a *feedAPI) listTopics(w http.ResponseWriter, r *http.Request) {
	uid, _ := userIDFrom(r)
	ts, err := db.ListTopics(r.Context(), a.db, uid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_topics_failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"topics": ts})
}

// POST /api/v1/feed/topics  body: {name: string, weight?: number}
func (a *feedAPI) createTopic(w http.ResponseWriter, r *http.Request) {
	uid, _ := userIDFrom(r)
	var body struct {
		Name   string  `json:"name"`
		Weight float64 `json:"weight"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body: "+err.Error())
		return
	}
	if body.Name == "" {
		writeError(w, http.StatusBadRequest, "name_required")
		return
	}
	t := &model.Topic{UserID: uid, Name: body.Name, Weight: body.Weight}
	if err := db.CreateTopic(r.Context(), a.db, t); err != nil {
		writeError(w, http.StatusInternalServerError, "create_topic_failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

// DELETE /api/v1/feed/topics/{id}
func (a *feedAPI) deleteTopic(w http.ResponseWriter, r *http.Request) {
	uid, _ := userIDFrom(r)
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	if err := db.DeleteTopic(r.Context(), a.db, id, uid); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeError(w, http.StatusNotFound, "topic_not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "delete_failed: "+err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /api/v1/feed/events?limit=50
func (a *feedAPI) listEvents(w http.ResponseWriter, r *http.Request) {
	uid, _ := userIDFrom(r)
	limit := 50
	if s := r.URL.Query().Get("limit"); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			limit = v
		}
	}
	rows, err := db.ListUserFeedEvents(r.Context(), a.db, uid, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_events_failed: "+err.Error())
		return
	}
	type wireEvent struct {
		ID            int64   `json:"id"`
		URL           string  `json:"url"`
		Title         string  `json:"title"`
		Summary       string  `json:"summary"`
		Lang          string  `json:"lang"`
		PublishedAt   *string `json:"published_at,omitempty"`
		FetchedAt     string  `json:"fetched_at"`
		SourceID      int64   `json:"source_id"`
		SourceName    string  `json:"source_name"`
		Relevance     float64 `json:"relevance"`
		MatchedTopics []int64 `json:"matched_topics"`
	}
	out := make([]wireEvent, 0, len(rows))
	for _, row := range rows {
		ev := wireEvent{
			ID:            row.Event.ID,
			URL:           row.Event.URL,
			Title:         row.Event.Title,
			Summary:       row.Event.Summary,
			Lang:          row.Event.Lang,
			FetchedAt:     row.Event.FetchedAt.UTC().Format(timeFmtRFC3339),
			SourceID:      row.Source.ID,
			SourceName:    row.Source.Name,
			Relevance:     row.Relevance,
			MatchedTopics: row.MatchedTopics,
		}
		if row.Event.PublishedAt != nil {
			s := row.Event.PublishedAt.UTC().Format(timeFmtRFC3339)
			ev.PublishedAt = &s
		}
		out = append(out, ev)
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": out})
}

// GET /api/v1/feed/graph?limit=80
func (a *feedAPI) graph(w http.ResponseWriter, r *http.Request) {
	uid, _ := userIDFrom(r)
	limit := 80
	if s := r.URL.Query().Get("limit"); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			limit = v
		}
	}
	snap, err := db.ListGraphForUser(r.Context(), a.db, uid, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "graph_failed: "+err.Error())
		return
	}
	type wireNode struct {
		ID         int64   `json:"id"`
		Type       string  `json:"type"`
		Name       string  `json:"name"`
		Weight     float64 `json:"weight"`
		LastSeenAt string  `json:"last_seen_at"`
	}
	type wireEdge struct {
		ID         int64   `json:"id"`
		SrcID      int64   `json:"src_id"`
		DstID      int64   `json:"dst_id"`
		Label      string  `json:"label"`
		Weight     float64 `json:"weight"`
		LastSeenAt string  `json:"last_seen_at"`
	}
	nodes := make([]wireNode, 0, len(snap.Nodes))
	for _, n := range snap.Nodes {
		nodes = append(nodes, wireNode{
			ID: n.ID, Type: n.Type, Name: n.Name, Weight: n.Weight,
			LastSeenAt: n.LastSeenAt.UTC().Format(timeFmtRFC3339),
		})
	}
	edges := make([]wireEdge, 0, len(snap.Edges))
	for _, e := range snap.Edges {
		edges = append(edges, wireEdge{
			ID: e.ID, SrcID: e.SrcID, DstID: e.DstID, Label: e.Label, Weight: e.Weight,
			LastSeenAt: e.LastSeenAt.UTC().Format(timeFmtRFC3339),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"nodes": nodes, "edges": edges})
}

// GET /api/v1/feed/sources
// 列出全部新闻源（含 enabled 状态、category、description），给客户端
// 「源库管理」用。
func (a *feedAPI) listSources(w http.ResponseWriter, r *http.Request) {
	srcs, err := db.ListNewsSources(r.Context(), a.db, false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_sources_failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sources": srcs})
}

// POST /api/v1/feed/sources/toggle  body: {ids: [int64], enabled: bool}
// 批量启用 / 禁用源。
func (a *feedAPI) toggleSources(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs     []int64 `json:"ids"`
		Enabled bool    `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body: "+err.Error())
		return
	}
	if len(body.IDs) == 0 {
		writeError(w, http.StatusBadRequest, "ids_required")
		return
	}
	if err := db.SetSourceEnabled(r.Context(), a.db, body.IDs, body.Enabled); err != nil {
		writeError(w, http.StatusInternalServerError, "toggle_failed: "+err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /api/v1/feed/sources/recommend
// 让 LLM 按当前用户的话题做一次智能选源，返回新启用的源列表。
func (a *feedAPI) recommendSources(w http.ResponseWriter, r *http.Request) {
	uid, _ := userIDFrom(r)
	rec, err := a.svc.Recommend(r.Context(), uid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "recommend_failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"newly_enabled": rec.NewlyEnabled,
		"already_on":    rec.AlreadyOn,
		"reason":        rec.Reason,
	})
}

const timeFmtRFC3339 = "2006-01-02T15:04:05Z07:00"
