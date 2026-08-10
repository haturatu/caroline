package alert

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"caroline/internal/explorer"
)

func TestEnginePersistsAndRestoresAlertState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "alerts.json")
	engine, err := NewEngineWithPersistence(nil, nil, path)
	if err != nil {
		t.Fatalf("NewEngineWithPersistence returned error: %v", err)
	}
	created, err := engine.Create(RuleSpec{
		Name:            "Persisted errors",
		Query:           "severity >= ERROR",
		Severity:        "critical",
		Labels:          map[string]string{"service": "api"},
		RunbookURL:      "https://runbooks.example.test/api-errors",
		SampleMode:      SampleModeFull,
		Threshold:       1,
		WindowSeconds:   60,
		CooldownSeconds: 300,
		WebhookURL:      "https://example.test/webhook",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	engine.processEntry(context.Background(), explorer.Entry{
		Severity: "ERROR",
		Summary:  "request failed",
	})

	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat alert store: %v", err)
	}
	if permissions := fileInfo.Mode().Perm(); permissions != 0o600 {
		t.Fatalf("alert store permissions = %o, want 600", permissions)
	}

	restored, err := NewEngineWithPersistence(nil, nil, path)
	if err != nil {
		t.Fatalf("restoring engine returned error: %v", err)
	}
	views := restored.List()
	if len(views) != 1 {
		t.Fatalf("restored %d rules, want 1", len(views))
	}
	if views[0].ID != created.ID || views[0].Status != StatusFiring || views[0].MatchCount != 1 || !views[0].WebhookConfigured || views[0].Severity != "critical" || views[0].Labels["service"] != "api" || views[0].SampleMode != SampleModeFull {
		t.Fatalf("unexpected restored rule: %#v", views[0])
	}
	restored.mu.RLock()
	firingNotificationSent := restored.states[created.ID].FiringNotificationSent
	restored.mu.RUnlock()
	if !firingNotificationSent {
		t.Fatal("restored firing state lost notification status")
	}
	restored.mu.RLock()
	firingSince := restored.states[created.ID].FiringSince
	restored.mu.RUnlock()
	if firingSince == nil {
		t.Fatal("restored firing state lost firing start time")
	}

	if err := restored.Delete(created.ID); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	withoutRule, err := NewEngineWithPersistence(nil, nil, path)
	if err != nil {
		t.Fatalf("restoring after delete returned error: %v", err)
	}
	if views := withoutRule.List(); len(views) != 0 {
		t.Fatalf("restored %d rules after delete, want 0", len(views))
	}
}

func TestEngineRejectsUnsupportedAlertStoreVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "alerts.json")
	if err := os.WriteFile(path, []byte(`{"version":99}`), 0o600); err != nil {
		t.Fatalf("write alert store: %v", err)
	}
	if _, err := NewEngineWithPersistence(nil, nil, path); err == nil {
		t.Fatal("NewEngineWithPersistence accepted unsupported store version")
	}
}
