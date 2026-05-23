package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/agentzero/server/internal/auth"
	"github.com/agentzero/server/internal/db"
	"github.com/agentzero/server/internal/service"
	"github.com/go-chi/chi/v5"
)

type Handlers struct {
	db     *sql.DB
	apple  *auth.AppleVerifier
	tokens *auth.TokenIssuer
	logger *slog.Logger
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]any{"error": msg})
}

type appleSignInReq struct {
	IdentityToken string `json:"identity_token"`
	FullName      string `json:"full_name"`
	Email         string `json:"email"`
}

type tokenResp struct {
	Token string `json:"token"`
	User  any    `json:"user"`
}

func (h *Handlers) appleSignIn(w http.ResponseWriter, r *http.Request) {
	var req appleSignInReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.IdentityToken == "" {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	claims, err := h.apple.Verify(req.IdentityToken)
	if err != nil {
		h.logger.Warn("apple verify failed", "err", err)
		writeError(w, http.StatusUnauthorized, "apple_verify_failed")
		return
	}
	email := req.Email
	if email == "" {
		email = claims.Email
	}
	nickname := strings.TrimSpace(req.FullName)
	if nickname == "" {
		nickname = "AgentZero 用户"
	}
	u, err := db.UpsertUserByApple(r.Context(), h.db, claims.Subject, email, nickname)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error")
		return
	}
	tk, _, err := h.tokens.Issue(u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token_issue_failed")
		return
	}
	writeJSON(w, http.StatusOK, tokenResp{Token: tk, User: u})
}

func (h *Handlers) me(w http.ResponseWriter, r *http.Request) {
	uid, _ := userIDFrom(r)
	u, err := db.GetUserByID(r.Context(), h.db, uid)
	if err != nil {
		writeError(w, http.StatusNotFound, "user_not_found")
		return
	}
	writeJSON(w, http.StatusOK, u)
}

func (h *Handlers) listCategories(w http.ResponseWriter, r *http.Request) {
	cs, err := db.ListCategories(r.Context(), h.db)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": cs})
}

func (h *Handlers) listAgents(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	uid, _ := userIDFrom(r)
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	items, err := db.ListAgents(r.Context(), h.db, db.AgentFilter{
		CategorySlug: q.Get("category"),
		Query:        q.Get("q"),
		Featured:     q.Get("featured") == "1",
		Sort:         q.Get("sort"),
		Limit:        limit,
		Offset:       offset,
		UserID:       uid,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handlers) getAgent(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	uid, _ := userIDFrom(r)
	a, err := db.GetAgent(r.Context(), h.db, slug, uid)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeError(w, http.StatusNotFound, "agent_not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "db_error")
		return
	}
	writeJSON(w, http.StatusOK, a)
}

func (h *Handlers) getToday(w http.ResponseWriter, r *http.Request) {
	cards, err := db.ListTodayCards(r.Context(), h.db)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error")
		return
	}
	uid, _ := userIDFrom(r)
	featured, err := db.ListAgents(r.Context(), h.db, db.AgentFilter{Featured: true, UserID: uid, Limit: 20})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"cards": cards, "featured": featured})
}

func (h *Handlers) install(w http.ResponseWriter, r *http.Request) {
	uid, _ := userIDFrom(r)
	slug := chi.URLParam(r, "slug")
	a, err := db.GetAgent(r.Context(), h.db, slug, uid)
	if err != nil {
		writeError(w, http.StatusNotFound, "agent_not_found")
		return
	}
	if err := db.Install(r.Context(), h.db, uid, a.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "db_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"installed": true})
}

func (h *Handlers) uninstall(w http.ResponseWriter, r *http.Request) {
	uid, _ := userIDFrom(r)
	slug := chi.URLParam(r, "slug")
	a, err := db.GetAgent(r.Context(), h.db, slug, uid)
	if err != nil {
		writeError(w, http.StatusNotFound, "agent_not_found")
		return
	}
	if err := db.Uninstall(r.Context(), h.db, uid, a.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "db_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"installed": false})
}

func (h *Handlers) listInstalled(w http.ResponseWriter, r *http.Request) {
	uid, _ := userIDFrom(r)
	items, err := db.ListInstalled(r.Context(), h.db, uid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handlers) listReviews(w http.ResponseWriter, r *http.Request) {
	uid, _ := userIDFrom(r)
	a, err := db.GetAgent(r.Context(), h.db, chi.URLParam(r, "slug"), uid)
	if err != nil {
		writeError(w, http.StatusNotFound, "agent_not_found")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	items, err := db.ListReviews(r.Context(), h.db, a.ID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "agent_rating": a.Rating, "agent_rating_count": a.RatingCount})
}

type reviewReq struct {
	Rating int    `json:"rating"`
	Title  string `json:"title"`
	Body   string `json:"body"`
}

func (h *Handlers) submitReview(w http.ResponseWriter, r *http.Request) {
	uid, _ := userIDFrom(r)
	var req reviewReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	if req.Rating < 1 || req.Rating > 5 {
		writeError(w, http.StatusBadRequest, "invalid_rating")
		return
	}
	a, err := db.GetAgent(r.Context(), h.db, chi.URLParam(r, "slug"), uid)
	if err != nil {
		writeError(w, http.StatusNotFound, "agent_not_found")
		return
	}
	if err := db.UpsertReview(r.Context(), h.db, a.ID, uid, req.Rating, req.Title, req.Body); err != nil {
		writeError(w, http.StatusInternalServerError, "db_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

type sessionReq struct {
	Title string `json:"title"`
}

func (h *Handlers) createSession(w http.ResponseWriter, r *http.Request) {
	uid, _ := userIDFrom(r)
	a, err := db.GetAgent(r.Context(), h.db, chi.URLParam(r, "slug"), uid)
	if err != nil {
		writeError(w, http.StatusNotFound, "agent_not_found")
		return
	}
	var req sessionReq
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Title == "" {
		req.Title = fmt.Sprintf("与 %s 的对话", a.Name)
	}
	s, err := db.CreateSession(r.Context(), h.db, uid, a.ID, req.Title)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error")
		return
	}
	greeting, _ := db.AddMessage(r.Context(), h.db, s.ID, "assistant",
		fmt.Sprintf("你好，我是 %s。%s\n\n你可以试着和我说说你的需求。", a.Name, a.Tagline))
	writeJSON(w, http.StatusOK, map[string]any{"session": s, "greeting": greeting})
}

func (h *Handlers) listSessions(w http.ResponseWriter, r *http.Request) {
	uid, _ := userIDFrom(r)
	items, err := db.ListSessions(r.Context(), h.db, uid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

type messageReq struct {
	Content string `json:"content"`
}

func (h *Handlers) sendMessage(w http.ResponseWriter, r *http.Request) {
	uid, _ := userIDFrom(r)
	sidStr := chi.URLParam(r, "id")
	sid, err := strconv.ParseInt(sidStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_session_id")
		return
	}
	var req messageReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Content) == "" {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}

	msgs, err := db.ListMessages(r.Context(), h.db, sid, uid)
	if err != nil {
		writeError(w, http.StatusNotFound, "session_not_found")
		return
	}
	_ = msgs

	userMsg, err := db.AddMessage(r.Context(), h.db, sid, "user", req.Content)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error")
		return
	}

	var session struct {
		AgentID int64
	}
	row := h.db.QueryRowContext(r.Context(), `SELECT agent_id FROM chat_sessions WHERE id=?`, sid)
	_ = row.Scan(&session.AgentID)
	agent, _ := db.GetAgentByID(r.Context(), h.db, session.AgentID)
	reply := service.MockReply(agent, req.Content)

	assistantMsg, err := db.AddMessage(r.Context(), h.db, sid, "assistant", reply)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": userMsg, "assistant": assistantMsg})
}

func (h *Handlers) listMessages(w http.ResponseWriter, r *http.Request) {
	uid, _ := userIDFrom(r)
	sid, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_session_id")
		return
	}
	items, err := db.ListMessages(r.Context(), h.db, sid, uid)
	if err != nil {
		writeError(w, http.StatusNotFound, "session_not_found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
