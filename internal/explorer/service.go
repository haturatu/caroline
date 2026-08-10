package explorer

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"caroline/internal/docker"
)

type fetchedEntries struct {
	container docker.Container
	entries   []Entry
	err       error
}

func (s *Service) Search(ctx context.Context, request SearchRequest) (Response, error) {
	containers, err := s.docker.ListRunning(ctx)
	if err != nil {
		return Response{}, err
	}

	selectedContainers := make([]docker.Container, 0, len(containers))
	for _, container := range containers {
		if MatchesContainerSelection(container, request.Selected) || len(request.Selected) == 0 {
			selectedContainers = append(selectedContainers, container)
		}
	}

	response := Response{
		Entries:     make([]Entry, 0),
		Containers:  make([]ContainerInfo, 0, len(containers)),
		GeneratedAt: time.Now().UTC(),
		From:        request.From,
		To:          request.To,
		Duration:    request.Duration,
		Query:       request.Query,
		Approximate: true,
		LogTail:     MaxLogTail,
		EntryLimit:  MaxEntries,
	}

	containerInfos := make(map[string]ContainerInfo, len(containers))
	for _, container := range containers {
		containerInfos[container.ID] = ToContainerInfo(container)
	}

	results := make(chan fetchedEntries, len(selectedContainers))
	semaphore := make(chan struct{}, MaxConcurrentDockerRequests)
	var wait sync.WaitGroup
	for _, container := range selectedContainers {
		container := container
		wait.Add(1)
		go func() {
			defer wait.Done()
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				results <- fetchedEntries{container: container, err: ctx.Err()}
				return
			}

			frames, fetchErr := s.docker.Logs(ctx, container.ID, MaxLogTail, request.From)
			if fetchErr != nil {
				results <- fetchedEntries{container: container, err: fetchErr}
				return
			}

			entries := make([]Entry, 0, MaxLogTail)
			for _, frame := range frames {
				for _, line := range ParseLogFrame(frame, container) {
					entry := ToEntry(line, container)
					if entry.Timestamp.Before(request.From) || entry.Timestamp.After(request.To) {
						continue
					}
					if !MatchesFilters(entry, request.Query, request.Severity, request.Stream) {
						continue
					}
					entries = append(entries, entry)
				}
			}
			results <- fetchedEntries{container: container, entries: entries}
		}()
	}
	go func() {
		wait.Wait()
		close(results)
	}()

	for result := range results {
		info := ToContainerInfo(result.container)
		if result.err != nil {
			response.Errors = append(response.Errors, info.Name+": "+result.err.Error())
			containerInfos[result.container.ID] = info
			continue
		}

		info.LogCount = len(result.entries)
		for _, entry := range result.entries {
			switch entry.Severity {
			case "ERROR":
				info.ErrorCount++
			case "WARNING":
				info.WarningCount++
			}
		}
		containerInfos[result.container.ID] = info

		response.Entries = append(response.Entries, result.entries...)
	}

	for _, container := range containers {
		response.Containers = append(response.Containers, containerInfos[container.ID])
	}

	sortEntries(response.Entries, request.Sort)
	if len(response.Entries) > MaxEntries {
		response.Entries = response.Entries[:MaxEntries]
		response.Truncated = true
	}
	sort.Slice(response.Containers, func(i, j int) bool {
		return response.Containers[i].Name < response.Containers[j].Name
	})

	response.Total = len(response.Entries)
	response.Timeline = BuildTimeline(
		response.Entries,
		request.From,
		request.To,
		request.TimelineBuckets,
	)
	response.Fields = BuildFieldGroups(response.Entries)

	start := 0
	if request.Cursor != nil {
		start = len(response.Entries)
		for index, entry := range response.Entries {
			if IsAfterCursor(entry, *request.Cursor, request.Sort) {
				start = index
				break
			}
		}
	}
	if start >= len(response.Entries) {
		response.Entries = []Entry{}
		return response, nil
	}

	limit := request.Limit
	if limit < 1 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	end := min(start+limit, len(response.Entries))
	page := response.Entries[start:end]
	response.Entries = page
	if end < response.Total && len(page) > 0 {
		response.NextPageToken = EncodeCursor(page[len(page)-1])
	}
	return response, nil
}

func MatchesFilters(entry Entry, query, severity, stream string) bool {
	return (severity == "" || severity == "ALL" || strings.EqualFold(entry.Severity, severity)) &&
		(stream == "" || entry.Stream == stream) &&
		MatchesQuery(entry, query)
}

func sortEntries(entries []Entry, sortOrder string) {
	sort.Slice(entries, func(i, j int) bool {
		left, right := entries[i], entries[j]
		if left.Timestamp.Equal(right.Timestamp) {
			if sortOrder == "asc" {
				return left.InsertID < right.InsertID
			}
			return left.InsertID > right.InsertID
		}
		if sortOrder == "asc" {
			return left.Timestamp.Before(right.Timestamp)
		}
		return left.Timestamp.After(right.Timestamp)
	})
}
