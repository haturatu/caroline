package logstream

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"caroline/internal/docker"
	"caroline/internal/explorer"
)

type fakeSource struct {
	container     docker.Container
	containers    []docker.Container
	followCalls   atomic.Int32
	followStarted chan struct{}
	followRelease chan struct{}
}

func (s *fakeSource) ListRunning(context.Context) ([]docker.Container, error) {
	if len(s.containers) > 0 {
		return s.containers, nil
	}
	return []docker.Container{s.container}, nil
}

func (s *fakeSource) FollowLogs(
	ctx context.Context,
	_ string,
	_ time.Time,
	onFrame func(docker.Frame) error,
) error {
	s.followCalls.Add(1)
	if s.followStarted != nil {
		close(s.followStarted)
		select {
		case <-s.followRelease:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
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
	}, followStarted: make(chan struct{}), followRelease: make(chan struct{})}
	manager := NewManager(source)
	defer manager.Close()

	first, err := manager.Subscribe(context.Background(), nil, time.Time{}, 0)
	if err != nil {
		t.Fatalf("first subscription: %v", err)
	}
	select {
	case <-source.followStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for shared stream to start")
	}
	second, err := manager.Subscribe(context.Background(), nil, time.Time{}, 0)
	if err != nil {
		t.Fatalf("second subscription: %v", err)
	}
	close(source.followRelease)
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

func TestManagerDoesNotCapLiveTailWhenLimitIsDisabled(t *testing.T) {
	containers := make([]docker.Container, 9)
	for index := range containers {
		containers[index] = docker.Container{
			ID:    fmt.Sprintf("container-%d", index),
			Names: []string{fmt.Sprintf("/worker-%d", index)},
		}
	}
	source := &fakeSource{containers: containers}
	manager := NewManager(source)
	defer manager.Close()

	subscription, err := manager.Subscribe(context.Background(), nil, time.Time{}, explorer.MaxTailStreams)
	if err != nil {
		t.Fatalf("Subscribe returned error: %v", err)
	}
	defer subscription.Close()
	if subscription.SelectedContainers != len(containers) || subscription.StreamedContainers != len(containers) {
		t.Fatalf("tail selected=%d streamed=%d, want %d", subscription.SelectedContainers, subscription.StreamedContainers, len(containers))
	}
}

type retrySource struct {
	base     *fakeSource
	failures atomic.Int32
}

func (s *retrySource) ListRunning(ctx context.Context) ([]docker.Container, error) {
	if s.failures.Load() > 0 && s.failures.Add(-1) >= 0 {
		return nil, errors.New("temporary Docker API failure")
	}
	return s.base.ListRunning(ctx)
}

func (s *retrySource) FollowLogs(
	ctx context.Context,
	containerID string,
	since time.Time,
	onFrame func(docker.Frame) error,
) error {
	return s.base.FollowLogs(ctx, containerID, since, onFrame)
}

func TestManagerRetriesContainerDiscovery(t *testing.T) {
	base := &fakeSource{container: docker.Container{
		ID:    "container-id",
		Names: []string{"/api"},
	}}
	source := &retrySource{base: base}
	source.failures.Store(1)
	manager := NewManager(source)
	defer manager.Close()

	subscription, err := manager.Subscribe(context.Background(), nil, time.Time{}, explorer.MaxTailStreams)
	if err != nil {
		t.Fatalf("Subscribe returned error after transient failure: %v", err)
	}
	subscription.Close()
}
