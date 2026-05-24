package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/agentzero/server/internal/db"
	"github.com/agentzero/server/internal/model"
	"github.com/go-chi/chi/v5"
)

type reviewReq struct {
	Rating  int    `json:"rating"`
	Comment string `json:"comment"`
}

// POST /missions/:id/review
// 写入或更新一次行动点评。要求 mission 已进入终态，跨用户访问 404。
func (m *missionAPI) postReview(w http.ResponseWriter, r *http.Request) {
	uid, _ := userIDFrom(r)
	id := chi.URLParam(r, "id")
	mi, err := db.GetMission(r.Context(), m.db, id, uid)
	if err != nil {
		writeError(w, http.StatusNotFound, "mission_not_found")
		return
	}
	if !isTerminal(mi.Status) {
		writeError(w, http.StatusBadRequest, "mission_not_finished")
		return
	}
	var req reviewReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	if req.Rating < 1 || req.Rating > 5 {
		writeError(w, http.StatusBadRequest, "rating_out_of_range")
		return
	}
	review := &model.Review{
		MissionID: id,
		Rating:    req.Rating,
		Comment:   strings.TrimSpace(req.Comment),
	}
	if err := db.UpsertReview(r.Context(), m.db, review); err != nil {
		m.logger.Error("upsert review failed", "err", err)
		writeError(w, http.StatusInternalServerError, "db_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"review": review})
}

// GET /missions/:id/review
func (m *missionAPI) getReview(w http.ResponseWriter, r *http.Request) {
	uid, _ := userIDFrom(r)
	id := chi.URLParam(r, "id")
	if _, err := db.GetMission(r.Context(), m.db, id, uid); err != nil {
		writeError(w, http.StatusNotFound, "mission_not_found")
		return
	}
	rv, err := db.GetReview(r.Context(), m.db, id)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeJSON(w, http.StatusOK, map[string]any{"review": nil})
			return
		}
		writeError(w, http.StatusInternalServerError, "db_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"review": rv})
}

func isTerminal(s model.MissionStatus) bool {
	switch s {
	case model.StatusDone, model.StatusAborted, model.StatusError:
		return true
	}
	return false
}
