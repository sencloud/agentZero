package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/agentzero/server/internal/db"
	"github.com/agentzero/server/internal/model"
)

type skillReq struct {
	Name            string  `json:"name"`
	Description     string  `json:"description"`
	TriggerHint     string  `json:"trigger_hint"`
	PromptTemplate  string  `json:"prompt_template"`
	SourceMissionID *string `json:"source_mission_id,omitempty"`
}

// POST /skills
// 新增一项技能。若指定 source_mission_id，则校验该 mission 必须属于当前用户。
func (m *missionAPI) createSkill(w http.ResponseWriter, r *http.Request) {
	uid, _ := userIDFrom(r)
	var req skillReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name_required")
		return
	}
	if req.SourceMissionID != nil && *req.SourceMissionID != "" {
		if _, err := db.GetMission(r.Context(), m.db, *req.SourceMissionID, uid); err != nil {
			writeError(w, http.StatusBadRequest, "source_mission_not_found")
			return
		}
	}
	s := &model.Skill{
		UserID:          uid,
		Name:            req.Name,
		Description:     strings.TrimSpace(req.Description),
		TriggerHint:     strings.TrimSpace(req.TriggerHint),
		PromptTemplate:  strings.TrimSpace(req.PromptTemplate),
		SourceMissionID: req.SourceMissionID,
	}
	if err := db.CreateSkill(r.Context(), m.db, s); err != nil {
		m.logger.Error("create skill failed", "err", err)
		writeError(w, http.StatusInternalServerError, "db_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"skill": s})
}

// GET /skills
func (m *missionAPI) listSkills(w http.ResponseWriter, r *http.Request) {
	uid, _ := userIDFrom(r)
	items, err := db.ListSkills(r.Context(), m.db, uid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
