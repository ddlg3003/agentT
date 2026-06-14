// Package digeststore provides a SQLite-backed digest.Repository. Digests are
// stored as a JSON document keyed by date, which keeps the schema trivial and
// the domain struct the single source of truth — swapping to another store later
// means writing a new adapter, nothing more. Uses the pure-Go modernc.org/sqlite
// driver, so no CGO is required.
package digeststore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	_ "modernc.org/sqlite" // registers the "sqlite" database/sql driver

	"github.com/vngcloud/agentt/internal/domain/digest"
)

// SQLite persists digests in a single table keyed by date.
type SQLite struct {
	db *sql.DB
}

var _ digest.Repository = (*SQLite)(nil)

// Open opens (and migrates) a SQLite digest store at path. Use ":memory:" for
// tests. The caller owns Close.
func Open(path string) (*SQLite, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}
	// One writer at a time keeps the file-backed store simple and safe.
	db.SetMaxOpenConns(1)
	s := &SQLite{db: db}
	if err := s.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// Close releases the underlying database handle.
func (s *SQLite) Close() error { return s.db.Close() }

func (s *SQLite) migrate(ctx context.Context) error {
	const ddl = `CREATE TABLE IF NOT EXISTS digests (
		date         TEXT PRIMARY KEY,
		generated_at TEXT NOT NULL,
		data         TEXT NOT NULL
	)`
	if _, err := s.db.ExecContext(ctx, ddl); err != nil {
		return fmt.Errorf("migrate digests table: %w", err)
	}
	return nil
}

// Save upserts a digest by date.
func (s *SQLite) Save(ctx context.Context, d digest.DailyDigest) error {
	data, err := json.Marshal(d)
	if err != nil {
		return fmt.Errorf("marshal digest: %w", err)
	}
	const q = `INSERT INTO digests (date, generated_at, data) VALUES (?, ?, ?)
		ON CONFLICT(date) DO UPDATE SET generated_at = excluded.generated_at, data = excluded.data`
	if _, err := s.db.ExecContext(ctx, q, d.Date, d.GeneratedAt.Format("2006-01-02T15:04:05Z07:00"), string(data)); err != nil {
		return fmt.Errorf("save digest %s: %w", d.Date, err)
	}
	return nil
}

// Get loads the digest for a date, or digest.ErrNotFound if none exists.
func (s *SQLite) Get(ctx context.Context, date string) (digest.DailyDigest, error) {
	const q = `SELECT data FROM digests WHERE date = ?`
	var data string
	switch err := s.db.QueryRowContext(ctx, q, date).Scan(&data); {
	case errors.Is(err, sql.ErrNoRows):
		return digest.DailyDigest{}, digest.ErrNotFound
	case err != nil:
		return digest.DailyDigest{}, fmt.Errorf("get digest %s: %w", date, err)
	}
	var d digest.DailyDigest
	if err := json.Unmarshal([]byte(data), &d); err != nil {
		return digest.DailyDigest{}, fmt.Errorf("unmarshal digest %s: %w", date, err)
	}
	return d, nil
}

// ListDates returns all stored dates, newest first.
func (s *SQLite) ListDates(ctx context.Context) ([]string, error) {
	const q = `SELECT date FROM digests ORDER BY date DESC`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list dates: %w", err)
	}
	defer rows.Close()

	var dates []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, fmt.Errorf("scan date: %w", err)
		}
		dates = append(dates, d)
	}
	return dates, rows.Err()
}

// ListMonth returns all digests whose date falls in ym ("2026-03"), oldest first.
func (s *SQLite) ListMonth(ctx context.Context, ym string) ([]digest.DailyDigest, error) {
	const q = `SELECT data FROM digests WHERE date LIKE ? ORDER BY date ASC`
	rows, err := s.db.QueryContext(ctx, q, ym+"-%")
	if err != nil {
		return nil, fmt.Errorf("list month %s: %w", ym, err)
	}
	defer rows.Close()

	var out []digest.DailyDigest
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, fmt.Errorf("scan digest: %w", err)
		}
		var d digest.DailyDigest
		if err := json.Unmarshal([]byte(data), &d); err != nil {
			return nil, fmt.Errorf("unmarshal digest: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
