// Command validate runs every library team with its reference build and
// checks the rotation: the config parses, the rotation finishes, every
// member deals damage, and the team reproduces the DPS its source config
// reported.
//
// Usage:
//
//	validate                      # every team under teams/
//	validate teams/<id> ...       # selected teams
//
// It exits with status 1 if any team fails.
package main

import (
	"cmp"
	"context"
	"flag"
	"fmt"
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

func main() {
	var (
		root       = flag.String("root", "teams", "team library directory, used when no teams are given")
		iterations = flag.Int("iter", 100, "number of iterations")
		verbose    = flag.Bool("v", false, "print per-character results")
		detail     = flag.Bool("detail", false, "print what each character did in one run: actions, damage by source, reactions")
	)
	flag.Parse()

	dirs := flag.Args()
	if len(dirs) == 0 {
		entries, err := os.ReadDir(*root)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(2)
		}
		for _, e := range entries {
			if e.IsDir() {
				dirs = append(dirs, filepath.Join(*root, e.Name()))
			}
		}
	}

	opts := score.Options{Iterations: *iterations, Seed: 1}
	cond := config.Conditions{Iterations: *iterations}
	var failed, warned int
	for _, dir := range dirs {
		r := checkWithTimeout(dir, cond, opts)
		switch {
		case len(r.failures) > 0:
			failed++
		case len(r.warnings) > 0:
			warned++
		}
		r.print(*verbose)
		if *detail && r.cfg != "" {
			printDetail(r.cfg)
		}
	}
	fmt.Printf("\n%d teams, %d failed, %d with warnings\n", len(dirs), failed, warned)
	if failed > 0 {
		os.Exit(1)
	}
}

type report struct {
	dir      string
	team     *library.Team
	res      *score.Result
	cfg      string
	skipped  string
	failures []string
	warnings []string
}

func checkWithTimeout(dir string, cond config.Conditions, opts score.Options) *report {
	done := make(chan *report, 1)
	go func() { done <- check(dir, cond, opts) }()
	select {
	case r := <-done:
		return r
	case <-time.After(timeout):
		// The simulation cannot be interrupted; the process exits at the end.
		return &report{dir: dir, failures: []string{fmt.Sprintf("no result after %v: the rotation probably never ends (an unconditional loop?)", timeout)}}
	}
}

func check(dir string, cond config.Conditions, opts score.Options) *report {
	r := &report{dir: dir}
	team, err := library.Load(dir)
	if err != nil {
		r.failures = append(r.failures, err.Error())
		return r
	}
	r.team = team
	if team.ID != filepath.Base(dir) {
		r.failures = append(r.failures, fmt.Sprintf("id %q does not match directory", team.ID))
	}
	if team.Status == library.StatusBlocked {
		r.skipped = "blocked: " + team.Notes
		return r
	}
	if len(team.Sources) == 0 {
		r.failures = append(r.failures, "no source: rotations must come from a community config")
	}

	raw, err := os.ReadFile(filepath.Join(dir, "build.txt"))
	if err != nil {
		r.failures = append(r.failures, "reference build: "+err.Error())
		return r
	}
	build := string(raw)
	for _, m := range slices.Sorted(maps.Keys(team.Params)) {
		build = config.AddParams(build, m, team.Params[m])
	}
	cfg := config.Assemble(cond, build, team.Rotation)
	chars, err := config.Characters(cfg)
	if err != nil {
		r.failures = append(r.failures, "config: "+err.Error())
		return r
	}
	if !team.Matches(chars) {
		r.failures = append(r.failures, fmt.Sprintf("build characters %v do not match members %v", chars, team.Members))
		return r
	}

	r.cfg = cfg
	res, err := score.Run(context.Background(), cfg, opts)
	if err != nil {
		r.failures = append(r.failures, "simulation: "+err.Error())
		return r
	}
	r.res = res

	if ref := team.Reference; ref != nil && ref.DPS > 0 {
		diff := (res.DPS - ref.DPS) / ref.DPS
		msg := fmt.Sprintf("%+.1f%% from the source's %.0f DPS (gcsim %s)", diff*100, ref.DPS, ref.GcsimVersion)
		switch d := math.Abs(diff); {
		case d > failReference:
			r.failures = append(r.failures, msg)
		case d > warnReference:
			r.warnings = append(r.warnings, msg)
		}
	} else {
		r.warnings = append(r.warnings, "no reference DPS to compare with")
	}
	for _, c := range res.Characters {
		switch {
		case c.DPS <= 0:
			r.warnings = append(r.warnings, c.Name+" deals no damage")
		case c.Share < warnShare:
			r.warnings = append(r.warnings, fmt.Sprintf("%s deals only %.1f%% of the damage", c.Name, c.Share*100))
		}
		for _, reason := range slices.Sorted(maps.Keys(c.Waits)) {
			if w := c.Waits[reason]; w >= warnWait {
				r.warnings = append(r.warnings, fmt.Sprintf("%s waits %.1fs/min (%s)", c.Name, w, reason))
			}
		}
	}
	return r
}

func (r *report) print(verbose bool) {
	verdict := "OK"
	switch {
	case len(r.failures) > 0:
		verdict = "FAIL"
	case r.skipped != "":
		verdict = "SKIP"
	case len(r.warnings) > 0:
		verdict = "WARN"
	}
	name := filepath.Base(r.dir)
	if r.res != nil {
		ref := "        -"
		if r.team.Reference != nil {
			ref = fmt.Sprintf("%9.0f", r.team.Reference.DPS)
		}
		fmt.Printf("%-4s %-50s %8.0f DPS  source %s  rotation %4.1fs\n", verdict, name, r.res.DPS, ref, r.res.Window)
	} else {
		fmt.Printf("%-4s %s\n", verdict, name)
	}
	if r.skipped != "" {
		fmt.Printf("       %s\n", r.skipped)
	}
	for _, msg := range r.failures {
		fmt.Printf("       fail: %s\n", msg)
	}
	for _, msg := range r.warnings {
		fmt.Printf("       warn: %s\n", msg)
	}
	if verbose && r.res != nil {
		for _, c := range r.res.Characters {
			var waits []string
			for _, reason := range slices.Sorted(maps.Keys(c.Waits)) {
				waits = append(waits, fmt.Sprintf("%s %.2fs/min", reason, c.Waits[reason]))
			}
			fmt.Printf("       %-18s %8.0f DPS %6.1f%%  %s\n", c.Name, c.DPS, c.Share*100, strings.Join(waits, ", "))
		}
	}
}

// printDetail runs the config once and prints, per character, the actions
// taken, damage by source and reactions triggered.
func printDetail(cfg string) {
	res, err := score.Once(cfg, 1)
	if err != nil {
		fmt.Printf("       detail: %v\n", err)
		return
	}
	list := func(m map[string]float64, format string) string {
		keys := slices.SortedFunc(maps.Keys(m), func(a, b string) int { return cmp.Compare(m[b], m[a]) })
		var parts []string
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s "+format, k, m[k]))
		}
		return strings.Join(parts, ", ")
	}
	fmt.Printf("       one run, %.1fs:\n", float64(res.Duration)/60)
	for _, c := range res.Characters {
		actions := map[string]float64{}
		for _, e := range c.ActionEvents {
			actions[e.Action]++
		}
		damage := map[string]float64{}
		for _, e := range c.DamageEvents {
			damage[e.Source] += e.Damage
		}
		reactions := map[string]float64{}
		for _, e := range c.ReactionEvents {
			reactions[e.Reaction]++
		}
		fmt.Printf("       %s\n", c.Name)
		fmt.Printf("         actions:   %s\n", list(actions, "%.0f"))
		fmt.Printf("         damage:    %s\n", list(damage, "%.0f"))
		if len(reactions) > 0 {
			fmt.Printf("         reactions: %s\n", list(reactions, "%.0f"))
		}
	}
}
