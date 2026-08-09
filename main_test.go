package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func dockerTestFrame(stream byte, payload string) []byte {
	frame := make([]byte, 8+len(payload))
	frame[0] = stream
	binary.BigEndian.PutUint32(frame[4:8], uint32(len(payload)))
	copy(frame[8:], payload)
	return frame
}

func TestReadDockerFrames(t *testing.T) {
	input := append(dockerTestFrame(1, "stdout line\n"), dockerTestFrame(2, "stderr line\n")...)
	frames, err := readDockerFrames(bytes.NewReader(input))
	if err != nil {
		t.Fatalf("readDockerFrames returned error: %v", err)
	}
	if len(frames) != 2 {
		t.Fatalf("got %d frames, want 2", len(frames))
	}
	if frames[0].stream != "stdout" || string(frames[0].data) != "stdout line\n" {
		t.Fatalf("unexpected stdout frame: %#v", frames[0])
	}
	if frames[1].stream != "stderr" || string(frames[1].data) != "stderr line\n" {
		t.Fatalf("unexpected stderr frame: %#v", frames[1])
	}

	raw, err := readDockerFrames(strings.NewReader("raw tty log\n"))
	if err != nil || len(raw) != 1 || raw[0].stream != "stdout" || string(raw[0].data) != "raw tty log\n" {
		t.Fatalf("unexpected raw frame: %#v, error: %v", raw, err)
	}
}

func TestParseLogFrame(t *testing.T) {
	container := dockerContainer{ID: "0123456789abcdef", Names: []string{"/api"}}
	frame := dockerFrame{stream: "stderr", data: []byte("2026-08-09T03:00:00.123456789Z request failed: ERROR 500\n")}
	lines := parseLogFrame(frame, container)
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

func TestMatchesExplorerQuery(t *testing.T) {
	entry := explorerEntry{
		Severity:    "ERROR",
		Summary:     "request failed for user",
		TextPayload: "request failed for user",
		LogName:     "containers/api/stderr",
		Stream:      "stderr",
		Resource:    cloudResource{Type: "docker_container", Labels: map[string]string{"container_name": "api"}},
		JSONPayload: map[string]any{"status": "500", "route": "/health"},
	}
	tests := []struct {
		query string
		want  bool
	}{
		{`severity >= ERROR`, true},
		{`resource.labels.container_name = "api" AND stream = "stderr"`, true},
		{`jsonPayload.status = "500"`, true},
		{`SEARCH("failed")`, true},
		{`severity = INFO`, false},
		{`container = "worker"`, false},
	}
	for _, test := range tests {
		if got := matchesExplorerQuery(entry, test.query); got != test.want {
			t.Errorf("matchesExplorerQuery(%q) = %v, want %v", test.query, got, test.want)
		}
	}
}

func TestHandleExplorer(t *testing.T) {
	container := dockerContainer{
		ID:    "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		Names: []string{"/api"}, Image: "example/api:latest", State: "running", Status: "Up 2 minutes",
	}
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/containers/json":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]dockerContainer{container})
		case r.URL.Path == "/containers/"+container.ID+"/logs":
			payload := append(
				dockerTestFrame(1, "2026-08-09T03:00:00Z started\n"),
				dockerTestFrame(2, "2026-08-09T03:01:00Z ERROR request failed\n")...,
			)
			payload = append(payload, dockerTestFrame(1, "2026-08-09T03:02:00Z {\"status\":\"200\",\"route\":\"/health\"}\n")...)
			_, _ = w.Write(payload)
		default:
			http.NotFound(w, r)
		}
	}))
	defer testServer.Close()

	app := &server{docker: &dockerClient{client: testServer.Client(), baseURL: testServer.URL}}
	req := httptest.NewRequest(http.MethodGet, "/api/explorer?from=2026-08-09T02:59:00Z&to=2026-08-09T03:03:00Z&q=severity%20%3E%3D%20ERROR&limit=10", nil)
	recorder := httptest.NewRecorder()
	app.handleExplorer(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("handleExplorer returned status %d: %s", recorder.Code, recorder.Body.String())
	}
	var response explorerResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Total != 1 || len(response.Entries) != 1 || response.Entries[0].Severity != "ERROR" {
		t.Fatalf("unexpected explorer result: total=%d entries=%d severity=%s", response.Total, len(response.Entries), response.Entries[0].Severity)
	}
	if len(response.Timeline) != 24 || len(response.Fields) == 0 {
		t.Fatalf("expected timeline and fields, got timeline=%d fields=%d", len(response.Timeline), len(response.Fields))
	}
}
