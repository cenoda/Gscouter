// Package links keeps the site's short links on disk, in SQLite.
package links

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"time"

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
	) WITHOUT ROWID;
	CREATE TABLE IF NOT EXISTS sends (
		target  TEXT PRIMARY KEY,
		created INTEGER NOT NULL DEFAULT (unixepoch())
	) WITHOUT ROWID;
	CREATE TABLE IF NOT EXISTS messages (
		id      INTEGER PRIMARY KEY,
		text    TEXT NOT NULL,
		page    TEXT NOT NULL DEFAULT '',
		created INTEGER NOT NULL DEFAULT (unixepoch())
	)`)
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

// AddSend keeps a result link a visitor sent to the developer: a
// custom rotation that scores higher than the library's
// (docs/cycle-swap.md §7). Sending the same link again keeps it once.
func (s *Store) AddSend(ctx context.Context, target string) error {
	_, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO sends (target) VALUES (?)`, target)
	return err
}

// Send is a result link a visitor sent, and when.
type Send struct {
	Target  string
	Created time.Time
}

// Sends are the links sent, newest first.
func (s *Store) Sends(ctx context.Context) ([]Send, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT target, created FROM sends ORDER BY created DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Send
	for rows.Next() {
		var t string
		var c int64
		if err := rows.Scan(&t, &c); err != nil {
			return nil, err
		}
		out = append(out, Send{t, time.Unix(c, 0).UTC()})
	}
	return out, rows.Err()
}

// AddMessage keeps a message a visitor wrote to the developer, and the
// site page it was written on ("" for none).
func (s *Store) AddMessage(ctx context.Context, text, page string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO messages (text, page) VALUES (?, ?)`, text, page)
	return err
}

// Message is a message a visitor wrote, the page it was written on, and
// when.
type Message struct {
	Text    string
	Page    string
	Created time.Time
}

// Messages are the messages kept, newest first.
func (s *Store) Messages(ctx context.Context) ([]Message, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT text, page, created FROM messages ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Message
	for rows.Next() {
		var m Message
		var c int64
		if err := rows.Scan(&m.Text, &m.Page, &c); err != nil {
			return nil, err
		}
		m.Created = time.Unix(c, 0).UTC()
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) Close() error { return s.db.Close() }
