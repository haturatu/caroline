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
	retention := parseDurationEnv("CAROLINE_RETENTION")
	maxStorageBytes := parseByteSizeEnv("CAROLINE_MAX_STORAGE_SIZE")
	if retention > 0 || maxStorageBytes > 0 {
		retentionContext, cancelRetention := context.WithCancel(context.Background())
		defer cancelRetention()
		go runRetention(retentionContext, store, retention, maxStorageBytes)
	}
	if err := server.Run(port); err != nil {
		log.Fatal(err)
	}
}

type retentionStore interface {
	Cleanup(context.Context, time.Time, int64) (int, error)
}

func runRetention(ctx context.Context, store retentionStore, retention time.Duration, maxStorageBytes int64) {
	cleanup := func() {
		before := time.Time{}
		if retention > 0 {
			before = time.Now().UTC().Add(-retention)
		}
		deleted, err := store.Cleanup(ctx, before, maxStorageBytes)
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

func parseDurationEnv(key string) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return 0
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		log.Printf("ignoring invalid %s=%q; expected a positive duration", key, value)
		return 0
	}
	return parsed
}

func parseByteSizeEnv(key string) int64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return 0
	}
	units := map[string]int64{"b": 1, "kb": 1 << 10, "mb": 1 << 20, "gb": 1 << 30, "tb": 1 << 40}
	lower := strings.ToLower(value)
	multiplier := int64(1)
	number := lower
	for suffix, factor := range units {
		if strings.HasSuffix(lower, suffix) {
			multiplier = factor
			number = strings.TrimSpace(strings.TrimSuffix(lower, suffix))
			break
		}
	}
	parsed, err := strconv.ParseInt(number, 10, 64)
	if err != nil || parsed <= 0 || parsed > (1<<62)/multiplier {
		log.Printf("ignoring invalid %s=%q; expected a positive byte size", key, value)
		return 0
	}
	return parsed * multiplier
}
