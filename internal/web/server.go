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
	client    *clientFiles           // nil: the server calculates every result
	links     LinkStore              // nil: no short links (see SetLinks)
	linkLimit *rateLimit

	site     string                     // the site's URL (see SetSite)
	byID     map[string]*library.Team   // library teams by ID
	byChar   map[string][]*library.Team // by canonical member, best score first
	indexNow *indexNow                  // nil: no IndexNow (see SetIndexNow)
}

func New(lib *library.Library, profiles ProfileSource, settings Settings) (*Server, error) {
	tmpls := map[localizer]*template.Template{}
	for code := range catalog {
		for _, auto := range []bool{false, true} {
			l := localizer{Lang: code, Auto: auto}
			tmpl, err := parseTemplates(l)
			if err != nil {
				return nil, err
			}
			tmpls[l] = tmpl
		}
	}
	byID, byChar := indexLibrary(lib)
	return &Server{
		byID:      byID,
		byChar:    byChar,
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
	mux.HandleFunc("POST /showcase", s.handleShowcase)
	mux.HandleFunc("GET /teams", s.handleTeams)
	mux.HandleFunc("GET /teams/{id}", s.handleLibraryTeam)
	mux.HandleFunc("GET /characters/{key}", s.handleCharacter)
	mux.HandleFunc("GET /robots.txt", s.handleRobots)
	mux.HandleFunc("GET /sitemap.xml", s.handleSitemapIndex)
	mux.HandleFunc("GET /sitemaps/{file}", s.handleSitemap)
	if s.indexNow != nil {
		mux.HandleFunc("GET /"+s.indexNow.key+".txt", s.handleIndexNowKey)
	}
	mux.HandleFunc("GET /score", s.handleScore)
	mux.HandleFunc("GET /versus", s.handleVersus)
	mux.HandleFunc("POST /lang", s.handleLang)
	mux.HandleFunc("GET /card/score.png", s.handleScoreCard)
	mux.HandleFunc("GET /card/versus.png", s.handleVersusCard)
	mux.HandleFunc("GET /static/{file}", s.handleStatic)
	for name := range iconTypes {
		mux.HandleFunc("GET /"+name, handleIcon)
	}
	mux.HandleFunc("POST /s", s.handleShorten)
	mux.HandleFunc("GET /s/{id}", s.handleShort)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	return mux
}

// selection is four characters picked from one player's showcase, written
// as "UID:char1,char2,char3,char4" in URLs. A selection can carry the
// characters' builds instead, written as "UID~packed" (see packBuilds),
// so that its link keeps working after they leave the showcase.
type selection struct {
	UID   string
	Chars []string
	// Builds are Chars as they were picked, nil to look them up in the
	// player's current showcase.
	Builds   []enka.Character
	Nickname string // with Builds
}

func (sel selection) String() string {
	if sel.Builds != nil {
		return sel.UID + "~" + packBuilds(sel.Nickname, sel.Builds)
	}
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
	if uid, packed, ok := strings.Cut(s, "~"); ok {
		nickname, builds, err := unpackBuilds(packed)
		if err != nil || !uidPattern.MatchString(uid) {
			return selection{}, inputError{"err.selection", []any{s}}
		}
		sel, err := newSelection(uid, keys(builds))
		sel.Builds, sel.Nickname = builds, nickname
		return sel, err
	}
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
	// Enemy is the target, nil for the standard enemy. Both teams of a
	// comparison have the same.
	Enemy    *config.Enemy
	Nickname string
	Chars    []enka.Character
	Team     *library.Team
	Result   *score.Result // the looped rotation: the score
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

// Simulator runs the simulations of a score: the server's own, queued
// and cached, or a browser's workers (cmd/wasm).
type Simulator interface {
	// Score is score.Run of cfg with the given warmup and iterations.
	Score(ctx context.Context, cfg string, warmup float64, iterations int) (*score.Result, error)
	// Timeline runs cfg once (seed 1) and reduces seconds [from, to).
	Timeline(ctx context.Context, cfg string, from, to float64) (*timeline.Timeline, error)
}

// serverSim is the server's own Simulator.
type serverSim struct{ *Server }

func (s serverSim) Score(ctx context.Context, cfg string, warmup float64, iterations int) (*score.Result, error) {
	return s.score(ctx, cfg, warmup, iterations)
}

func (s serverSim) Timeline(ctx context.Context, cfg string, from, to float64) (*timeline.Timeline, error) {
	return s.timeline(ctx, cfg, from, to)
}

// evaluate prepares and calculates a team on the server.
func (s *Server) evaluate(ctx context.Context, l localizer, sel selection, enemy *config.Enemy) teamResult {
	tr := s.prepare(ctx, l, sel, enemy)
	if tr.Problem == "" {
		compute(ctx, l, serverSim{s}, s.settings, &tr)
	}
	return tr
}

// prepare looks up a selection's characters and library team: the cheap
// part of a score, which the server always does. A team without a
// Problem is ready for compute.
func (s *Server) prepare(ctx context.Context, l localizer, sel selection, enemy *config.Enemy) teamResult {
	tr := teamResult{Selection: sel, Enemy: enemy}
	if sel.Builds != nil {
		tr.Nickname, tr.Chars = sel.Nickname, sel.Builds
	} else {
		p, err := s.profiles.Profile(ctx, sel.UID)
		if err != nil {
			tr.Problem = enkaMessage(l, err)
			return tr
		}
		tr.Nickname = p.Nickname
		for _, want := range sel.Chars {
			i := slices.IndexFunc(p.Characters, func(c enka.Character) bool { return c.Key == want })
			if i < 0 {
				tr.Problem = l.T("problem.missing", l.charName(want))
				return tr
			}
			tr.Chars = append(tr.Chars, p.Characters[i])
		}
	}

	tr.Team = s.lib.Find(sel.Chars)
	switch {
	case tr.Team == nil:
		slog.Info("unsupported team", "members", strings.Join(sorted(sel.Chars), ","))
		tr.Problem = l.T("problem.unsupported")
		return tr
	case tr.Team.Status == library.StatusBlocked:
		tr.Problem = l.T("problem.blocked", tr.Team.Notes)
	}
	return tr
}

// exploreIterations is the iterations of the runs that try fillers for a
// rotation that stands still (steady.Score): enough to rank them. The
// best is then run again with the full iterations.
const exploreIterations = 20

// compute calculates a prepared team with sim, filling in its score, or
// its Problem when the calculation fails.
func compute(ctx context.Context, l localizer, sim Simulator, set Settings, tr *teamResult) {
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
	once := config.Assemble(config.Conditions{Iterations: set.Iterations, Enemy: tr.Enemy}, build.String(), rotation)
	looped := func(rotation string, iterations int) (*score.Result, error) {
		cfg := config.Assemble(config.Conditions{Iterations: iterations, Duration: set.Duration, Enemy: tr.Enemy}, build.String(), rotation)
		return sim.Score(ctx, cfg, set.Warmup, iterations)
	}

	// The rotation as written runs with the full iterations; most teams
	// stop there. Fillers are only ranked, with fewer.
	var first *steady.Outcome
	out, err := steady.Score(rotation, members, func(rotation string) (*score.Result, error) {
		iterations := set.Iterations
		if first != nil {
			iterations = min(iterations, exploreIterations)
		}
		res, err := looped(rotation, iterations)
		if err != nil && (errors.Is(err, errBusy) || ctx.Err() != nil) {
			return nil, steady.Fatal(err)
		}
		if err == nil && first == nil {
			first = &steady.Outcome{Result: res, Looped: rotation}
		}
		return res, err
	})
	if err == nil && out.Filled && out.Result.Iterations < set.Iterations {
		var res *score.Result
		if res, err = looped(out.Looped, set.Iterations); err == nil {
			out.Result = res
			// Ranked on fewer iterations, the filler can come out
			// below the rotation as written.
			if res.DPS <= first.Result.DPS {
				out = first
			}
		}
	}
	var oneRes *score.Result
	if err == nil {
		oneRes, err = sim.Score(ctx, once, 0, set.Iterations)
	}
	if err != nil {
		if errors.Is(err, errBusy) {
			tr.Problem = l.T("problem.busy")
		} else {
			slog.Error("score failed", "team", tr.Team.ID, "uid", tr.Selection.UID, "err", err)
			tr.Problem = l.T("problem.failed")
		}
		return
	}
	res := out.Result
	tr.Filler = fillerNote(l, out)
	tr.Result = res
	tr.OneRotation = oneRes
	tr.Warnings = warnings(l, tr.Team, res)
	tr.Rotation = rotation
	tr.Timelines = cycleViews(ctx, l, sim, set, *tr, once, config.Assemble(config.Conditions{Iterations: 1, Duration: set.Duration, Enemy: tr.Enemy}, build.String(), out.Looped), out.Filled)
	for _, c := range tr.Chars {
		if c.UnknownSets > 0 {
			tr.Warnings = append(tr.Warnings, l.T("warn.unknown_sets", l.charName(c.Key), c.UnknownSets))
		}
	}
}

// score runs a simulation, reusing the cached result for the same config.
func (s *Server) score(ctx context.Context, cfg string, warmup float64, iterations int) (*score.Result, error) {
	sum := sha256.Sum256(fmt.Appendf(nil, "%v %v\n%s", warmup, iterations, cfg))
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
		Iterations: iterations,
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
func cycleViews(ctx context.Context, l localizer, sim Simulator, set Settings, tr teamResult, once, looped string, filled bool) []*timelineView {
	cycle, err := sim.Timeline(ctx, once, 0, maxCycle)
	if err != nil {
		slog.Warn("timeline failed", "team", tr.Team.ID, "uid", tr.Selection.UID, "err", err)
		return nil
	}
	note := l.T("tl.note_cycle")
	if cycle.End >= maxCycle {
		note = l.T("tl.note_cycle_to", maxCycle)
	}
	views := []*timelineView{layoutTimeline(l, "cycle", l.T("tl.label_cycle"), note, cycle)}

	from := set.Warmup
	to := from + min(max(cycle.End, minSteady), maxSteady)
	steady, err := sim.Timeline(ctx, looped, from, to)
	if err != nil {
		slog.Warn("timeline failed", "team", tr.Team.ID, "uid", tr.Selection.UID, "err", err)
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
	if team.HasForkOnly() {
		out = append(out, l.T("warn.experimental"))
	}
	if team.Written() {
		out = append(out, l.T("warn.written"))
	}
	for _, m := range team.Members {
		if key, ok := knownIssues[library.Canonical(m)]; ok {
			out = append(out, l.T(key))
		}
	}
	for _, c := range res.Characters {
		if c.EnergyWait >= 1 {
			out = append(out, l.T("warn.energy", l.charName(c.Name), c.EnergyWait))
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
		chars := l.charNames(g.Chars)
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
		parts = append(parts, l.T("filler.skill", l.charNames(o.Skill)))
	}
	return l.T("filler.wrap", strings.Join(parts, l.T("filler.sep")))
}
