package node

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"caroline/internal/agentproto"
)

var (
	ErrNodeNotFound = errors.New("node was not found")
	ErrNodeRevoked  = errors.New("node has been revoked")
	ErrEnrollment   = errors.New("enrollment token is invalid or expired")
)

const defaultEnrollmentTTL = 15 * time.Minute

type Service struct {
	store      Store
	hubKeyID   string
	hubPrivate ed25519.PrivateKey
	hubPublic  ed25519.PublicKey

	mu       sync.Mutex
	sessions map[string]session
}

type session struct {
	AgentID   string
	ExpiresAt time.Time
}

func NewService(store Store, hubKeyID string, hubPrivate ed25519.PrivateKey) (*Service, error) {
	if store == nil {
		return nil, errors.New("node store is required")
	}
	if len(hubPrivate) != ed25519.PrivateKeySize {
		return nil, errors.New("hub private key is invalid")
	}
	publicKey := make(ed25519.PublicKey, ed25519.PublicKeySize)
	copy(publicKey, hubPrivate[ed25519.SeedSize:])
	return &Service{store: store, hubKeyID: strings.TrimSpace(hubKeyID), hubPrivate: hubPrivate, hubPublic: publicKey, sessions: make(map[string]session)}, nil
}

func (s *Service) HubPublicKey() ed25519.PublicKey {
	return append(ed25519.PublicKey(nil), s.hubPublic...)
}

func (s *Service) CreateEnrollmentToken(ctx context.Context, ttl time.Duration) (string, EnrollmentToken, error) {
	if ttl <= 0 {
		ttl = defaultEnrollmentTTL
	}
	plain, err := agentproto.NewNonce()
	if err != nil {
		return "", EnrollmentToken{}, err
	}
	id, err := agentproto.NewNonce()
	if err != nil {
		return "", EnrollmentToken{}, err
	}
	now := time.Now().UTC()
	token := EnrollmentToken{ID: "ent_" + id, CreatedAt: now, ExpiresAt: now.Add(ttl)}
	if err := s.store.PutEnrollmentToken(ctx, token.ID, tokenHash(plain), token.CreatedAt, token.ExpiresAt); err != nil {
		return "", EnrollmentToken{}, err
	}
	return "car_enroll_" + plain, token, nil
}

func (s *Service) Register(ctx context.Context, request agentproto.RegisterRequest) (agentproto.RegisterResponse, Node, error) {
	if request.ProtocolVersion != agentproto.ProtocolVersion {
		return agentproto.RegisterResponse{}, Node{}, fmt.Errorf("unsupported agent protocol %d", request.ProtocolVersion)
	}
	if len(request.PublicKey) != ed25519.PublicKeySize {
		return agentproto.RegisterResponse{}, Node{}, errors.New("agent public key is invalid")
	}
	token := strings.TrimSpace(request.EnrollmentToken)
	token = strings.TrimPrefix(token, "car_enroll_")
	valid, err := s.store.ConsumeEnrollmentToken(ctx, tokenHash(token), time.Now().UTC())
	if err != nil {
		return agentproto.RegisterResponse{}, Node{}, err
	}
	if !valid {
		return agentproto.RegisterResponse{}, Node{}, ErrEnrollment
	}
	agentID := "agt_" + publicKeyID(request.PublicKey)
	if request.AgentID != "" && request.AgentID != agentID {
		return agentproto.RegisterResponse{}, Node{}, errors.New("agentId does not match public key")
	}
	now := time.Now().UTC()
	nodeValue := Node{
		ID: agentID, Name: firstNonEmpty(request.Hostname, agentID),
		Fingerprint: request.Fingerprint, PublicKey: append([]byte(nil), request.PublicKey...),
		Hostname: request.Hostname, OS: request.OS, Architecture: request.Architecture,
		AgentVersion: request.AgentVersion, ProtocolVersion: request.ProtocolVersion,
		ConnectedAt: now, LastSeenAt: now, Status: StatusOnline,
	}
	if err := s.store.SaveNode(ctx, nodeValue); err != nil {
		return agentproto.RegisterResponse{}, Node{}, err
	}
	sessionID, err := agentproto.NewNonce()
	if err != nil {
		return agentproto.RegisterResponse{}, Node{}, err
	}
	expiresAt := now.Add(5 * time.Minute)
	s.mu.Lock()
	s.sessions[sessionID] = session{AgentID: agentID, ExpiresAt: expiresAt}
	s.mu.Unlock()
	response := agentproto.RegisterResponse{
		ProtocolVersion: agentproto.ProtocolVersion, AgentID: agentID, SessionID: sessionID,
		HubKeyID: s.hubKeyID, HubPublicKey: s.HubPublicKey(), Nonce: request.Nonce, ExpiresAt: expiresAt,
		Signature: agentproto.SignChallenge(s.hubPrivate, agentproto.ProtocolVersion, agentID, request.Nonce, sessionID, expiresAt),
	}
	return response, nodeValue, nil
}

func (s *Service) Session(ctx context.Context, request agentproto.SessionRequest) (agentproto.SessionResponse, error) {
	value, err := s.store.GetNode(ctx, request.AgentID)
	if err != nil {
		return agentproto.SessionResponse{}, ErrNodeNotFound
	}
	if value.Status == StatusRevoked {
		return agentproto.SessionResponse{}, ErrNodeRevoked
	}
	if len(value.PublicKey) != ed25519.PublicKeySize {
		return agentproto.SessionResponse{}, errors.New("registered agent public key is invalid")
	}
	now := time.Now().UTC()
	sessionID, err := agentproto.NewNonce()
	if err != nil {
		return agentproto.SessionResponse{}, err
	}
	expiresAt := now.Add(5 * time.Minute)
	s.mu.Lock()
	s.sessions[sessionID] = session{AgentID: request.AgentID, ExpiresAt: expiresAt}
	s.mu.Unlock()
	return agentproto.SessionResponse{
		ProtocolVersion: agentproto.ProtocolVersion, SessionID: sessionID,
		HubKeyID: s.hubKeyID, HubPublicKey: s.HubPublicKey(), Nonce: request.Nonce,
		ExpiresAt: expiresAt,
		Signature: agentproto.SignChallenge(s.hubPrivate, agentproto.ProtocolVersion, request.AgentID, request.Nonce, sessionID, expiresAt),
	}, nil
}

func (s *Service) Authenticate(ctx context.Context, agentID, method, path, timestamp, nonce string, body, signature []byte, now time.Time) (Node, error) {
	value, err := s.store.GetNode(ctx, agentID)
	if err != nil {
		return Node{}, ErrNodeNotFound
	}
	if value.Status == StatusRevoked {
		return Node{}, ErrNodeRevoked
	}
	if err := agentproto.VerifyRequest(ed25519.PublicKey(value.PublicKey), method, path, timestamp, nonce, body, signature, now); err != nil {
		return Node{}, err
	}
	accepted, err := s.store.RememberNonce(ctx, agentID, nonce, now.Add(agentproto.RequestClockSkew))
	if err != nil {
		return Node{}, err
	}
	if !accepted {
		return Node{}, agentproto.ErrReplay
	}
	if err := s.store.TouchNode(ctx, agentID, now); err != nil {
		return Node{}, err
	}
	return value, nil
}

func (s *Service) List(ctx context.Context) ([]Node, error) {
	return s.store.ListNodes(ctx)
}

func (s *Service) Get(ctx context.Context, id string) (Node, error) {
	value, err := s.store.GetNode(ctx, id)
	if err != nil {
		return Node{}, ErrNodeNotFound
	}
	return value, nil
}

func (s *Service) Revoke(ctx context.Context, id string) error {
	return s.store.RevokeNode(ctx, id, time.Now().UTC())
}

func (s *Service) Touch(ctx context.Context, id string) error {
	if err := s.store.TouchNode(ctx, id, time.Now().UTC()); err != nil {
		return ErrNodeNotFound
	}
	return nil
}

func tokenHash(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func publicKeyID(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])[:20]
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return "unknown"
}
