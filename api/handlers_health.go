package main

import (
	"net/http"
	"time"

	"github.com/bylexus/linkomat/shared"
)

type healthResponse struct {
	Status     string `json:"status"`
	Version    string `json:"version"`
	ServerTime string `json:"server_time"`
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{
		Status:     "ok",
		Version:    shared.Version,
		ServerTime: time.Now().UTC().Format(time.RFC3339),
	})
}