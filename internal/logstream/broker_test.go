package logstream

import (
	"context"
	"testing"
	"time"

	"caroline/internal/explorer"
)

func TestBrokerPublishesWithNodeAndContainerFilters(t *testing.T) {
	broker := NewBroker()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sub := broker.Subscribe(ctx, map[string]bool{"container-1": true}, map[string]bool{"node-1": true}, time.Time{})
	defer sub.Close()

	matching := explorer.Entry{InsertID: "entry-1", Resource: explorer.Resource{Labels: map[string]string{
		"node_id": "node-1", "container_id": "container-1",
	}}}
	broker.Publish(explorer.Entry{InsertID: "other-node", Resource: explorer.Resource{Labels: map[string]string{
		"node_id": "node-2", "container_id": "container-1",
	}}})
	broker.Publish(matching)
	broker.Publish(matching)

	select {
	case received := <-sub.Entries:
		if received.InsertID != matching.InsertID {
			t.Fatalf("received insert id %q, want %q", received.InsertID, matching.InsertID)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for matching entry")
	}
	select {
	case duplicate := <-sub.Entries:
		t.Fatalf("duplicate or filtered entry received: %#v", duplicate)
	case <-time.After(25 * time.Millisecond):
	}
}
