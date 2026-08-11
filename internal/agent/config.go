package agent

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HubURL             string
	EnrollmentToken    string
	HubPublicKey       []byte
	StateDir           string
	DockerHost         string
	AgentVersion       string
	FlushInterval      time.Duration
	MaxBatchEntries    int
	MaxBatchBytes      int
	QueueCapacity      int
	SpoolMaxBytes      int64
	SpoolMaxAge        time.Duration
	HeartbeatInterval  time.Duration
	DiscoveryInterval  time.Duration
	TrustHubOnFirstUse bool
	Compression        string
}

func ConfigFromEnv() (Config, error) {
	stateDir := strings.TrimSpace(os.Getenv("CAROLINE_AGENT_STATE_DIR"))
	if stateDir == "" {
		stateDir = "/var/lib/caroline-agent"
	}
	config := Config{
		HubURL:          strings.TrimRight(strings.TrimSpace(os.Getenv("CAROLINE_HUB_URL")), "/"),
		EnrollmentToken: strings.TrimSpace(os.Getenv("CAROLINE_ENROLLMENT_TOKEN")),
		StateDir:        stateDir, DockerHost: strings.TrimSpace(os.Getenv("DOCKER_HOST")),
		AgentVersion:       firstEnv("CAROLINE_AGENT_VERSION", "dev"),
		FlushInterval:      durationEnv("CAROLINE_AGENT_FLUSH_INTERVAL", 500*time.Millisecond),
		MaxBatchEntries:    intEnv("CAROLINE_AGENT_MAX_BATCH_ENTRIES", 500),
		MaxBatchBytes:      intEnv("CAROLINE_AGENT_MAX_BATCH_BYTES", 1<<20),
		QueueCapacity:      intEnv("CAROLINE_AGENT_QUEUE_CAPACITY", 2048),
		SpoolMaxBytes:      int64Env("CAROLINE_AGENT_SPOOL_MAX_SIZE", 1<<30),
		SpoolMaxAge:        durationEnv("CAROLINE_AGENT_SPOOL_MAX_AGE", 24*time.Hour),
		HeartbeatInterval:  durationEnv("CAROLINE_AGENT_HEARTBEAT_INTERVAL", 15*time.Second),
		DiscoveryInterval:  durationEnv("CAROLINE_AGENT_DISCOVERY_INTERVAL", 15*time.Second),
		TrustHubOnFirstUse: strings.EqualFold(os.Getenv("CAROLINE_AGENT_TRUST_ON_FIRST_USE"), "true"),
		Compression:        strings.ToLower(firstEnv("CAROLINE_AGENT_COMPRESSION", "gzip")),
	}
	if encoded := strings.TrimSpace(os.Getenv("CAROLINE_HUB_PUBLIC_KEY")); encoded != "" {
		key, err := base64.RawStdEncoding.DecodeString(encoded)
		if err != nil {
			return Config{}, fmt.Errorf("CAROLINE_HUB_PUBLIC_KEY: %w", err)
		}
		config.HubPublicKey = key
	}
	if config.HubURL == "" {
		return Config{}, fmt.Errorf("CAROLINE_HUB_URL is required")
	}
	if config.MaxBatchEntries < 1 || config.MaxBatchEntries > 500 {
		return Config{}, fmt.Errorf("CAROLINE_AGENT_MAX_BATCH_ENTRIES must be between 1 and 500")
	}
	if config.MaxBatchBytes < 1024 || config.MaxBatchBytes > 1<<20 {
		return Config{}, fmt.Errorf("CAROLINE_AGENT_MAX_BATCH_BYTES must be between 1024 and 1048576")
	}
	if config.Compression != "identity" && config.Compression != "gzip" && config.Compression != "zstd" {
		return Config{}, fmt.Errorf("CAROLINE_AGENT_COMPRESSION must be identity, gzip, or zstd")
	}
	return config, nil
}

func (c Config) IdentityPath() string { return filepath.Join(c.StateDir, "identity.json") }
func (c Config) SpoolDir() string     { return filepath.Join(c.StateDir, "spool") }

func firstEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil && parsed > 0 {
			return parsed
		}
	}
	return fallback
}

func intEnv(key string, fallback int) int {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			return parsed
		}
	}
	return fallback
}

func int64Env(key string, fallback int64) int64 {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		if parsed, err := strconv.ParseInt(value, 10, 64); err == nil && parsed > 0 {
			return parsed
		}
	}
	return fallback
}
