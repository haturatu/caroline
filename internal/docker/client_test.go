package docker

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func testHTTPClient(server *httptest.Server) *Client {
	return &Client{client: server.Client(), followClient: server.Client(), baseURL: server.URL}
}

func TestInspectContainerReturnsLoggingConfiguration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/containers/container-1/json" {
			t.Fatalf("request path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"HostConfig":{"LogConfig":{"Type":"local","Config":{"max-size":"10m","max-file":"3"}}}}`))
	}))
	defer server.Close()

	config, err := testHTTPClient(server).InspectContainer(context.Background(), "container-1")
	if err != nil {
		t.Fatalf("InspectContainer: %v", err)
	}
	if config.Type != "local" || config.Config["max-size"] != "10m" || config.Config["max-file"] != "3" {
		t.Fatalf("unexpected logging config: %#v", config)
	}
}

func TestOldestLogTimeReadsFirstAvailableTimestamp(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/containers/container-1/logs" {
			t.Fatalf("request path = %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("tail"); got != "all" {
			t.Fatalf("tail = %q, want all", got)
		}
		_, _ = w.Write([]byte("2026-08-08T13:21:42.000000000Z oldest\n2026-08-08T13:22:42.000000000Z newer\n"))
	}))
	defer server.Close()

	got, err := testHTTPClient(server).OldestLogTime(context.Background(), "container-1")
	want := time.Date(2026, 8, 8, 13, 21, 42, 0, time.UTC)
	if err != nil || !got.Equal(want) {
		t.Fatalf("OldestLogTime = %s, err=%v; want %s", got, err, want)
	}
}

func TestLogsUsesAllForNegativeTail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("tail"); got != "all" {
			t.Fatalf("tail = %q, want all", got)
		}
		_, _ = w.Write([]byte("log\n"))
	}))
	defer server.Close()

	if _, err := testHTTPClient(server).Logs(context.Background(), "container-1", -1, time.Time{}); err != nil {
		t.Fatalf("Logs: %v", err)
	}
}

func TestFollowLogsReturnsTypedNotFoundError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"No such container: old-id"}`))
	}))
	defer server.Close()

	err := testHTTPClient(server).FollowLogs(context.Background(), "old-id", time.Time{}, func(Frame) error {
		return nil
	})
	if !IsNotFound(err) {
		t.Fatalf("FollowLogs error = %v, want Docker not-found error", err)
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
		t.Fatalf("FollowLogs error type = %#v, want status %d", err, http.StatusNotFound)
	}
}
