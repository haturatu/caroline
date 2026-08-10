package alert

import (
	"strings"
	"testing"

	"caroline/internal/explorer"
)

func TestSampleForNotificationRedactsSensitiveValues(t *testing.T) {
	entry := &explorer.Entry{
		Summary:     `Authorization: Bearer abc123 password=hunter2 email=user@example.com`,
		TextPayload: `DATABASE_URL=postgres://user:secret@example.test/db`,
		JSONPayload: map[string]any{
			"message": "request failed",
			"token":   "abc123",
			"nested":  []any{map[string]any{"api_key": "key123"}},
		},
	}

	full := sampleForNotification(SampleModeFull, entry)
	if full == nil {
		t.Fatal("full sample was omitted")
	}
	for _, value := range []string{full.Summary, full.TextPayload} {
		if strings.Contains(value, "abc123") || strings.Contains(value, "hunter2") || strings.Contains(value, "secret") || strings.Contains(value, "user@example.com") {
			t.Fatalf("sensitive value was not redacted: %q", value)
		}
	}
	if full.JSONPayload["token"] != redactedValue {
		t.Fatalf("token was not redacted: %#v", full.JSONPayload)
	}

	summary := sampleForNotification(SampleModeSummary, entry)
	if summary == nil || summary.TextPayload != "" || summary.JSONPayload != nil {
		t.Fatalf("summary sample included full payload: %#v", summary)
	}
	if sampleForNotification(SampleModeOff, entry) != nil {
		t.Fatal("off sample mode included a sample")
	}
}
