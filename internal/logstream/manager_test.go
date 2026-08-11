package logstream

import (
	"context"
	"errors"
	"fmt"
	"sync"
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

type reconciliationSource struct {
	mu         sync.RWMutex
	containers []docker.Container
	started    chan string
	canceled   chan string
}

func (s *reconciliationSource) ListRunning(context.Context) ([]docker.Container, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]docker.Container(nil), s.containers...), nil
}

func (s *reconciliationSource) FollowLogs(
	ctx context.Context,
	containerID string,
	_ time.Time,
	_ func(docker.Frame) error,
) error {
	select {
	case s.started <- containerID:
	default:
	}
	<-ctx.Done()
	select {
	case s.canceled <- containerID:
	default:
	}
	return ctx.Err()
}

func (s *reconciliationSource) setContainers(containers ...docker.Container) {
	s.mu.Lock()
	s.containers = append([]docker.Container(nil), containers...)
	s.mu.Unlock()
}

func waitForContainerEvent(t *testing.T, events <-chan string, want string) {
	t.Helper()
	select {
	case got := <-events:
		if got != want {
			t.Fatalf("container event = %q, want %q", got, want)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for container event %q", want)
	}
}

func TestManagerReconcilesRecreatedContainers(t *testing.T) {
	oldContainer := docker.Container{ID: "old-id", Names: []string{"/caroline"}}
	newContainer := docker.Container{ID: "new-id", Names: []string{"/caroline"}}
	source := &reconciliationSource{
		containers: []docker.Container{oldContainer},
		started:    make(chan string, 4),
		canceled:   make(chan string, 4),
	}
	manager := NewManager(source)
	defer manager.Close()

	subscription, err := manager.Subscribe(context.Background(), nil, time.Time{}, 0)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer subscription.Close()
	waitForContainerEvent(t, source.started, oldContainer.ID)

	source.setContainers(newContainer)
	if err := manager.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	waitForContainerEvent(t, source.canceled, oldContainer.ID)
	waitForContainerEvent(t, source.started, newContainer.ID)

	manager.mu.Lock()
	_, oldExists := manager.streams[oldContainer.ID]
	_, newExists := manager.streams[newContainer.ID]
	manager.mu.Unlock()
	if oldExists || !newExists {
		t.Fatalf("streams after reconciliation: old=%v new=%v", oldExists, newExists)
	}
}

type notFoundSource struct {
	container docker.Container
	started   chan struct{}
	calls     atomic.Int32
}

func (s *notFoundSource) ListRunning(context.Context) ([]docker.Container, error) {
	return []docker.Container{s.container}, nil
}

func (s *notFoundSource) FollowLogs(
	context.Context,
	string,
	time.Time,
	func(docker.Frame) error,
) error {
	s.calls.Add(1)
	select {
	case s.started <- struct{}{}:
	default:
	}
	return &docker.APIError{StatusCode: 404, Status: "404 Not Found", Message: "No such container"}
}

func TestManagerDoesNotRetryRemovedContainerAfterNotFound(t *testing.T) {
	source := &notFoundSource{
		container: docker.Container{ID: "removed-id", Names: []string{"/caroline"}},
		started:   make(chan struct{}, 1),
	}
	manager := NewManager(source)
	defer manager.Close()

	subscription, err := manager.Subscribe(context.Background(), nil, time.Time{}, 0)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer subscription.Close()
	select {
	case <-source.started:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for FollowLogs")
	}

	deadline := time.After(time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-deadline:
			t.Fatal("not-found stream remained in manager")
		case <-ticker.C:
			manager.mu.Lock()
			_, exists := manager.streams[source.container.ID]
			manager.mu.Unlock()
			if !exists {
				if calls := source.calls.Load(); calls != 1 {
					t.Fatalf("FollowLogs calls = %d, want 1", calls)
				}
				return
			}
		}
	}
}

func TestManagerSkipsContainersWithCollectFalseLabel(t *testing.T) {
	collected := docker.Container{ID: "collected", Names: []string{"/api"}}
	skipped := docker.Container{
		ID: "skipped", Names: []string{"/caroline-agent"},
		Labels: map[string]string{docker.CollectLabel: "false"},
	}
	source := &fakeSource{containers: []docker.Container{collected, skipped}}
	manager := NewManager(source)
	defer manager.Close()

	subscription, err := manager.Subscribe(context.Background(), nil, time.Time{}, 0)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer subscription.Close()
	if subscription.SelectedContainers != 1 || subscription.StreamedContainers != 1 {
		t.Fatalf("selected=%d streamed=%d, want 1/1", subscription.SelectedContainers, subscription.StreamedContainers)
	}
}
