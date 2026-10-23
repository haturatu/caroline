package sqlite

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"caroline/internal/explorer"
)

// Restore the same data outside the timer so every iteration measures a real
// cleanup, rather than an empty database left by the preceding iteration.
func BenchmarkCleanupBudget(b *testing.B) {
	for _, rows := range []int{100, 1000, 5000} {
		for _, percent := range []int{50, 100} {
			b.Run(fmt.Sprintf("rows=%d/retain=%d%%", rows, percent), func(b *testing.B) {
				store, err := OpenMemory()
				if err != nil {
					b.Fatal(err)
				}
				defer store.Close()
				base := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
				entries := make([]explorer.Entry, rows)
				for i := range entries {
					entries[i] = explorer.Entry{
						InsertID: fmt.Sprintf("%08d", i), Timestamp: base.Add(time.Duration(i) * time.Second),
						TextPayload: strings.Repeat("x", 256), Summary: "benchmark",
					}
				}
				// nil maps are persisted as the four-character JSON text "null".
				const rowBytes = 256 + len("benchmark") + 3*len("null")
				budget := int64(rows * rowBytes * percent / 100)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					b.StopTimer()
					if _, err := store.WriteBatch(context.Background(), explorer.EntryBatch{Entries: entries}); err != nil {
						b.Fatal(err)
					}
					b.StartTimer()
					deleted, err := store.Cleanup(context.Background(), time.Time{}, budget, RetentionModeIndependent)
					if err != nil {
						b.Fatal(err)
					}
					if want := rows * (100 - percent) / 100; deleted != want {
						b.Fatalf("deleted %d entries, want %d", deleted, want)
					}
				}
			})
		}
	}
}
