package sqlite

import (
	"context"
	"testing"
	"time"

	"caroline/internal/explorer"
	"caroline/internal/node"
)

func TestStoreWritesAndDeduplicatesBatch(t *testing.T) {
	store, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer store.Close()

	when := time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)
	entry := explorer.Entry{
		InsertID: "agt-1/boot-1/1/0", Timestamp: when, Severity: "ERROR",
		LogName: "containers/api/stderr", Stream: "stderr", TextPayload: "request failed",
		Summary: "request failed", Resource: explorer.Resource{Type: "docker_container", Labels: map[string]string{
			"node_id": "node-1", "node_name": "server-a", "container_id": "container-1",
			"container_name": "api", "image": "example/api:latest",
		}},
	}
	batch := explorer.EntryBatch{
		AgentID: "agent-1", BootID: "boot-1", Sequence: 1, Entries: []explorer.Entry{entry},
		Containers: []explorer.ContainerInfo{{
			ID: "container-1", Name: "api", NodeID: "node-1", NodeName: "server-a",
			Image: "example/api:latest", State: "running", Created: when,
		}},
	}
	accepted, err := store.WriteBatch(context.Background(), batch)
	if err != nil || !accepted {
		t.Fatalf("WriteBatch accepted=%v err=%v", accepted, err)
	}
	accepted, err = store.WriteBatch(context.Background(), batch)
	if err != nil || accepted {
		t.Fatalf("duplicate WriteBatch accepted=%v err=%v", accepted, err)
	}

	entries, err := store.SearchEntries(context.Background(), explorer.SearchRequest{From: when.Add(-time.Minute), To: when.Add(time.Minute), Sort: "desc"})
	if err != nil {
		t.Fatalf("SearchEntries: %v", err)
	}
	if len(entries) != 1 || entries[0].Resource.Labels["node_name"] != "server-a" {
		t.Fatalf("unexpected entries: %#v", entries)
	}
	containers, err := store.ListContainers(context.Background())
	if err != nil || len(containers) != 1 || containers[0].NodeID != "node-1" {
		t.Fatalf("unexpected containers: %#v err=%v", containers, err)
	}
}

func TestStoreNodeAndNonceOperations(t *testing.T) {
	store, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer store.Close()

	now := time.Now().UTC()
	value := node.Node{ID: "node-1", Name: "server-a", PublicKey: []byte("public-key"), Status: node.StatusOnline, ConnectedAt: now, LastSeenAt: now}
	if err := store.SaveNode(context.Background(), value); err != nil {
		t.Fatalf("SaveNode: %v", err)
	}
	loaded, err := store.GetNode(context.Background(), value.ID)
	if err != nil || string(loaded.PublicKey) != "public-key" {
		t.Fatalf("GetNode: %#v err=%v", loaded, err)
	}
	accepted, err := store.RememberNonce(context.Background(), value.ID, "nonce-1", now.Add(time.Minute))
	if err != nil || !accepted {
		t.Fatalf("RememberNonce first accepted=%v err=%v", accepted, err)
	}
	accepted, err = store.RememberNonce(context.Background(), value.ID, "nonce-1", now.Add(time.Minute))
	if err != nil || accepted {
		t.Fatalf("RememberNonce duplicate accepted=%v err=%v", accepted, err)
	}
}
