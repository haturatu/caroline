package sqlite

import (
	"context"
	"reflect"
	"testing"
	"time"

	"zombiezen.com/go/sqlite"
)

func TestCleanupBudgetRetainsNewestEntries(t *testing.T) {
	type row struct {
		id        string
		timestamp int64
		text      string
		json      any
		labels    any
		resource  any
		summary   string
	}
	for _, test := range []struct {
		name    string
		rows    []row
		budget  int64
		before  time.Time
		deleted int
		want    []string
	}{
		{name: "empty", budget: 1, want: []string{}},
		{name: "disabled", rows: []row{{id: "a", text: "large"}}, want: []string{"a"}},
		{name: "exact budget", rows: []row{{id: "a", text: "1234"}, {id: "b", text: "5678"}}, budget: 8, want: []string{"a", "b"}},
		{name: "all removed", rows: []row{{id: "a", text: "12"}, {id: "b", text: "345"}}, budget: 1, deleted: 2, want: []string{}},
		{
			name: "timestamp then ID determines boundary",
			rows: []row{
				{id: "b", timestamp: 1, text: "1234"},
				{id: "c", timestamp: 2, text: "12345"},
				{id: "a", timestamp: 1, text: "123"},
			},
			budget: 9, deleted: 1, want: []string{"b", "c"},
		},
		{
			name:   "zero length rows in deleted prefix",
			rows:   []row{{id: "zero", timestamp: 1}, {id: "small", timestamp: 2, text: "12"}, {id: "large", timestamp: 3, text: "123456789"}},
			budget: 9, deleted: 2, want: []string{"large"},
		},
		{
			name:   "zero length row remains when already within budget",
			rows:   []row{{id: "zero", timestamp: 1}, {id: "large", timestamp: 2, text: "123456789"}},
			budget: 9, want: []string{"zero", "large"},
		},
		{
			name:   "SQLite Unicode text lengths",
			rows:   []row{{id: "a", text: "日本語"}, {id: "b", text: "🌸"}},
			budget: 1, deleted: 1, want: []string{"b"},
		},
		{
			name:   "SQLite embedded NUL text lengths",
			rows:   []row{{id: "a", text: "a\x00trailing"}, {id: "b", text: "b"}},
			budget: 1, deleted: 1, want: []string{"b"},
		},
		{
			name: "all five columns count and nullable columns contribute zero",
			rows: []row{
				{id: "a", text: "1", json: "{}", labels: "{}", resource: "{}", summary: "2"},
				{id: "b", text: "12345678"},
			},
			budget: 8, deleted: 1, want: []string{"b"},
		},
		{
			name:   "age cleanup precedes budget calculation",
			rows:   []row{{id: "old", timestamp: 1, text: "123456789"}, {id: "a", timestamp: 2, text: "1234"}, {id: "b", timestamp: 3, text: "5678"}},
			before: time.Unix(0, 2), budget: 8, deleted: 1, want: []string{"a", "b"},
		},
		{
			name:   "age and budget delete counts combine",
			rows:   []row{{id: "old", timestamp: 1, text: "123456789"}, {id: "a", timestamp: 2, text: "1234"}, {id: "b", timestamp: 3, text: "5678"}},
			before: time.Unix(0, 2), budget: 4, deleted: 2, want: []string{"b"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			store, err := OpenMemory()
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			for _, row := range test.rows {
				if err := exec(store.conn, `INSERT INTO logs (
insert_id, timestamp_ns, received_at_ns, text_payload, json_payload, labels_json, resource_labels_json, summary
) VALUES (?1, ?2, 0, ?3, ?4, ?5, ?6, ?7)`, []any{row.id, row.timestamp, row.text, row.json, row.labels, row.resource, row.summary}); err != nil {
					t.Fatal(err)
				}
			}
			deleted, err := store.Cleanup(context.Background(), test.before, test.budget, RetentionModeIndependent)
			if err != nil || deleted != test.deleted {
				t.Fatalf("Cleanup deleted=%d, err=%v; want %d", deleted, err, test.deleted)
			}
			got := make([]string, 0)
			if err := execRows(store.conn, `SELECT insert_id FROM logs ORDER BY timestamp_ns, insert_id`, nil, func(stmt *sqlite.Stmt) error {
				got = append(got, stmt.ColumnText(0))
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("retained IDs = %v, want %v", got, test.want)
			}
		})
	}
}

func TestCleanupBudgetFailureRollsBackAgeAndSizeDeletion(t *testing.T) {
	store, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := exec(store.conn, `INSERT INTO logs (insert_id, timestamp_ns, received_at_ns, text_payload)
VALUES ('old', 1, 0, '1234'), ('a', 2, 0, '1234'), ('b', 3, 0, '1234')`, nil); err != nil {
		t.Fatal(err)
	}
	// The age deletion succeeds, then the size deletion fails. The entire
	// cleanup must roll back, including any earlier rows in the range deletion.
	if err := exec(store.conn, `CREATE TRIGGER reject_budget_delete BEFORE DELETE ON logs
WHEN OLD.insert_id = 'b' BEGIN SELECT RAISE(ABORT, 'injected delete failure'); END`, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Cleanup(context.Background(), time.Unix(0, 2), 1, RetentionModeIndependent); err == nil {
		t.Fatal("Cleanup succeeded despite failing deletion")
	}
	var count int
	if err := execRows(store.conn, `SELECT COUNT(*) FROM logs`, nil, func(stmt *sqlite.Stmt) error {
		count = stmt.ColumnInt(0)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("retained %d rows after failed cleanup, want all 3", count)
	}
	// Reusing the connection also checks that the early-stopped SELECT and
	// failed transaction did not leave a busy statement or open transaction.
	if err := exec(store.conn, `DROP TRIGGER reject_budget_delete`, nil); err != nil {
		t.Fatal(err)
	}
	if deleted, err := store.Cleanup(context.Background(), time.Unix(0, 2), 1, RetentionModeIndependent); err != nil || deleted != 3 {
		t.Fatalf("retry deleted=%d err=%v, want 3", deleted, err)
	}
}
