package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"caroline/internal/alert"
)

type Webhook struct {
	Client          *http.Client
	ExplorerBaseURL string
}

type webhookProvider string

const (
	providerGeneric webhookProvider = "generic"
	providerDiscord webhookProvider = "discord"
	providerSlack   webhookProvider = "slack"
	providerNtfy    webhookProvider = "ntfy"
	providerTeams   webhookProvider = "teams"
)

func (w Webhook) Notify(ctx context.Context, rule alert.Rule, notification alert.Notification) error {
	if rule.WebhookURL == "" {
		return nil
	}
	client := w.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	notification = alert.SanitizeNotification(notification)
	if notification.RunbookURL == "" {
		notification.RunbookURL = rule.RunbookURL
	}
	notification = enrichNotification(notification, w.ExplorerBaseURL)
	endpoint := rule.WebhookURL
	var body []byte
	var err error
	headers := map[string]string{"Content-Type": "application/json"}
	switch detectWebhookProvider(endpoint) {
	case providerDiscord:
		endpoint, err = discordEndpoint(endpoint)
		if err != nil {
			return err
		}
		body, err = json.Marshal(discordPayload(notification))
	case providerSlack:
		body, err = json.Marshal(buildSlackPayload(notification))
	case providerNtfy:
		body = []byte(ntfyMessage(notification))
		headers = ntfyHeaders(notification)
	case providerTeams:
		body, err = json.Marshal(buildTeamsPayload(notification))
	case providerGeneric:
		body, err = json.Marshal(notification)
	}
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	request.Header.Set("User-Agent", "Caroline-Alert/1.0")
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4*1024))
		return fmt.Errorf("webhook returned %s: %s", response.Status, string(message))
	}
	return nil
}

type discordWebhookPayload struct {
	Username        string                 `json:"username"`
	Embeds          []discordEmbed         `json:"embeds"`
	AllowedMentions discordAllowedMentions `json:"allowed_mentions"`
}

type discordAllowedMentions struct {
	Parse []string `json:"parse"`
}

type discordEmbed struct {
	Title       string              `json:"title"`
	URL         string              `json:"url,omitempty"`
	Description string              `json:"description"`
	Color       int                 `json:"color"`
	Timestamp   string              `json:"timestamp"`
	Fields      []discordEmbedField `json:"fields,omitempty"`
	Footer      *discordEmbedFooter `json:"footer,omitempty"`
}

type discordEmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

type discordEmbedFooter struct {
	Text string `json:"text"`
}

func discordPayload(notification alert.Notification) discordWebhookPayload {
	event := "🔴 FIRING"
	color := 0xED4245
	if notification.Event == "alert.resolved" {
		event = "🟢 RESOLVED"
		color = 0x57F287
	}

	fields := make([]discordEmbedField, 0, 10)
	if notification.Container != "" {
		fields = append(fields, discordEmbedField{Name: "Container", Value: truncateDiscordText(notification.Container, 1024), Inline: true})
	}
	if notification.Event == "alert.resolved" {
		peak := notification.PeakValue
		if peak < notification.Value {
			peak = notification.Value
		}
		fields = append(fields,
			discordEmbedField{Name: "Peak / Current", Value: fmt.Sprintf("%d → %d matches / %s", peak, notification.Value, formatSeconds(notification.WindowSeconds)), Inline: true},
			discordEmbedField{Name: "Duration", Value: formatDuration(notification.StartedAt, notification.Timestamp), Inline: true},
			discordEmbedField{Name: "Resolved at", Value: formatTimestamp(notification.Timestamp), Inline: true},
		)
	} else {
		fields = append(fields,
			discordEmbedField{Name: "Condition", Value: fmt.Sprintf("%d matches ≥ %d / %s", notification.Value, notification.Threshold, formatSeconds(notification.WindowSeconds)), Inline: true},
			discordEmbedField{Name: "First triggered", Value: formatTimestamp(pointerOr(&notification.Timestamp, notification.StartedAt)), Inline: true},
			discordEmbedField{Name: "Duration", Value: formatDuration(notification.StartedAt, notification.Timestamp), Inline: true},
		)
	}
	if notification.Severity != "" {
		fields = append(fields, discordEmbedField{Name: "Severity", Value: truncateDiscordText(notification.Severity, 1024), Inline: true})
	}
	query := notification.Query
	if query == "" {
		query = "All logs"
	}
	fields = append(fields, discordEmbedField{Name: "Query", Value: truncateDiscordText(query, 1024)})
	if len(notification.Labels) > 0 {
		fields = append(fields, discordEmbedField{Name: "Labels", Value: truncateDiscordText(formatLabels(notification.Labels), 1024)})
	}
	if notification.Sample != nil {
		sample := strings.TrimSpace(notification.Sample.Summary)
		if sample == "" {
			sample = strings.TrimSpace(notification.Sample.TextPayload)
		}
		if sample != "" {
			fields = append(fields, discordEmbedField{
				Name:  "Sample",
				Value: truncateDiscordText(sample, 1024),
			})
		}
	}
	links := make([]string, 0, 2)
	if notification.ExplorerURL != "" {
		links = append(links, fmt.Sprintf("[Open in Caroline](%s)", notification.ExplorerURL))
	}
	if notification.RunbookURL != "" {
		links = append(links, fmt.Sprintf("[Runbook](%s)", notification.RunbookURL))
	}
	if len(links) > 0 {
		fields = append(fields, discordEmbedField{Name: "Actions", Value: strings.Join(links, " · ")})
	}
	footerText := "Cooldown " + formatSeconds(notification.CooldownSeconds)
	if notification.RuleID != "" {
		footerText += " · " + notification.RuleID
	}

	return discordWebhookPayload{
		Username: "Caroline",
		Embeds: []discordEmbed{{
			Title:       truncateDiscordText(fmt.Sprintf("%s · %s", event, notification.Rule), 256),
			URL:         notification.ExplorerURL,
			Description: discordDescription(notification),
			Color:       color,
			Timestamp:   notification.Timestamp.UTC().Format(time.RFC3339Nano),
			Fields:      fields,
			Footer:      &discordEmbedFooter{Text: truncateDiscordText(footerText, 2048)},
		}},
		AllowedMentions: discordAllowedMentions{Parse: []string{}},
	}
}

func discordDescription(notification alert.Notification) string {
	if notification.Event == "alert.resolved" {
		return "The alert condition has recovered."
	}
	return "The alert condition is firing."
}

func pointerOr(fallback *time.Time, value *time.Time) time.Time {
	if value != nil {
		return *value
	}
	return *fallback
}

func formatTimestamp(value time.Time) string {
	return value.UTC().Format("2006-01-02 15:04:05 MST")
}

func formatDuration(startedAt *time.Time, end time.Time) string {
	if startedAt == nil {
		return "Unknown"
	}
	duration := end.Sub(*startedAt)
	if duration < 0 {
		duration = 0
	}
	return formatDurationValue(duration)
}

func formatDurationValue(duration time.Duration) string {
	seconds := int64(duration / time.Second)
	if seconds < 1 {
		return "0s"
	}
	parts := make([]string, 0, 3)
	if days := seconds / (24 * 60 * 60); days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
		seconds %= 24 * 60 * 60
	}
	if hours := seconds / (60 * 60); hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
		seconds %= 60 * 60
	}
	if minutes := seconds / 60; minutes > 0 {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
		seconds %= 60
	}
	if seconds > 0 {
		parts = append(parts, fmt.Sprintf("%ds", seconds))
	}
	return strings.Join(parts, " ")
}

func formatLabels(labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+labels[key])
	}
	return strings.Join(parts, ", ")
}

func enrichNotification(notification alert.Notification, baseURL string) alert.Notification {
	if notification.ExplorerURL == "" {
		notification.ExplorerURL = buildExplorerURL(baseURL, notification)
	}
	return notification
}

func buildExplorerURL(baseURL string, notification alert.Notification) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return ""
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ""
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/"
	query := parsed.Query()
	if notification.Query != "" {
		query.Set("q", notification.Query)
	}
	from := notification.Timestamp.Add(-5 * time.Minute)
	if notification.StartedAt != nil {
		from = notification.StartedAt.Add(-5 * time.Minute)
	}
	query.Set("from", from.UTC().Format(time.RFC3339Nano))
	query.Set("to", notification.Timestamp.Add(time.Minute).UTC().Format(time.RFC3339Nano))
	query.Set("sort", "desc")
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func formatSeconds(seconds int) string {
	return formatDurationValue(time.Duration(seconds) * time.Second)
}

func truncateDiscordText(value string, limit int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit-1]) + "…"
}

func isDiscordWebhookURL(raw string) bool {
	return detectWebhookProvider(raw) == providerDiscord
}

func detectWebhookProvider(raw string) webhookProvider {
	parsed, err := url.Parse(raw)
	if err != nil || !strings.EqualFold(parsed.Scheme, "https") {
		return providerGeneric
	}
	host := strings.ToLower(parsed.Hostname())
	path := strings.ToLower(parsed.Path)
	switch host {
	case "discord.com", "discordapp.com", "canary.discord.com", "ptb.discord.com":
		if strings.HasPrefix(path, "/api/webhooks/") {
			return providerDiscord
		}
	case "hooks.slack.com", "hooks.slack-gov.com":
		if strings.HasPrefix(path, "/services/") {
			return providerSlack
		}
	case "ntfy.sh":
		if strings.Trim(path, "/") != "" {
			return providerNtfy
		}
	}
	if strings.HasSuffix(host, ".logic.azure.com") &&
		strings.Contains(path, "/workflows/") &&
		strings.Contains(path, "/triggers/") &&
		strings.Contains(path, "/paths/invoke") {
		return providerTeams
	}
	if strings.HasSuffix(host, ".api.powerplatform.com") && strings.Contains(path, "/powerautomate") {
		return providerTeams
	}
	return providerGeneric
}

func discordEndpoint(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	query.Set("wait", "true")
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}
