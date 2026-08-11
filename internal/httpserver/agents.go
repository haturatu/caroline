package httpserver

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"caroline/internal/agentproto"
	"caroline/internal/ingest"
	"caroline/internal/node"
	"github.com/klauspost/compress/zstd"
)

const (
	maxAgentRequestBytes = 8 * 1024 * 1024
	maxRegisterBytes     = 128 * 1024
	maxHeartbeatBytes    = 512 * 1024
)

func (s *Server) handleAgentRegister(w http.ResponseWriter, r *http.Request) {
	if s.nodes == nil {
		writeError(w, http.StatusServiceUnavailable, "agent registration is unavailable")
		return
	}
	body, err := readAgentBody(w, r, maxRegisterBytes)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var request agentproto.RegisterRequest
	if err := json.Unmarshal(body, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid registration payload")
		return
	}
	s.registerAgent(w, r, request)
}

func (s *Server) handleAgentEnroll(w http.ResponseWriter, r *http.Request) {
	if s.nodes == nil {
		writeError(w, http.StatusServiceUnavailable, "agent enrollment is unavailable")
		return
	}
	body, err := readAgentBody(w, r, maxRegisterBytes)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var request agentproto.RegisterRequest
	if err := json.Unmarshal(body, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid enrollment payload")
		return
	}
	token, err := enrollmentTokenFromPath(r.URL.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	request.EnrollmentToken = token
	s.registerAgent(w, r, request)
}

func (s *Server) registerAgent(w http.ResponseWriter, r *http.Request, request agentproto.RegisterRequest) {
	response, _, err := s.nodes.Register(r.Context(), request)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, node.ErrEnrollment) {
			status = http.StatusUnauthorized
		} else if errors.Is(err, node.ErrNodeRevoked) {
			status = http.StatusForbidden
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func enrollmentTokenFromPath(path string) (string, error) {
	for _, prefix := range []string{"/api/v1/agent/enroll/", "/api/agent/enroll/"} {
		if strings.HasPrefix(path, prefix) {
			value := strings.Trim(strings.TrimPrefix(path, prefix), "/")
			if value == "" || strings.Contains(value, "/") {
				return "", errors.New("enrollment token is required")
			}
			decoded, err := url.PathUnescape(value)
			if err != nil || decoded == "" {
				return "", errors.New("enrollment token is invalid")
			}
			return decoded, nil
		}
	}
	return "", errors.New("enrollment URL is invalid")
}

func (s *Server) handleAgentChallenge(w http.ResponseWriter, r *http.Request) {
	if s.nodes == nil {
		writeError(w, http.StatusServiceUnavailable, "agent challenge endpoint is unavailable")
		return
	}
	body, err := readAgentBody(w, r, maxRegisterBytes)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var request agentproto.ChallengeRequest
	if err := json.Unmarshal(body, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid agent challenge payload")
		return
	}
	if _, err := s.authenticateAgent(r, request.AgentID, body); err != nil {
		writeAgentAuthError(w, err)
		return
	}
	response, err := s.nodes.Challenge(r.Context(), request)
	if err != nil {
		writeAgentAuthError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleAgentLogs(w http.ResponseWriter, r *http.Request) {
	if s.ingest == nil || s.nodes == nil {
		writeError(w, http.StatusServiceUnavailable, "agent ingestion is unavailable")
		return
	}
	body, err := readAgentBody(w, r, maxAgentRequestBytes)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var batch agentproto.LogBatch
	if err := json.Unmarshal(body, &batch); err != nil {
		writeError(w, http.StatusBadRequest, "invalid log batch payload")
		return
	}
	authenticated, err := s.authenticateAgent(r, batch.AgentID, body)
	if err != nil {
		writeAgentAuthError(w, err)
		return
	}
	count, accepted, err := s.ingest.Ingest(r.Context(), authenticated, batch)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, ingest.ErrAgentMismatch) {
			status = http.StatusForbidden
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"accepted": accepted, "entries": count, "sequence": batch.Sequence,
	})
}

func (s *Server) handleAgentHeartbeat(w http.ResponseWriter, r *http.Request) {
	if s.ingest == nil || s.nodes == nil {
		writeError(w, http.StatusServiceUnavailable, "agent heartbeat is unavailable")
		return
	}
	body, err := readAgentBody(w, r, maxHeartbeatBytes)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var heartbeat agentproto.Heartbeat
	if err := json.Unmarshal(body, &heartbeat); err != nil {
		writeError(w, http.StatusBadRequest, "invalid heartbeat payload")
		return
	}
	authenticated, err := s.authenticateAgent(r, heartbeat.AgentID, body)
	if err != nil {
		writeAgentAuthError(w, err)
		return
	}
	if err := s.ingest.Heartbeat(r.Context(), authenticated, heartbeat); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, ingest.ErrAgentMismatch) {
			status = http.StatusForbidden
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "lastSeenAt": time.Now().UTC()})
}

func (s *Server) handleAgentEvents(w http.ResponseWriter, r *http.Request) {
	if s.nodes == nil {
		writeError(w, http.StatusServiceUnavailable, "agent control plane is unavailable")
		return
	}
	agentID := strings.TrimSpace(r.URL.Query().Get("agentId"))
	if _, err := s.authenticateAgent(r, agentID, nil); err != nil {
		writeAgentAuthError(w, err)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming is not supported by this server")
		return
	}
	setSSEHeaders(w)
	w.WriteHeader(http.StatusOK)
	_ = writeSSE(w, "connected", agentproto.ControlEvent{Type: "connected"})
	flusher.Flush()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			if _, err := fmt.Fprint(w, ": keep-alive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (s *Server) authenticateAgent(r *http.Request, agentID string, body []byte) (node.Node, error) {
	if s.nodes == nil || strings.TrimSpace(agentID) == "" {
		return node.Node{}, errors.New("agentId is required")
	}
	timestamp, nonce, signature, err := agentproto.ParseSignatureHeaders(r)
	if err != nil {
		return node.Node{}, err
	}
	return s.nodes.Authenticate(r.Context(), agentID, r.Method, r.URL.Path, timestamp, nonce, body, signature, time.Now().UTC())
}

func readAgentBody(w http.ResponseWriter, r *http.Request, limit int64) ([]byte, error) {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	var reader io.Reader = r.Body
	encoding := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Encoding")))
	if encoding == "gzip" {
		compressed, err := io.ReadAll(io.LimitReader(r.Body, limit))
		if err != nil {
			return nil, err
		}
		gzipReader, err := gzip.NewReader(bytes.NewReader(compressed))
		if err != nil {
			return nil, fmt.Errorf("invalid gzip body: %w", err)
		}
		defer gzipReader.Close()
		reader = io.LimitReader(gzipReader, limit)
	} else if encoding == "zstd" {
		decoder, err := zstd.NewReader(r.Body)
		if err != nil {
			return nil, fmt.Errorf("invalid zstd body: %w", err)
		}
		defer decoder.Close()
		reader = io.LimitReader(decoder, limit)
	} else if encoding != "" && encoding != "identity" {
		return nil, fmt.Errorf("unsupported content encoding %q", encoding)
	}
	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, errors.New("request body is too large")
	}
	return body, nil
}

func writeAgentAuthError(w http.ResponseWriter, err error) {
	status := http.StatusUnauthorized
	if errors.Is(err, node.ErrNodeRevoked) {
		status = http.StatusForbidden
	}
	writeError(w, status, err.Error())
}
