package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/agentzero/server/internal/auth"
	"github.com/agentzero/server/internal/db"
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
		nickname = "代号零特工"
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
		if errors.Is(err, db.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user_not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "db_error")
		return
	}
	writeJSON(w, http.StatusOK, u)
}
