package explorer

import (
	"context"
	"strings"
	"testing"
	"time"

	"caroline/internal/docker"
)

type orderedSearchSource struct {
	containers []docker.Container
	frames     map[string]docker.Frame
	delays     map[string]time.Duration
}

func (s *orderedSearchSource) ListRunning(context.Context) ([]docker.Container, error) {
	return s.containers, nil
}

func (s *orderedSearchSource) Logs(ctx context.Context, id string, _ int, _ time.Time) ([]docker.Frame, error) {
	if delay := s.delays[id]; delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return []docker.Frame{s.frames[id]}, nil
}

func (s *orderedSearchSource) FollowLogs(context.Context, string, time.Time, func(docker.Frame) error) error {
	return nil
}

func TestSearchSortsBeforeApplyingGlobalEntryLimit(t *testing.T) {
	from := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	oldContainer := docker.Container{ID: "old", Names: []string{"/old"}}
	newContainer := docker.Container{ID: "new", Names: []string{"/new"}}
	var oldLines strings.Builder
	for index := 0; index < MaxEntries; index++ {
		if index > 0 {
			oldLines.WriteByte('\n')
		}
		oldLines.WriteString(from.Add(time.Duration(index) * time.Millisecond).Format(time.RFC3339Nano))
		oldLines.WriteString(" old entry")
	}
	source := &orderedSearchSource{
		containers: []docker.Container{oldContainer, newContainer},
		frames: map[string]docker.Frame{
			oldContainer.ID: {Stream: "stdout", Data: []byte(oldLines.String())},
			newContainer.ID: {Stream: "stdout", Data: []byte(from.Add(time.Hour).Format(time.RFC3339Nano) + " newest entry")},
		},
		delays: map[string]time.Duration{newContainer.ID: 25 * time.Millisecond},
	}
	service := NewService(source)
	response, err := service.Search(context.Background(), SearchRequest{
		From:  from,
		To:    from.Add(2 * time.Hour),
		Sort:  "desc",
		Limit: 1,
	})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(response.Entries) != 1 || response.Entries[0].Summary != "newest entry" {
		t.Fatalf("global limit discarded newest entry: %#v", response.Entries)
	}
	if !response.Truncated {
		t.Fatal("Search did not mark the response as truncated")
	}
}
