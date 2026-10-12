// Package db persists REST history, saved queries and table preferences
// in a local SQLite database.
package db

import (
	"context"
	"errors"
	"time"

	"github.com/egoist/mygo/plugins/sqlite"
)

// HistoryEntry is one executed REST request.
type HistoryEntry struct {
	ID          int64  `json:"id"`
	ClusterUUID string `json:"cluster_uuid"`
	Method      string `json:"method"`
	Path        string `json:"path"`
	Body        string `json:"body"`
	Status      int    `json:"status"`
	DurationMS  int64  `json:"duration_ms"`
	CreatedAt   int64  `json:"created_at"` // unix milliseconds
}

// SavedQuery is a named request kept by the user.
type SavedQuery struct {
	ID          int64  `json:"id"`
	ClusterUUID string `json:"cluster_uuid"`
	Name        string `json:"name"`
	Method      string `json:"method"`
	Path        string `json:"path"`
	Body        string `json:"body"`
	CreatedAt   int64  `json:"created_at"` // unix milliseconds
	UpdatedAt   int64  `json:"updated_at"` // unix milliseconds
}

// TablePrefs stores per-cluster column visibility for a module's table.
type TablePrefs struct {
	ClusterUUID string `json:"cluster_uuid"`
	Module      string `json:"module"`
	ColumnsJSON string `json:"columns_json"`
}

// Store owns the SQLite connection.
type Store struct {
	db *sqlite.DB
}

// Open creates or opens the database at path. The parent directory must
// already exist.
func Open(ctx context.Context, path string) (*Store, error) {
	conn, err := sqlite.Open(ctx, path, sqlite.OpenOptions{})
	if err != nil {
		return nil, err
	}
	for _, stmt := range []string{
		"CREATE TABLE IF NOT EXISTS rest_history (id INTEGER PRIMARY KEY AUTOINCREMENT, cluster_uuid TEXT NOT NULL, method TEXT, path TEXT, body TEXT, status INTEGER, duration_ms INTEGER, created_at INTEGER)",
		"CREATE TABLE IF NOT EXISTS saved_queries (id INTEGER PRIMARY KEY AUTOINCREMENT, cluster_uuid TEXT NOT NULL, name TEXT, method TEXT, path TEXT, body TEXT, created_at INTEGER, updated_at INTEGER)",
		"CREATE TABLE IF NOT EXISTS table_prefs (cluster_uuid TEXT, module TEXT, columns_json TEXT, PRIMARY KEY (cluster_uuid, module))",
	} {
		if _, err := conn.Execute(ctx, stmt); err != nil {
			conn.Close()
			return nil, err
		}
	}
	return &Store{db: conn}, nil
}

// Close closes the connection.
func (s *Store) Close() error { return s.db.Close() }

// AddHistory records one executed request.
func (s *Store) AddHistory(ctx context.Context, e HistoryEntry) (int64, error) {
	res, err := s.db.Execute(ctx,
		"INSERT INTO rest_history (cluster_uuid, method, path, body, status, duration_ms, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		e.ClusterUUID, e.Method, e.Path, e.Body, e.Status, e.DurationMS, e.CreatedAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertID, nil
}

// ListHistory returns the newest entries for one cluster.
func (s *Store) ListHistory(ctx context.Context, clusterUUID string, limit, offset int) ([]HistoryEntry, error) {
	rows, err := s.db.Query(ctx,
		"SELECT id, cluster_uuid, method, path, body, status, duration_ms, created_at FROM rest_history WHERE cluster_uuid = ? ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?",
		clusterUUID, limit, offset)
	if err != nil {
		return nil, err
	}
	return scanHistory(rows.Columns, rows.Values), nil
}

func scanHistory(cols []string, vals [][]any) []HistoryEntry {
	idx := map[string]int{}
	for i, c := range cols {
		idx[c] = i
	}
	out := make([]HistoryEntry, 0, len(vals))
	for _, row := range vals {
		e := HistoryEntry{
			ID:          asInt64(row[idx["id"]]),
			ClusterUUID: asString(row[idx["cluster_uuid"]]),
			Method:      asString(row[idx["method"]]),
			Path:        asString(row[idx["path"]]),
			Body:        asString(row[idx["body"]]),
			Status:      int(asInt64(row[idx["status"]])),
			DurationMS:  asInt64(row[idx["duration_ms"]]),
			CreatedAt:   asInt64(row[idx["created_at"]]),
		}
		out = append(out, e)
	}
	return out
}

// ClearHistory deletes every history entry of one cluster.
func (s *Store) ClearHistory(ctx context.Context, clusterUUID string) error {
	_, err := s.db.Execute(ctx, "DELETE FROM rest_history WHERE cluster_uuid = ?", clusterUUID)
	return err
}

// SaveQuery inserts q, or updates the existing query with the same name
// within the same cluster.
func (s *Store) SaveQuery(ctx context.Context, q SavedQuery) (int64, error) {
	if q.ClusterUUID == "" || q.Name == "" {
		return 0, errors.New("db: saved query requires cluster uuid and name")
	}
	now := time.Now().UnixMilli()
	if q.CreatedAt == 0 {
		q.CreatedAt = now
	}
	q.UpdatedAt = now
	rows, err := s.db.Query(ctx, "SELECT id FROM saved_queries WHERE cluster_uuid = ? AND name = ?", q.ClusterUUID, q.Name)
	if err != nil {
		return 0, err
	}
	var existing int64
	if len(rows.Values) > 0 {
		existing = asInt64(rows.Values[0][0])
	}
	if existing > 0 {
		_, err = s.db.Execute(ctx,
			"UPDATE saved_queries SET method = ?, path = ?, body = ?, updated_at = ? WHERE id = ?",
			q.Method, q.Path, q.Body, q.UpdatedAt, existing)
		if err != nil {
			return 0, err
		}
		return existing, nil
	}
	res, err := s.db.Execute(ctx,
		"INSERT INTO saved_queries (cluster_uuid, name, method, path, body, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		q.ClusterUUID, q.Name, q.Method, q.Path, q.Body, q.CreatedAt, q.UpdatedAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertID, nil
}

// ListQueries returns the cluster's saved queries ordered by name.
func (s *Store) ListQueries(ctx context.Context, clusterUUID string) ([]SavedQuery, error) {
	rows, err := s.db.Query(ctx,
		"SELECT id, cluster_uuid, name, method, path, body, created_at, updated_at FROM saved_queries WHERE cluster_uuid = ? ORDER BY name",
		clusterUUID)
	if err != nil {
		return nil, err
	}
	idx := map[string]int{}
	for i, c := range rows.Columns {
		idx[c] = i
	}
	out := make([]SavedQuery, 0, len(rows.Values))
	for _, row := range rows.Values {
		q := SavedQuery{
			ID:          asInt64(row[idx["id"]]),
			ClusterUUID: asString(row[idx["cluster_uuid"]]),
			Name:        asString(row[idx["name"]]),
			Method:      asString(row[idx["method"]]),
			Path:        asString(row[idx["path"]]),
			Body:        asString(row[idx["body"]]),
			CreatedAt:   asInt64(row[idx["created_at"]]),
			UpdatedAt:   asInt64(row[idx["updated_at"]]),
		}
		out = append(out, q)
	}
	return out, nil
}

// DeleteQuery removes one saved query by id, scoped to the cluster.
func (s *Store) DeleteQuery(ctx context.Context, clusterUUID string, id int64) error {
	_, err := s.db.Execute(ctx, "DELETE FROM saved_queries WHERE cluster_uuid = ? AND id = ?", clusterUUID, id)
	return err
}

// SetTablePrefs upserts column preferences for one cluster and module.
func (s *Store) SetTablePrefs(ctx context.Context, p TablePrefs) error {
	_, err := s.db.Execute(ctx,
		"INSERT INTO table_prefs (cluster_uuid, module, columns_json) VALUES (?, ?, ?) "+
			"ON CONFLICT (cluster_uuid, module) DO UPDATE SET columns_json = excluded.columns_json",
		p.ClusterUUID, p.Module, p.ColumnsJSON)
	return err
}

// GetTablePrefs returns stored preferences; ok is false when absent.
func (s *Store) GetTablePrefs(ctx context.Context, clusterUUID, module string) (TablePrefs, bool, error) {
	rows, err := s.db.Query(ctx,
		"SELECT cluster_uuid, module, columns_json FROM table_prefs WHERE cluster_uuid = ? AND module = ?",
		clusterUUID, module)
	if err != nil {
		return TablePrefs{}, false, err
	}
	if len(rows.Values) == 0 {
		return TablePrefs{}, false, nil
	}
	row := rows.Values[0]
	return TablePrefs{
		ClusterUUID: asString(row[0]),
		Module:      asString(row[1]),
		ColumnsJSON: asString(row[2]),
	}, true, nil
}

func asInt64(v any) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case int:
		return int64(x)
	case float64:
		return int64(x)
	case []byte:
		var n int64
		for _, c := range x {
			if c < '0' || c > '9' {
				return 0
			}
			n = n*10 + int64(c-'0')
		}
		return n
	default:
		return 0
	}
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	if b, ok := v.([]byte); ok {
		return string(b)
	}
	return ""
}
