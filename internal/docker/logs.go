package docker

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var errOldestLogFound = errors.New("oldest Docker log timestamp found")

func (c *Client) Logs(ctx context.Context, containerID string, tail int, since time.Time) ([]Frame, error) {
	params := url.Values{}
	params.Set("stdout", "1")
	params.Set("stderr", "1")
	params.Set("timestamps", "1")
	params.Set("follow", "0")
	if tail < 0 {
		params.Set("tail", "all")
	} else {
		params.Set("tail", strconv.Itoa(tail))
	}
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
		return nil, responseError(resp)
	}
	return readDockerFrames(resp.Body)
}

// OldestLogTime returns the first timestamp exposed by Docker's logs API.
// Docker returns logs in chronological order for tail=all, so the first valid
// timestamp is the oldest log still available from the source. The response is
// stopped as soon as that timestamp is found; it is never fully buffered.
func (c *Client) OldestLogTime(ctx context.Context, containerID string) (time.Time, error) {
	params := url.Values{}
	params.Set("stdout", "1")
	params.Set("stderr", "1")
	params.Set("timestamps", "1")
	params.Set("follow", "0")
	params.Set("tail", "all")
	endpoint := "/containers/" + url.PathEscape(containerID) + "/logs?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+endpoint, nil)
	if err != nil {
		return time.Time{}, err
	}
	req.Header.Set("Accept", "application/vnd.docker.raw-stream")
	resp, err := c.client.Do(req)
	if err != nil {
		return time.Time{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return time.Time{}, responseError(resp)
	}

	var oldest time.Time
	err = streamDockerFrames(resp.Body, func(frame Frame) error {
		for _, rawLine := range strings.Split(strings.ReplaceAll(string(frame.Data), "\r\n", "\n"), "\n") {
			fields := strings.Fields(rawLine)
			if len(fields) == 0 {
				continue
			}
			parsed, parseErr := time.Parse(time.RFC3339Nano, fields[0])
			if parseErr != nil {
				continue
			}
			oldest = parsed.UTC()
			return errOldestLogFound
		}
		return nil
	})
	if errors.Is(err, errOldestLogFound) {
		return oldest, nil
	}
	if err != nil {
		return time.Time{}, err
	}
	return time.Time{}, nil
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
		return responseError(resp)
	}
	return streamDockerFrames(resp.Body, onFrame)
}
