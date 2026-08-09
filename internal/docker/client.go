package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

const requestTimeout = 35 * time.Second

type Client struct {
	client       *http.Client
	followClient *http.Client
	baseURL      string
}

type Container struct {
	ID      string            `json:"Id"`
	Names   []string          `json:"Names"`
	Image   string            `json:"Image"`
	State   string            `json:"State"`
	Status  string            `json:"Status"`
	Created int64             `json:"Created"`
	Labels  map[string]string `json:"Labels"`
}

type Version struct {
	Version    string `json:"Version"`
	APIVersion string `json:"ApiVersion"`
}

type Frame struct {
	Stream string
	Data   []byte
}

func NewClient(host string) *Client {
	if host == "" {
		host = "unix:///var/run/docker.sock"
	}

	transport := &http.Transport{
		Proxy:                 nil,
		MaxIdleConns:          10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	baseURL := "http://docker"

	switch {
	case strings.HasPrefix(host, "unix://"):
		socketPath := strings.TrimPrefix(host, "unix://")
		transport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
			dialer := &net.Dialer{Timeout: 5 * time.Second}
			return dialer.DialContext(ctx, "unix", socketPath)
		}
	case strings.HasPrefix(host, "tcp://"):
		baseURL = "http://" + strings.TrimPrefix(host, "tcp://")
	case strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://"):
		baseURL = strings.TrimRight(host, "/")
	default:
		log.Printf("unsupported DOCKER_HOST %q, using default Docker socket", host)
		socketPath := "/var/run/docker.sock"
		transport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
			dialer := &net.Dialer{Timeout: 5 * time.Second}
			return dialer.DialContext(ctx, "unix", socketPath)
		}
	}

	return &Client{
		client:       &http.Client{Transport: transport, Timeout: requestTimeout},
		followClient: &http.Client{Transport: transport},
		baseURL:      baseURL,
	}
}

func (c *Client) do(ctx context.Context, method, endpoint string, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
		return fmt.Errorf("docker API returned %s: %s", resp.Status, strings.TrimSpace(string(message)))
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *Client) Check(ctx context.Context) (Version, error) {
	var version Version
	err := c.do(ctx, http.MethodGet, "/version", &version)
	return version, err
}

func HostDescription(host string) string {
	if host != "" {
		return host
	}
	return "unix:///var/run/docker.sock"
}
