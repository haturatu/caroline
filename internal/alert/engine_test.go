package alert

import (
	"context"
	"testing"
	"time"

	"caroline/internal/explorer"
)

type recordingNotifier struct {
	notifications []Notification
}

func (n *recordingNotifier) Notify(_ context.Context, _ Rule, notification Notification) error {
	n.notifications = append(n.notifications, notification)
	return nil
}

func TestNormalizeSpec(t *testing.T) {
	enabled := false
	rule, err := normalizeSpec(RuleSpec{
		Name:            " API errors ",
		Query:           "severity >= ERROR",
		Threshold:       3,
		WindowSeconds:   60,
		CooldownSeconds: 300,
		Enabled:         &enabled,
		WebhookURL:      "https://example.test/hooks/caroline",
	})
	if err != nil {
		t.Fatalf("normalizeSpec returned error: %v", err)
	}
	if rule.Name != "API errors" || rule.Enabled || rule.WebhookURL == "" {
		t.Fatalf("unexpected normalized rule: %#v", rule)
	}

	for _, spec := range []RuleSpec{
		{Name: "", Threshold: 1, WindowSeconds: 60},
		{Name: "invalid threshold", Threshold: 0, WindowSeconds: 60},
		{Name: "invalid window", Threshold: 1, WindowSeconds: 0},
		{Name: "invalid webhook", Threshold: 1, WindowSeconds: 60, WebhookURL: "ftp://example.test"},
	} {
		if _, err := normalizeSpec(spec); err == nil {
			t.Fatalf("normalizeSpec accepted invalid rule: %#v", spec)
		}
	}
}

func TestEngineFiresAndResolvesRule(t *testing.T) {
	notifier := &recordingNotifier{}
	engine := NewEngine(nil, notifier)
	rule, err := engine.Create(RuleSpec{
		Name:            "Errors",
		Query:           "severity >= ERROR",
		Threshold:       2,
		WindowSeconds:   60,
		CooldownSeconds: 600,
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	now := time.Now().UTC()
	entry := explorer.Entry{
		InsertID:  "entry-1",
		Timestamp: now,
		Severity:  "ERROR",
		Summary:   "request failed",
		Stream:    "stderr",
	}
	engine.processEntry(context.Background(), entry)
	if len(notifier.notifications) != 0 {
		t.Fatalf("rule fired before threshold: %#v", notifier.notifications)
	}
	entry.InsertID = "entry-2"
	entry.Timestamp = now.Add(time.Second)
	engine.processEntry(context.Background(), entry)
	if len(notifier.notifications) != 1 || notifier.notifications[0].Event != "alert.firing" {
		t.Fatalf("unexpected firing notifications: %#v", notifier.notifications)
	}

	engine.resolve(context.Background())
	if len(notifier.notifications) != 1 {
		t.Fatalf("rule resolved while matches were in window: %#v", notifier.notifications)
	}

	engine.mu.Lock()
	state := engine.states[rule.ID]
	state.Matches = []time.Time{now.Add(-2 * time.Minute)}
	engine.states[rule.ID] = state
	engine.mu.Unlock()
	engine.resolve(context.Background())
	if len(notifier.notifications) != 2 || notifier.notifications[1].Event != "alert.resolved" {
		t.Fatalf("unexpected resolved notifications: %#v", notifier.notifications)
	}
}

func TestEngineDoesNotNotifyResolutionForSuppressedRefire(t *testing.T) {
	notifier := &recordingNotifier{}
	engine := NewEngine(nil, notifier)
	rule, err := engine.Create(RuleSpec{
		Name:            "Errors",
		Query:           "severity >= ERROR",
		Threshold:       1,
		WindowSeconds:   60,
		CooldownSeconds: 10 * 60,
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	entry := explorer.Entry{Severity: "ERROR", Summary: "request failed"}
	engine.processEntry(context.Background(), entry)
	if len(notifier.notifications) != 1 || notifier.notifications[0].Event != "alert.firing" {
		t.Fatalf("unexpected initial firing notifications: %#v", notifier.notifications)
	}

	expireRuleMatches(engine, rule.ID)
	engine.resolve(context.Background())
	if len(notifier.notifications) != 2 || notifier.notifications[1].Event != "alert.resolved" {
		t.Fatalf("unexpected initial resolution notifications: %#v", notifier.notifications)
	}

	entry.InsertID = "refire"
	engine.processEntry(context.Background(), entry)
	if len(notifier.notifications) != 2 {
		t.Fatalf("cooldown-suppressed refire sent a notification: %#v", notifier.notifications)
	}

	expireRuleMatches(engine, rule.ID)
	engine.resolve(context.Background())
	if len(notifier.notifications) != 2 {
		t.Fatalf("suppressed refire produced a duplicate resolution: %#v", notifier.notifications)
	}
}

func expireRuleMatches(engine *Engine, ruleID string) {
	engine.mu.Lock()
	state := engine.states[ruleID]
	state.Matches = []time.Time{time.Now().UTC().Add(-2 * time.Minute)}
	engine.states[ruleID] = state
	engine.mu.Unlock()
}
