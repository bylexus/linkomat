package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"
)

type ctxKey int

const (
	ctxKeyRequestID ctxKey = iota
	ctxKeyUserID
	ctxKeyRemoteIP
)

type Server struct {
	cfg     *Config
	meta    *MetaDB
	limiter *loginLimiter
	mux     *http.ServeMux
}

func NewServer(cfg *Config, meta *MetaDB) *Server {
	s := &Server{
		cfg:     cfg,
		meta:    meta,
		limiter: newLoginLimiter(),
		mux:     http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.mux.Handle("POST /auth/login", http.HandlerFunc(s.handleLogin))
	s.mux.Handle("POST /auth/refresh", http.HandlerFunc(s.handleRefresh))
	s.mux.Handle("POST /auth/logout", http.HandlerFunc(s.handleLogout))
}

func (s *Server) Handler() http.Handler {
	return s.middleware(s.mux)
}

func (s *Server) ListenAndServe() error {
	srv := &http.Server{
		Addr:              s.cfg.Listen,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	if s.cfg.TLSCert != "" && s.cfg.TLSKey != "" {
		return srv.ListenAndServeTLS(s.cfg.TLSCert, s.cfg.TLSKey)
	}
	return srv.ListenAndServe()
}

func (s *Server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		reqID := newRequestID()
		ctx := context.WithValue(r.Context(), ctxKeyRequestID, reqID)
		ip := clientIP(r, s.cfg.BehindProxy)
		ctx = context.WithValue(ctx, ctxKeyRemoteIP, ip)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Request-Id", reqID)
		rw := &statusRecorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(rw, r.WithContext(ctx))
		slog.Info("request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", rw.status),
			slog.String("user_id", userIDFromCtx(r.Context())),
			slog.Int64("duration_ms", time.Since(start).Milliseconds()),
			slog.String("request_id", reqID),
			slog.String("remote_ip", ip),
		)
	})
}

func clientIP(r *http.Request, behindProxy bool) string {
	if behindProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			for i := len(xff) - 1; i >= 0; i-- {
				if xff[i] == ',' {
					return trimSpace(xff[i+1:])
				}
			}
			return trimSpace(xff)
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == ',') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ') {
		s = s[:len(s)-1]
	}
	return s
}

func newRequestID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(b)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func userIDFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyUserID).(uint)
	if v == 0 {
		return ""
	}
	return strconv.FormatUint(uint64(v), 10)
}

func requestIDFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyRequestID).(string)
	return v
}

func remoteIPFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyRemoteIP).(string)
	return v
}

var _ = errors.New