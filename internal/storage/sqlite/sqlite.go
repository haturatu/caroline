package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"caroline/internal/explorer"
	"caroline/internal/node"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

const searchReadLimit = explorer.MaxEntries * 4

var errNotFound = errors.New("sqlite row was not found")

type Store struct {
	conn *sqlite.Conn
	mu   sync.Mutex
}

var _ explorer.LogStore = (*Store)(nil)
var _ node.Store = (*Store)(nil)

func Open(path string) (*Store, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		path = "caroline.db"
	}
	conn, err := sqlite.OpenConn(path)
	if err != nil {
		return nil, err
	}
	store := &Store{conn: conn}
	if err := store.withConn(context.Background(), func(conn *sqlite.Conn) error {
		return sqlitex.ExecScript(conn, schema)
	}); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("migrate sqlite store: %w", err)
	}
	return store, nil
}

func OpenMemory() (*Store, error) {
	return Open(":memory:")
}

func (s *Store) Close() error {
	if s == nil || s.conn == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.conn.Close()
	s.conn = nil
	return err
}

func (s *Store) withConn(ctx context.Context, fn func(*sqlite.Conn) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn == nil {
		return errors.New("sqlite store is closed")
	}
	previousInterrupt := s.conn.SetInterrupt(ctx.Done())
	defer s.conn.SetInterrupt(previousInterrupt)
	return fn(s.conn)
}

func exec(conn *sqlite.Conn, query string, args []any) error {
	return sqlitex.Execute(conn, query, &sqlitex.ExecOptions{Args: args})
}

func execRows(conn *sqlite.Conn, query string, args []any, fn func(*sqlite.Stmt) error) error {
	return sqlitex.Execute(conn, query, &sqlitex.ExecOptions{Args: args, ResultFunc: fn})
}

func (s *Store) WriteBatch(ctx context.Context, batch explorer.EntryBatch) (accepted bool, err error) {
	if len(batch.Entries) > explorer.MaxEntries {
		return false, fmt.Errorf("ingest batch contains too many entries")
	}
	if len(batch.Entries) == 0 && len(batch.Containers) == 0 {
		return true, nil
	}
	err = s.withConn(ctx, func(conn *sqlite.Conn) (err error) {
		end, err := sqlitex.ImmediateTransaction(conn)
		if err != nil {
			return err
		}
		defer end(&err)
		now := time.Now().UTC().UnixNano()
		accepted = true
		if batch.AgentID != "" {
			err = exec(conn, `
INSERT OR IGNORE INTO ingest_batches(agent_id, boot_id, sequence, received_at_ns)
VALUES (?1, ?2, ?3, ?4)`, []any{batch.AgentID, batch.BootID, int64(batch.Sequence), now})
			if err != nil {
				return err
			}
			accepted = conn.Changes() == 1
			if !accepted {
				return nil
			}
		}
		for index, entry := range batch.Entries {
			if strings.TrimSpace(entry.InsertID) == "" {
				return fmt.Errorf("entry %d has no insertId", index)
			}
			resourceLabelsJSON, marshalErr := json.Marshal(entry.Resource.Labels)
			if marshalErr != nil {
				return marshalErr
			}
			labelsJSON, marshalErr := json.Marshal(entry.Labels)
			if marshalErr != nil {
				return marshalErr
			}
			jsonPayloadJSON, marshalErr := json.Marshal(entry.JSONPayload)
			if marshalErr != nil {
				return marshalErr
			}
			labels := entry.Resource.Labels
			err = exec(conn, `
INSERT OR IGNORE INTO logs(
    insert_id, node_id, node_name, container_id, container_name, image,
    timestamp_ns, received_at_ns, severity, log_name, stream, text_payload,
    json_payload, labels_json, resource_labels_json, summary, agent_id,
    boot_id, sequence, line_index
) VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?11, ?12, ?13, ?14, ?15, ?16, ?17, ?18, ?19, ?20)`, []any{
				entry.InsertID, labels["node_id"], labels["node_name"], labels["container_id"],
				labels["container_name"], labels["image"], entry.Timestamp.UnixNano(), now,
				entry.Severity, entry.LogName, entry.Stream, entry.TextPayload,
				string(jsonPayloadJSON), string(labelsJSON), string(resourceLabelsJSON),
				entry.Summary, batch.AgentID, batch.BootID, int64(batch.Sequence), index,
			})
			if err != nil {
				return err
			}
		}
		for _, container := range batch.Containers {
			labelsJSON, marshalErr := json.Marshal(container.Labels)
			if marshalErr != nil {
				return marshalErr
			}
			err = exec(conn, `
INSERT INTO containers(
    node_id, node_name, container_id, container_name, image, state, status,
    created_ns, labels_json, log_count, error_count, warning_count, last_seen_ns
) VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?11, ?12, ?13)
ON CONFLICT(node_id, container_id) DO UPDATE SET
    node_name=excluded.node_name, container_name=excluded.container_name,
    image=excluded.image, state=excluded.state, status=excluded.status,
    created_ns=excluded.created_ns, labels_json=excluded.labels_json,
    last_seen_ns=excluded.last_seen_ns`, []any{
				container.NodeID, container.NodeName, container.ID, container.Name,
				container.Image, container.State, container.Status, container.Created.UnixNano(),
				string(labelsJSON), container.LogCount, container.ErrorCount,
				container.WarningCount, now,
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	return accepted, err
}

func (s *Store) SearchEntries(ctx context.Context, request explorer.SearchRequest) ([]explorer.Entry, error) {
	from := request.From
	to := request.To
	if from.IsZero() {
		from = time.Unix(0, 0).UTC()
	}
	if to.IsZero() {
		to = time.Now().UTC()
	}
	order := "DESC"
	if strings.EqualFold(request.Sort, "asc") {
		order = "ASC"
	}
	entries := make([]explorer.Entry, 0)
	err := s.withConn(ctx, func(conn *sqlite.Conn) error {
		return execRows(conn, `
SELECT insert_id, timestamp_ns, severity, log_name, stream, text_payload,
       json_payload, labels_json, resource_labels_json, summary
FROM logs
WHERE timestamp_ns >= ?1 AND timestamp_ns <= ?2
ORDER BY timestamp_ns `+order+`, insert_id `+order+`
LIMIT ?3`, []any{from.UnixNano(), to.UnixNano(), searchReadLimit}, func(stmt *sqlite.Stmt) error {
			entry := explorer.Entry{
				InsertID: stmt.ColumnText(0), Timestamp: time.Unix(0, stmt.ColumnInt64(1)).UTC(),
				Severity: stmt.ColumnText(2), LogName: stmt.ColumnText(3), Stream: stmt.ColumnText(4),
				TextPayload: stmt.ColumnText(5), Resource: explorer.Resource{Type: "docker_container", Labels: map[string]string{}},
				Summary: stmt.ColumnText(9),
			}
			if err := decodeJSON(stmt.ColumnText(6), &entry.JSONPayload); err != nil {
				return err
			}
			if err := decodeJSON(stmt.ColumnText(7), &entry.Labels); err != nil {
				return err
			}
			if err := decodeJSON(stmt.ColumnText(8), &entry.Resource.Labels); err != nil {
				return err
			}
			entries = append(entries, entry)
			return nil
		})
	})
	return entries, err
}

func (s *Store) ListContainers(ctx context.Context) ([]explorer.ContainerInfo, error) {
	containers := make([]explorer.ContainerInfo, 0)
	err := s.withConn(ctx, func(conn *sqlite.Conn) error {
		return execRows(conn, `
SELECT node_id, node_name, container_id, container_name, image, state, status,
       created_ns, labels_json, log_count, error_count, warning_count
FROM containers ORDER BY node_name, container_name, container_id`, nil, func(stmt *sqlite.Stmt) error {
			info := explorer.ContainerInfo{
				NodeID: stmt.ColumnText(0), NodeName: stmt.ColumnText(1), ID: stmt.ColumnText(2),
				Name: stmt.ColumnText(3), Image: stmt.ColumnText(4), State: stmt.ColumnText(5),
				Status: stmt.ColumnText(6), Created: time.Unix(0, stmt.ColumnInt64(7)).UTC(),
				LogCount: stmt.ColumnInt(9), ErrorCount: stmt.ColumnInt(10), WarningCount: stmt.ColumnInt(11),
			}
			if err := decodeJSON(stmt.ColumnText(8), &info.Labels); err != nil {
				return err
			}
			containers = append(containers, info)
			return nil
		})
	})
	return containers, err
}

func (s *Store) SaveNode(ctx context.Context, value node.Node) error {
	return s.withConn(ctx, func(conn *sqlite.Conn) error {
		return exec(conn, `
INSERT INTO nodes(id, name, fingerprint, public_key, hostname, os, architecture,
                  agent_version, protocol_version, connected_at_ns, last_seen_at_ns, status)
VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?11, ?12)
ON CONFLICT(id) DO UPDATE SET name=excluded.name, fingerprint=excluded.fingerprint,
    public_key=excluded.public_key, hostname=excluded.hostname, os=excluded.os,
    architecture=excluded.architecture, agent_version=excluded.agent_version,
    protocol_version=excluded.protocol_version, connected_at_ns=excluded.connected_at_ns,
    last_seen_at_ns=excluded.last_seen_at_ns, status=excluded.status`, []any{
			value.ID, value.Name, value.Fingerprint, value.PublicKey, value.Hostname, value.OS,
			value.Architecture, value.AgentVersion, value.ProtocolVersion, value.ConnectedAt.UnixNano(),
			value.LastSeenAt.UnixNano(), string(value.Status),
		})
	})
}

func (s *Store) GetNode(ctx context.Context, id string) (node.Node, error) {
	var value node.Node
	err := s.withConn(ctx, func(conn *sqlite.Conn) error {
		found := false
		err := execRows(conn, `SELECT id, name, fingerprint, public_key, hostname, os, architecture, agent_version, protocol_version, connected_at_ns, last_seen_at_ns, status FROM nodes WHERE id = ?1`, []any{id}, func(stmt *sqlite.Stmt) error {
			found = true
			value = scanNode(stmt)
			return nil
		})
		if err != nil {
			return err
		}
		if !found {
			return errNotFound
		}
		return nil
	})
	return value, err
}

func (s *Store) ListNodes(ctx context.Context) ([]node.Node, error) {
	values := make([]node.Node, 0)
	err := s.withConn(ctx, func(conn *sqlite.Conn) error {
		return execRows(conn, `SELECT id, name, fingerprint, public_key, hostname, os, architecture, agent_version, protocol_version, connected_at_ns, last_seen_at_ns, status FROM nodes ORDER BY name, id`, nil, func(stmt *sqlite.Stmt) error {
			values = append(values, scanNode(stmt))
			return nil
		})
	})
	return values, err
}

func (s *Store) RevokeNode(ctx context.Context, id string, now time.Time) error {
	return s.withConn(ctx, func(conn *sqlite.Conn) error {
		if err := exec(conn, `UPDATE nodes SET status = ?1, last_seen_at_ns = ?2 WHERE id = ?3`, []any{string(node.StatusRevoked), now.UnixNano(), id}); err != nil {
			return err
		}
		if conn.Changes() == 0 {
			return errNotFound
		}
		return nil
	})
}

func (s *Store) TouchNode(ctx context.Context, id string, now time.Time) error {
	return s.withConn(ctx, func(conn *sqlite.Conn) error {
		if err := exec(conn, `UPDATE nodes SET last_seen_at_ns = ?1, status = CASE WHEN status = ?2 THEN status ELSE ?3 END WHERE id = ?4`, []any{now.UnixNano(), string(node.StatusRevoked), string(node.StatusOnline), id}); err != nil {
			return err
		}
		if conn.Changes() == 0 {
			return errNotFound
		}
		return nil
	})
}

func (s *Store) PutEnrollmentToken(ctx context.Context, id, tokenHash string, createdAt, expiresAt time.Time) error {
	return s.withConn(ctx, func(conn *sqlite.Conn) error {
		return exec(conn, `INSERT INTO enrollment_tokens(id, token_hash, created_at_ns, expires_at_ns) VALUES (?1, ?2, ?3, ?4)`, []any{id, tokenHash, createdAt.UnixNano(), expiresAt.UnixNano()})
	})
}

func (s *Store) ConsumeEnrollmentToken(ctx context.Context, tokenHash string, now time.Time) (bool, error) {
	var accepted bool
	err := s.withConn(ctx, func(conn *sqlite.Conn) error {
		if err := exec(conn, `UPDATE enrollment_tokens SET used_at_ns = ?1 WHERE token_hash = ?2 AND used_at_ns IS NULL AND expires_at_ns > ?3`, []any{now.UnixNano(), tokenHash, now.UnixNano()}); err != nil {
			return err
		}
		accepted = conn.Changes() == 1
		return nil
	})
	return accepted, err
}

func (s *Store) RememberNonce(ctx context.Context, agentID, value string, expiresAt time.Time) (bool, error) {
	var accepted bool
	err := s.withConn(ctx, func(conn *sqlite.Conn) error {
		if err := exec(conn, `DELETE FROM request_nonces WHERE expires_at_ns <= ?1`, []any{time.Now().UTC().UnixNano()}); err != nil {
			return err
		}
		if err := exec(conn, `INSERT OR IGNORE INTO request_nonces(agent_id, nonce, expires_at_ns) VALUES (?1, ?2, ?3)`, []any{agentID, value, expiresAt.UnixNano()}); err != nil {
			return err
		}
		accepted = conn.Changes() == 1
		return nil
	})
	return accepted, err
}

func decodeJSON(value string, target any) error {
	if strings.TrimSpace(value) == "" || strings.TrimSpace(value) == "null" {
		return nil
	}
	return json.Unmarshal([]byte(value), target)
}

func scanNode(stmt *sqlite.Stmt) node.Node {
	publicKey := make([]byte, stmt.ColumnLen(3))
	stmt.ColumnBytes(3, publicKey)
	return node.Node{
		ID: stmt.ColumnText(0), Name: stmt.ColumnText(1), Fingerprint: stmt.ColumnText(2),
		PublicKey: publicKey, Hostname: stmt.ColumnText(4),
		OS: stmt.ColumnText(5), Architecture: stmt.ColumnText(6), AgentVersion: stmt.ColumnText(7),
		ProtocolVersion: stmt.ColumnInt(8), ConnectedAt: time.Unix(0, stmt.ColumnInt64(9)).UTC(),
		LastSeenAt: time.Unix(0, stmt.ColumnInt64(10)).UTC(), Status: node.Status(stmt.ColumnText(11)),
	}
}

const schema = `
PRAGMA foreign_keys = ON;
PRAGMA busy_timeout = 5000;
CREATE TABLE IF NOT EXISTS schema_meta (key TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS logs (
    insert_id TEXT PRIMARY KEY, node_id TEXT NOT NULL DEFAULT '', node_name TEXT NOT NULL DEFAULT '',
    container_id TEXT NOT NULL DEFAULT '', container_name TEXT NOT NULL DEFAULT '', image TEXT NOT NULL DEFAULT '',
    timestamp_ns INTEGER NOT NULL, received_at_ns INTEGER NOT NULL, severity TEXT NOT NULL DEFAULT 'INFO',
    log_name TEXT NOT NULL DEFAULT '', stream TEXT NOT NULL DEFAULT '', text_payload TEXT NOT NULL DEFAULT '',
    json_payload TEXT, labels_json TEXT, resource_labels_json TEXT, summary TEXT NOT NULL DEFAULT '',
    agent_id TEXT NOT NULL DEFAULT '', boot_id TEXT NOT NULL DEFAULT '', sequence INTEGER NOT NULL DEFAULT 0,
    line_index INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS logs_timestamp_idx ON logs(timestamp_ns DESC, insert_id DESC);
CREATE INDEX IF NOT EXISTS logs_node_idx ON logs(node_id, timestamp_ns DESC);
CREATE INDEX IF NOT EXISTS logs_container_idx ON logs(container_id, timestamp_ns DESC);
CREATE INDEX IF NOT EXISTS logs_severity_idx ON logs(severity, timestamp_ns DESC);
CREATE TABLE IF NOT EXISTS ingest_batches (
    agent_id TEXT NOT NULL, boot_id TEXT NOT NULL, sequence INTEGER NOT NULL, received_at_ns INTEGER NOT NULL,
    PRIMARY KEY (agent_id, boot_id, sequence)
);
CREATE TABLE IF NOT EXISTS containers (
    node_id TEXT NOT NULL, node_name TEXT NOT NULL DEFAULT '', container_id TEXT NOT NULL,
    container_name TEXT NOT NULL DEFAULT '', image TEXT NOT NULL DEFAULT '', state TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT '', created_ns INTEGER NOT NULL DEFAULT 0, labels_json TEXT,
    log_count INTEGER NOT NULL DEFAULT 0, error_count INTEGER NOT NULL DEFAULT 0,
    warning_count INTEGER NOT NULL DEFAULT 0, last_seen_ns INTEGER NOT NULL,
    PRIMARY KEY (node_id, container_id)
);
CREATE INDEX IF NOT EXISTS containers_node_idx ON containers(node_id, container_name);
CREATE TABLE IF NOT EXISTS nodes (
    id TEXT PRIMARY KEY, name TEXT NOT NULL, fingerprint TEXT NOT NULL, public_key BLOB NOT NULL,
    hostname TEXT NOT NULL DEFAULT '', os TEXT NOT NULL DEFAULT '', architecture TEXT NOT NULL DEFAULT '',
    agent_version TEXT NOT NULL DEFAULT '', protocol_version INTEGER NOT NULL DEFAULT 1,
    connected_at_ns INTEGER NOT NULL DEFAULT 0, last_seen_at_ns INTEGER NOT NULL DEFAULT 0, status TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS enrollment_tokens (
    id TEXT PRIMARY KEY, token_hash TEXT NOT NULL UNIQUE, created_at_ns INTEGER NOT NULL,
    expires_at_ns INTEGER NOT NULL, used_at_ns INTEGER
);
CREATE TABLE IF NOT EXISTS request_nonces (
    agent_id TEXT NOT NULL, nonce TEXT NOT NULL, expires_at_ns INTEGER NOT NULL,
    PRIMARY KEY (agent_id, nonce)
);
`
