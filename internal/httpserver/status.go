package httpserver

import (
	"context"
	"net/http"
	"time"
)

type statusResponse struct {
	Connected     bool      `json:"connected"`
	DockerVersion string    `json:"dockerVersion,omitempty"`
	APIVersion    string    `json:"apiVersion,omitempty"`
	CheckedAt     time.Time `json:"checkedAt"`
	Message       string    `json:"message,omitempty"`
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "service": "caroline"})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	version, err := s.docker.Check(ctx)
	status := statusResponse{
		Connected:     err == nil,
		DockerVersion: version.Version,
		APIVersion:    version.APIVersion,
		CheckedAt:     time.Now().UTC(),
	}
	if err != nil {
		status.Message = "Docker daemon is unavailable"
	}
	writeJSON(w, http.StatusOK, status)
}
