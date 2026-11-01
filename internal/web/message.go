package web

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

// MessageStore keeps the messages visitors write to the developer from
// the form at the bottom of every page. Only the developer reads them
// (cmd/messages); nothing about them is ever shown on the site.
type MessageStore interface {
	AddMessage(ctx context.Context, text, page string) error
}

const (
	// messagesPerMinute is how many messages one visitor can send.
	messagesPerMinute = 3
	// messageMaxRunes bounds one message's text.
	messageMaxRunes = 4000
)

// SetMessages turns on the message form, its messages kept in store.
func (s *Server) SetMessages(store MessageStore) {
	s.messages = store
	s.messageLimit = newRateLimit(messagesPerMinute, time.Minute)
}

// handleMessage keeps a message (text) and the page it was written on
// (page, its path and query), so a score report carries its result.
func (s *Server) handleMessage(w http.ResponseWriter, r *http.Request) {
	if s.messages == nil {
		http.NotFound(w, r)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, formLimit)
	text := strings.TrimSpace(r.PostFormValue("text"))
	if text == "" || !utf8.ValidString(text) || utf8.RuneCountInString(text) > messageMaxRunes {
		http.Error(w, "empty or too long", http.StatusBadRequest)
		return
	}
	page := messagePage(r.PostFormValue("page"))
	if !s.messageLimit.allow(clientIP(r)) {
		http.Error(w, "too many messages", http.StatusTooManyRequests)
		return
	}
	if err := s.messages.AddMessage(r.Context(), text, page); err != nil {
		slog.Error("store message", "err", err)
		http.Error(w, "messages unavailable", http.StatusServiceUnavailable)
		return
	}
	slog.Info("message received")
	w.WriteHeader(http.StatusNoContent)
}

// messagePage is the site path and query a message was written on, ""
// for anything else.
func messagePage(u string) string {
	if len(u) > maxTarget {
		return ""
	}
	p, err := url.Parse(u)
	if err != nil || p.Scheme != "" || p.Host != "" || !strings.HasPrefix(p.Path, "/") {
		return ""
	}
	return p.RequestURI()
}
