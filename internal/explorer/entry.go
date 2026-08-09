package explorer

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"caroline/internal/docker"
)

type LogLine struct {
	ID          string
	Timestamp   time.Time
	Container   string
	ContainerID string
	Severity    string
	Stream      string
	Message     string
}

func ContainerName(container docker.Container) string {
	if len(container.Names) == 0 {
		return container.ID[:min(12, len(container.ID))]
	}
	return strings.TrimPrefix(container.Names[0], "/")
}

func ToContainerInfo(container docker.Container) ContainerInfo {
	created := time.Unix(container.Created, 0).UTC()
	return ContainerInfo{
		ID:      container.ID,
		Name:    ContainerName(container),
		Image:   container.Image,
		State:   container.State,
		Status:  container.Status,
		Created: created,
		Labels:  container.Labels,
	}
}

func ParseLogFrame(frame docker.Frame, container docker.Container) []LogLine {
	contents := strings.ReplaceAll(string(frame.Data), "\r\n", "\n")
	contents = strings.TrimSuffix(contents, "\n")
	if contents == "" {
		return nil
	}
	lines := strings.Split(contents, "\n")
	result := make([]LogLine, 0, len(lines))
	for index, raw := range lines {
		if raw == "" {
			continue
		}
		timestamp := time.Now().UTC()
		message := raw
		parts := strings.SplitN(raw, " ", 2)
		if len(parts) == 2 {
			if parsed, err := time.Parse(time.RFC3339Nano, parts[0]); err == nil {
				timestamp = parsed.UTC()
				message = parts[1]
			}
		}
		result = append(result, LogLine{
			ID:          lineID(container.ID, frame.Stream, timestamp, message, index),
			Timestamp:   timestamp,
			Container:   ContainerName(container),
			ContainerID: container.ID,
			Severity:    DetectSeverity(message),
			Stream:      frame.Stream,
			Message:     message,
		})
	}
	return result
}

func lineID(containerID, stream string, timestamp time.Time, message string, index int) string {
	value := fmt.Sprintf("%s|%s|%s|%d|%s", containerID, stream, timestamp.Format(time.RFC3339Nano), index, message)
	return fmt.Sprintf("%x", sha1.Sum([]byte(value)))[:16]
}

func DetectSeverity(message string) string {
	upper := strings.ToUpper(message)
	for _, marker := range []string{"PANIC", "FATAL", "CRITICAL", "ERROR", "EXCEPTION"} {
		if strings.Contains(upper, marker) {
			return "ERROR"
		}
	}
	for _, marker := range []string{"WARN", "DEPRECATED"} {
		if strings.Contains(upper, marker) {
			return "WARNING"
		}
	}
	for _, marker := range []string{"DEBUG", "TRACE"} {
		if strings.Contains(upper, marker) {
			return "DEBUG"
		}
	}
	return "INFO"
}

func ToEntry(line LogLine, container docker.Container) Entry {
	textPayload := line.Message
	var jsonPayload map[string]any
	var decoded map[string]any
	if json.Unmarshal([]byte(line.Message), &decoded) == nil {
		jsonPayload = decoded
		if value, ok := decoded["log"].(string); ok {
			textPayload = value
		} else if value, ok := decoded["message"].(string); ok {
			textPayload = value
		}
	}
	labels := map[string]string{
		"container_id":   container.ID,
		"container_name": ContainerName(container),
		"stream":         line.Stream,
	}
	return Entry{
		InsertID:  line.ID,
		Timestamp: line.Timestamp,
		Severity:  line.Severity,
		LogName:   fmt.Sprintf("containers/%s/%s", ContainerName(container), line.Stream),
		Resource: Resource{
			Type: "docker_container",
			Labels: map[string]string{
				"container_name": ContainerName(container),
				"container_id":   container.ID,
				"image":          container.Image,
			},
		},
		Labels:      labels,
		TextPayload: textPayload,
		JSONPayload: jsonPayload,
		Summary:     textPayload,
		Stream:      line.Stream,
	}
}

func explorerSearchText(entry Entry) string {
	parts := []string{entry.Summary, entry.TextPayload, entry.LogName, entry.Resource.Type}
	for key, value := range entry.Resource.Labels {
		parts = append(parts, key, value)
	}
	if entry.JSONPayload != nil {
		if encoded, err := json.Marshal(entry.JSONPayload); err == nil {
			parts = append(parts, string(encoded))
		}
	}
	return strings.Join(parts, " ")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
