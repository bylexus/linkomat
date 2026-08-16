package main

import (
	"net/http"
	"time"

	"github.com/bylexus/linkomat/shared"
)

func setAuthCookies(w http.ResponseWriter, cfg *Config, access, refresh string, accessExpires time.Time, refreshExpires time.Time) {
	accessCookie := &http.Cookie{
		Name:     shared.CookieAuth,
		Value:    access,
		Path:     shared.CookiePathAuth,
		Expires:  accessExpires,
		HttpOnly: true,
		Secure:   cfg.CookieSecure,
		SameSite: http.SameSiteStrictMode,
	}
	refreshCookie := &http.Cookie{
		Name:     shared.CookieRefresh,
		Value:    refresh,
		Path:     shared.CookiePathRefresh,
		Expires:  refreshExpires,
		HttpOnly: true,
		Secure:   cfg.CookieSecure,
		SameSite: http.SameSiteStrictMode,
	}
	http.SetCookie(w, accessCookie)
	http.SetCookie(w, refreshCookie)
}

func clearAuthCookies(w http.ResponseWriter, cfg *Config) {
	past := time.Unix(0, 0)
	accessCookie := &http.Cookie{
		Name:     shared.CookieAuth,
		Value:    "",
		Path:     shared.CookiePathAuth,
		Expires:  past,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   cfg.CookieSecure,
		SameSite: http.SameSiteStrictMode,
	}
	refreshCookie := &http.Cookie{
		Name:     shared.CookieRefresh,
		Value:    "",
		Path:     shared.CookiePathRefresh,
		Expires:  past,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   cfg.CookieSecure,
		SameSite: http.SameSiteStrictMode,
	}
	http.SetCookie(w, accessCookie)
	http.SetCookie(w, refreshCookie)
}

func readCookie(r *http.Request, name string) (string, bool) {
	c, err := r.Cookie(name)
	if err != nil {
		return "", false
	}
	return c.Value, true
}