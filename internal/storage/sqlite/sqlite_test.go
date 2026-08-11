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

func TestStoreCleanupRemovesOldEntriesAndPayloadOverBudget(t *testing.T) {
	store, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer store.Close()

	base := time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)
	entries := []explorer.Entry{
		{InsertID: "old", Timestamp: base.Add(-time.Hour), Severity: "INFO", TextPayload: "old payload", Summary: "old", Resource: explorer.Resource{Labels: map[string]string{}}},
		{InsertID: "new-1", Timestamp: base, Severity: "INFO", TextPayload: "first payload", Summary: "first", Resource: explorer.Resource{Labels: map[string]string{}}},
		{InsertID: "new-2", Timestamp: base.Add(time.Second), Severity: "INFO", TextPayload: "second payload", Summary: "second", Resource: explorer.Resource{Labels: map[string]string{}}},
	}
	for index, entry := range entries {
		accepted, writeErr := store.WriteBatch(context.Background(), explorer.EntryBatch{
			AgentID: "agent-1", BootID: "boot-1", Sequence: uint64(index + 1), Entries: []explorer.Entry{entry},
		})
		if writeErr != nil || !accepted {
			t.Fatalf("WriteBatch(%s) accepted=%v err=%v", entry.InsertID, accepted, writeErr)
		}
	}
	deleted, err := store.Cleanup(context.Background(), base.Add(-time.Minute), 0)
	if err != nil || deleted != 1 {
		t.Fatalf("Cleanup by age deleted=%d err=%v, want 1", deleted, err)
	}
	deleted, err = store.Cleanup(context.Background(), time.Time{}, 50)
	if err != nil || deleted != 1 {
		t.Fatalf("Cleanup by budget deleted=%d err=%v, want 1", deleted, err)
	}
	remaining, err := store.SearchEntries(context.Background(), explorer.SearchRequest{From: base.Add(-time.Hour), To: base.Add(time.Hour)})
	if err != nil || len(remaining) != 1 || remaining[0].InsertID != "new-2" {
		t.Fatalf("unexpected remaining entries: %#v err=%v", remaining, err)
	}
}
