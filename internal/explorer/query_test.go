package explorer

import (
	"testing"
	"time"

	"caroline/internal/docker"
)

func TestParseLogFrame(t *testing.T) {
	container := docker.Container{ID: "0123456789abcdef", Names: []string{"/api"}}
	frame := docker.Frame{Stream: "stderr", Data: []byte("2026-08-09T03:00:00.123456789Z request failed: ERROR 500\n")}
	lines := ParseLogFrame(frame, container)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	line := lines[0]
	if line.Container != "api" || line.Stream != "stderr" || line.Severity != "ERROR" {
		t.Fatalf("unexpected parsed line: %#v", line)
	}
	if line.Message != "request failed: ERROR 500" {
		t.Fatalf("unexpected message: %q", line.Message)
	}
	if line.Timestamp.IsZero() {
		t.Fatal("timestamp was not parsed")
	}
}

func TestMatchesQuery(t *testing.T) {
	entry := Entry{
		Timestamp:   time.Date(2026, 8, 9, 3, 0, 0, 0, time.UTC),
		Severity:    "ERROR",
		Summary:     "request failed for user",
		TextPayload: "request failed for user",
		LogName:     "containers/api/stderr",
		Stream:      "stderr",
		Resource:    Resource{Type: "docker_container", Labels: map[string]string{"container_name": "api"}},
		JSONPayload: map[string]any{"status": "500", "route": "/health"},
	}
	tests := []struct {
		query string
		want  bool
	}{
		{`severity >= ERROR`, true},
		{`resource.labels.container_name = "api" AND stream = "stderr"`, true},
		{`severity >= ERROR
severity = "ERROR"`, true},
		{`jsonPayload.status = "500"`, true},
		{`SEARCH("failed")`, true},
		{`severity = INFO`, false},
		{`container = "worker"`, false},
	}
	for _, test := range tests {
		if got := MatchesQuery(entry, test.query); got != test.want {
			t.Errorf("MatchesQuery(%q) = %v, want %v", test.query, got, test.want)
		}
	}
}
