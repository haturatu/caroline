package notifier

import (
	"fmt"
	"strings"

	"caroline/internal/alert"
)

type slackPayload struct {
	Text   string       `json:"text"`
	Blocks []slackBlock `json:"blocks,omitempty"`
}

type slackBlock struct {
	Type   string            `json:"type"`
	Text   *slackTextObject  `json:"text,omitempty"`
	Fields []slackTextObject `json:"fields,omitempty"`
}

type slackTextObject struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func buildSlackPayload(notification alert.Notification) slackPayload {
	title := notificationTitle(notification)
	condition := fmt.Sprintf("%d matches ≥ %d / %s", notification.Value, notification.Threshold, formatSeconds(notification.WindowSeconds))
	if notification.Event == "alert.resolved" {
		peak := notification.PeakValue
		if peak < notification.Value {
			peak = notification.Value
		}
		condition = fmt.Sprintf("%d → %d matches / %s", peak, notification.Value, formatSeconds(notification.WindowSeconds))
	}
	container := notification.Container
	if container == "" {
		container = "Unknown"
	}
	query := notification.Query
	if query == "" {
		query = "All logs"
	}
	fields := []slackTextObject{
		{Type: "mrkdwn", Text: "*Container*\n" + truncateSlackText(container)},
		{Type: "mrkdwn", Text: "*Condition*\n" + truncateSlackText(condition)},
		{Type: "mrkdwn", Text: "*Duration*\n" + formatDuration(notification.StartedAt, notification.Timestamp)},
		{Type: "mrkdwn", Text: "*Query*\n" + truncateSlackText(query)},
	}
	if notification.Severity != "" {
		fields = append(fields, slackTextObject{Type: "mrkdwn", Text: "*Severity*\n" + truncateSlackText(notification.Severity)})
	}
	if len(notification.Labels) > 0 {
		fields = append(fields, slackTextObject{Type: "mrkdwn", Text: "*Labels*\n" + truncateSlackText(formatLabels(notification.Labels))})
	}
	if notification.Event == "alert.resolved" {
		fields = append(fields, slackTextObject{Type: "mrkdwn", Text: "*Resolved at*\n" + formatTimestamp(notification.Timestamp)})
	} else if notification.StartedAt != nil {
		fields = append(fields, slackTextObject{Type: "mrkdwn", Text: "*First triggered*\n" + formatTimestamp(*notification.StartedAt)})
	}
	blocks := []slackBlock{
		{Type: "header", Text: &slackTextObject{Type: "plain_text", Text: truncateSlackHeader(title)}},
		{Type: "section", Fields: fields},
	}
	if notification.Sample != nil {
		sample := strings.TrimSpace(notification.Sample.Summary)
		if sample == "" {
			sample = strings.TrimSpace(notification.Sample.TextPayload)
		}
		if sample != "" {
			blocks = append(blocks, slackBlock{
				Type: "section",
				Text: &slackTextObject{Type: "mrkdwn", Text: "*Sample*\n" + truncateSlackText(sample)},
			})
		}
	}
	links := make([]string, 0, 2)
	if notification.ExplorerURL != "" {
		links = append(links, fmt.Sprintf("<%s|Open in Caroline>", notification.ExplorerURL))
	}
	if notification.RunbookURL != "" {
		links = append(links, fmt.Sprintf("<%s|Runbook>", notification.RunbookURL))
	}
	if len(links) > 0 {
		blocks = append(blocks, slackBlock{Type: "section", Text: &slackTextObject{Type: "mrkdwn", Text: strings.Join(links, " · ")}})
	}
	return slackPayload{
		Text:   title + " — " + condition,
		Blocks: blocks,
	}
}

func notificationTitle(notification alert.Notification) string {
	status := "🔴 FIRING"
	if notification.Event == "alert.resolved" {
		status = "🟢 RESOLVED"
	}
	return truncateSlackText(status + " · " + notification.Rule)
}

func truncateSlackText(value string) string {
	return truncateDiscordText(value, 3000)
}

func truncateSlackHeader(value string) string {
	return truncateDiscordText(value, 150)
}
