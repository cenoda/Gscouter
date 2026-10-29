package web

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUsage(t *testing.T) {
	file := filepath.Join(t.TempDir(), "usage.json")
	s := &Server{}
	if err := s.SetUsage(file); err != nil {
		t.Fatal(err)
	}
	day := time.Date(2026, 10, 29, 23, 0, 0, 0, time.UTC)
	s.usage.now = func() time.Time { return day }

	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("User-Agent", "Mozilla/5.0")
	bot := httptest.NewRequest("GET", "/", nil)
	bot.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Googlebot/2.1)")

	s.usage.page(r)
	s.usage.page(bot)
	s.usage.uid(r, "800000001")
	s.usage.uid(r, "800000001")
	s.usage.uid(r, "800000002")
	s.usage.result(r)
	day = day.Add(2 * time.Hour) // the next day
	s.usage.uid(r, "800000001")
	if err := s.FlushUsage(); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]usageDay
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]usageDay{
		"2026-10-29": {Pages: 1, Results: 1, UIDs: 2},
		"2026-10-30": {UIDs: 1},
	}
	if len(got) != len(want) || got["2026-10-29"] != want["2026-10-29"] || got["2026-10-30"] != want["2026-10-30"] {
		t.Errorf("saved %v, want %v", got, want)
	}

	// The counts continue after a restart.
	s = &Server{}
	if err := s.SetUsage(file); err != nil {
		t.Fatal(err)
	}
	s.usage.now = func() time.Time { return day }
	s.usage.page(r)
	if d := s.usage.days["2026-10-30"]; d.Pages != 1 || d.UIDs != 1 {
		t.Errorf("after restart %+v, want 1 page and 1 UID", *d)
	}
}

func TestUsageOnHealthz(t *testing.T) {
	s := &Server{}
	if err := s.SetUsage(filepath.Join(t.TempDir(), "usage.json")); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "/healthz", nil)
	r.Header.Set("User-Agent", "Mozilla/5.0")
	s.usage.page(r)

	health := func(auth string) string {
		t.Helper()
		req := httptest.NewRequest("GET", "/healthz", nil)
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		w := httptest.NewRecorder()
		s.handleHealth(w, req)
		return w.Body.String()
	}
	// No token set: nobody reads the counts, not even with an empty one.
	if got := health("Bearer "); got != "ok" {
		t.Errorf("no token: %q", got)
	}
	s.SetUsageToken("secret")
	for _, auth := range []string{"", "Bearer wrong", "secret", "Basic secret"} {
		if got := health(auth); got != "ok" {
			t.Errorf("Authorization %q: %q", auth, got)
		}
	}
	// Today's counts, before they are saved.
	var got map[string]usageDay
	if err := json.Unmarshal([]byte(health("Bearer secret")), &got); err != nil {
		t.Fatal(err)
	}
	today := time.Now().UTC().Format(time.DateOnly)
	if got[today].Pages != 1 {
		t.Errorf("counts %v, want 1 page today", got)
	}
}
