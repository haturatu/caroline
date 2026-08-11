package agentproto

import (
	"crypto/ed25519"
	"crypto/rand"
	"net/http"
	"testing"
	"time"
)

func TestRequestSignatureRoundTripAndReplayInputs(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	now := time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)
	timestamp := now.Format(time.RFC3339Nano)
	nonce := "nonce-1"
	body := []byte(`{"agentId":"agt-test"}`)
	signature := SignRequest(privateKey, http.MethodPost, "/api/v1/agent/logs", timestamp, nonce, body)
	if err := VerifyRequest(publicKey, http.MethodPost, "/api/v1/agent/logs", timestamp, nonce, body, signature, now); err != nil {
		t.Fatalf("VerifyRequest: %v", err)
	}
	if err := VerifyRequest(publicKey, http.MethodPost, "/api/v1/agent/logs", timestamp, nonce, []byte("tampered"), signature, now); err != ErrInvalidSignature {
		t.Fatalf("tampered body error = %v, want %v", err, ErrInvalidSignature)
	}
	if err := VerifyRequest(publicKey, http.MethodPost, "/api/v1/agent/logs", timestamp, nonce, body, signature, now.Add(RequestClockSkew+time.Nanosecond)); err != ErrStaleRequest {
		t.Fatalf("stale request error = %v, want %v", err, ErrStaleRequest)
	}
}

func TestChallengeRoundTrip(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	expiresAt := time.Date(2026, 8, 11, 12, 5, 0, 0, time.UTC)
	signature := SignChallenge(privateKey, ProtocolVersion, "agt-test", "agent-nonce", "session", expiresAt)
	if !VerifyChallenge(publicKey, signature, ProtocolVersion, "agt-test", "agent-nonce", "session", expiresAt) {
		t.Fatal("VerifyChallenge rejected a valid challenge")
	}
	if VerifyChallenge(publicKey, signature, ProtocolVersion, "agt-other", "agent-nonce", "session", expiresAt) {
		t.Fatal("VerifyChallenge accepted a challenge for another agent")
	}
}
