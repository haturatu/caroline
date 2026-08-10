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

func TestParseDurationClampsDayRanges(t *testing.T) {
	if got := ParseDuration("31d"); got != 30*24*time.Hour {
		t.Fatalf("ParseDuration(31d) = %s, want 720h", got)
	}
	if got := ParseDuration("-1d"); got != 5*time.Minute {
		t.Fatalf("ParseDuration(-1d) = %s, want 5m", got)
	}
	if got := ParseDuration("365d"); got != 30*24*time.Hour {
		t.Fatalf("ParseDuration(365d) = %s, want 720h", got)
	}
	if got := ParseDuration("999999999999999999999999999d"); got != 30*24*time.Hour {
		t.Fatalf("ParseDuration(huge day range) = %s, want 720h", got)
	}
}

func TestBuildTimelineHandlesSubNanosecondBucketSpans(t *testing.T) {
	from := time.Unix(0, 0).UTC()
	to := from.Add(23 * time.Nanosecond)
	buckets := BuildTimeline([]Entry{{Timestamp: from.Add(time.Nanosecond), Severity: "UNKNOWN"}}, from, to, 24)
	if len(buckets) != 1 {
		t.Fatalf("BuildTimeline returned %d buckets, want one for a sub-24ns range", len(buckets))
	}
	if buckets[0].Total != 1 || buckets[0].Severities["INFO"] != 1 {
		t.Fatalf("unexpected short-range bucket: %#v", buckets[0])
	}
}

func TestBuildFieldGroupsKeepsCountsForTopValues(t *testing.T) {
	entries := make([]Entry, 0, 20)
	for index := 0; index < 9; index++ {
		entries = append(entries, Entry{JSONPayload: map[string]any{"status": string(rune('A' + index))}})
	}
	for index := 0; index < 10; index++ {
		entries = append(entries, Entry{JSONPayload: map[string]any{"status": "A"}})
	}
	groups := BuildFieldGroups(entries)
	if len(groups) != 1 || len(groups[0].Fields) != 1 {
		t.Fatalf("unexpected field groups: %#v", groups)
	}
	field := groups[0].Fields[0]
	if field.Count != len(entries) || field.Values["A"] != 11 {
		t.Fatalf("top value count = %d/%d, want 11/%d", field.Values["A"], field.Count, len(entries))
	}
	if len(field.Values) != 8 {
		t.Fatalf("top values contains %d values, want 8: %#v", len(field.Values), field.Values)
	}
}
