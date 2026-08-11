package main

import (
	"context"
	"crypto/ed25519"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"caroline/internal/alert"
	"caroline/internal/alert/notifier"
	"caroline/internal/explorer"
	"caroline/internal/httpserver"
	"caroline/internal/ingest"
	"caroline/internal/logstream"
	"caroline/internal/node"
	"caroline/internal/storage/sqlite"
)

const (
	defaultRetention       = 7 * 24 * time.Hour
	defaultMaxStorageBytes = 10 * (1 << 30)
	defaultRetentionMode   = sqlite.RetentionModeIndependent
)

func main() {
	port := os.Getenv("PORT")
	dataDir := strings.TrimSpace(os.Getenv("CAROLINE_DATA_DIR"))
	if dataDir == "" {
		dataDir = "."
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		log.Fatalf("create Caroline data directory: %v", err)
	}
	databasePath := strings.TrimSpace(os.Getenv("CAROLINE_DB"))
	if databasePath == "" {
		databasePath = filepath.Join(dataDir, "caroline.db")
	}
	store, err := sqlite.Open(databasePath)
	if err != nil {
		log.Fatalf("open Caroline database %q: %v", databasePath, err)
	}
	defer store.Close()

	hubKeyPath := strings.TrimSpace(os.Getenv("CAROLINE_HUB_KEY"))
	if hubKeyPath == "" {
		hubKeyPath = filepath.Join(dataDir, "hub.key")
	}
	hubPrivate, err := node.LoadOrCreateKey(hubKeyPath)
	if err != nil {
		log.Fatalf("load Hub identity: %v", err)
	}
	hubPublic, ok := hubPrivate.Public().(ed25519.PublicKey)
	if !ok {
		log.Fatal("Hub private key did not produce an Ed25519 public key")
	}
	nodeService, err := node.NewService(store, node.KeyID(hubPublic), hubPrivate)
	if err != nil {
		log.Fatalf("create node service: %v", err)
	}
	broker := logstream.NewBroker()
	streamManager := logstream.NewBrokerManager(broker)
	defer streamManager.Close()
	explorerService := explorer.NewStoreService(store)
	ingestService, err := ingest.NewService(store, nodeService, broker)
	if err != nil {
		log.Fatalf("create ingest service: %v", err)
	}

	alertStore := strings.TrimSpace(os.Getenv("ALERTS_FILE"))
	if alertStore == "" {
		alertStore = filepath.Join(dataDir, "alerts.json")
	}
	alertEngine, err := alert.NewEngineWithPersistence(streamManager, notifier.Webhook{
		ExplorerBaseURL: strings.TrimSpace(os.Getenv("CAROLINE_URL")),
	}, alertStore)
	if err != nil {
		log.Fatalf("load alert store %q: %v", alertStore, err)
	}
	alertContext, cancelAlerts := context.WithCancel(context.Background())
	defer cancelAlerts()
	go func() {
		if err := alertEngine.Run(alertContext); err != nil {
			log.Printf("alert engine stopped: %v", err)
		}
	}()

	server := httpserver.New(explorerService, nil, streamManager, alertEngine)
	server.ConfigureHub(store, nodeService, ingestService, broker)
	retention := parseDurationEnv("CAROLINE_RETENTION", defaultRetention)
	maxStorageBytes := parseByteSizeEnv("CAROLINE_MAX_STORAGE_SIZE", defaultMaxStorageBytes)
	retentionMode := parseRetentionMode()
	if retention > 0 || maxStorageBytes > 0 {
		retentionContext, cancelRetention := context.WithCancel(context.Background())
		defer cancelRetention()
		go runRetention(retentionContext, store, retention, maxStorageBytes, retentionMode)
	}
	if err := server.Run(port); err != nil {
		log.Fatal(err)
	}
}

type retentionStore interface {
	Cleanup(context.Context, time.Time, int64, sqlite.RetentionMode) (int, error)
}

func runRetention(ctx context.Context, store retentionStore, retention time.Duration, maxStorageBytes int64, mode sqlite.RetentionMode) {
	cleanup := func() {
		before := time.Time{}
		if retention > 0 {
			before = time.Now().UTC().Add(-retention)
		}
		deleted, err := store.Cleanup(ctx, before, maxStorageBytes, mode)
		if err != nil && ctx.Err() == nil {
			log.Printf("log retention cleanup failed: %v", err)
			return
		}
		if deleted > 0 {
			log.Printf("log retention cleanup removed %d entries", deleted)
		}
	}
	cleanup()
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cleanup()
		}
	}
}

func parseRetentionMode() sqlite.RetentionMode {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("CAROLINE_RETENTION_MODE")))
	switch value {
	case "", string(sqlite.RetentionModeIndependent):
		return sqlite.RetentionModeIndependent
	case string(sqlite.RetentionModeSource):
		return sqlite.RetentionModeSource
	case string(sqlite.RetentionModeMin):
		return sqlite.RetentionModeMin
	default:
		log.Printf("ignoring invalid CAROLINE_RETENTION_MODE=%q; using default %q", value, defaultRetentionMode)
		return defaultRetentionMode
	}
}

func parseDurationEnv(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	if strings.EqualFold(value, "0") || strings.EqualFold(value, "off") || strings.EqualFold(value, "disabled") {
		return 0
	}
	parsed, err := parseDuration(value)
	if err != nil || parsed <= 0 {
		log.Printf("ignoring invalid %s=%q; using default %s", key, value, fallback)
		return fallback
	}
	return parsed
}

func parseDuration(value string) (time.Duration, error) {
	lower := strings.ToLower(strings.TrimSpace(value))
	if strings.HasSuffix(lower, "d") {
		days, err := strconv.ParseInt(strings.TrimSpace(strings.TrimSuffix(lower, "d")), 10, 64)
		const maxDurationDays = int64(1<<63-1) / int64(24*time.Hour)
		if err != nil || days <= 0 || days > maxDurationDays {
			return 0, strconv.ErrRange
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(value)
}

func parseByteSizeEnv(key string, fallback int64) int64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	if strings.EqualFold(value, "0") || strings.EqualFold(value, "off") || strings.EqualFold(value, "disabled") {
		return 0
	}
	units := []struct {
		suffix     string
		multiplier int64
	}{
		{"tib", 1 << 40}, {"tb", 1 << 40},
		{"gib", 1 << 30}, {"gb", 1 << 30},
		{"mib", 1 << 20}, {"mb", 1 << 20},
		{"kib", 1 << 10}, {"kb", 1 << 10},
		{"b", 1},
	}
	lower := strings.ToLower(value)
	multiplier := int64(1)
	number := lower
	for _, unit := range units {
		if strings.HasSuffix(lower, unit.suffix) {
			multiplier = unit.multiplier
			number = strings.TrimSpace(strings.TrimSuffix(lower, unit.suffix))
			break
		}
	}
	parsed, err := strconv.ParseInt(number, 10, 64)
	if err != nil || parsed <= 0 || parsed > (1<<62)/multiplier {
		log.Printf("ignoring invalid %s=%q; using default %d bytes", key, value, fallback)
		return fallback
	}
	return parsed * multiplier
}
