package main

import (
	"testing"
	"time"

	"caroline/internal/storage/sqlite"
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

func TestRetentionModes(t *testing.T) {
	t.Setenv("CAROLINE_RETENTION_MODE", "")
	if got := parseRetentionMode(); got != sqlite.RetentionModeIndependent {
		t.Fatalf("default retention mode = %q, want independent", got)
	}
	for _, test := range []struct {
		value string
		want  sqlite.RetentionMode
	}{
		{value: "source", want: sqlite.RetentionModeSource},
		{value: "MIN", want: sqlite.RetentionModeMin},
		{value: "unknown", want: sqlite.RetentionModeIndependent},
	} {
		t.Setenv("CAROLINE_RETENTION_MODE", test.value)
		if got := parseRetentionMode(); got != test.want {
			t.Fatalf("CAROLINE_RETENTION_MODE=%q -> %q, want %q", test.value, got, test.want)
		}
	}
}

func TestRetentionSupportsBinaryUnitsAndExplicitDisable(t *testing.T) {
	t.Setenv("CAROLINE_RETENTION", "7d")
	if got := parseDurationEnv("CAROLINE_RETENTION", defaultRetention); got != 7*24*time.Hour {
		t.Fatalf("7d retention = %s, want 168h", got)
	}
	t.Setenv("CAROLINE_RETENTION", "off")
	if got := parseDurationEnv("CAROLINE_RETENTION", defaultRetention); got != 0 {
		t.Fatalf("disabled retention = %s, want 0", got)
	}
	t.Setenv("CAROLINE_MAX_STORAGE_SIZE", "10GiB")
	if got := parseByteSizeEnv("CAROLINE_MAX_STORAGE_SIZE", defaultMaxStorageBytes); got != 10*(1<<30) {
		t.Fatalf("10GiB = %d, want 10 GiB", got)
	}
}
