package agent

import (
	"testing"
	"time"
)

func TestPruneContainerRetentionRemovesStoppedContainers(t *testing.T) {
	retention := map[string]containerRetentionState{
		"old-id":     {checkedAt: time.Now().UTC()},
		"current-id": {checkedAt: time.Now().UTC()},
	}
	pruneContainerRetention(retention, map[string]struct{}{
		"current-id": {},
		"new-id":     {},
	})

	if _, exists := retention["old-id"]; exists {
		t.Fatal("stopped container retention was not pruned")
	}
	if _, exists := retention["current-id"]; !exists {
		t.Fatal("running container retention was pruned")
	}
	if _, exists := retention["new-id"]; exists {
		t.Fatal("pruning unexpectedly created retention state")
	}
}
