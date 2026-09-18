// Package sqlite is the sole implementation of store.Store, backed by
// modernc.org/sqlite (pure Go, no CGO — DEC-002).
package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"sort"
	"time"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/store"
	_ "modernc.org/sqlite"
)

// Store is the SQLite-backed store.Store implementation.
type Store struct {
	db *sql.DB
}

var _ store.Store = (*Store)(nil)

// Open opens (creating if necessary) a SQLite database at dsn. Tests use
// "file::memory:?cache=shared" so the DB lives only for the test process and
// leaves no files behind (S1 c6).
func Open(dsn string) (*Store, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlite: open %q: %w", dsn, err)
	}
	// SQLite's single-writer model (DEC-002): one connection avoids
	// "database is locked" errors from concurrent writers within a process.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlite: enable foreign keys: %w", err)
	}
	return &Store{db: db}, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// Migrate applies every migration in internal/store/migrations, in filename
// order. Each migration is idempotent (CREATE TABLE IF NOT EXISTS), so
// running Migrate twice on the same DB succeeds (S1 c3).
func (s *Store) Migrate(ctx context.Context) error {
	entries, err := store.MigrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("sqlite: read migrations dir: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		contents, err := store.MigrationsFS.ReadFile(path.Join("migrations", name))
		if err != nil {
			return fmt.Errorf("sqlite: read migration %s: %w", name, err)
		}
		if _, err := s.db.ExecContext(ctx, string(contents)); err != nil {
			return fmt.Errorf("sqlite: apply migration %s: %w", name, err)
		}
	}
	return nil
}

// timeToString and stringToTime round-trip timestamps as RFC3339Nano UTC
// text, SQLite having no native timestamp type.
func timeToString(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func stringToTime(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("sqlite: parse timestamp %q: %w", s, err)
	}
	return t.UTC(), nil
}

func nullableTimeToString(t *time.Time) sql.NullString {
	if t == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: timeToString(*t), Valid: true}
}

func stringToNullableTime(ns sql.NullString) (*time.Time, error) {
	if !ns.Valid {
		return nil, nil
	}
	t, err := stringToTime(ns.String)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func nullableString(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

func stringOrNil(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	v := ns.String
	return &v
}

func nullableInt(i *int) sql.NullInt64 {
	if i == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*i), Valid: true}
}

func intOrNil(ni sql.NullInt64) *int {
	if !ni.Valid {
		return nil
	}
	v := int(ni.Int64)
	return &v
}

func nullableInt64(i *int64) sql.NullInt64 {
	if i == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *i, Valid: true}
}

func int64OrNil(ni sql.NullInt64) *int64 {
	if !ni.Valid {
		return nil
	}
	v := ni.Int64
	return &v
}

func boolToInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func isNoRows(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

func marshalStrings(v []string) (string, error) {
	if v == nil {
		v = []string{}
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("marshal string list: %w", err)
	}
	return string(b), nil
}

func unmarshalStrings(s string) ([]string, error) {
	var v []string
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return nil, fmt.Errorf("unmarshal string list %q: %w", s, err)
	}
	return v, nil
}
