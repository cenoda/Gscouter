// Package validate checks a library team with its reference build: one
// pass of the rotation under the source's own conditions must reproduce
// the DPS its source config reported, and the looped rotation, scored as
// the site does under the common conditions, is checked for action waits
// and members with almost no damage.
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
	// A rotation that does not end within this time probably loops
	// forever: with the enemy's infinite HP, gcsim stops only when the
	// action list ends.
	timeout = 2 * time.Minute
)

type Settings struct {
	Iterations       int
	Warmup, Duration float64
	Workers          int // simulations run in parallel per check; 0 means one per CPU
}

type Report struct {
	Dir  string
	Team *library.Team
	Once *score.Result // one pass under the source's conditions, compared with the source
	// SourceConditions is false when the team has no source.txt and the
	// one pass ran under the common conditions.
	SourceConditions bool
	Res              *score.Result // looped, the score; nil after CheckReference
	Cfg              string        // the looped config of the score
	Filler           string        // how the score fills waits, "" if it doesn't
	Skipped          string
	Failures         []string
	Warnings         []string
}

// CheckReference runs one pass of the team's rotation in dir with its
// reference build under the source's own conditions (source.txt) and
// compares the DPS with the source's. It is the check cmd/harvest uses
// to accept a team.
func CheckReference(dir string, s Settings) *Report {
	return withTimeout(dir, func(ctx context.Context) *Report {
		r, _ := reference(ctx, dir, s)
		return r
	})
}

// Check validates the team in dir: CheckReference, then the looped
// rotation scored as the site does, under the common conditions. A
// check that runs longer than the timeout fails.
func Check(dir string, s Settings) *Report {
	return withTimeout(dir, func(ctx context.Context) *Report {
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
func withTimeout(dir string, check func(context.Context) *Report) *Report {
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
	if team.ID != filepath.Base(dir) {
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

	onceRes, err := score.Run(ctx, once, score.Options{Iterations: s.Iterations, Workers: s.Workers, Seed: 1})
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
	} else {
		r.Warnings = append(r.Warnings, "no reference DPS to compare with")
	}
	return r, build
}

// looped scores the looped rotation under the common conditions and
// warns about action waits and members with almost no damage.
func looped(ctx context.Context, r *Report, build string, s Settings) {
	team := r.Team
	out, err := steady.Score(team.Rotation, team.Members, func(looped string) (*score.Result, error) {
		cfg := config.Assemble(config.Conditions{Iterations: s.Iterations, Duration: s.Duration}, build, looped)
		res, err := score.Run(ctx, cfg, score.Options{Iterations: s.Iterations, Workers: s.Workers, Warmup: s.Warmup, Seed: 1})
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
