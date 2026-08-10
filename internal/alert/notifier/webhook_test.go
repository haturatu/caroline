package notifier

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"caroline/internal/alert"
	"caroline/internal/explorer"
)

func TestWebhookNotifyKeepsGenericPayload(t *testing.T) {
	var received alert.Notification
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("wait") != "" {
			t.Fatalf("generic webhook unexpectedly received wait query: %s", r.URL.RawQuery)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("decode generic payload: %v", err)
		}
		if received.Event != "alert.firing" || received.Rule != "API errors" {
			t.Fatalf("unexpected generic payload: %#v", received)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	notification := testNotification()
	err := (Webhook{ExplorerBaseURL: "https://caroline.example.test"}).Notify(context.Background(), alert.Rule{WebhookURL: server.URL}, notification)
	if err != nil {
		t.Fatalf("Notify returned error: %v", err)
	}
	if !strings.Contains(received.ExplorerURL, "q=severity%3E%3DERROR") || !strings.Contains(received.ExplorerURL, "from=") {
		t.Fatalf("generic payload did not include Explorer URL: %q", received.ExplorerURL)
	}
}

func TestWebhookNotifyUsesDiscordPayload(t *testing.T) {
	var request *http.Request
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		request = r
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader(`{}`)),
			Header:     make(http.Header),
			Request:    r,
		}, nil
	})}

	err := (Webhook{Client: client}).Notify(context.Background(), alert.Rule{
		WebhookURL: "https://discord.com/api/webhooks/123/token",
	}, testNotification())
	if err != nil {
		t.Fatalf("Notify returned error: %v", err)
	}
	if request == nil {
		t.Fatal("Discord request was not sent")
	}
	if request.URL.Query().Get("wait") != "true" {
		t.Fatalf("Discord request did not request confirmation: %s", request.URL.RawQuery)
	}

	var payload discordWebhookPayload
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		t.Fatalf("decode Discord payload: %v", err)
	}
	if payload.Username != "Caroline" || len(payload.Embeds) != 1 {
		t.Fatalf("unexpected Discord payload: %#v", payload)
	}
	if payload.Embeds[0].Title != "🔴 FIRING · API errors" {
		t.Fatalf("unexpected Discord embed title: %#v", payload.Embeds[0])
	}
	if len(payload.Embeds[0].Fields) < 3 || payload.Embeds[0].Fields[0].Name != "Condition" {
		t.Fatalf("unexpected Discord embed field order: %#v", payload.Embeds[0].Fields)
	}
	if len(payload.AllowedMentions.Parse) != 0 {
		t.Fatalf("Discord mentions were not disabled: %#v", payload.AllowedMentions)
	}
}

func TestWebhookNotifyUsesSlackPayload(t *testing.T) {
	var request *http.Request
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		request = r
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader(`{}`)),
			Header:     make(http.Header),
			Request:    r,
		}, nil
	})}

	err := (Webhook{Client: client}).Notify(context.Background(), alert.Rule{
		WebhookURL: "https://hooks.slack.com/services/T000/B000/secret",
	}, testNotification())
	if err != nil {
		t.Fatalf("Notify returned error: %v", err)
	}
	if request == nil {
		t.Fatal("Slack request was not sent")
	}
	if got := request.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("Slack request Content-Type = %q", got)
	}
	var payload slackPayload
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		t.Fatalf("decode Slack payload: %v", err)
	}
	if !strings.HasPrefix(payload.Text, "🔴 FIRING · API errors") {
		t.Fatalf("unexpected Slack fallback text: %q", payload.Text)
	}
	if len(payload.Blocks) < 2 || payload.Blocks[0].Type != "header" || payload.Blocks[1].Type != "section" {
		t.Fatalf("unexpected Slack blocks: %#v", payload.Blocks)
	}
	if payload.Blocks[0].Text == nil || payload.Blocks[0].Text.Text != "🔴 FIRING · API errors" {
		t.Fatalf("unexpected Slack header: %#v", payload.Blocks[0])
	}
}

func TestIsDiscordWebhookURL(t *testing.T) {
	for _, test := range []struct {
		name string
		url  string
		want bool
	}{
		{name: "discord", url: "https://discord.com/api/webhooks/123/token", want: true},
		{name: "legacy discord host", url: "https://discordapp.com/api/webhooks/123/token", want: true},
		{name: "generic", url: "https://example.test/api/webhooks/123/token", want: false},
		{name: "insecure", url: "http://discord.com/api/webhooks/123/token", want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := isDiscordWebhookURL(test.url); got != test.want {
				t.Fatalf("isDiscordWebhookURL(%q) = %t, want %t", test.url, got, test.want)
			}
		})
	}
}

func TestDetectWebhookProvider(t *testing.T) {
	for _, test := range []struct {
		name     string
		url      string
		provider webhookProvider
	}{
		{name: "discord", url: "https://discord.com/api/webhooks/123/token", provider: providerDiscord},
		{name: "slack", url: "https://hooks.slack.com/services/T000/B000/token", provider: providerSlack},
		{name: "generic", url: "https://alerts.example.test/caroline", provider: providerGeneric},
		{name: "insecure discord", url: "http://discord.com/api/webhooks/123/token", provider: providerGeneric},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := detectWebhookProvider(test.url); got != test.provider {
				t.Fatalf("detectWebhookProvider(%q) = %q, want %q", test.url, got, test.provider)
			}
		})
	}
}

func testNotification() alert.Notification {
	return alert.Notification{
		Event:           "alert.firing",
		RuleID:          "rule-85af",
		Rule:            "API errors",
		Severity:        "critical",
		Query:           "severity>=ERROR",
		Value:           2,
		PeakValue:       2,
		Threshold:       1,
		WindowSeconds:   60,
		CooldownSeconds: 300,
		Timestamp:       time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC),
		Labels:          map[string]string{"service": "api"},
		Sample: &explorer.Entry{
			Summary:  "request failed",
			Resource: explorer.Resource{Labels: map[string]string{"container_name": "api-1"}},
			Stream:   "stderr",
		},
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
