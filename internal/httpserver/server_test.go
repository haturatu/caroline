package httpserver

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"caroline/internal/docker"
	"caroline/internal/explorer"
)

func testDockerFrame(stream byte, payload string) []byte {
	frame := make([]byte, 8+len(payload))
	frame[0] = stream
	binary.BigEndian.PutUint32(frame[4:8], uint32(len(payload)))
	copy(frame[8:], payload)
	return frame
}

func TestHandleExplorer(t *testing.T) {
	container := docker.Container{
		ID:    "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		Names: []string{"/api"}, Image: "example/api:latest", State: "running", Status: "Up 2 minutes",
	}
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/containers/json":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]docker.Container{container})
		case r.URL.Path == "/containers/"+container.ID+"/logs":
			payload := append(
				testDockerFrame(1, "2026-08-09T03:00:00Z started\n"),
				testDockerFrame(2, "2026-08-09T03:01:00Z ERROR request failed\n")...,
			)
			payload = append(payload, testDockerFrame(1, "2026-08-09T03:02:00Z {\"status\":\"200\",\"route\":\"/health\"}\n")...)
			_, _ = w.Write(payload)
		default:
			http.NotFound(w, r)
		}
	}))
	defer testServer.Close()

	dockerClient := docker.NewClient(testServer.URL)
	server := New(explorer.NewService(dockerClient), dockerClient)
	req := httptest.NewRequest(http.MethodGet, "/api/explorer?from=2026-08-09T02:59:00Z&to=2026-08-09T03:03:00Z&q=severity%20%3E%3D%20ERROR&limit=10&timelineBuckets=48", nil)
	recorder := httptest.NewRecorder()
	server.handleExplorer(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("handleExplorer returned status %d: %s", recorder.Code, recorder.Body.String())
	}
	var response explorer.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Total != 1 || len(response.Entries) != 1 || response.Entries[0].Severity != "ERROR" {
		t.Fatalf("unexpected explorer result: total=%d entries=%d severity=%s", response.Total, len(response.Entries), response.Entries[0].Severity)
	}
	if len(response.Timeline) != 48 || len(response.Fields) == 0 {
		t.Fatalf("expected timeline and fields, got timeline=%d fields=%d", len(response.Timeline), len(response.Fields))
	}
	if response.LogTail != explorer.MaxLogTail || response.EntryLimit != explorer.MaxEntries || response.Truncated {
		t.Fatalf("unexpected result limits: tail=%d entryLimit=%d truncated=%v", response.LogTail, response.EntryLimit, response.Truncated)
	}

	firstRequest := httptest.NewRequest(http.MethodGet, "/api/explorer?from=2026-08-09T02:59:00Z&to=2026-08-09T03:03:00Z&limit=1&sort=desc", nil)
	firstRecorder := httptest.NewRecorder()
	server.handleExplorer(firstRecorder, firstRequest)
	var firstPage explorer.Response
	if err := json.Unmarshal(firstRecorder.Body.Bytes(), &firstPage); err != nil {
		t.Fatalf("decode first cursor response: %v", err)
	}
	if len(firstPage.Entries) != 1 || firstPage.NextPageToken == "" {
		t.Fatalf("expected one entry and a next cursor, got entries=%d token=%q", len(firstPage.Entries), firstPage.NextPageToken)
	}

	nextQuery := url.Values{
		"from": {"2026-08-09T02:59:00Z"}, "to": {"2026-08-09T03:03:00Z"},
		"limit": {"1"}, "sort": {"desc"}, "pageToken": {firstPage.NextPageToken},
	}
	secondRecorder := httptest.NewRecorder()
	server.handleExplorer(secondRecorder, httptest.NewRequest(http.MethodGet, "/api/explorer?"+nextQuery.Encode(), nil))
	var secondPage explorer.Response
	if err := json.Unmarshal(secondRecorder.Body.Bytes(), &secondPage); err != nil {
		t.Fatalf("decode second cursor response: %v", err)
	}
	if len(secondPage.Entries) != 1 || secondPage.Entries[0].InsertID == firstPage.Entries[0].InsertID {
		t.Fatalf("cursor did not advance: first=%#v second=%#v", firstPage.Entries, secondPage.Entries)
	}
}

func TestHandleExplorerLimitsDockerConcurrency(t *testing.T) {
	containers := make([]docker.Container, 17)
	for index := range containers {
		containers[index] = docker.Container{
			ID: "container-" + string(rune('a'+index)), Names: []string{"/worker"},
			Image: "example/worker:latest", State: "running", Status: "Up 2 minutes",
		}
	}
	var active int32
	var maximum int32
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/containers/json" {
			_ = json.NewEncoder(w).Encode(containers)
			return
		}
		current := atomic.AddInt32(&active, 1)
		for {
			previous := atomic.LoadInt32(&maximum)
			if current <= previous || atomic.CompareAndSwapInt32(&maximum, previous, current) {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
		atomic.AddInt32(&active, -1)
		_, _ = w.Write([]byte("2026-08-09T03:00:00Z worker ready\n"))
	}))
	defer testServer.Close()
	dockerClient := docker.NewClient(testServer.URL)
	server := New(explorer.NewService(dockerClient), dockerClient)
	recorder := httptest.NewRecorder()
	server.handleExplorer(recorder, httptest.NewRequest(http.MethodGet, "/api/explorer?from=2026-08-09T02:59:00Z&to=2026-08-09T03:01:00Z&limit=100", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("handleExplorer returned status %d: %s", recorder.Code, recorder.Body.String())
	}
	if maximum > explorer.MaxConcurrentDockerRequests {
		t.Fatalf("maximum concurrent Docker requests = %d, want <= %d", maximum, explorer.MaxConcurrentDockerRequests)
	}
}

func TestLoggingMiddlewareSkipsSuccessfulResponses(t *testing.T) {
	originalWriter := log.Writer()
	defer log.SetOutput(originalWriter)
	var output bytes.Buffer
	log.SetOutput(&output)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/error" {
			http.Error(w, "failed", http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte("ok"))
	})
	handler := loggingMiddleware(next)
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if output.Len() != 0 {
		t.Fatalf("successful request was logged: %q", output.String())
	}
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/error", nil))
	if !strings.Contains(output.String(), "502") {
		t.Fatalf("failed request did not include its status: %q", output.String())
	}
}

func TestSecurityHeaders(t *testing.T) {
	handler := (&Server{}).Handler()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/health", nil))

	expected := map[string]string{
		"Content-Security-Policy": contentSecurityPolicy,
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":         "DENY",
		"Referrer-Policy":         "strict-origin-when-cross-origin",
		"Permissions-Policy":      "camera=(), microphone=(), geolocation=()",
	}
	for header, value := range expected {
		if actual := recorder.Header().Get(header); actual != value {
			t.Errorf("%s = %q, want %q", header, actual, value)
		}
	}
}

func TestWriteJSONUsesJSONContentType(t *testing.T) {
	recorder := httptest.NewRecorder()
	writeJSON(recorder, http.StatusOK, map[string]bool{"ok": true})
	if actual := recorder.Header().Get("Content-Type"); actual != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want application/json; charset=utf-8", actual)
	}
}

func TestReadOnlyMethodsRejectUnsupportedMethods(t *testing.T) {
	called := false
	handler := getOnly(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})
	for _, method := range []string{http.MethodPost, http.MethodDelete, http.MethodOptions, http.MethodTrace, http.MethodConnect} {
		recorder := httptest.NewRecorder()
		handler(recorder, httptest.NewRequest(method, "/api/status", nil))
		if recorder.Code != http.StatusMethodNotAllowed || recorder.Header().Get("Allow") != "GET, HEAD" {
			t.Fatalf("%s returned status=%d allow=%q", method, recorder.Code, recorder.Header().Get("Allow"))
		}
	}
	unknownRecorder := httptest.NewRecorder()
	handler(unknownRecorder, httptest.NewRequest("BREW", "/api/status", nil))
	if unknownRecorder.Code != http.StatusNotImplemented {
		t.Fatalf("unknown method returned status %d, want 501", unknownRecorder.Code)
	}
	if called {
		t.Fatal("unsupported method reached the handler")
	}
}

func TestTailSupportsHead(t *testing.T) {
	recorder := httptest.NewRecorder()
	(&Server{}).handleTail(recorder, httptest.NewRequest(http.MethodHead, "/api/tail", nil))
	if recorder.Code != http.StatusOK || recorder.Header().Get("Content-Type") != "text/event-stream" {
		t.Fatalf("HEAD /api/tail returned status=%d content-type=%q", recorder.Code, recorder.Header().Get("Content-Type"))
	}
}

func TestHandleTailStreamsFollowLogs(t *testing.T) {
	container := docker.Container{ID: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789", Names: []string{"/api"}}
	var followValue string
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/containers/json":
			_ = json.NewEncoder(w).Encode([]docker.Container{container})
		case "/containers/" + container.ID + "/logs":
			followValue = r.URL.Query().Get("follow")
			if r.URL.Query().Get("tail") != "0" {
				http.Error(w, "expected tail=0", http.StatusBadRequest)
				return
			}
			_, _ = w.Write(testDockerFrame(2, "2026-08-09T03:01:00Z ERROR live failure\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer testServer.Close()
	dockerClient := docker.NewClient(testServer.URL)
	server := New(explorer.NewService(dockerClient), dockerClient)
	recorder := httptest.NewRecorder()
	server.handleTail(recorder, httptest.NewRequest(http.MethodGet, "/api/tail?since=2026-08-09T03:00:00Z&severity=ERROR", nil))
	if recorder.Code != http.StatusOK || followValue != "1" {
		t.Fatalf("handleTail returned status=%d follow=%q body=%s", recorder.Code, followValue, recorder.Body.String())
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "event: ready") || !strings.Contains(body, "event: log") || !strings.Contains(body, "live failure") {
		t.Fatalf("unexpected SSE body: %s", body)
	}
}
