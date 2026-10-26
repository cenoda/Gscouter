// Package web serves the Gscouter site: look up a UID, pick four showcase
// characters, and score or compare teams.
package web

import (
	"cmp"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/cenoda/gscouter/internal/config"
	"github.com/cenoda/gscouter/internal/enka"
	"github.com/cenoda/gscouter/internal/library"
	"github.com/cenoda/gscouter/internal/score"
	"github.com/cenoda/gscouter/internal/steady"
	"github.com/cenoda/gscouter/internal/timeline"
)

//go:embed templates/*.html
var templateFS embed.FS

// Settings control the simulation cost per score.
type Settings struct {
	Iterations    int
	Warmup        float64 // seconds excluded from the score
	Duration      float64 // seconds of the looped fight
	MaxConcurrent int     // simulations running at once
	MaxQueue      int     // simulations waiting before requests are rejected
}

// DefaultSettings score the looped rotation over 150-450 s, after the
// full starting energy has run out (design §3.1).
var DefaultSettings = Settings{
	Iterations:    100,
	Warmup:        150,
	Duration:      450,
	MaxConcurrent: 1,
	MaxQueue:      8,
}

// ProfileSource returns a player's converted showcase.
type ProfileSource interface {
	Profile(ctx context.Context, uid string) (*enka.Profile, error)
}

var errBusy = errors.New("server busy")

type Server struct {
	lib      *library.Library
	profiles ProfileSource
	settings Settings
	tmpl     map[localizer]*template.Template // by language

	sem       chan struct{}
	waiting   atomic.Int32
	results   *cache[string, *score.Result]
	timelines *cache[string, *timeline.Timeline]
	cards     *cache[string, []byte] // link preview PNGs by URL
}

func New(lib *library.Library, profiles ProfileSource, settings Settings) (*Server, error) {
	tmpls := map[localizer]*template.Template{}
	for code := range catalog {
		for _, auto := range []bool{false, true} {
			l := localizer{Lang: code, Auto: auto}
			tmpl, err := template.New("").Funcs(funcs).Funcs(localeFuncs(l)).ParseFS(templateFS, "templates/*.html")
			if err != nil {
				return nil, err
			}
			tmpls[l] = tmpl
		}
	}
	return &Server{
		lib:       lib,
		profiles:  profiles,
		settings:  settings,
		tmpl:      tmpls,
		sem:       make(chan struct{}, max(settings.MaxConcurrent, 1)),
		results:   newCache[string, *score.Result](1000),
		timelines: newCache[string, *timeline.Timeline](200),
		cards:     newCache[string, []byte](200),
	}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("GET /teams", s.handleTeams)
	mux.HandleFunc("GET /score", s.handleScore)
	mux.HandleFunc("GET /versus", s.handleVersus)
	mux.HandleFunc("POST /lang", s.handleLang)
	mux.HandleFunc("GET /card/score.png", s.handleScoreCard)
	mux.HandleFunc("GET /card/versus.png", s.handleVersusCard)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	return mux
}

// selection is four characters picked from one player's showcase, written
// as "UID:char1,char2,char3,char4" in URLs.
type selection struct {
	UID   string
	Chars []string
}

func (sel selection) String() string {
	return sel.UID + ":" + strings.Join(sel.Chars, ",")
}

// inputError is a problem with a request, told to the visitor in their
// language by its catalog key.
type inputError struct {
	key  string
	args []any
}

func (e inputError) Error() string { return english.T(e.key, e.args...) }

// message is err for the visitor in l's language.
func (l localizer) message(err error) string {
	if e, ok := err.(inputError); ok {
		return l.T(e.key, e.args...)
	}
	return err.Error()
}

func parseSelection(s string) (selection, error) {
	uid, chars, ok := strings.Cut(s, ":")
	if !ok {
		return selection{}, inputError{"err.selection", []any{s}}
	}
	return newSelection(uid, strings.Split(chars, ","))
}

func newSelection(uid string, chars []string) (selection, error) {
	if len(chars) != 4 {
		return selection{}, inputError{"err.count", []any{len(chars)}}
	}
	seen := map[string]bool{}
	for _, c := range chars {
		if c == "" || seen[c] {
			return selection{}, inputError{key: "err.chars"}
		}
		seen[c] = true
	}
	return selection{UID: uid, Chars: chars}, nil
}

// selectionFromQuery reads a selection either from a packed parameter
// (e.g. "a") or from the picker form's uid and c parameters.
func selectionFromQuery(r *http.Request, packed string) (selection, error) {
	q := r.URL.Query()
	if v := q.Get(packed); v != "" {
		return parseSelection(v)
	}
	return newSelection(strings.TrimSpace(q.Get("uid")), q["c"])
}

// teamResult is one evaluated team, for the result templates.
type teamResult struct {
	Selection selection
	Nickname  string
	Chars     []enka.Character
	Team      *library.Team
	Result    *score.Result // the looped rotation: the score
	// OneRotation is a single pass from full energy, the number community
	// gcsim configs report.
	OneRotation *score.Result
	// Filler says how the score fills time the rotation would stand
	// still, "" when it doesn't.
	Filler   string
	Warnings []string
	// Timelines show one run of the rotation: the one pass, then the
	// looped fight in the scoring window. Empty when they failed; the
	// score stands without them.
	Timelines []*timelineView
	// Rotation is the library rotation as run for this player.
	Rotation string
	// ID tells the teams on one page apart, for element ids.
	ID string
	// Problem explains why there is no score: unsupported team, blocked
	// team, Enka error, and so on.
	Problem string
}

func (s *Server) evaluate(ctx context.Context, l localizer, sel selection) teamResult {
	tr := teamResult{Selection: sel}
	p, err := s.profiles.Profile(ctx, sel.UID)
	if err != nil {
		tr.Problem = enkaMessage(l, err)
		return tr
	}
	tr.Nickname = p.Nickname
	for _, want := range sel.Chars {
		i := slices.IndexFunc(p.Characters, func(c enka.Character) bool { return c.Key == want })
		if i < 0 {
			tr.Problem = l.T("problem.missing", want)
			return tr
		}
		tr.Chars = append(tr.Chars, p.Characters[i])
	}

	tr.Team = s.lib.Find(sel.Chars)
	switch {
	case tr.Team == nil:
		slog.Info("unsupported team", "members", strings.Join(sorted(sel.Chars), ","))
		tr.Problem = l.T("problem.unsupported")
		return tr
	case tr.Team.Status == library.StatusBlocked:
		tr.Problem = l.T("problem.blocked", tr.Team.Notes)
		return tr
	}

	var build strings.Builder
	renames := map[string]string{}
	for _, c := range tr.Chars {
		line := c.Config()
		for _, m := range tr.Team.Members {
			if library.Canonical(m) == library.Canonical(c.Key) {
				renames[m] = c.Key
				line = config.AddParams(line, c.Key, tr.Team.Params[m])
			}
		}
		build.WriteString(line)
		build.WriteString("\n")
	}
	rotation := config.RenameCharacters(tr.Team.Rotation, renames)
	var members []string
	for _, m := range tr.Team.Members {
		members = append(members, cmp.Or(renames[m], m))
	}
	once := config.Assemble(config.Conditions{Iterations: s.settings.Iterations}, build.String(), rotation)

	out, err := steady.Score(rotation, members, func(looped string) (*score.Result, error) {
		cfg := config.Assemble(config.Conditions{Iterations: s.settings.Iterations, Duration: s.settings.Duration}, build.String(), looped)
		res, err := s.score(ctx, cfg, s.settings.Warmup)
		if errors.Is(err, errBusy) {
			return nil, steady.Fatal(err)
		}
		return res, err
	})
	var oneRes *score.Result
	if err == nil {
		oneRes, err = s.score(ctx, once, 0)
	}
	if err != nil {
		if errors.Is(err, errBusy) {
			tr.Problem = l.T("problem.busy")
		} else {
			slog.Error("score failed", "team", tr.Team.ID, "uid", sel.UID, "err", err)
			tr.Problem = l.T("problem.failed")
		}
		return tr
	}
	res := out.Result
	tr.Filler = fillerNote(l, out)
	tr.Result = res
	tr.OneRotation = oneRes
	tr.Warnings = warnings(l, tr.Team, res)
	tr.Rotation = rotation
	tr.Timelines = s.cycleViews(ctx, l, sel, tr.Team, once, config.Assemble(config.Conditions{Iterations: 1, Duration: s.settings.Duration}, build.String(), out.Looped), out.Filled)
	for _, c := range tr.Chars {
		if c.UnknownSets > 0 {
			tr.Warnings = append(tr.Warnings, l.T("warn.unknown_sets", c.Key, c.UnknownSets))
		}
	}
	return tr
}

// score runs a simulation, reusing the cached result for the same config.
func (s *Server) score(ctx context.Context, cfg string, warmup float64) (*score.Result, error) {
	sum := sha256.Sum256(fmt.Appendf(nil, "%v\n%s", warmup, cfg))
	key := hex.EncodeToString(sum[:])
	if res, ok := s.results.get(key); ok {
		return res, nil
	}

	if int(s.waiting.Add(1)) > s.settings.MaxQueue+cap(s.sem) {
		s.waiting.Add(-1)
		return nil, errBusy
	}
	defer s.waiting.Add(-1)
	select {
	case s.sem <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-s.sem }()

	start := time.Now()
	res, err := score.Run(ctx, cfg, score.Options{
		Iterations: s.settings.Iterations,
		Warmup:     warmup,
		Seed:       1,
	})
	if err != nil {
		return nil, err
	}
	slog.Info("scored", "dps", int(res.DPS), "took", time.Since(start).Round(time.Millisecond))
	s.results.put(key, res, 24*time.Hour)
	return res, nil
}

// cycleViews lays out one run of the one pass and of the looped fight
// from the warmup on, for as long as one pass takes.
func (s *Server) cycleViews(ctx context.Context, l localizer, sel selection, team *library.Team, once, looped string, filled bool) []*timelineView {
	cycle, err := s.timeline(ctx, once, 0, maxCycle)
	if err != nil {
		slog.Warn("timeline failed", "team", team.ID, "uid", sel.UID, "err", err)
		return nil
	}
	note := l.T("tl.note_cycle")
	if cycle.End >= maxCycle {
		note = l.T("tl.note_cycle_to", maxCycle)
	}
	views := []*timelineView{layoutTimeline(l, "cycle", l.T("tl.label_cycle"), note, cycle)}

	from := s.settings.Warmup
	to := from + min(max(cycle.End, minSteady), maxSteady)
	steady, err := s.timeline(ctx, looped, from, to)
	if err != nil {
		slog.Warn("timeline failed", "team", team.ID, "uid", sel.UID, "err", err)
		return views
	}
	note = l.T("tl.note_steady", from, to-from)
	if filled {
		note += l.T("tl.note_filled")
	}
	return append(views, layoutTimeline(l, "steady", l.T("tl.label_steady"), note, steady))
}

// knownIssues are the catalog keys of notes shown on every team with the
// character, keyed by canonical name.
var knownIssues = map[string]string{
	"arlecchino": "issue.arlecchino",
}

func warnings(l localizer, team *library.Team, res *score.Result) []string {
	var out []string
	if team.Status == library.StatusDraft {
		out = append(out, l.T("warn.draft"))
	}
	for _, m := range team.Members {
		if key, ok := knownIssues[library.Canonical(m)]; ok {
			out = append(out, l.T(key))
		}
	}
	for _, c := range res.Characters {
		if c.EnergyWait >= 1 {
			out = append(out, l.T("warn.energy", c.Name, c.EnergyWait))
		}
	}
	return out
}

func enkaMessage(l localizer, err error) string {
	switch {
	case errors.Is(err, enka.ErrInvalidUID):
		return l.T("enka.invalid")
	case errors.Is(err, enka.ErrNotFound):
		return l.T("enka.not_found")
	case errors.Is(err, enka.ErrMaintenance):
		return l.T("enka.maintenance")
	case errors.Is(err, enka.ErrRateLimited):
		return l.T("enka.rate_limited")
	default:
		slog.Error("enka fetch failed", "err", err)
		return l.T("enka.failed")
	}
}

func sorted(s []string) []string {
	out := slices.Clone(s)
	slices.Sort(out)
	return out
}

// fillerNote explains how the score fills the rotation's waits.
func fillerNote(l localizer, o *steady.Outcome) string {
	if !o.Filled {
		return ""
	}
	var parts []string
	for _, g := range o.ByFiller() {
		chars := strings.Join(g.Chars, ", ")
		switch g.How {
		case config.FillSkip:
			parts = append(parts, l.T("filler.skip", chars))
		case config.FillParty:
			parts = append(parts, l.T("filler.party", chars))
		default:
			parts = append(parts, l.T("filler.self", chars))
		}
	}
	if len(o.Skill) > 0 {
		parts = append(parts, l.T("filler.skill", strings.Join(o.Skill, ", ")))
	}
	return l.T("filler.wrap", strings.Join(parts, l.T("filler.sep")))
}
