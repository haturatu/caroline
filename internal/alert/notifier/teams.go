package notifier

import (
	"fmt"
	"strings"

	"caroline/internal/alert"
)

type teamsPayload struct {
	Type        string            `json:"type"`
	Attachments []teamsAttachment `json:"attachments"`
}

type teamsAttachment struct {
	ContentType string            `json:"contentType"`
	ContentURL  *string           `json:"contentUrl"`
	Content     teamsAdaptiveCard `json:"content"`
}

type teamsAdaptiveCard struct {
	Schema  string             `json:"$schema"`
	Type    string             `json:"type"`
	Version string             `json:"version"`
	Body    []teamsCardElement `json:"body"`
	Actions []teamsCardAction  `json:"actions,omitempty"`
}

type teamsCardElement struct {
	Type      string      `json:"type"`
	Text      string      `json:"text,omitempty"`
	Weight    string      `json:"weight,omitempty"`
	Size      string      `json:"size,omitempty"`
	Wrap      bool        `json:"wrap,omitempty"`
	Spacing   string      `json:"spacing,omitempty"`
	Separator bool        `json:"separator,omitempty"`
	Facts     []teamsFact `json:"facts,omitempty"`
}

type teamsFact struct {
	Title string `json:"title"`
	Value string `json:"value"`
}

type teamsCardAction struct {
	Type  string `json:"type"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

func buildTeamsPayload(notification alert.Notification) teamsPayload {
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
	facts := []teamsFact{
		{Title: "Container", Value: truncateDiscordText(container, 500)},
		{Title: "Condition", Value: truncateDiscordText(condition, 500)},
		{Title: "Duration", Value: formatDuration(notification.StartedAt, notification.Timestamp)},
		{Title: "Query", Value: truncateDiscordText(query, 1000)},
	}
	if notification.Severity != "" {
		facts = append(facts, teamsFact{Title: "Severity", Value: truncateDiscordText(notification.Severity, 500)})
	}
	if len(notification.Labels) > 0 {
		facts = append(facts, teamsFact{Title: "Labels", Value: truncateDiscordText(formatLabels(notification.Labels), 1000)})
	}
	if notification.Event == "alert.resolved" {
		facts = append(facts, teamsFact{Title: "Resolved at", Value: formatTimestamp(notification.Timestamp)})
	} else if notification.StartedAt != nil {
		facts = append(facts, teamsFact{Title: "First triggered", Value: formatTimestamp(*notification.StartedAt)})
	}
	body := []teamsCardElement{
		{Type: "TextBlock", Text: notificationTitle(notification), Weight: "Bolder", Size: "Large", Wrap: true},
		{Type: "FactSet", Facts: facts, Spacing: "Medium"},
	}
	if notification.Sample != nil {
		sample := strings.TrimSpace(notification.Sample.Summary)
		if sample == "" {
			sample = strings.TrimSpace(notification.Sample.TextPayload)
		}
		if sample != "" {
			body = append(body, teamsCardElement{Type: "TextBlock", Text: "Sample\n" + truncateDiscordText(sample, 2000), Wrap: true, Spacing: "Medium", Separator: true})
		}
	}
	actions := make([]teamsCardAction, 0, 2)
	if notification.ExplorerURL != "" {
		actions = append(actions, teamsCardAction{Type: "Action.OpenUrl", Title: "Open in Caroline", URL: notification.ExplorerURL})
	}
	if notification.RunbookURL != "" {
		actions = append(actions, teamsCardAction{Type: "Action.OpenUrl", Title: "Runbook", URL: notification.RunbookURL})
	}
	return teamsPayload{
		Type: "message",
		Attachments: []teamsAttachment{{
			ContentType: "application/vnd.microsoft.card.adaptive",
			Content: teamsAdaptiveCard{
				Schema:  "http://adaptivecards.io/schemas/adaptive-card.json",
				Type:    "AdaptiveCard",
				Version: "1.2",
				Body:    body,
				Actions: actions,
			},
		}},
	}
}
