package httpserver

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"caroline/internal/agentproto"
	"caroline/internal/explorer"
	"caroline/internal/ingest"
	"caroline/internal/logstream"
	"caroline/internal/node"
	"caroline/internal/storage/sqlite"
)

func TestAgentRegistrationAndCompressedIngest(t *testing.T) {
	store, err := sqlite.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer store.Close()
	_, hubPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey hub: %v", err)
	}
	nodes, err := node.NewService(store, "hub-test", hubPrivate)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	broker := logstream.NewBroker()
	ingestService, err := ingest.NewService(store, nodes, broker)
	if err != nil {
		t.Fatalf("NewService ingest: %v", err)
	}
	server := New(explorer.NewStoreService(store), nil, logstream.NewBrokerManager(broker), nil)
	server.ConfigureHub(store, nodes, ingestService, broker)

	token, _, err := nodes.CreateEnrollmentToken(context.Background(), time.Minute)
	if err != nil {
		t.Fatalf("CreateEnrollmentToken: %v", err)
	}
	agentPublic, agentPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey agent: %v", err)
	}
	registerBody, _ := json.Marshal(agentproto.RegisterRequest{
		ProtocolVersion: agentproto.ProtocolVersion, EnrollmentToken: token, PublicKey: agentPublic,
		Hostname: "server-a", Fingerprint: "fingerprint", OS: "linux", Architecture: "amd64",
		Nonce: "register-nonce",
	})
	registerRecorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(registerRecorder, httptest.NewRequest(http.MethodPost, "/api/v1/agent/register", bytes.NewReader(registerBody)))
	if registerRecorder.Code != http.StatusOK {
		t.Fatalf("registration status=%d body=%s", registerRecorder.Code, registerRecorder.Body.String())
	}
	var registerResponse agentproto.RegisterResponse
	if err := json.Unmarshal(registerRecorder.Body.Bytes(), &registerResponse); err != nil {
		t.Fatalf("decode registration: %v", err)
	}

	now := time.Now().UTC()
	batchBody, _ := json.Marshal(agentproto.LogBatch{
		ProtocolVersion: agentproto.ProtocolVersion, AgentID: registerResponse.AgentID, BootID: "boot-1", Sequence: 1,
		Entries: []explorer.Entry{{
			InsertID: "agent-entry-1", Timestamp: now, Severity: "ERROR", TextPayload: "failed",
			Summary: "failed", Resource: explorer.Resource{Type: "docker_container", Labels: map[string]string{
				"container_id": "container-1", "container_name": "api", "node_id": registerResponse.AgentID,
			}},
		}},
	})
	var compressed bytes.Buffer
	gzipWriter := gzip.NewWriter(&compressed)
	_, _ = gzipWriter.Write(batchBody)
	_ = gzipWriter.Close()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/agent/logs", bytes.NewReader(compressed.Bytes()))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Content-Encoding", "gzip")
	if err := agentproto.ApplyRequestHeaders(request, agentPrivate, batchBody, now); err != nil {
		t.Fatalf("ApplyRequestHeaders: %v", err)
	}
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("ingest status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	entries, err := store.SearchEntries(context.Background(), explorer.SearchRequest{From: now.Add(-time.Minute), To: now.Add(time.Minute)})
	if err != nil || len(entries) != 1 || entries[0].Resource.Labels["node_name"] != "server-a" {
		t.Fatalf("unexpected stored entries: %#v err=%v", entries, err)
	}
}
