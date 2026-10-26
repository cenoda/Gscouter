package web

import (
	"cmp"
	"context"
	"fmt"
	"html/template"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cenoda/gscouter/internal/enka"
	"github.com/cenoda/gscouter/internal/library"
)

var funcs = template.FuncMap{
	"thousands": thousands,
	"percent":   func(v float64) string { return fmt.Sprintf("%.1f%%", v*100) },
	"oneDecimal": func(v float64) string {
		return fmt.Sprintf("%.1f", v)
	},
	// width is a fraction as a CSS percentage, for bars.
	"width": func(v float64) string { return fmt.Sprintf("%.1f", math.Max(0, math.Min(v, 1))*100) },
	// ratio is a over the larger of a and b.
	"ratio": func(a, b float64) float64 {
		if m := math.Max(a, b); m > 0 {
			return a / m
		}
		return 0
	},
	"neg": func(v float64) float64 { return -v },
	// px is a+b as an SVG coordinate.
	"px":      func(a, b float64) string { return strconv.FormatFloat(a+b, 'f', 1, 64) },
	"element": element,
	"face":    face,
	"keys": func(chars []enka.Character) []string {
		out := make([]string, len(chars))
		for i, c := range chars {
			out[i] = c.Key
		}
		return out
	},
}

// thousands formats a number rounded to an integer with thousands
// separators, e.g. "85,512".
func thousands(v float64) string {
	s := fmt.Sprintf("%.0f", v)
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	return b.String()
}

var elementCache sync.Map // canonical name -> element

// element returns a character's element ("pyro", ...), or "".
func element(name string) string {
	key := library.Canonical(name)
	if v, ok := elementCache.Load(key); ok {
		return v.(string)
	}
	e := ""
	if elem, ok := strings.CutPrefix(key, "traveler"); ok {
		e = elem
	} else {
		e = enka.Element(key)
	}
	elementCache.Store(key, e)
	return e
}

var enkaKeys sync.Map // name -> Enka data key

// enkaKey returns the gcsim key Enka's data knows a character by: a
// showcase or result name as is, a library member name by its canonical
// key. The Traveler of a library team ("travelercryo") is Aether.
func enkaKey(name string) string {
	if v, ok := enkaKeys.Load(name); ok {
		return v.(string)
	}
	key := strings.ToLower(name)
	if enka.IconURL(key) == "" {
		key = library.Canonical(name)
		if elem, ok := strings.CutPrefix(key, "traveler"); ok {
			key = "aether" + elem
		}
	}
	enkaKeys.Store(name, key)
	return key
}

// face returns the URL of a character's face icon, or "".
func face(name string) string { return enka.IconURL(enkaKey(name)) }

// charName returns a character's in-game name in l's language, or name
// itself when the game data doesn't know it.
func (l localizer) charName(name string) string {
	return cmp.Or(enka.CharacterName(enkaKey(name), l.Lang), name)
}

// charNames is charName of each name, joined by ", ".
func (l localizer) charNames(names []string) string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = l.charName(n)
	}
	return strings.Join(out, ", ")
}

// weaponName is charName for a weapon's gcsim key.
func (l localizer) weaponName(key string) string {
	return cmp.Or(enka.WeaponName(key, l.Lang), key)
}

// teamName names a library team by its members' in-game names.
func (l localizer) teamName(t *library.Team) string {
	out := make([]string, len(t.Members))
	for i, m := range t.Members {
		out[i] = l.charName(m)
	}
	return strings.Join(out, " / ")
}

// searchNames lists what finds a team's members: gcsim names, canonical
// keys, and in-game names in l's language and in English.
func (l localizer) searchNames(t *library.Team) []string {
	var out []string
	for _, m := range t.Members {
		for _, n := range []string{m, library.Canonical(m), l.charName(m), english.charName(m)} {
			if n = strings.ToLower(n); !slices.Contains(out, n) {
				out = append(out, n)
			}
		}
	}
	return out
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
	// Buildable lists the suggestions' characters, sorted and joined by
	// commas, so the picker can tell when four picks are not a team.
	Buildable []string
	TeamCount int
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	l := localizerFor(r)
	q := r.URL.Query()
	data := indexData{
		UID:       strings.TrimSpace(q.Get("uid")),
		VS:        q.Get("vs"),
		TeamCount: len(s.lib.Teams),
	}
	if data.UID != "" {
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		p, err := s.profiles.Profile(ctx, data.UID)
		if err != nil {
			data.Problem = enkaMessage(l, err)
		} else {
			data.Profile = p
			data.Suggested = s.suggest(p)
			for _, sg := range data.Suggested {
				data.Buildable = append(data.Buildable, strings.Join(sorted(sg.Selection.Chars), ","))
			}
		}
	}
	s.render(w, l, "index.html", data)
}

// suggest lists the library teams that can be built from a showcase.
func (s *Server) suggest(p *enka.Profile) []suggestion {
	have := map[string]string{} // canonical name -> showcase key
	for _, c := range p.Characters {
		have[library.Canonical(c.Key)] = c.Key
	}
	var out []suggestion
	for _, t := range s.lib.Teams {
		var chars []string
		for _, m := range t.Members {
			key, ok := have[library.Canonical(m)]
			if !ok {
				break
			}
			chars = append(chars, key)
		}
		if len(chars) == 4 {
			out = append(out, suggestion{Team: t, Selection: selection{UID: p.UID, Chars: chars}})
		}
	}
	return out
}

type teamsData struct {
	Query string
	Teams []*library.Team // matching the query, by name
	Total int
}

// handleTeams lists the library, or the teams that have, for every word
// of the query, a character whose name contains it.
func (s *Server) handleTeams(w http.ResponseWriter, r *http.Request) {
	l := localizerFor(r)
	data := teamsData{Query: strings.TrimSpace(r.URL.Query().Get("q")), Total: len(s.lib.Teams)}
	words := strings.Fields(strings.ToLower(data.Query))
	for _, t := range s.lib.Teams {
		names := l.searchNames(t)
		if !slices.ContainsFunc(words, func(w string) bool {
			return !slices.ContainsFunc(names, func(n string) bool { return strings.Contains(n, w) })
		}) {
			data.Teams = append(data.Teams, t)
		}
	}
	names := map[*library.Team]string{}
	for _, t := range data.Teams {
		names[t] = l.teamName(t)
	}
	slices.SortFunc(data.Teams, func(a, b *library.Team) int { return strings.Compare(names[a], names[b]) })
	s.render(w, l, "teams.html", data)
}

type scoreData struct {
	Team   teamResult
	Meta   pageMeta
	Client *clientData // set when the browser calculates Team
}

func (s *Server) handleScore(w http.ResponseWriter, r *http.Request) {
	l := localizerFor(r)
	sel, err := selectionFromQuery(r, "a")
	if err != nil {
		s.renderProblem(w, l, l.message(err))
		return
	}
	// Normalize the URL so results can be shared as /score?a=UID:...
	if r.URL.Query().Get("a") == "" {
		http.Redirect(w, r, "/score?a="+url.QueryEscape(sel.String()), http.StatusSeeOther)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	if s.inBrowser(r) {
		tr := s.prepare(ctx, l, sel)
		tr.ID = "a"
		job := s.job(l, "score", tr)
		s.render(w, l, "score.html", scoreData{Team: tr, Meta: pendingScoreMeta(l, tr, baseURL(r)), Client: s.clientData(r, job)})
		return
	}
	tr := s.evaluate(ctx, l, sel)
	tr.ID = "a"
	s.render(w, l, "score.html", scoreData{Team: tr, Meta: scoreMeta(l, tr, baseURL(r))})
}

type versusData struct {
	A, B teamResult
	// Diff is (A - B) / B; only set when both teams have a score.
	Diff   float64
	Scored bool
	Tie    bool
	Meta   pageMeta
	Client *clientData // set when the browser calculates the teams
}

// tieThreshold is the relative difference below which two scores are shown
// as a tie. Rotation error is larger than simulation noise (design §7).
const tieThreshold = 0.02

func (s *Server) handleVersus(w http.ResponseWriter, r *http.Request) {
	l := localizerFor(r)
	a, err := parseSelection(r.URL.Query().Get("a"))
	if err != nil {
		s.renderProblem(w, l, l.message(err))
		return
	}
	b, err := selectionFromQuery(r, "b")
	if err != nil {
		s.renderProblem(w, l, l.message(err))
		return
	}
	if r.URL.Query().Get("b") == "" {
		v := url.Values{"a": {a.String()}, "b": {b.String()}}
		http.Redirect(w, r, "/versus?"+v.Encode(), http.StatusSeeOther)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()
	if s.inBrowser(r) {
		data := newVersus(s.prepare(ctx, l, a), s.prepare(ctx, l, b))
		data.Meta = pendingVersusMeta(l, data, baseURL(r))
		data.Client = s.clientData(r, s.job(l, "versus", data.A, data.B))
		s.render(w, l, "versus.html", data)
		return
	}
	data := s.versus(ctx, l, a, b)
	data.Meta = versusMeta(l, data, baseURL(r))
	s.render(w, l, "versus.html", data)
}

func (s *Server) versus(ctx context.Context, l localizer, a, b selection) versusData {
	return newVersus(s.evaluate(ctx, l, a), s.evaluate(ctx, l, b))
}

// newVersus compares two teams, scored or not.
func newVersus(a, b teamResult) versusData {
	data := versusData{A: a, B: b}
	data.A.ID, data.B.ID = "a", "b"
	if data.A.Result != nil && data.B.Result != nil && data.B.Result.DPS > 0 {
		data.Scored = true
		data.Diff = (data.A.Result.DPS - data.B.Result.DPS) / data.B.Result.DPS
		data.Tie = math.Abs(data.Diff) < tieThreshold
	}
	return data
}

func (s *Server) renderProblem(w http.ResponseWriter, l localizer, msg string) {
	s.writeHeader(w, l)
	w.WriteHeader(http.StatusBadRequest)
	s.render(w, l, "problem.html", msg)
}

// writeHeader sets the headers of a page in l's language.
func (s *Server) writeHeader(w http.ResponseWriter, l localizer) {
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Content-Language", l.Lang)
	h.Set("Vary", "Accept-Language, Cookie")
}

func (s *Server) render(w http.ResponseWriter, l localizer, name string, data any) {
	s.writeHeader(w, l)
	if err := s.tmpl[l].ExecuteTemplate(w, name, data); err != nil {
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
