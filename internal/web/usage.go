package web

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// usage counts, per UTC day, how much the site is used: front page and
// result page views, and the distinct UIDs looked up. Only the counts are
// kept. UIDs are told apart by a hash with a key drawn each day and held
// in memory only, and the key and hashes are dropped when the day ends,
// so neither the counts nor the file can be traced back to a player. No
// address or cookie is involved.
//
// The day's UIDs are not kept across restarts, so a UID looked up both
// before and after a restart that day counts twice.
type usage struct {
	mu    sync.Mutex
	file  string
	days  map[string]*usageDay // by date, as saved
	today string
	key   [32]byte
	uids  map[[16]byte]bool
	dirty bool
	now   func() time.Time
}

type usageDay struct {
	Pages   int `json:"pages"`   // front page views
	Results int `json:"results"` // score and versus page views
	UIDs    int `json:"uids"`    // distinct UIDs looked up
}

// SetUsage turns usage counts on, kept in file. Call it before Handler,
// and FlushUsage before exiting.
func (s *Server) SetUsage(file string) error {
	u := &usage{file: file, days: map[string]*usageDay{}, now: time.Now}
	raw, err := os.ReadFile(file)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return err
	default:
		if err := json.Unmarshal(raw, &u.days); err != nil {
			return fmt.Errorf("%s: %w", file, err)
		}
	}
	s.usage = u
	go func() {
		for range time.Tick(10 * time.Minute) {
			if err := u.flush(); err != nil {
				slog.Error("usage", "file", file, "err", err)
			}
		}
	}()
	return nil
}

// FlushUsage saves the counts not saved yet.
func (s *Server) FlushUsage() error {
	if s.usage == nil {
		return nil
	}
	return s.usage.flush()
}

// page counts a front page view, result a score or versus page view.
func (u *usage) page(r *http.Request)   { u.count(r, func(d *usageDay) { d.Pages++ }) }
func (u *usage) result(r *http.Request) { u.count(r, func(d *usageDay) { d.Results++ }) }

// uid counts uid if it was not looked up yet today.
func (u *usage) uid(r *http.Request, uid string) {
	u.count(r, func(d *usageDay) {
		h := sha256.Sum256(append(u.key[:], uid...))
		if id := [16]byte(h[:16]); !u.uids[id] {
			u.uids[id] = true
			d.UIDs++
		}
	})
}

func (u *usage) count(r *http.Request, add func(*usageDay)) {
	if u == nil || isBot(r) {
		return
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	add(u.day())
	u.dirty = true
}

// day returns today's counts, starting a new day with a new key when the
// date has changed. Call it with mu held.
func (u *usage) day() *usageDay {
	today := u.now().UTC().Format(time.DateOnly)
	if today != u.today {
		if d := u.days[u.today]; d != nil {
			slog.Info("usage", "day", u.today, "pages", d.Pages, "results", d.Results, "uids", d.UIDs)
		}
		u.today = today
		rand.Read(u.key[:])
		u.uids = map[[16]byte]bool{}
	}
	d := u.days[today]
	if d == nil {
		d = &usageDay{}
		u.days[today] = d
	}
	return d
}

func (u *usage) flush() error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if !u.dirty {
		return nil
	}
	raw, err := json.MarshalIndent(u.days, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(u.file), ".usage-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), u.file); err != nil {
		return err
	}
	u.dirty = false
	return nil
}

// isBot reports whether r looks like it comes from a crawler or a link
// preview rather than a person's browser.
func isBot(r *http.Request) bool {
	ua := strings.ToLower(r.UserAgent())
	if ua == "" {
		return true
	}
	for _, s := range []string{"bot", "crawl", "spider", "slurp", "preview", "facebookexternalhit", "headless", "curl", "wget", "python", "go-http-client"} {
		if strings.Contains(ua, s) {
			return true
		}
	}
	return false
}
