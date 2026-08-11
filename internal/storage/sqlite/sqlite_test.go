package sqlite

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"caroline/internal/explorer"
	"caroline/internal/node"
	zombiesqlite "zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
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
			LoggingDriver: "local", LoggingOptions: map[string]string{"max-size": "10m", "max-file": "3"},
			OldestLogAt: when.Add(-time.Hour),
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

	entries, _, err := store.SearchEntries(context.Background(), explorer.SearchRequest{From: when.Add(-time.Minute), To: when.Add(time.Minute), Sort: "desc"})
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
	if containers[0].LoggingDriver != "local" || containers[0].LoggingOptions["max-file"] != "3" || !containers[0].OldestLogAt.Equal(when.Add(-time.Hour)) {
		t.Fatalf("container retention metadata was not persisted: %#v", containers[0])
	}
}

func TestStoreMigratesContainerRetentionColumns(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "legacy.db")
	conn, err := zombiesqlite.OpenConn(databasePath)
	if err != nil {
		t.Fatalf("OpenConn: %v", err)
	}
	legacySchema := `CREATE TABLE containers(
node_id TEXT NOT NULL, node_name TEXT NOT NULL DEFAULT '', container_id TEXT NOT NULL,
container_name TEXT NOT NULL DEFAULT '', image TEXT NOT NULL DEFAULT '', state TEXT NOT NULL DEFAULT '',
status TEXT NOT NULL DEFAULT '', created_ns INTEGER NOT NULL DEFAULT 0, labels_json TEXT,
log_count INTEGER NOT NULL DEFAULT 0, error_count INTEGER NOT NULL DEFAULT 0,
warning_count INTEGER NOT NULL DEFAULT 0, last_seen_ns INTEGER NOT NULL,
PRIMARY KEY (node_id, container_id)
)`
	if err := sqlitex.Execute(conn, legacySchema, &sqlitex.ExecOptions{}); err != nil {
		_ = conn.Close()
		t.Fatalf("create legacy schema: %v", err)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("close legacy database: %v", err)
	}

	store, err := Open(databasePath)
	if err != nil {
		t.Fatalf("Open migrated store: %v", err)
	}
	defer store.Close()
	_, err = store.WriteBatch(context.Background(), explorer.EntryBatch{Containers: []explorer.ContainerInfo{{
		ID: "container-1", NodeID: "node-1", Name: "api", LoggingDriver: "json-file",
		LoggingOptions: map[string]string{"max-size": "10m"}, OldestLogAt: time.Unix(1700000000, 0).UTC(),
	}}})
	if err != nil {
		t.Fatalf("WriteBatch after migration: %v", err)
	}
	containers, err := store.ListContainers(context.Background())
	if err != nil || len(containers) != 1 || containers[0].LoggingDriver != "json-file" {
		t.Fatalf("migrated metadata = %#v err=%v", containers, err)
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
	deleted, err := store.Cleanup(context.Background(), base.Add(-time.Minute), 0, RetentionModeIndependent)
	if err != nil || deleted != 1 {
		t.Fatalf("Cleanup by age deleted=%d err=%v, want 1", deleted, err)
	}
	deleted, err = store.Cleanup(context.Background(), time.Time{}, 50, RetentionModeIndependent)
	if err != nil || deleted != 1 {
		t.Fatalf("Cleanup by budget deleted=%d err=%v, want 1", deleted, err)
	}
	remaining, _, err := store.SearchEntries(context.Background(), explorer.SearchRequest{From: base.Add(-time.Hour), To: base.Add(time.Hour)})
	if err != nil || len(remaining) != 1 || remaining[0].InsertID != "new-2" {
		t.Fatalf("unexpected remaining entries: %#v err=%v", remaining, err)
	}
}

func TestStoreCleanupHonorsSourceAndMinBoundaries(t *testing.T) {
	store, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer store.Close()

	base := time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)
	entries := []explorer.Entry{
		{InsertID: "source-old", Timestamp: base.Add(-time.Hour), TextPayload: "source old", Resource: explorer.Resource{Labels: map[string]string{
			"node_id": "node-a", "container_id": "container-a",
		}}},
		{InsertID: "source-window", Timestamp: base.Add(-20 * time.Minute), TextPayload: "source window", Resource: explorer.Resource{Labels: map[string]string{
			"node_id": "node-a", "container_id": "container-a",
		}}},
		{InsertID: "source-new", Timestamp: base.Add(-5 * time.Minute), TextPayload: "source new", Resource: explorer.Resource{Labels: map[string]string{
			"node_id": "node-a", "container_id": "container-a",
		}}},
		{InsertID: "unreported-old", Timestamp: base.Add(-time.Hour), TextPayload: "unreported old", Resource: explorer.Resource{Labels: map[string]string{
			"node_id": "node-b", "container_id": "container-b",
		}}},
	}
	accepted, err := store.WriteBatch(context.Background(), explorer.EntryBatch{
		AgentID: "agent-retention", BootID: "boot-retention", Sequence: 1, Entries: entries,
		Containers: []explorer.ContainerInfo{{
			ID: "container-a", NodeID: "node-a", Name: "api", OldestLogAt: base.Add(-30 * time.Minute),
		}},
	})
	if err != nil || !accepted {
		t.Fatalf("WriteBatch accepted=%v err=%v", accepted, err)
	}

	deleted, err := store.Cleanup(context.Background(), time.Time{}, 0, RetentionModeSource)
	if err != nil || deleted != 1 {
		t.Fatalf("source cleanup deleted=%d err=%v, want 1", deleted, err)
	}
	remaining, _, err := store.SearchEntries(context.Background(), explorer.SearchRequest{From: base.Add(-2 * time.Hour), To: base})
	if err != nil {
		t.Fatalf("SearchEntries after source cleanup: %v", err)
	}
	if len(remaining) != 3 {
		t.Fatalf("source cleanup remaining=%d entries: %#v", len(remaining), remaining)
	}

	deleted, err = store.Cleanup(context.Background(), base.Add(-15*time.Minute), 0, RetentionModeMin)
	if err != nil || deleted != 2 {
		t.Fatalf("min cleanup deleted=%d err=%v, want 2", deleted, err)
	}
	remaining, _, err = store.SearchEntries(context.Background(), explorer.SearchRequest{From: base.Add(-2 * time.Hour), To: base})
	if err != nil {
		t.Fatalf("SearchEntries after min cleanup: %v", err)
	}
	if len(remaining) != 1 || remaining[0].InsertID != "source-new" {
		t.Fatalf("min cleanup remaining=%#v, want source-new only", remaining)
	}
}

func TestSearchEntriesBoundsMemoryToMaxEntries(t *testing.T) {
	store, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer store.Close()

	base := time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)
	const total = explorer.MaxEntries + 1000
	for start := 0; start < total; start += 2500 {
		end := min(start+2500, total)
		entries := make([]explorer.Entry, 0, end-start)
		for index := start; index < end; index++ {
			payload := "payload"
			if index == 0 {
				payload = "oldest unique needle"
			}
			entries = append(entries, explorer.Entry{
				InsertID:  fmt.Sprintf("entry-%06d", index),
				Timestamp: base.Add(time.Duration(index) * time.Millisecond),
				Severity:  "INFO", TextPayload: payload, Summary: payload,
				Resource: explorer.Resource{Labels: map[string]string{
					"node_id": "node-1", "node_name": "server-a",
					"container_id": "container-1", "container_name": "api",
				}},
			})
		}
		accepted, writeErr := store.WriteBatch(context.Background(), explorer.EntryBatch{
			AgentID: "agent-1", BootID: "boot-memory", Sequence: uint64(start / 2500), Entries: entries,
		})
		if writeErr != nil || !accepted {
			t.Fatalf("WriteBatch(%d) accepted=%v err=%v", start, accepted, writeErr)
		}
	}

	entries, truncated, err := store.SearchEntries(context.Background(), explorer.SearchRequest{From: base.Add(-time.Minute), To: base.Add(time.Hour)})
	if err != nil {
		t.Fatalf("SearchEntries: %v", err)
	}
	if len(entries) != explorer.MaxEntries {
		t.Fatalf("SearchEntries loaded %d rows, want bounded scan of %d", len(entries), explorer.MaxEntries)
	}
	if !truncated {
		t.Fatal("SearchEntries did not report additional matching rows")
	}
	if entries[0].InsertID != "entry-050999" || entries[len(entries)-1].InsertID != "entry-001000" {
		t.Fatalf("descending bounded scan kept wrong range: first=%q last=%q", entries[0].InsertID, entries[len(entries)-1].InsertID)
	}
	response, err := explorer.NewStoreService(store).Search(context.Background(), explorer.SearchRequest{
		From: base.Add(-time.Minute), To: base.Add(time.Hour), Limit: 1,
	})
	if err != nil {
		t.Fatalf("store service Search: %v", err)
	}
	if !response.Truncated || len(response.Entries) != 1 || response.Entries[0].InsertID != "entry-050999" {
		t.Fatalf("bounded service response = %#v, want newest entry and truncated=true", response)
	}

	entries, truncated, err = store.SearchEntries(context.Background(), explorer.SearchRequest{
		From: base.Add(-time.Minute), To: base.Add(time.Hour), Query: "needle",
	})
	if err != nil {
		t.Fatalf("SearchEntries with sparse query: %v", err)
	}
	if truncated || len(entries) != 1 || entries[0].InsertID != "entry-000000" {
		t.Fatalf("sparse query result = %#v truncated=%v, want oldest matching entry", entries, truncated)
	}
}

func TestSearchEntriesAppliesIndexedFiltersAcrossChunks(t *testing.T) {
	store, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer store.Close()

	base := time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)
	entries := make([]explorer.Entry, 0, 1001)
	entries = append(entries, explorer.Entry{
		InsertID: "target", Timestamp: base, Severity: "ERROR", Stream: "stderr", TextPayload: "target",
		Resource: explorer.Resource{Labels: map[string]string{"node_id": "node-a", "node_name": "server-a", "container_id": "target-container", "container_name": "api"}},
	})
	for index := 0; index < 1000; index++ {
		entries = append(entries, explorer.Entry{
			InsertID: fmt.Sprintf("distractor-%04d", index), Timestamp: base.Add(time.Duration(index+1) * time.Second), Severity: "INFO", Stream: "stdout",
			Resource: explorer.Resource{Labels: map[string]string{"node_id": "node-b", "node_name": "server-b", "container_id": "other-container", "container_name": "worker"}},
		})
	}
	for start := 0; start < len(entries); start += 500 {
		end := min(start+500, len(entries))
		accepted, writeErr := store.WriteBatch(context.Background(), explorer.EntryBatch{
			AgentID: "agent-1", BootID: "boot-search", Sequence: uint64(start/500 + 1), Entries: entries[start:end],
		})
		if writeErr != nil || !accepted {
			t.Fatalf("WriteBatch(%d) accepted=%v err=%v", start, accepted, writeErr)
		}
	}

	request := explorer.SearchRequest{
		From: base.Add(-time.Minute), To: base.Add(30 * time.Minute), Severity: "ERROR", Stream: "stderr",
		SelectedNodes: map[string]bool{"server-a": true}, Sort: "desc",
	}
	entries, _, err = store.SearchEntries(context.Background(), request)
	if err != nil {
		t.Fatalf("SearchEntries: %v", err)
	}
	if len(entries) != 1 || entries[0].InsertID != "target" {
		t.Fatalf("filtered entries = %#v, want target only", entries)
	}
	response, err := explorer.NewStoreService(store).Search(context.Background(), explorer.SearchRequest{
		From: base.Add(-time.Minute), To: base.Add(30 * time.Minute), Query: "target", Sort: "desc", Limit: 100,
	})
	if err != nil {
		t.Fatalf("store Search: %v", err)
	}
	if response.Total != 1 || len(response.Entries) != 1 || response.Entries[0].InsertID != "target" {
		t.Fatalf("query response = %#v, want target only", response)
	}
}
