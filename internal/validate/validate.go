// Package validate checks a library team with its reference build: one
// pass of the rotation must reproduce the DPS its source config reported,
// and the looped rotation, scored as the site does, is checked for action
// waits and members with almost no damage.
package validate

import (
	"context"
	"fmt"
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
	// The source ran on another gcsim version and its own options, so
	// small differences are expected.
	failReference = 0.05
	warnReference = 0.02
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
}

type Report struct {
	Dir      string
	Team     *library.Team
	Once     *score.Result // one pass, compared with the source
	Res      *score.Result // looped, the score
	Cfg      string        // the looped config of the score
	Filler   string        // how the score fills waits, "" if it doesn't
	Skipped  string
	Failures []string
	Warnings []string
}

// Check validates the team in dir. A check that runs longer than the
// timeout fails; its simulation keeps running until the process exits.
func Check(dir string, s Settings) *Report {
	done := make(chan *Report, 1)
	go func() { done <- check(dir, s) }()
	select {
	case r := <-done:
		return r
	case <-time.After(timeout):
		return &Report{Dir: dir, Failures: []string{fmt.Sprintf("no result after %v: the rotation probably never ends (an unconditional loop?)", timeout)}}
	}
}

func check(dir string, s Settings) *Report {
	r := &Report{Dir: dir}
	team, err := library.Load(dir)
	if err != nil {
		r.Failures = append(r.Failures, err.Error())
		return r
	}
	r.Team = team
	if team.ID != filepath.Base(dir) {
		r.Failures = append(r.Failures, fmt.Sprintf("id %q does not match directory", team.ID))
	}
	if team.Status == library.StatusBlocked {
		r.Skipped = "blocked: " + team.Notes
		return r
	}
	if len(team.Sources) == 0 {
		r.Failures = append(r.Failures, "no source: rotations must come from a community config")
	}

	raw, err := os.ReadFile(filepath.Join(dir, "build.txt"))
	if err != nil {
		r.Failures = append(r.Failures, "reference build: "+err.Error())
		return r
	}
	build := string(raw)
	for _, m := range slices.Sorted(maps.Keys(team.Params)) {
		build = config.AddParams(build, m, team.Params[m])
	}
	once := config.Assemble(config.Conditions{Iterations: s.Iterations}, build, team.Rotation)
	chars, err := config.Characters(once)
	if err != nil {
		r.Failures = append(r.Failures, "config: "+err.Error())
		return r
	}
	if !team.Matches(chars) {
		r.Failures = append(r.Failures, fmt.Sprintf("build characters %v do not match members %v", chars, team.Members))
		return r
	}

	onceRes, err := score.Run(context.Background(), once, score.Options{Iterations: s.Iterations, Seed: 1})
	if err != nil {
		r.Failures = append(r.Failures, "simulation: "+err.Error())
		return r
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

	out, err := steady.Score(team.Rotation, team.Members, func(looped string) (*score.Result, error) {
		cfg := config.Assemble(config.Conditions{Iterations: s.Iterations, Duration: s.Duration}, build, looped)
		res, err := score.Run(context.Background(), cfg, score.Options{Iterations: s.Iterations, Warmup: s.Warmup, Seed: 1})
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
		return r
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
	return r
}
