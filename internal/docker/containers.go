package docker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
)

func (c *Client) ListRunning(ctx context.Context) ([]Container, error) {
	filters, _ := json.Marshal(map[string][]string{"status": {"running"}})
	endpoint := "/containers/json?all=0&filters=" + url.QueryEscape(string(filters))
	var containers []Container
	err := c.do(ctx, http.MethodGet, endpoint, &containers)
	return containers, err
}
