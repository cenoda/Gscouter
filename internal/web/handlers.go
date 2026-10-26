package web

import (
	"context"
	"fmt"
	"html/template"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/cenoda/gscouter/internal/enka"
	"github.com/cenoda/gscouter/internal/library"
)

var funcs = template.FuncMap{
	"thousands": func(v float64) string {
		s := fmt.Sprintf("%.0f", v)
		var b strings.Builder
		for i, r := range s {
			if i > 0 && (len(s)-i)%3 == 0 {
				b.WriteByte(',')
			}
			b.WriteRune(r)
		}
		return b.String()
	},
	"percent": func(v float64) string { return fmt.Sprintf("%.1f%%", v*100) },
	"oneDecimal": func(v float64) string {
		return fmt.Sprintf("%.1f", v)
	},
}

type suggestion struct {
	Team      *library.Team
	Selection selection
}

type indexData struct {
	UID       string
	VS        string // packed first selection when picking the opponent
	Profile   *enka.Profile
	Problem   string
	Suggested []suggestion
	Teams     []*library.Team
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	data := indexData{
		UID:   strings.TrimSpace(q.Get("uid")),
		VS:    q.Get("vs"),
		Teams: s.lib.Teams,
	}
	if data.UID != "" {
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		p, err := s.profiles.Profile(ctx, data.UID)
		if err != nil {
			data.Problem = enkaMessage(err)
		} else {
			data.Profile = p
			data.Suggested = s.suggest(p)
		}
	}
	s.render(w, "index.html", data)
}

// suggest lists the library teams that can be built from a showcase.
func (s *Server) suggest(p *enka.Profile) []suggestion {
	var out []suggestion
	for _, t := range s.lib.Teams {
		var chars []string
		for _, m := range t.Members {
			i := slices.IndexFunc(p.Characters, func(c enka.Character) bool {
				return library.Canonical(c.Key) == library.Canonical(m)
			})
			if i < 0 {
				break
			}
			chars = append(chars, p.Characters[i].Key)
		}
		if len(chars) == 4 {
			out = append(out, suggestion{Team: t, Selection: selection{UID: p.UID, Chars: chars}})
		}
	}
	return out
}

type scoreData struct {
	Team teamResult
}

func (s *Server) handleScore(w http.ResponseWriter, r *http.Request) {
	sel, err := selectionFromQuery(r, "a")
	if err != nil {
		s.renderProblem(w, err.Error())
		return
	}
	// Normalize the URL so results can be shared as /score?a=UID:...
	if r.URL.Query().Get("a") == "" {
		http.Redirect(w, r, "/score?a="+url.QueryEscape(sel.String()), http.StatusSeeOther)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	s.render(w, "score.html", scoreData{Team: s.evaluate(ctx, sel)})
}

type versusData struct {
	A, B teamResult
	// Diff is (A - B) / B; only set when both teams have a score.
	Diff   float64
	Scored bool
	Tie    bool
}

// tieThreshold is the relative difference below which two scores are shown
// as a tie. Rotation error is larger than simulation noise (design §7).
const tieThreshold = 0.02

func (s *Server) handleVersus(w http.ResponseWriter, r *http.Request) {
	a, err := parseSelection(r.URL.Query().Get("a"))
	if err != nil {
		s.renderProblem(w, err.Error())
		return
	}
	b, err := selectionFromQuery(r, "b")
	if err != nil {
		s.renderProblem(w, err.Error())
		return
	}
	if r.URL.Query().Get("b") == "" {
		v := url.Values{"a": {a.String()}, "b": {b.String()}}
		http.Redirect(w, r, "/versus?"+v.Encode(), http.StatusSeeOther)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()
	data := versusData{A: s.evaluate(ctx, a), B: s.evaluate(ctx, b)}
	if data.A.Result != nil && data.B.Result != nil && data.B.Result.DPS > 0 {
		data.Scored = true
		data.Diff = (data.A.Result.DPS - data.B.Result.DPS) / data.B.Result.DPS
		data.Tie = math.Abs(data.Diff) < tieThreshold
	}
	s.render(w, "versus.html", data)
}

func (s *Server) renderProblem(w http.ResponseWriter, msg string) {
	w.WriteHeader(http.StatusBadRequest)
	s.render(w, "problem.html", msg)
}

func (s *Server) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		slog.Error("render", "template", name, "err", err)
	}
}

// EnkaSource fetches profiles from Enka.Network and caches them for the
// time Enka asks for.
type EnkaSource struct {
	client *enka.Client
	cache  *cache[string, *enka.Profile]
}

func NewEnkaSource(client *enka.Client) *EnkaSource {
	return &EnkaSource{client: client, cache: newCache[string, *enka.Profile](500)}
}

func (e *EnkaSource) Profile(ctx context.Context, uid string) (*enka.Profile, error) {
	if p, ok := e.cache.get(uid); ok {
		return p, nil
	}
	raw, err := e.client.Fetch(ctx, uid)
	if err != nil {
		return nil, err
	}
	p, err := enka.Parse(raw)
	if err != nil {
		return nil, err
	}
	if p.UID == "" {
		p.UID = uid
	}
	ttl := max(time.Duration(p.TTL)*time.Second, time.Minute)
	e.cache.put(uid, p, ttl)
	return p, nil
}
