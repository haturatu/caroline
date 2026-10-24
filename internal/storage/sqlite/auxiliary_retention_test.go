package sqlite

import (
	"caroline/internal/explorer"
	"context"
	"testing"
	"time"
	"zombiezen.com/go/sqlite"
)

func TestAuxiliaryCleanupKeepsReplayWindowAndRetainedLogs(t *testing.T) {
	store, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Now().UTC()
	old := now.Add(-8 * 24 * time.Hour).UnixNano()
	batch := explorer.EntryBatch{AgentID: "a", BootID: "b", Sequence: 1, Entries: []explorer.Entry{{InsertID: "retained", Timestamp: now, TextPayload: "log"}}}
	if ok, e := store.WriteBatch(context.Background(), batch); e != nil || !ok {
		t.Fatal(ok, e)
	}
	err = store.withConn(context.Background(), func(c *sqlite.Conn) error {
		for _, q := range []string{
			`UPDATE ingest_batches SET received_at_ns = ?1`,
			`INSERT INTO ingest_batches VALUES ('a','b',2,?1)`,
			`INSERT INTO ingest_batches VALUES ('a','b',3,?2)`,
		} {
			args := []any{old}
			if q == `INSERT INTO ingest_batches VALUES ('a','b',3,?2)` {
				args = []any{old, now.UnixNano()}
			}
			if e := exec(c, q, args); e != nil {
				return e
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// No log age/byte limit: auxiliary cleanup must still run.
	if n, e := store.Cleanup(context.Background(), time.Time{}, 0, RetentionModeIndependent); e != nil || n != 0 {
		t.Fatal(n, e)
	}
	if ok, e := store.WriteBatch(context.Background(), batch); e != nil || ok {
		t.Fatal("retained log lost duplicate protection", ok, e)
	}
	err = store.withConn(context.Background(), func(c *sqlite.Conn) error {
		var count int64
		if e := execRows(c, `SELECT count(*) FROM ingest_batches`, nil, func(st *sqlite.Stmt) error { count = st.ColumnInt64(0); return nil }); e != nil {
			return e
		}
		if count != 2 {
			t.Errorf("batches=%d want old-with-log and recent", count)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, e := store.Cleanup(context.Background(), now.Add(time.Second), 0, RetentionModeIndependent); e != nil {
		t.Fatal(e)
	}
	err = store.withConn(context.Background(), func(c *sqlite.Conn) error {
		return execRows(c, `SELECT count(*) FROM ingest_batches`, nil, func(st *sqlite.Stmt) error {
			if st.ColumnInt64(0) != 1 {
				t.Error("orphan batch survived log deletion")
			}
			return nil
		})
	})
	if err != nil {
		t.Fatal(err)
	}
}
func TestAuxiliaryCleanupDeletesOnlyExpiredOrUsedEnrollmentTokens(t *testing.T) {
	store, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Now().UTC()
	err = store.withConn(context.Background(), func(c *sqlite.Conn) error {
		return exec(c, `INSERT INTO enrollment_tokens VALUES ('old','old',0,?1,NULL),('used','used',0,?2,1),('live','live',0,?2,NULL)`, []any{now.Add(-time.Hour).UnixNano(), now.Add(time.Hour).UnixNano()})
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, e := store.Cleanup(context.Background(), time.Time{}, 0, RetentionModeSource); e != nil {
		t.Fatal(e)
	}
	if ok, e := store.ConsumeEnrollmentToken(context.Background(), "live", now); e != nil || !ok {
		t.Fatal(ok, e)
	}
	if ok, e := store.ConsumeEnrollmentToken(context.Background(), "old", now); e != nil || ok {
		t.Fatal(ok, e)
	}
	if ok, e := store.ConsumeEnrollmentToken(context.Background(), "used", now); e != nil || ok {
		t.Fatal(ok, e)
	}
}
