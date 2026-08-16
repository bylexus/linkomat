package main

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/bylexus/linkomat/shared"
	"gorm.io/gorm"
)

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store, private")
	var req loginReq
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid-request", "Could not parse request body.", err.Error())
		return
	}
	username := strings.TrimSpace(req.Username)
	if username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "invalid-request", "Username and password are required.", "")
		return
	}

	ip := remoteIPFromCtx(r.Context())
	key := ip + "|" + strings.ToLower(username)
	if !s.limiter.allow(key) {
		writeError(w, http.StatusTooManyRequests, "rate-limited", "Too many login attempts. Try again later.", "")
		return
	}

	var u User
	err := s.meta.Where("username = ?", username).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		slog.Info("auth",
			slog.String("event", "login"),
			slog.String("outcome", "no-user"),
			slog.String("username", username),
			slog.String("request_id", requestIDFromCtx(r.Context())),
		)
		writeError(w, http.StatusUnauthorized, "invalid-credentials", "Invalid username or password.", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Could not look up user.", err.Error())
		return
	}
	if !u.Active {
		slog.Info("auth",
			slog.String("event", "login"),
			slog.String("outcome", "deactivated"),
			slog.String("username", username),
			slog.String("request_id", requestIDFromCtx(r.Context())),
		)
		writeError(w, http.StatusForbidden, "account-deactivated", "Account is deactivated.", "")
		return
	}
	ok, err := verifyPassword(req.Password, u.PasswordHash)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Password verification failed.", err.Error())
		return
	}
	if !ok {
		slog.Info("auth",
			slog.String("event", "login"),
			slog.String("outcome", "bad-password"),
			slog.String("username", username),
			slog.String("request_id", requestIDFromCtx(r.Context())),
		)
		writeError(w, http.StatusUnauthorized, "invalid-credentials", "Invalid username or password.", "")
		return
	}

	access, refresh, err := s.meta.IssueTokenPair(u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Could not issue tokens.", err.Error())
		return
	}
	accessExpires := time.Now().Add(time.Duration(s.cfg.AccessTTL) * time.Second)
	refreshExpires := time.Now().Add(time.Duration(s.cfg.RefreshTTL) * time.Second)
	setAuthCookies(w, s.cfg, access, refresh, accessExpires, refreshExpires)

	slog.Info("auth",
		slog.String("event", "login"),
		slog.String("outcome", "success"),
		slog.Uint64("user_id", uint64(u.ID)),
		slog.String("username", username),
		slog.String("request_id", requestIDFromCtx(r.Context())),
	)

	writeJSON(w, http.StatusOK, shared.AuthResponse{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    s.cfg.AccessTTL,
		Username:     u.Username,
	})
}

type refreshReq struct {
	RefreshToken string `json:"refresh_token"`
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store, private")
	refreshValue := readRefreshToken(r)
	if refreshValue == "" {
		writeError(w, http.StatusBadRequest, "invalid-request", "Refresh token is required.", "")
		return
	}
	tok, err := s.meta.FindActiveToken(refreshValue, TokenTypeRefresh)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusUnauthorized, "invalid-token", "Refresh token is invalid or expired.", "")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "Could not look up token.", err.Error())
		return
	}
	var u User
	if err := s.meta.First(&u, tok.UserID).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Could not look up user.", err.Error())
		return
	}
	if !u.Active {
		_ = s.meta.RevokeAllUserTokens(u.ID)
		writeError(w, http.StatusForbidden, "account-deactivated", "Account is deactivated.", "")
		return
	}
	if err := s.meta.RevokeToken(refreshValue); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Could not revoke old token.", err.Error())
		return
	}
	access, newRefresh, err := s.meta.IssueTokenPair(u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Could not issue tokens.", err.Error())
		return
	}
	accessExpires := time.Now().Add(time.Duration(s.cfg.AccessTTL) * time.Second)
	refreshExpires := time.Now().Add(time.Duration(s.cfg.RefreshTTL) * time.Second)
	setAuthCookies(w, s.cfg, access, newRefresh, accessExpires, refreshExpires)

	slog.Info("auth",
		slog.String("event", "refresh"),
		slog.String("outcome", "success"),
		slog.Uint64("user_id", uint64(u.ID)),
		slog.String("request_id", requestIDFromCtx(r.Context())),
	)

	writeJSON(w, http.StatusOK, shared.AuthResponse{
		AccessToken:  access,
		RefreshToken: newRefresh,
		ExpiresIn:    s.cfg.AccessTTL,
		Username:     u.Username,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store, private")
	refreshValue := readRefreshToken(r)
	if refreshValue != "" {
		if tok, err := s.meta.FindActiveToken(refreshValue, TokenTypeRefresh); err == nil {
			_ = s.meta.RevokeToken(refreshValue)
			slog.Info("auth",
				slog.String("event", "logout"),
				slog.String("outcome", "success"),
				slog.Uint64("user_id", uint64(tok.UserID)),
				slog.String("request_id", requestIDFromCtx(r.Context())),
			)
		}
	}
	clearAuthCookies(w, s.cfg)
	w.WriteHeader(http.StatusNoContent)
}

func readRefreshToken(r *http.Request) string {
	if v, ok := readCookie(r, shared.CookieRefresh); ok {
		return v
	}
	var req refreshReq
	if err := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 4096)).Decode(&req); err == nil {
		return req.RefreshToken
	}
	return ""
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, errType, title, detail string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(shared.ErrorResponse{
		Type:   "about:blank#" + errType,
		Title:  title,
		Status: status,
		Detail: detail,
	})
}