// Package links keeps the site's short links on disk, in SQLite.
package links

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"

	_ "modernc.org/sqlite"
)

// Store keeps short link IDs and the result URLs they stand for. It
// implements web.LinkStore.
type Store struct{ db *sql.DB }

// Open opens or creates the database at path.
func Open(path string) (*Store, error) {
	q := url.Values{"_pragma": {"journal_mode(WAL)", "busy_timeout(5000)", "synchronous(NORMAL)"}}
	db, err := sql.Open("sqlite", "file:"+path+"?"+q.Encode())
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS links (
		id      TEXT PRIMARY KEY,
		target  TEXT NOT NULL,
		created INTEGER NOT NULL DEFAULT (unixepoch())
	) WITHOUT ROWID`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	return &Store{db: db}, nil
}

// Add stores target under id unless id is taken, and returns the target
// id stands for: target, or the one stored first.
func (s *Store) Add(ctx context.Context, id, target string) (string, error) {
	if _, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO links (id, target) VALUES (?, ?)`, id, target); err != nil {
		return "", err
	}
	got, err := s.Get(ctx, id)
	if err == nil && got == "" {
		err = errors.New("short link vanished after insert")
	}
	return got, err
}

// Get returns the target of id, "" when there is none.
func (s *Store) Get(ctx context.Context, id string) (string, error) {
	var target string
	err := s.db.QueryRowContext(ctx, `SELECT target FROM links WHERE id = ?`, id).Scan(&target)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return target, err
}

func (s *Store) Close() error { return s.db.Close() }
