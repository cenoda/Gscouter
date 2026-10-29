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
	"strconv"
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
	Iterations:    50,
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
	// Talents are Chars' talent levels in the fight: the base levels
	// plus constellation and passive bonuses. Nil when they could not
	// be worked out.
	Talents [][3]int
	// Stats are Chars' total stats before the fight (score.Detail),
	// nil when they could not be worked out.
	Stats  []map[string]float64
	Team   *library.Team
	Result *score.Result // the looped rotation: the score
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
	// Alternative is the team's alternative (library.Team.Alternatives)
	// that scored best for this build, nil when its own rotation did, and
	// AlternativeNote says so.
	Alternative     *library.Team
	AlternativeNote string
	// ID tells the teams on one page apart, for element ids.
	ID string
	// Problem explains why there is no score: unsupported team, blocked
	// team, Enka error, and so on.
	Problem string
	// ArtifactScores are Chars' artifact scores, nil for a team without
	// weights; an entry is nil for a character the weights lack.
	ArtifactScores []*artifactScore
}

// TalentsOf describes character i's talent levels: each base level, or
// for one a bonus raises, the level in the fight and how it is made up,
// e.g. "9/13 (10+3)/9".
func (tr teamResult) TalentsOf(i int) string {
	base := tr.Chars[i].Talents
	var parts []string
	for j, b := range base {
		part := strconv.Itoa(b)
		if i < len(tr.Talents) {
			if lvl := tr.Talents[i][j]; lvl != b {
				part = fmt.Sprintf("%d (%d+%d)", lvl, b, lvl-b)
			}
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, "/")
}

// StatsOf is character i's total stats, nil without them.
func (tr teamResult) StatsOf(i int) map[string]float64 {
	if i < len(tr.Stats) {
		return tr.Stats[i]
	}
	return nil
}

// Experimental reports whether the result carries the experimental badge:
// the team is experimental, or the player's build uses a weapon only
// Gscouter's gcsim fork has.
func (tr teamResult) Experimental() bool {
	return tr.Team != nil && tr.Team.Tier == library.TierExperimental || usesForkOnlyWeapon(tr.Chars)
}

// usesForkOnlyWeapon reports whether a character wields a weapon only
// Gscouter's gcsim fork has.
func usesForkOnlyWeapon(chars []enka.Character) bool {
	return slices.ContainsFunc(chars, func(c enka.Character) bool { return library.ForkOnlyWeapons[library.WeaponKey(c.Weapon.Key)] })
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
	scoreArtifacts(&tr)
	return tr
}

// exploreIterations is the iterations of the runs that try fillers for a
// rotation that stands still (steady.Score): enough to rank them. The
// best is then run again with the full iterations. 10 ranks as well as
// 20 did; a team misses its best filler because of the search's cap on
// runs (steady), not because of these iterations. 5 lowered one team 8.5%.
const exploreIterations = 10

// compute calculates a prepared team with sim, filling in its score, or
// its Problem when the calculation fails.
func compute(ctx context.Context, l localizer, sim Simulator, set Settings, tr *teamResult) {
	// Each of the team's rotations (its own and its alternatives) names
	// the members its own way and may need params of its own.
	cons := map[string]int{}
	for _, c := range tr.Chars {
		cons[library.Canonical(c.Key)] = c.Cons
	}
	rotations := tr.Team.RotationsFor(cons)
	builds := make([]string, len(rotations))
	runs := make([]steady.Rotation, len(rotations))
	for i, t := range rotations {
		var build strings.Builder
		renames := map[string]string{}
		for _, c := range tr.Chars {
			line := c.Config()
			for _, m := range t.Members {
				if library.Canonical(m) == library.Canonical(c.Key) {
					renames[m] = c.Key
					line = config.AddParams(line, c.Key, t.Params[m])
				}
			}
			build.WriteString(line)
			build.WriteString("\n")
		}
		builds[i] = build.String()
		runs[i].Rotation = config.RenameCharacters(t.Rotation, renames)
		for _, m := range t.Members {
			runs[i].Members = append(runs[i].Members, cmp.Or(renames[m], m))
		}
	}

	// The rotation as written runs with the full iterations; most teams
	// stop there. Fillers and alternatives are only ranked, with fewer,
	// and the best of them confirmed with the full iterations.
	best, out, err := steady.Best(runs, func(i int, rotation string, rank bool) (*score.Result, error) {
		iterations := set.Iterations
		if rank {
			iterations = min(iterations, exploreIterations)
		}
		cfg := config.Assemble(config.Conditions{Iterations: iterations, Duration: set.Duration, Enemy: tr.Enemy}, builds[i], rotation)
		res, err := sim.Score(ctx, cfg, set.Warmup, iterations)
		if err != nil && (errors.Is(err, errBusy) || ctx.Err() != nil) {
			return nil, steady.Fatal(err)
		}
		if err != nil && i > 0 {
			slog.Warn("alternative rotation failed", "team", tr.Team.ID, "source", rotations[i].Sources, "err", err)
		}
		return res, err
	})
	var build, rotation, once string
	if err == nil {
		build, rotation = builds[best], runs[best].Rotation
		once = config.Assemble(config.Conditions{Iterations: set.Iterations, Enemy: tr.Enemy}, build, rotation)
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
	if best > 0 {
		tr.Alternative = rotations[best]
		tr.AlternativeNote = alternativeNote(l, tr.Team, tr.Alternative)
	}
	tr.Result = res
	tr.OneRotation = oneRes
	tr.Warnings = append(warnings(l, tr.Team, tr.Chars, res), duplicates(l, tr.Chars)...)
	tr.Rotation = rotation
	if details, err := score.Details(once); err != nil {
		slog.Error("character details failed", "team", tr.Team.ID, "err", err)
	} else if len(details) == len(tr.Chars) {
		for _, d := range details {
			tr.Talents = append(tr.Talents, d.Talents)
			tr.Stats = append(tr.Stats, d.Stats)
		}
	}
	tr.Timelines = cycleViews(ctx, l, sim, set, *tr, once, config.Assemble(config.Conditions{Iterations: 1, Duration: set.Duration, Enemy: tr.Enemy}, build, out.Looped), out.Filled)
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

// warnings are the notes on a team's score. chars is the player's build,
// nil for the library team's own page, which uses the team's build.
func warnings(l localizer, team *library.Team, chars []enka.Character, res *score.Result) []string {
	var out []string
	if team.Status == library.StatusDraft {
		out = append(out, l.T("warn.draft"))
	}
	if team.HasForkOnlyMember() {
		out = append(out, l.T("warn.experimental"))
	}
	if chars == nil && team.HasForkOnlyWeapon() || usesForkOnlyWeapon(chars) {
		out = append(out, l.T("warn.experimental_weapon"))
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

// alternativeNote says that the score uses one of the team's
// alternatives, and the constellations it was written for.
func alternativeNote(l localizer, team, alt *library.Team) string {
	if alt.Variant != nil {
		return l.T("rotation.variant", writtenFor(l, team, alt), l.charName(library.Canonical(alt.Variant.Char)))
	}
	return l.T("rotation.alternative", writtenFor(l, team, alt))
}

// writtenFor lists the constellations an alternative was written for
// that the team's rotation was not, e.g. "C6 Furina, C2 Xilonen"; all of
// its constellations if none is higher.
func writtenFor(l localizer, team, alt *library.Team) string {
	raised := alt.Raised(team)
	if len(raised) == 0 {
		cons := alt.Constellations()
		for _, m := range alt.Members {
			raised = append(raised, library.Cons{Char: library.Canonical(m), Cons: cons[library.Canonical(m)]})
		}
	}
	var parts []string
	for _, c := range raised {
		parts = append(parts, fmt.Sprintf("C%d %s", c.Cons, l.charName(c.Char)))
	}
	return strings.Join(parts, ", ")
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
		case config.FillBrief:
			parts = append(parts, l.T("filler.brief", chars))
		default:
			parts = append(parts, l.T("filler.self", chars))
		}
	}
	if len(o.Skill) > 0 {
		parts = append(parts, l.T("filler.skill", l.charNames(o.Skill)))
	}
	return l.T("filler.wrap", strings.Join(parts, l.T("filler.sep")))
}
