package httpserver

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"caroline/internal/node"
)

func (s *Server) handleNodes(w http.ResponseWriter, r *http.Request) {
	if s.nodes == nil {
		writeError(w, http.StatusServiceUnavailable, "node management is unavailable")
		return
	}
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		values, err := s.nodes.List(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, values)
	case http.MethodPost:
		var request struct {
			TTLSeconds int `json:"ttlSeconds"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024)).Decode(&request); err != nil && !errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, "invalid enrollment request")
			return
		}
		ttl := 15 * time.Minute
		if request.TTLSeconds > 0 {
			ttl = time.Duration(request.TTLSeconds) * time.Second
		}
		plain, token, err := s.nodes.CreateEnrollmentToken(r.Context(), ttl)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{
			"token": plain, "enrollment": token,
			"enrollmentUrl": buildEnrollmentURL(r, plain),
		})
	default:
		w.Header().Set("Allow", "GET, HEAD, POST")
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func buildEnrollmentURL(r *http.Request, token string) string {
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("CAROLINE_PUBLIC_URL")), "/")
	if base == "" {
		protocol := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0])
		if protocol != "http" && protocol != "https" {
			protocol = "http"
			if r.TLS != nil {
				protocol = "https"
			}
		}
		if strings.TrimSpace(r.Host) == "" {
			return ""
		}
		base = protocol + "://" + r.Host
	}
	return base + "/api/v1/agent/enroll/" + url.PathEscape(token)
}

func (s *Server) handleNode(w http.ResponseWriter, r *http.Request) {
	if s.nodes == nil {
		writeError(w, http.StatusServiceUnavailable, "node management is unavailable")
		return
	}
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/nodes/"), "/")
	if id == "" || strings.Contains(id, "/") {
		writeError(w, http.StatusNotFound, "node was not found")
		return
	}
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		value, err := s.nodes.Get(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, value)
	case http.MethodDelete:
		if err := s.nodes.Revoke(r.Context(), id); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"id": id, "status": node.StatusRevoked})
	default:
		w.Header().Set("Allow", "GET, HEAD, DELETE")
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}
