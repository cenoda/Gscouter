package web

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Short links (/s/<id>) stand for result links, which carry the builds
// and run to hundreds of characters. The long link stays the result's
// own: a short link only redirects to it, so losing the store breaks
// short links and nothing else.

// LinkStore keeps short link IDs and the result URLs they stand for.
type LinkStore interface {
	// Add stores target under id unless id is taken, and returns the
	// target id stands for: target, or the one stored first.
	Add(ctx context.Context, id, target string) (string, error)
	// Get returns the target of id, "" when there is none.
	Get(ctx context.Context, id string) (string, error)
}

// SetLinks turns on short links, kept in store.
func (s *Server) SetLinks(store LinkStore) {
	s.links = store
	s.linkLimit = newRateLimit(linksPerMinute, time.Minute)
}

const (
	// shortID is the length of a short link ID: 48 bits of the target's
	// hash. A collision lengthens the newer ID, up to maxShortID.
	shortID    = 8
	maxShortID = 12
	// maxTarget bounds a result URL: two teams' packed builds and the
	// enemy.
	maxTarget = 2*maxPacked + 512
	// linksPerMinute is how many short links one visitor can make.
	linksPerMinute = 20
)

var shortIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{8,12}$`)

// handleShorten makes the short link for a result URL, given as the
// path and query of the page (u), and answers with it.
func (s *Server) handleShorten(w http.ResponseWriter, r *http.Request) {
	if s.links == nil {
		http.NotFound(w, r)
		return
	}
	target, err := shortTarget(r.PostFormValue("u"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !s.linkLimit.allow(clientIP(r)) {
		http.Error(w, "too many links", http.StatusTooManyRequests)
		return
	}
	sum := sha256.Sum256([]byte(target))
	enc := base64.RawURLEncoding.EncodeToString(sum[:])
	for n := shortID; n <= maxShortID; n++ {
		id := enc[:n]
		got, err := s.links.Add(r.Context(), id, target)
		if err != nil {
			slog.Error("store short link", "err", err)
			http.Error(w, "short links unavailable", http.StatusServiceUnavailable)
			return
		}
		if got == target {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Write([]byte(baseURL(r) + "/s/" + id))
			return
		}
	}
	http.Error(w, "short links unavailable", http.StatusServiceUnavailable)
}

// shortTarget checks that u is a result page's path and query and
// returns it in the form result pages redirect to (canonical). Only
// selections that carry their builds qualify, so the link keeps its
// result after the showcase changes.
func shortTarget(u string) (string, error) {
	if len(u) > maxTarget {
		return "", errors.New("link too long")
	}
	p, err := url.Parse(u)
	if err != nil || p.Scheme != "" || p.Host != "" {
		return "", errors.New("not a result link")
	}
	var params []string
	switch p.Path {
	case "/score":
		params = []string{"a"}
	case "/versus":
		params = []string{"a", "b"}
	default:
		return "", errors.New("not a result link")
	}
	q := p.Query()
	for _, k := range params {
		sel, err := parseSelection(q.Get(k))
		if err != nil || sel.Builds == nil {
			return "", errors.New("not a result link")
		}
	}
	enemy, err := enemyFromQuery(q)
	if err != nil {
		return "", errors.New("not a result link")
	}
	// Leave out the server fallback: the opener's browser decides that.
	want := url.Values{}
	for _, k := range params {
		want.Set(k, q.Get(k))
	}
	addEnemy(want, enemy)
	return p.Path + "?" + want.Encode(), nil
}

// handleShort redirects a short link to its result.
func (s *Server) handleShort(w http.ResponseWriter, r *http.Request) {
	l := localizerFor(r)
	id := r.PathValue("id")
	if s.links == nil || !shortIDPattern.MatchString(id) {
		s.renderMissingLink(w, l)
		return
	}
	target, err := s.links.Get(r.Context(), id)
	if err != nil {
		slog.Error("read short link", "id", id, "err", err)
		http.Error(w, "short links unavailable", http.StatusServiceUnavailable)
		return
	}
	if target == "" {
		s.renderMissingLink(w, l)
		return
	}
	http.Redirect(w, r, target, http.StatusFound)
}

func (s *Server) renderMissingLink(w http.ResponseWriter, l localizer) {
	s.writeHeader(w, l)
	w.WriteHeader(http.StatusNotFound)
	s.render(w, l, "problem.html", l.T("err.link"))
}

// clientIP is the visitor's address. Caddy replaces X-Forwarded-For with
// the address it was reached from, so behind it that is the header's
// first entry.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		first, _, _ := strings.Cut(xff, ",")
		return strings.TrimSpace(first)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// rateLimit allows each key n events per window.
type rateLimit struct {
	n      int
	window time.Duration

	mu    sync.Mutex
	start time.Time
	count map[string]int
}

func newRateLimit(n int, window time.Duration) *rateLimit {
	return &rateLimit{n: n, window: window, count: map[string]int{}}
}

func (l *rateLimit) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if now := time.Now(); now.Sub(l.start) >= l.window {
		l.start, l.count = now, map[string]int{}
	}
	if l.count[key] >= l.n {
		return false
	}
	l.count[key]++
	return true
}
