package main

import (
	"net/http"
	"time"

	"github.com/bylexus/linkomat/shared"
)

type versionResponse struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	ServerTime string `json:"server_time"`
}

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, versionResponse{
		Name:       "linkomat-server",
		Version:    shared.Version,
		ServerTime: time.Now().UTC().Format(time.RFC3339),
	})
}