package docker

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func (c *Client) Logs(ctx context.Context, containerID string, tail int, since time.Time) ([]Frame, error) {
	params := url.Values{}
	params.Set("stdout", "1")
	params.Set("stderr", "1")
	params.Set("timestamps", "1")
	params.Set("follow", "0")
	params.Set("tail", strconv.Itoa(tail))
	if !since.IsZero() {
		params.Set("since", strconv.FormatInt(since.Unix(), 10))
	}

	endpoint := "/containers/" + url.PathEscape(containerID) + "/logs?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
		return nil, fmt.Errorf("docker API returned %s: %s", resp.Status, strings.TrimSpace(string(message)))
	}
	return readDockerFrames(resp.Body)
}

func (c *Client) FollowLogs(ctx context.Context, containerID string, since time.Time, onFrame func(Frame) error) error {
	params := url.Values{}
	params.Set("stdout", "1")
	params.Set("stderr", "1")
	params.Set("timestamps", "1")
	params.Set("follow", "1")
	params.Set("tail", "0")
	if !since.IsZero() {
		params.Set("since", strconv.FormatInt(since.Unix(), 10))
	}

	endpoint := "/containers/" + url.PathEscape(containerID) + "/logs?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.docker.raw-stream")
	client := c.followClient
	if client == nil {
		client = c.client
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
		return fmt.Errorf("docker API returned %s: %s", resp.Status, strings.TrimSpace(string(message)))
	}
	return streamDockerFrames(resp.Body, onFrame)
}
