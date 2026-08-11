package agent

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"caroline/internal/agentproto"
	"caroline/internal/explorer"
)

type Sender struct {
	config   Config
	identity Identity
	client   *http.Client

	mu            sync.Mutex
	sessionID     string
	sessionExpiry time.Time
	hubPublicKey  ed25519.PublicKey
}

func NewSender(config Config, identity Identity) *Sender {
	return &Sender{config: config, identity: identity, client: &http.Client{Timeout: 20 * time.Second}, hubPublicKey: append(ed25519.PublicKey(nil), config.HubPublicKey...)}
}

func (s *Sender) SendBatch(ctx context.Context, batch explorer.EntryBatch) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureSessionLocked(ctx); err != nil {
		return err
	}
	body, err := json.Marshal(agentproto.LogBatch{
		ProtocolVersion: agentproto.ProtocolVersion, AgentID: s.identity.AgentID,
		BootID: s.identity.BootID, Sequence: batch.Sequence, Entries: batch.Entries,
		Containers: batch.Containers,
	})
	if err != nil {
		return err
	}
	response, err := s.doSignedLocked(ctx, http.MethodPost, "/api/v1/agent/logs", body)
	if err != nil && errors.Is(err, errUnauthorized) {
		if sessionErr := s.refreshSessionLocked(ctx); sessionErr != nil {
			return sessionErr
		}
		_, err = s.doSignedLocked(ctx, http.MethodPost, "/api/v1/agent/logs", body)
	}
	if err != nil {
		return err
	}
	_ = response
	return nil
}

func (s *Sender) Heartbeat(ctx context.Context, heartbeat agentproto.Heartbeat) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureSessionLocked(ctx); err != nil {
		return err
	}
	body, err := json.Marshal(heartbeat)
	if err != nil {
		return err
	}
	_, err = s.doSignedLocked(ctx, http.MethodPost, "/api/v1/agent/heartbeat", body)
	return err
}

func (s *Sender) ensureSessionLocked(ctx context.Context) error {
	if s.sessionID != "" && time.Now().UTC().Before(s.sessionExpiry.Add(-time.Minute)) {
		return nil
	}
	if strings.TrimSpace(s.config.EnrollmentToken) != "" {
		return s.registerLocked(ctx)
	}
	return s.refreshSessionLocked(ctx)
}

func (s *Sender) registerLocked(ctx context.Context) error {
	nonce, err := agentproto.NewNonce()
	if err != nil {
		return err
	}
	body, err := json.Marshal(agentproto.RegisterRequest{
		ProtocolVersion: agentproto.ProtocolVersion, AgentVersion: s.config.AgentVersion,
		EnrollmentToken: s.config.EnrollmentToken, AgentID: s.identity.AgentID,
		PublicKey: s.identity.PublicKey, Fingerprint: s.identity.Fingerprint,
		Hostname: s.identity.Hostname, OS: s.identity.OS, Architecture: s.identity.Architecture,
		Nonce: nonce,
	})
	if err != nil {
		return err
	}
	responseBody, err := s.doJSONLocked(ctx, http.MethodPost, "/api/v1/agent/register", body, false)
	if err != nil {
		return err
	}
	var response agentproto.RegisterResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return err
	}
	if err := s.acceptHubChallenge(response.HubPublicKey, response.HubKeyID, response.Signature, response.AgentID, response.Nonce, response.SessionID, response.ExpiresAt); err != nil {
		return err
	}
	s.sessionID, s.sessionExpiry = response.SessionID, response.ExpiresAt
	s.config.EnrollmentToken = ""
	return nil
}

func (s *Sender) refreshSessionLocked(ctx context.Context) error {
	nonce, err := agentproto.NewNonce()
	if err != nil {
		return err
	}
	body, err := json.Marshal(agentproto.SessionRequest{ProtocolVersion: agentproto.ProtocolVersion, AgentID: s.identity.AgentID, SessionID: s.sessionID, Nonce: nonce})
	if err != nil {
		return err
	}
	responseBody, err := s.doJSONLocked(ctx, http.MethodPost, "/api/v1/agent/session", body, true)
	if err != nil {
		return err
	}
	var response agentproto.SessionResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return err
	}
	if err := s.acceptHubChallenge(response.HubPublicKey, response.HubKeyID, response.Signature, s.identity.AgentID, response.Nonce, response.SessionID, response.ExpiresAt); err != nil {
		return err
	}
	s.sessionID, s.sessionExpiry = response.SessionID, response.ExpiresAt
	return nil
}

func (s *Sender) acceptHubChallenge(publicKey []byte, keyID string, signature []byte, agentID, agentNonce, sessionID string, expiresAt time.Time) error {
	if len(publicKey) != ed25519.PublicKeySize || len(signature) != ed25519.SignatureSize {
		return errors.New("hub challenge is malformed")
	}
	if len(s.hubPublicKey) == 0 {
		if !s.config.TrustHubOnFirstUse {
			return errors.New("CAROLINE_HUB_PUBLIC_KEY is required unless trust-on-first-use is enabled")
		}
		s.hubPublicKey = append(ed25519.PublicKey(nil), publicKey...)
	}
	if !ed25519.PublicKey(s.hubPublicKey).Equal(ed25519.PublicKey(publicKey)) {
		return fmt.Errorf("hub key %q does not match the pinned key", keyID)
	}
	if !agentproto.VerifyChallenge(s.hubPublicKey, signature, agentproto.ProtocolVersion, agentID, agentNonce, sessionID, expiresAt) {
		return errors.New("hub challenge signature verification failed")
	}
	return nil
}

var errUnauthorized = errors.New("agent request was unauthorized")

func (s *Sender) doSignedLocked(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	return s.doJSONLocked(ctx, method, path, body, true)
}

func (s *Sender) doJSONLocked(ctx context.Context, method, path string, body []byte, signed bool) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, method, s.config.HubURL+path, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	if signed {
		if err := agentproto.ApplyRequestHeaders(request, s.identity.Private(), body, time.Now().UTC()); err != nil {
			return nil, err
		}
	}
	response, err := s.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	data, readErr := io.ReadAll(io.LimitReader(response.Body, 8*1024*1024))
	if readErr != nil {
		return nil, readErr
	}
	if response.StatusCode == http.StatusUnauthorized {
		return nil, errUnauthorized
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("hub returned %s: %s", response.Status, strings.TrimSpace(string(data)))
	}
	return data, nil
}
