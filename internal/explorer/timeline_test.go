package explorer

import (
	"testing"
	"time"
)

func TestNormalizeTimelineBuckets(t *testing.T) {
	tests := []struct {
		name  string
		value int
		want  int
	}{
		{name: "default", value: 0, want: DefaultTimelineBuckets},
		{name: "minimum", value: 1, want: MinTimelineBuckets},
		{name: "maximum", value: 1000, want: MaxTimelineBuckets},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := NormalizeTimelineBuckets(test.value); got != test.want {
				t.Fatalf("NormalizeTimelineBuckets(%d) = %d, want %d", test.value, got, test.want)
			}
		})
	}
}

func TestBuildTimelineUsesRequestedBucketCount(t *testing.T) {
	from := time.Date(2026, 8, 9, 3, 0, 0, 0, time.UTC)
	to := from.Add(4 * time.Minute)
	entries := []Entry{
		{Timestamp: from.Add(30 * time.Second), Severity: "INFO"},
		{Timestamp: from.Add(2*time.Minute + 30*time.Second), Severity: "ERROR"},
	}

	buckets := BuildTimeline(entries, from, to, 48)
	if len(buckets) != 48 {
		t.Fatalf("BuildTimeline returned %d buckets, want 48", len(buckets))
	}
	if buckets[6].Total != 1 || buckets[30].Total != 1 {
		t.Fatalf("entries were assigned to unexpected buckets: first=%d second=%d", buckets[6].Total, buckets[30].Total)
	}
}
