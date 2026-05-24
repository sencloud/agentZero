package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"

	"github.com/agentzero/server/internal/db"
	"github.com/agentzero/server/internal/feed"
	"github.com/go-chi/chi/v5"
)

// briefingAPI 聚合所有 /briefings 端点。
type briefingAPI struct {
	*Handlers
	svc *feed.Service
}

// GET /briefings?limit=30
func (a *briefingAPI) list(w http.ResponseWriter, r *http.Request) {
	uid, _ := userIDFrom(r)
	limit := 30
	if s := r.URL.Query().Get("limit"); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			limit = v
		}
	}
	rows, err := db.ListBriefings(r.Context(), a.db, uid, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"briefings": rows})
}

// GET /briefings/:id
func (a *briefingAPI) detail(w http.ResponseWriter, r *http.Request) {
	uid, _ := userIDFrom(r)
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	b, err := db.GetBriefing(r.Context(), a.db, id, uid)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeError(w, http.StatusNotFound, "briefing_not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "get_failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, b)
}

// GET /briefings/:id/html
// 直接以 text/html 返回 HTML 文档原文，供客户端 WebView 加载。
func (a *briefingAPI) html(w http.ResponseWriter, r *http.Request) {
	uid, _ := userIDFrom(r)
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	b, err := db.GetBriefing(r.Context(), a.db, id, uid)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeError(w, http.StatusNotFound, "briefing_not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "get_failed: "+err.Error())
		return
	}
	if b.HTMLPath == "" {
		writeError(w, http.StatusGone, "html_missing")
		return
	}
	buf, err := os.ReadFile(b.HTMLPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "read_html_failed: "+err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf)
}

// GET /briefings/generate/stream?window=1h
// 同步驱动一次简报生成，把每个阶段进度以 SSE 推回客户端。
func (a *briefingAPI) generateStream(w http.ResponseWriter, r *http.Request) {
	uid, _ := userIDFrom(r)
	window := r.URL.Query().Get("window")
	if window == "" {
		window = "1h"
	}
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

	send := func(payload any) {
		buf, err := json.Marshal(payload)
		if err != nil {
			return
		}
		_, _ = w.Write([]byte("data: "))
		_, _ = w.Write(buf)
		_, _ = w.Write([]byte("\n\n"))
		flusher.Flush()
	}

	send(map[string]any{"phase": "start", "message": "开始生成简报 · " + window})

	b, err := a.svc.GenerateBriefingNow(r.Context(), uid, window, func(p feed.AnalyzeProgress) {
		send(map[string]any{
			"phase":   p.Phase,
			"message": p.Message,
			"data":    p.Data,
		})
	})
	if err != nil {
		send(map[string]any{"phase": "error", "message": err.Error()})
		return
	}
	send(map[string]any{
		"phase":   "done",
		"message": "简报已生成 · #" + strconv.FormatInt(b.ID, 10),
		"data": map[string]any{
			"briefing_id":   b.ID,
			"title":         b.Title,
			"summary":       b.Summary,
			"event_count":   b.EventCount,
			"cluster_count": b.ClusterCount,
		},
	})
}
