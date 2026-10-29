package web

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// SendStore keeps result links visitors send to the developer: a chart
// that scores higher than the library's rotations on their build
// (docs/cycle-swap.md §7). Whoever reviews one calculates it again from
// the link; the result itself is never sent.
type SendStore interface {
	AddSend(ctx context.Context, target string) error
}

// sendsPerMinute is how many links one visitor can send.
const sendsPerMinute = 5

// SetSends turns on sending charts, kept in store.
func (s *Server) SetSends(store SendStore) {
	s.sends = store
	s.sendLimit = newRateLimit(sendsPerMinute, time.Minute)
}

// handleSend keeps a result page's link (u, its path and query) that
// scores a chart.
func (s *Server) handleSend(w http.ResponseWriter, r *http.Request) {
	if s.sends == nil {
		http.NotFound(w, r)
		return
	}
	target, err := shortTarget(r.PostFormValue("u"))
	if err != nil || !strings.HasPrefix(target, "/score?") {
		http.Error(w, "not a result link", http.StatusBadRequest)
		return
	}
	if q, _ := url.ParseQuery(strings.TrimPrefix(target, "/score?")); q.Get("r") == "" {
		http.Error(w, "no rotation chart", http.StatusBadRequest)
		return
	}
	if !s.sendLimit.allow(clientIP(r)) {
		http.Error(w, "too many sends", http.StatusTooManyRequests)
		return
	}
	if err := s.sends.AddSend(r.Context(), target); err != nil {
		slog.Error("store send", "err", err)
		http.Error(w, "sending unavailable", http.StatusServiceUnavailable)
		return
	}
	slog.Info("chart sent")
	w.WriteHeader(http.StatusNoContent)
}

// Beats reports whether a chart scored higher than the library's
// rotations on the same build, which the page offers to send.
func (tr teamResult) Beats() bool {
	return tr.Library != nil && tr.Result != nil && tr.Result.DPS > tr.Library.DPS
}
