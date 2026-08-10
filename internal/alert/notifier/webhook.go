package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	body, err := json.Marshal(notification)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, rule.WebhookURL, bytes.NewReader(body))
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
