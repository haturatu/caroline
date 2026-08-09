package explorer

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"caroline/internal/docker"
)

func ParseWindow(fromValue, toValue, durationName string, now time.Time) (time.Time, time.Time, string) {
	to := now.UTC()
	if value := strings.TrimSpace(toValue); value != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
			to = parsed.UTC()
		}
	}
	duration := ParseDuration(durationName)
	from := to.Add(-duration)
	if value := strings.TrimSpace(fromValue); value != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
			from = parsed.UTC()
			duration = to.Sub(from)
		}
	}
	if durationName == "" {
		durationName = duration.String()
	}
	return from, to, durationName
}

func RequestedContainers(value string) map[string]bool {
	result := make(map[string]bool)
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			result[item] = true
		}
	}
	return result
}

func MatchesContainerSelection(container docker.Container, selected map[string]bool) bool {
	if selected[container.ID] || selected[container.ID[:min(12, len(container.ID))]] {
		return true
	}
	return selected[ContainerName(container)]
}

func EncodeCursor(entry Entry) string {
	payload, _ := json.Marshal(Cursor{Timestamp: entry.Timestamp, InsertID: entry.InsertID})
	return base64.RawURLEncoding.EncodeToString(payload)
}

func DecodeCursor(value string) (Cursor, error) {
	var cursor Cursor
	payload, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return cursor, fmt.Errorf("invalid page cursor")
	}
	if err := json.Unmarshal(payload, &cursor); err != nil || cursor.Timestamp.IsZero() || cursor.InsertID == "" {
		return cursor, fmt.Errorf("invalid page cursor")
	}
	return cursor, nil
}

func IsAfterCursor(entry Entry, cursor Cursor, sortOrder string) bool {
	if entry.Timestamp.Equal(cursor.Timestamp) {
		if sortOrder == "asc" {
			return entry.InsertID > cursor.InsertID
		}
		return entry.InsertID < cursor.InsertID
	}
	if sortOrder == "asc" {
		return entry.Timestamp.After(cursor.Timestamp)
	}
	return entry.Timestamp.Before(cursor.Timestamp)
}
