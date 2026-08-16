package main

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/bylexus/linkomat/shared"
	"gorm.io/gorm"
)

func (s *Server) authenticate(w http.ResponseWriter, r *http.Request) *http.Request {
	if !needsAuth(r.URL.Path) {
		return r
	}
	token := extractAccessToken(r)
	if token == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required.", "")
		return nil
	}
	tok, err := s.meta.FindActiveToken(token, TokenTypeAccess)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusUnauthorized, "invalid-token", "Access token is invalid or expired.", "")
			return nil
		}
		writeError(w, http.StatusInternalServerError, "internal", "Could not look up token.", err.Error())
		return nil
	}
	var u User
	if err := s.meta.First(&u, tok.UserID).Error; err != nil {
		writeError(w, http.StatusUnauthorized, "invalid-token", "Access token is invalid or expired.", "")
		return nil
	}
	if !u.Active {
		writeError(w, http.StatusForbidden, "account-deactivated", "Account is deactivated.", "")
		return nil
	}
	ctx := context.WithValue(r.Context(), ctxKeyUserID, u.ID)
	ctx = context.WithValue(ctx, ctxKeyUsername, u.Username)
	return r.WithContext(ctx)
}

func needsAuth(path string) bool {
	return strings.HasPrefix(path, "/entries")
}

func extractAccessToken(r *http.Request) string {
	if v, ok := readCookie(r, shared.CookieAuth); ok {
		return v
	}
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
	}
	return ""
}
