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
	ctxKeyUsername
	ctxKeyRemoteIP
)

type Server struct {
	cfg     *Config
	meta    *MetaDB
	userDBs *UserDBs
	limiter *loginLimiter
	mux     *http.ServeMux
}

func NewServer(cfg *Config, meta *MetaDB, userDBs *UserDBs) *Server {
	s := &Server{
		cfg:     cfg,
		meta:    meta,
		userDBs: userDBs,
		limiter: newLoginLimiter(),
		mux:     http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	apiH := s.apiHeaders
	s.mux.Handle("POST /auth/login", apiH(http.HandlerFunc(s.handleLogin)))
	s.mux.Handle("POST /auth/refresh", apiH(http.HandlerFunc(s.handleRefresh)))
	s.mux.Handle("POST /auth/logout", apiH(http.HandlerFunc(s.handleLogout)))
	s.mux.Handle("GET /entries", apiH(http.HandlerFunc(s.handleEntriesList)))
	s.mux.Handle("GET /entries/{id}", apiH(http.HandlerFunc(s.handleEntryGet)))
	s.mux.Handle("PUT /entries/{id}", apiH(http.HandlerFunc(s.handleEntryPut)))
	s.mux.Handle("DELETE /entries/{id}", apiH(http.HandlerFunc(s.handleEntryDelete)))
	s.mux.Handle("GET /healthz", apiH(http.HandlerFunc(s.handleHealthz)))
	s.mux.Handle("GET /version", apiH(http.HandlerFunc(s.handleVersion)))
	s.mux.Handle("/", http.HandlerFunc(s.handleWebAssets))
}

func (s *Server) apiHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store, private")
		next.ServeHTTP(w, r)
	})
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
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'")
		rw := &statusRecorder{ResponseWriter: w, status: 200}
		ar := s.authenticate(rw, r.WithContext(ctx))
		if ar != nil {
			next.ServeHTTP(rw, ar)
		}
		userID := ""
		if ar != nil {
			userID = userIDFromCtx(ar.Context())
		}
		slog.Info("request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", rw.status),
			slog.String("user_id", userID),
			slog.Int64("duration_ms", time.Since(start).Milliseconds()),
			slog.String("request_id", reqID),
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

func usernameFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyUsername).(string)
	return v
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