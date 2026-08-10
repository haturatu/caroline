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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("wait") != "" {
			t.Fatalf("generic webhook unexpectedly received wait query: %s", r.URL.RawQuery)
		}
		var payload alert.Notification
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode generic payload: %v", err)
		}
		if payload.Event != "alert.firing" || payload.Rule != "API errors" {
			t.Fatalf("unexpected generic payload: %#v", payload)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	notification := testNotification()
	err := (Webhook{}).Notify(context.Background(), alert.Rule{WebhookURL: server.URL}, notification)
	if err != nil {
		t.Fatalf("Notify returned error: %v", err)
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
	if payload.Embeds[0].Title != "Alert firing: API errors" {
		t.Fatalf("unexpected Discord embed title: %#v", payload.Embeds[0])
	}
	if len(payload.AllowedMentions.Parse) != 0 {
		t.Fatalf("Discord mentions were not disabled: %#v", payload.AllowedMentions)
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

func testNotification() alert.Notification {
	return alert.Notification{
		Event:           "alert.firing",
		Rule:            "API errors",
		Value:           2,
		Threshold:       1,
		WindowSeconds:   60,
		CooldownSeconds: 300,
		Timestamp:       time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC),
		Sample: &explorer.Entry{
			Summary: "request failed",
			Stream:  "stderr",
		},
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
