package agent

import (
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"caroline/internal/agentproto"
)

func TestBootIDIsFreshAcrossAgentInstances(t *testing.T) {
	stateDir := t.TempDir()
	config := Config{HubURL: "http://127.0.0.1:1", StateDir: stateDir, SpoolMaxBytes: 1 << 20, SpoolMaxAge: time.Hour}
	identity, err := LoadOrCreateIdentity(config.IdentityPath())
	if err != nil {
		t.Fatalf("LoadOrCreateIdentity: %v", err)
	}
	first, err := New(config, identity)
	if err != nil {
		t.Fatalf("New first: %v", err)
	}
	second, err := New(config, identity)
	if err != nil {
		t.Fatalf("New second: %v", err)
	}
	if first.bootID == second.bootID {
		t.Fatalf("boot ID was reused: %q", first.bootID)
	}
	data, err := os.ReadFile(config.IdentityPath())
	if err != nil {
		t.Fatalf("Read identity: %v", err)
	}
	if string(data) == "" || containsJSONField(string(data), "bootId") {
		t.Fatalf("persistent identity unexpectedly contains bootId: %s", data)
	}
}

func TestHubPinSurvivesSenderRestartAndRejectsKeyChanges(t *testing.T) {
	stateDir := t.TempDir()
	config := Config{HubURL: "http://127.0.0.1:1", StateDir: stateDir, TrustHubOnFirstUse: true}
	identity := Identity{AgentID: "agt-test"}
	identity.PublicKey, identity.PrivateKey, _ = ed25519.GenerateKey(rand.Reader)
	first, err := NewSender(config, identity, "boot-1")
	if err != nil {
		t.Fatalf("NewSender first: %v", err)
	}
	hubPublic, hubPrivate, _ := ed25519.GenerateKey(rand.Reader)
	expiresAt := time.Now().UTC().Add(time.Minute)
	challengeID := "challenge-1"
	signature := agentproto.SignChallenge(hubPrivate, agentproto.ProtocolVersion, identity.AgentID, "agent-nonce", challengeID, expiresAt)
	if err := first.acceptHubChallenge(hubPublic, "hub-key-1", signature, identity.AgentID, "agent-nonce", challengeID, expiresAt); err != nil {
		t.Fatalf("acceptHubChallenge: %v", err)
	}
	info, err := os.Stat(filepath.Join(stateDir, "hub.json"))
	if err != nil {
		t.Fatalf("Stat hub pin: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("hub pin mode = %o, want 600", info.Mode().Perm())
	}

	second, err := NewSender(config, identity, "boot-2")
	if err != nil {
		t.Fatalf("NewSender second: %v", err)
	}
	if !ed25519.PublicKey(second.hubPublicKey).Equal(hubPublic) {
		t.Fatal("sender did not load the persisted Hub pin")
	}
	otherPublic, otherPrivate, _ := ed25519.GenerateKey(rand.Reader)
	otherSignature := agentproto.SignChallenge(otherPrivate, agentproto.ProtocolVersion, identity.AgentID, "agent-nonce-2", challengeID, expiresAt)
	if err := second.acceptHubChallenge(otherPublic, "hub-key-2", otherSignature, identity.AgentID, "agent-nonce-2", challengeID, expiresAt); err == nil {
		t.Fatal("sender accepted a changed Hub key after restart")
	}
}

func containsJSONField(data, field string) bool {
	return len(data) > 0 && (data == field || strings.Contains(data, "\""+field+"\""))
}
