package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"caroline/internal/alert"
)

type Webhook struct {
	Client *http.Client
}

func (w Webhook) Notify(ctx context.Context, rule alert.Rule, notification alert.Notification) error {
	if rule.WebhookURL == "" {
		return nil
	}
	client := w.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	endpoint := rule.WebhookURL
	var body []byte
	var err error
	if isDiscordWebhookURL(endpoint) {
		endpoint, err = discordEndpoint(endpoint)
		if err != nil {
			return err
		}
		body, err = json.Marshal(discordPayload(notification))
	} else {
		body, err = json.Marshal(notification)
	}
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
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
	Description string              `json:"description"`
	Color       int                 `json:"color"`
	Timestamp   string              `json:"timestamp"`
	Fields      []discordEmbedField `json:"fields,omitempty"`
}

type discordEmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

func discordPayload(notification alert.Notification) discordWebhookPayload {
	event := "Alert firing"
	color := 0xED4245
	if notification.Event == "alert.resolved" {
		event = "Alert resolved"
		color = 0x57F287
	}

	fields := []discordEmbedField{
		{Name: "Matches", Value: fmt.Sprintf("%d / %d", notification.Value, notification.Threshold), Inline: true},
		{Name: "Window", Value: formatSeconds(notification.WindowSeconds), Inline: true},
		{Name: "Cooldown", Value: formatSeconds(notification.CooldownSeconds), Inline: true},
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

	return discordWebhookPayload{
		Username: "Caroline",
		Embeds: []discordEmbed{{
			Title:       truncateDiscordText(fmt.Sprintf("%s: %s", event, notification.Rule), 256),
			Description: "A Caroline log alert changed state.",
			Color:       color,
			Timestamp:   notification.Timestamp.UTC().Format(time.RFC3339Nano),
			Fields:      fields,
		}},
		AllowedMentions: discordAllowedMentions{Parse: []string{}},
	}
}

func formatSeconds(seconds int) string {
	return (time.Duration(seconds) * time.Second).String()
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
	parsed, err := url.Parse(raw)
	if err != nil || !strings.EqualFold(parsed.Scheme, "https") {
		return false
	}
	switch strings.ToLower(parsed.Hostname()) {
	case "discord.com", "discordapp.com", "canary.discord.com", "ptb.discord.com":
		return strings.HasPrefix(parsed.Path, "/api/webhooks/")
	default:
		return false
	}
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
