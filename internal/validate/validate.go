// Package validate checks a library team with its reference build: one
// pass of the rotation under the source's own conditions must reproduce
// the DPS its source config reported, and the looped rotation, scored as
// the site does under the common conditions, is checked for action waits
// and members with almost no damage. An experimental team whose rotation
// was written from a guide has no source DPS: its reference comparison is
// skipped, and the looped check runs as for any team.
package validate

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/cenoda/gscouter/internal/config"
	"github.com/cenoda/gscouter/internal/library"
	"github.com/cenoda/gscouter/internal/score"
	"github.com/cenoda/gscouter/internal/steady"
)

// Thresholds. A failure means the team must not be served; a warning
// means it should be looked at.
const (
	// The source ran on another gcsim version, so some difference is
	// expected even under its own conditions.
	failReference = 0.15
	warnReference = 0.05
	warnWait      = 2.0 // seconds per minute an action is delayed, for any reason
	warnShare     = 0.01
	// The looped score moved this much since the recorded validation
	// (team.yaml): a gcsim update or a rotation change to look at.
	warnShift = 0.05
	// A character that stands still this long, in seconds per minute of
	// the scoring window, has stalled the looped rotation (e.g. a burst
	// it can never charge), even with fillers.
	failStall = 50.0
	// A rotation that does not end within this time probably loops
	// forever: with the enemy's infinite HP, gcsim stops only when the
	// action list ends. Settings.Timeout overrides it.
	defaultTimeout = 2 * time.Minute
	// One fight of the looped rotation takes well under a second. One
	// that runs longer is stuck: a loop whose body takes no action (for
	// example a counter set once in the setup) spins without time passing.
	probeTimeout = 20 * time.Second

	// A team that passes the check, reproduces its source this closely
	// and keeps this much of its one pass when looped is served as
	// supported (Report.Unpromotable).
	promoteReference = warnReference
	promoteKept      = 0.8
	// Keeping much more than the one pass means the one pass under the
	// common conditions does not play as written: the rotation depends on
	// the source's own (several targets, targets that die, more energy).
	promoteKeptMax = 1.3

	// The score with the weapons whose mods the rotation reads swapped
	// out (config.SwapModWeapons) keeps less than this of the score.
	// Under failSwapKept the rotation waits for a weapon's passive that
	// never comes: such loops kept 1-20% before config.GuardWeaponMods.
	// Under warnSwapKept it depends on the energy the weapon's particles
	// give, more than the fillers make up for (docs/cycle.md §5).
	failSwapKept = 0.2
	warnSwapKept = 0.5
)

type Settings struct {
	Iterations       int
	Warmup, Duration float64
	Workers          int // simulations run in parallel per check; 0 means one per CPU
	// Timeout is how long one team's check may take before it fails as a
	// rotation that never ends; 0 means 2 minutes. A loaded machine (CI
	// runners checking several teams at once) needs more.
	Timeout time.Duration
	// Cache is a directory where simulation results are kept
	// (score.Cache), "" for none.
	Cache string

	cache *score.Cache
}

// timeout is the check's time limit.
func (s Settings) timeout() time.Duration {
	if s.Timeout > 0 {
		return s.Timeout
	}
	return defaultTimeout
}

// open opens s.Cache. A cache that cannot be used only costs time.
func (s Settings) open() Settings {
	if s.Cache != "" {
		s.cache, _ = score.OpenCache(s.Cache)
	}
	return s
}

type Report struct {
	Dir  string
	Team *library.Team
	Once *score.Result // one pass under the source's conditions, compared with the source
	// Common is one pass under the common conditions, comparable with
	// the score; nil after CheckReference.
	Common *score.Result
	// SourceConditions is false when the team has no source.txt and the
	// one pass ran under the common conditions.
	SourceConditions bool
	Res              *score.Result // looped, the score; nil after CheckReference
	Cfg              string        // the looped config of the score
	Filler           string        // how the score fills waits, "" if it doesn't
	Skipped          string
	// ReferenceSkipped says why the one pass was not compared with a
	// source DPS, for a rotation written from a guide (design §5.4).
	ReferenceSkipped string
	// SwapKept is the score with the weapons whose mods the rotation
	// reads swapped out, over the score; 0 when it reads no weapon mod.
	SwapKept float64 `json:",omitempty"`
	Failures []string
	Warnings []string
}

// Line is one line of cmd/validate -json: a report and how the run
// that made it was built, for cmd/compare.
type Line struct {
	*Report
	Gcsim string // the gcsim module the run was built with, path@version
	// Cached is true when results could come from the cache
	// (Settings.Cache), so a repeated run repeats the first.
	Cached bool
}

// Kept is the score over one pass under the same, common conditions:
// how much of its one-pass DPS the rotation keeps when it loops. It is 0
// when the full check did not get that far.
func (r *Report) Kept() float64 {
	if r.Res == nil || r.Common == nil || r.Common.DPS <= 0 {
		return 0
	}
	return r.Res.DPS / r.Common.DPS
}

// Gap is how far the one pass under the source's conditions is from the
// source's DPS, as a fraction of it; ok is false without a reference.
func (r *Report) Gap() (gap float64, ok bool) {
	if r.Team == nil || r.Team.Reference == nil || r.Team.Reference.DPS <= 0 || r.Once == nil {
		return 0, false
	}
	return (r.Once.DPS - r.Team.Reference.DPS) / r.Team.Reference.DPS, true
}

// Validation is the record of this check for team.yaml, run with the
// given iterations today. The date is in UTC, as the database's update dates
// are, so it does not depend on the machine's time zone.
func (r *Report) Validation(iterations int) *library.Validation {
	v := &library.Validation{
		Date:       time.Now().UTC().Format(time.DateOnly),
		Iterations: iterations,
		Kept:       round(r.Kept(), 3),
		Draft:      r.Unpromotable(),
	}
	if gap, ok := r.Gap(); ok {
		gap = round(gap, 3)
		v.Gap = &gap
	}
	if r.Res != nil {
		v.DPS = int(math.Round(r.Res.DPS))
	}
	return v
}

func round(x float64, digits int) float64 {
	p := math.Pow(10, float64(digits))
	x = math.Round(x*p) / p
	if x == 0 {
		return 0 // not -0
	}
	return x
}

// Unpromotable says why the team does not meet the criteria for
// status: supported, or "" if it does: the full check passes, the one
// pass is within promoteReference of the source's DPS (a variant has
// none), and the looped score keeps between promoteKept and
// promoteKeptMax of the one pass.
func (r *Report) Unpromotable() string {
	switch {
	case len(r.Failures) > 0:
		return "the check fails: " + r.Failures[0]
	case r.Skipped != "":
		return "skipped"
	case r.Res == nil:
		return "no looped score"
	}
	gap, ok := r.Gap()
	switch {
	case !ok && r.Team.Variant == nil:
		return "no reference DPS"
	case math.Abs(gap) > promoteReference:
		return fmt.Sprintf("%+.1f%% from the source's DPS, more than %.0f%%", gap*100, promoteReference*100)
	case r.Kept() < promoteKept:
		return fmt.Sprintf("the looped score keeps %.0f%% of one pass, less than %.0f%%", r.Kept()*100, promoteKept*100)
	case r.Kept() > promoteKeptMax:
		return fmt.Sprintf("the looped score keeps %.0f%% of one pass, more than %.0f%%: the rotation depends on the source's conditions", r.Kept()*100, promoteKeptMax*100)
	}
	return ""
}

// CheckReference runs one pass of the team's rotation in dir with its
// reference build under the source's own conditions (source.txt) and
// compares the DPS with the source's. It is the check cmd/harvest uses
// to accept a team.
func CheckReference(dir string, s Settings) *Report {
	s = s.open()
	return withTimeout(dir, s.timeout(), func(ctx context.Context) *Report {
		r, _ := reference(ctx, dir, s)
		return r
	})
}

// Check validates the team in dir: CheckReference, then the looped
// rotation scored as the site does, under the common conditions. A
// check that runs longer than the timeout fails.
func Check(dir string, s Settings) *Report {
	s = s.open()
	return withTimeout(dir, s.timeout(), func(ctx context.Context) *Report {
		r, build := reference(ctx, dir, s)
		if build != "" {
			looped(ctx, r, build, s)
		}
		return r
	})
}

// withTimeout runs check with a context that ends after the timeout. The
// simulation stops at its next iteration; one that never ends keeps
// running until the process exits.
func withTimeout(dir string, timeout time.Duration, check func(context.Context) *Report) *Report {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	done := make(chan *Report, 1)
	go func() { done <- check(ctx) }()
	select {
	case r := <-done:
		return r
	case <-ctx.Done():
		return &Report{Dir: dir, Failures: []string{fmt.Sprintf("no result after %v: the rotation probably never ends (an unconditional loop?)", timeout)}}
	}
}

// reference runs the one pass. It returns the build with the team's
// params when the looped check can follow, "" otherwise.
func reference(ctx context.Context, dir string, s Settings) (*Report, string) {
	r := &Report{Dir: dir}
	team, err := library.Load(dir)
	if err != nil {
		r.Failures = append(r.Failures, err.Error())
		return r, ""
	}
	r.Team = team
	// A team's alternative is in <team>/alt/<entry>.
	if want, _, _ := strings.Cut(library.DirName(dir), "/"); team.ID != want {
		r.Failures = append(r.Failures, fmt.Sprintf("id %q does not match directory", team.ID))
	}
	if team.Status == library.StatusBlocked {
		r.Skipped = "blocked: " + team.Notes
		return r, ""
	}
	if len(team.Sources) == 0 {
		r.Failures = append(r.Failures, "no source: rotations must come from a community config")
	}

	raw, err := os.ReadFile(filepath.Join(dir, "build.txt"))
	if err != nil {
		r.Failures = append(r.Failures, "reference build: "+err.Error())
		return r, ""
	}
	build := string(raw)
	for _, m := range slices.Sorted(maps.Keys(team.Params)) {
		build = config.AddParams(build, m, team.Params[m])
	}
	var once string
	switch conditions, err := os.ReadFile(filepath.Join(dir, "source.txt")); {
	case err == nil:
		r.SourceConditions = true
		once = config.AssembleSource(string(conditions), s.Iterations, build, team.Rotation)
	case errors.Is(err, fs.ErrNotExist):
		once = config.Assemble(config.Conditions{Iterations: s.Iterations}, build, team.Rotation)
	default:
		r.Failures = append(r.Failures, "source conditions: "+err.Error())
		return r, ""
	}
	chars, err := config.Characters(once)
	if err != nil {
		r.Failures = append(r.Failures, "config: "+err.Error())
		return r, ""
	}
	if !team.Matches(chars) {
		r.Failures = append(r.Failures, fmt.Sprintf("build characters %v do not match members %v", chars, team.Members))
		return r, ""
	}

	onceRes, err := s.cache.Run(ctx, once, score.Options{Iterations: s.Iterations, Workers: s.Workers, Seed: 1})
	if err != nil {
		r.Failures = append(r.Failures, "simulation: "+err.Error())
		return r, ""
	}
	r.Once = onceRes
	if ref := team.Reference; ref != nil && ref.DPS > 0 {
		diff := (onceRes.DPS - ref.DPS) / ref.DPS
		msg := fmt.Sprintf("%+.1f%% from the source's %.0f DPS", diff*100, ref.DPS)
		switch d := math.Abs(diff); {
		case d > failReference:
			r.Failures = append(r.Failures, msg)
		case d > warnReference:
			r.Warnings = append(r.Warnings, msg)
		}
	} else if team.Written() {
		// No community config, so no source DPS: the looped check
		// still runs, and a public number, if any, is compared instead.
		r.ReferenceSkipped = "rotation written from a guide, no source DPS"
		switch cc := team.CrossCheck; {
		case cc == nil:
			r.Warnings = append(r.Warnings, "no cross_check: record a public calculation or showcase DPS, or what was searched")
		case cc.DPS > 0:
			diff := (onceRes.DPS - cc.DPS) / cc.DPS
			if math.Abs(diff) > failReference {
				r.Warnings = append(r.Warnings, fmt.Sprintf("%+.1f%% from the cross-check's %.0f DPS", diff*100, cc.DPS))
			}
		}
	} else if team.Variant != nil {
		// Derived from the team's rotation, so no config scored it;
		// cmd/variant measured it against the rotation it changes.
		r.ReferenceSkipped = "variant of the team's rotation, no source DPS"
	} else {
		r.Warnings = append(r.Warnings, "no reference DPS to compare with")
	}
	return r, build
}

// looped scores the looped rotation under the common conditions and
// warns about action waits and members with almost no damage.
func looped(ctx context.Context, r *Report, build string, s Settings) {
	team := r.Team
	r.Common = r.Once
	if r.SourceConditions {
		once := config.Assemble(config.Conditions{Iterations: s.Iterations}, build, team.Rotation)
		res, err := s.cache.Run(ctx, once, score.Options{Iterations: s.Iterations, Workers: s.Workers, Seed: 1})
		if err != nil {
			r.Failures = append(r.Failures, "simulation under the common conditions: "+err.Error())
			return
		}
		r.Common = res
	}
	rot, probed, msg := probe(ctx, build, team.Rotation, team.Members, s)
	if msg != "" {
		r.Failures = append(r.Failures, msg)
		return
	}
	out, err := steady.Score(team.Rotation, team.Members, func(looped string) (*score.Result, error) {
		cfg := config.Assemble(config.Conditions{Iterations: s.Iterations, Duration: s.Duration}, build, looped)
		opts := score.Options{Iterations: s.Iterations, Workers: s.Workers, Warmup: s.Warmup, Seed: 1}
		var res *score.Result
		var err error
		if looped == rot && probed != nil {
			// The probe was the first iteration of this run: the
			// config differs only in the iteration count, which gcsim
			// does not read.
			res, err = rest(ctx, s.cache, cfg, opts, probed)
			probed = nil
		} else {
			res, err = s.cache.Run(ctx, cfg, opts)
		}
		if err == nil {
			// The last run is not necessarily the best; keep the best's config.
			if r.Res == nil || res.DPS > r.Res.DPS {
				r.Res, r.Cfg = res, cfg
			}
		}
		return res, err
	})
	if err != nil {
		r.Failures = append(r.Failures, "looped simulation: "+err.Error())
		return
	}
	res := out.Result
	r.Filler = out.String()
	r.Res = res
	if res.DPS <= 0 {
		r.Failures = append(r.Failures, "the looped rotation deals no damage in the scoring window: it stalls")
	}
	for _, c := range res.Characters {
		for _, reason := range slices.Sorted(maps.Keys(c.Waits)) {
			if w := c.Waits[reason]; w >= failStall {
				r.Failures = append(r.Failures, fmt.Sprintf("the looped rotation stalls: %s waits %.1fs/min (%s)", c.Name, w, reason))
			}
		}
	}
	swapWeapons(ctx, r, build, s)
	if last := team.Validation; last != nil && last.DPS > 0 && res.DPS > 0 {
		if shift := res.DPS/float64(last.DPS) - 1; math.Abs(shift) > warnShift {
			r.Warnings = append(r.Warnings, fmt.Sprintf("the looped score moved %+.1f%% from %d DPS on %s", shift*100, last.DPS, last.Date))
		}
	}
	for _, c := range res.Characters {
		switch {
		case c.DPS <= 0:
			r.Warnings = append(r.Warnings, c.Name+" deals no damage")
		case c.Share < warnShare:
			r.Warnings = append(r.Warnings, fmt.Sprintf("%s deals only %.1f%% of the damage", c.Name, c.Share*100))
		}
		for _, reason := range slices.Sorted(maps.Keys(c.Waits)) {
			if w := c.Waits[reason]; w >= warnWait {
				r.Warnings = append(r.Warnings, fmt.Sprintf("%s waits %.1fs/min (%s)", c.Name, w, reason))
			}
		}
	}
}

// swapWeapons scores the team as the site does, with the weapons whose
// mods its rotation reads swapped out, and fails a rotation that
// collapses without them.
func swapWeapons(ctx context.Context, r *Report, build string, s Settings) {
	swapped, chars := config.SwapModWeapons(build, r.Team.Rotation)
	if len(chars) == 0 || r.Res == nil || r.Res.DPS <= 0 {
		return
	}
	out, err := steady.Score(r.Team.Rotation, r.Team.Members, func(looped string) (*score.Result, error) {
		cfg := config.Assemble(config.Conditions{Iterations: s.Iterations, Duration: s.Duration}, swapped, looped)
		return s.cache.Run(ctx, cfg, score.Options{Iterations: s.Iterations, Workers: s.Workers, Warmup: s.Warmup, Seed: 1})
	})
	if err != nil {
		r.Failures = append(r.Failures, "looped simulation with weapons swapped: "+err.Error())
		return
	}
	r.SwapKept = out.Result.DPS / r.Res.DPS
	msg := fmt.Sprintf("with %s on other weapons the score keeps %.0f%%: the rotation depends on the reference build's weapons", strings.Join(chars, ", "), r.SwapKept*100)
	switch {
	case r.SwapKept < failSwapKept:
		r.Failures = append(r.Failures, msg)
	case r.SwapKept < warnSwapKept:
		r.Warnings = append(r.Warnings, msg)
	}
}

// probe runs one fight of the looped rotation and says why it failed,
// or "" if it finished. It returns the looped rotation and the fight,
// which is the first iteration of the looped score (seed 1). A stuck
// fight keeps running until the process exits; cmd/validate checks each
// team in its own process.
func probe(ctx context.Context, build, rotation string, members []string, s Settings) (string, []score.Iteration, string) {
	rot, err := steady.Looped(rotation, members)
	if err != nil {
		return "", nil, "looped simulation: " + err.Error()
	}
	cfg := config.Assemble(config.Conditions{Iterations: 1, Duration: s.Duration}, build, rot)
	type result struct {
		its []score.Iteration
		err error
	}
	done := make(chan result, 1)
	go func() {
		its, err := s.cache.RunIterations(ctx, cfg, score.Options{Iterations: 1, Workers: 1, Warmup: s.Warmup, Seed: 1})
		done <- result{its, err}
	}()
	select {
	case res := <-done:
		if res.err != nil {
			return "", nil, "looped simulation: " + res.err.Error()
		}
		return rot, res.its, ""
	case <-time.After(probeTimeout):
		return "", nil, fmt.Sprintf("the looped rotation does not advance: one fight did not end within %v (a loop that takes no action, such as a counter set once before it?)", probeTimeout)
	}
}

// rest scores cfg's opts.Iterations iterations, of which the first are
// done: it runs the others and scores them all as score.Run would.
func rest(ctx context.Context, cache *score.Cache, cfg string, opts score.Options, done []score.Iteration) (*score.Result, error) {
	its := done[:min(len(done), opts.Iterations)]
	if n := opts.Iterations - len(its); n > 0 {
		opts.First, opts.Iterations = len(its), n
		more, err := cache.RunIterations(ctx, cfg, opts)
		if err != nil {
			return nil, err
		}
		its = append(slices.Clip(its), more...)
	}
	return score.Aggregate(its, opts.Warmup), nil
}
