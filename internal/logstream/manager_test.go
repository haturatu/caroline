package logstream

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"caroline/internal/docker"
)

type fakeSource struct {
	container   docker.Container
	followCalls atomic.Int32
}

func (s *fakeSource) ListRunning(context.Context) ([]docker.Container, error) {
	return []docker.Container{s.container}, nil
}

func (s *fakeSource) FollowLogs(
	ctx context.Context,
	_ string,
	_ time.Time,
	onFrame func(docker.Frame) error,
) error {
	s.followCalls.Add(1)
	if err := onFrame(docker.Frame{
		Stream: "stdout",
		Data:   []byte("2026-08-10T00:00:01Z ERROR shared stream\n"),
	}); err != nil {
		return err
	}
	<-ctx.Done()
	return ctx.Err()
}

func TestManagerSharesContainerFollowStream(t *testing.T) {
	source := &fakeSource{container: docker.Container{
		ID:    "container-id",
		Names: []string{"/api"},
	}}
	manager := NewManager(source)
	defer manager.Close()

	first, err := manager.Subscribe(context.Background(), nil, time.Time{}, 0)
	if err != nil {
		t.Fatalf("first subscription: %v", err)
	}
	second, err := manager.Subscribe(context.Background(), nil, time.Time{}, 0)
	if err != nil {
		t.Fatalf("second subscription: %v", err)
	}
	defer first.Close()
	defer second.Close()

	select {
	case entry := <-first.Entries:
		if entry.Severity != "ERROR" || entry.Summary != "ERROR shared stream" {
			t.Fatalf("unexpected first entry: %#v", entry)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first subscriber")
	}
	select {
	case entry := <-second.Entries:
		if entry.InsertID == "" {
			t.Fatal("second subscriber received an empty entry")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for second subscriber")
	}
	if calls := source.followCalls.Load(); calls != 1 {
		t.Fatalf("FollowLogs called %d times, want 1", calls)
	}
}
