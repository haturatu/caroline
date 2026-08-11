package notifier

import (
	"fmt"
	"strings"

	"caroline/internal/alert"
)

func ntfyHeaders(notification alert.Notification) map[string]string {
	headers := map[string]string{
		"Content-Type": "text/plain; charset=utf-8",
		"Title":        truncateDiscordText(notificationTitle(notification), 250),
		"Priority":     ntfyPriority(notification),
		"Tags":         ntfyTags(notification),
	}
	if notification.ExplorerURL != "" {
		headers["Click"] = notification.ExplorerURL
	}
	return headers
}

func ntfyPriority(notification alert.Notification) string {
	if notification.Event == "alert.resolved" {
		return "3"
	}
	switch strings.ToLower(notification.Severity) {
	case "critical", "emergency", "fatal":
		return "5"
	case "warning", "warn":
		return "4"
	default:
		return "3"
	}
}

func ntfyTags(notification alert.Notification) string {
	if notification.Event == "alert.resolved" {
		return "white_check_mark"
	}
	switch strings.ToLower(notification.Severity) {
	case "critical", "emergency", "fatal":
		return "rotating_light"
	case "warning", "warn":
		return "warning"
	default:
		return "bell"
	}
}

func ntfyMessage(notification alert.Notification) string {
	condition := fmt.Sprintf("%d matches >= %d / %s", notification.Value, notification.Threshold, formatSeconds(notification.WindowSeconds))
	if notification.Event == "alert.resolved" {
		peak := notification.PeakValue
		if peak < notification.Value {
			peak = notification.Value
		}
		condition = fmt.Sprintf("%d -> %d matches / %s", peak, notification.Value, formatSeconds(notification.WindowSeconds))
	}
	container := notification.Container
	if container == "" {
		container = "Unknown"
	}
	query := notification.Query
	if query == "" {
		query = "All logs"
	}
	lines := []string{
		notificationTitle(notification),
		"",
		"Container: " + truncateDiscordText(container, 500),
		"Condition: " + truncateDiscordText(condition, 500),
		"Duration: " + formatDuration(notification.StartedAt, notification.Timestamp),
		"Query: " + truncateDiscordText(query, 1000),
	}
	if notification.Event == "alert.resolved" {
		lines = append(lines, "Resolved at: "+formatTimestamp(notification.Timestamp))
	} else if notification.StartedAt != nil {
		lines = append(lines, "First triggered: "+formatTimestamp(*notification.StartedAt))
	}
	if notification.Sample != nil {
		sample := strings.TrimSpace(notification.Sample.Summary)
		if sample == "" {
			sample = strings.TrimSpace(notification.Sample.TextPayload)
		}
		if sample != "" {
			lines = append(lines, "Sample: "+truncateDiscordText(sample, 2000))
		}
	}
	if notification.ExplorerURL != "" {
		lines = append(lines, "Caroline: "+notification.ExplorerURL)
	}
	if notification.RunbookURL != "" {
		lines = append(lines, "Runbook: "+notification.RunbookURL)
	}
	return strings.Join(lines, "\n")
}
