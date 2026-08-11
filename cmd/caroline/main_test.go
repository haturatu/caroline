package main

import (
	"testing"
	"time"
)

func TestRetentionDefaults(t *testing.T) {
	t.Setenv("CAROLINE_RETENTION", "")
	t.Setenv("CAROLINE_MAX_STORAGE_SIZE", "")
	if got := parseDurationEnv("CAROLINE_RETENTION", defaultRetention); got != 7*24*time.Hour {
		t.Fatalf("default retention = %s, want 168h", got)
	}
	if got := parseByteSizeEnv("CAROLINE_MAX_STORAGE_SIZE", defaultMaxStorageBytes); got != 10*(1<<30) {
		t.Fatalf("default storage size = %d, want 10 GiB", got)
	}
}

func TestRetentionSupportsBinaryUnitsAndExplicitDisable(t *testing.T) {
	t.Setenv("CAROLINE_RETENTION", "off")
	if got := parseDurationEnv("CAROLINE_RETENTION", defaultRetention); got != 0 {
		t.Fatalf("disabled retention = %s, want 0", got)
	}
	t.Setenv("CAROLINE_MAX_STORAGE_SIZE", "10GiB")
	if got := parseByteSizeEnv("CAROLINE_MAX_STORAGE_SIZE", defaultMaxStorageBytes); got != 10*(1<<30) {
		t.Fatalf("10GiB = %d, want 10 GiB", got)
	}
}
