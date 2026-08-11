package node_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"caroline/internal/agentproto"
	"caroline/internal/node"
	"caroline/internal/storage/sqlite"
)

func TestEnrollmentAndSignedChallenge(t *testing.T) {
	store, err := sqlite.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer store.Close()

	_, hubPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey hub: %v", err)
	}
	service, err := node.NewService(store, "hub-test", hubPrivate)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	plainToken, _, err := service.CreateEnrollmentToken(context.Background(), time.Minute)
	if err != nil {
		t.Fatalf("CreateEnrollmentToken: %v", err)
	}
	agentPublic, agentPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey agent: %v", err)
	}
	register, registered, err := service.Register(context.Background(), agentproto.RegisterRequest{
		ProtocolVersion: agentproto.ProtocolVersion, EnrollmentToken: plainToken,
		PublicKey: agentPublic, Fingerprint: "fingerprint", Hostname: "server-a",
		OS: "linux", Architecture: "amd64", Nonce: "register-nonce",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if registered.ID != register.AgentID || !agentproto.VerifyChallenge(service.HubPublicKey(), register.Signature, register.ProtocolVersion, register.AgentID, register.Nonce, register.ChallengeID, register.ExpiresAt) {
		t.Fatal("registration challenge could not be verified")
	}
	if _, _, err := service.Register(context.Background(), agentproto.RegisterRequest{
		ProtocolVersion: agentproto.ProtocolVersion, EnrollmentToken: plainToken,
		PublicKey: agentPublic, Nonce: "second-attempt",
	}); err != node.ErrEnrollment {
		t.Fatalf("second registration error = %v, want %v", err, node.ErrEnrollment)
	}

	now := time.Now().UTC()
	body := []byte(`{"agentId":"` + registered.ID + `"}`)
	timestamp := now.Format(time.RFC3339Nano)
	nonce := "signed-request"
	signature := agentproto.SignRequest(agentPrivate, "POST", "/api/v1/agent/heartbeat", timestamp, nonce, body)
	if _, err := service.Authenticate(context.Background(), registered.ID, "POST", "/api/v1/agent/heartbeat", timestamp, nonce, body, signature, now); err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if _, err := service.Authenticate(context.Background(), registered.ID, "POST", "/api/v1/agent/heartbeat", timestamp, nonce, body, signature, now); err != agentproto.ErrReplay {
		t.Fatalf("replayed request error = %v, want %v", err, agentproto.ErrReplay)
	}
}

func TestRevokedNodeCannotReenrollWithTheSameKey(t *testing.T) {
	store, err := sqlite.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer store.Close()
	_, hubPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey hub: %v", err)
	}
	service, err := node.NewService(store, "hub-test", hubPrivate)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	firstToken, _, err := service.CreateEnrollmentToken(context.Background(), time.Minute)
	if err != nil {
		t.Fatalf("CreateEnrollmentToken first: %v", err)
	}
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey agent: %v", err)
	}
	_, registered, err := service.Register(context.Background(), agentproto.RegisterRequest{
		ProtocolVersion: agentproto.ProtocolVersion, EnrollmentToken: firstToken, PublicKey: publicKey, Hostname: "server-a",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := service.Revoke(context.Background(), registered.ID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	secondToken, _, err := service.CreateEnrollmentToken(context.Background(), time.Minute)
	if err != nil {
		t.Fatalf("CreateEnrollmentToken second: %v", err)
	}
	if _, _, err := service.Register(context.Background(), agentproto.RegisterRequest{
		ProtocolVersion: agentproto.ProtocolVersion, EnrollmentToken: secondToken, PublicKey: publicKey,
	}); err != node.ErrNodeRevoked {
		t.Fatalf("reenrollment error = %v, want %v", err, node.ErrNodeRevoked)
	}

	otherPublicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey other agent: %v", err)
	}
	if _, _, err := service.Register(context.Background(), agentproto.RegisterRequest{
		ProtocolVersion: agentproto.ProtocolVersion, EnrollmentToken: secondToken, PublicKey: otherPublicKey,
	}); err != nil {
		t.Fatalf("registration with an unrelated key consumed by revoked-key check: %v", err)
	}
}
