// Command validate runs every library team with its reference build and
// checks that the rotation works: the config parses, every member deals
// damage, the team settles into a steady state and no one waits long for
// energy.
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
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cenoda/gscouter/internal/config"
	"github.com/cenoda/gscouter/internal/library"
	"github.com/cenoda/gscouter/internal/score"
)

// Thresholds. A failure means the rotation is broken; a warning means it
// runs but should be looked at.
const (
	// The fight starts with full energy, so the second half is normally a
	// little weaker than the first. Much weaker means the rotation stalls.
	failDrift = -0.5
	warnWait  = 2.0  // seconds per minute an action is delayed, for any reason
	failWait  = 10.0 // seconds per minute a character's actions are delayed in total
	warnShare = 0.01
)

func main() {
	var (
		root       = flag.String("root", "teams", "team library directory, used when no teams are given")
		iterations = flag.Int("iter", 100, "number of iterations")
		warmup     = flag.Float64("warmup", 0, "seconds excluded from the score")
		duration   = flag.Float64("duration", 90, "fight duration in seconds")
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

	opts := score.Options{Iterations: *iterations, Warmup: *warmup, Seed: 1}
	cond := config.Conditions{Duration: *duration, Iterations: *iterations}
	var failed, warned int
	for _, dir := range dirs {
		r := check(dir, cond, opts)
		switch {
		case len(r.failures) > 0:
			failed++
		case len(r.warnings) > 0:
			warned++
		}
		r.print(*verbose)
		if *detail && r.cfg != "" {
			printDetail(r.cfg, *warmup)
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

	build, err := os.ReadFile(filepath.Join(dir, "build.txt"))
	if err != nil {
		r.failures = append(r.failures, "reference build: "+err.Error())
		return r
	}
	cfg := config.Assemble(cond, string(build), team.Rotation)
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

	if res.Drift < failDrift {
		r.failures = append(r.failures, fmt.Sprintf("drift %+.1f%%: the rotation stalls in the second half", res.Drift*100))
	}
	for _, c := range res.Characters {
		switch {
		case c.DPS <= 0:
			r.failures = append(r.failures, c.Name+" deals no damage")
		case c.Share < warnShare:
			r.warnings = append(r.warnings, fmt.Sprintf("%s deals only %.1f%% of the damage", c.Name, c.Share*100))
		}
		var total float64
		for _, w := range c.Waits {
			total += w
		}
		if total >= failWait {
			r.failures = append(r.failures, fmt.Sprintf("%s waits %.1fs/min in total: the rotation stands idle", c.Name, total))
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
		fmt.Printf("%-4s %-50s %8.0f DPS  drift %+5.1f%%\n", verdict, name, r.res.DPS, r.res.Drift*100)
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
// taken, damage by source and reactions triggered, all per minute of the
// scoring window.
func printDetail(cfg string, warmup float64) {
	res, err := score.Once(cfg, 1)
	if err != nil {
		fmt.Printf("       detail: %v\n", err)
		return
	}
	start := int(warmup * 60)
	minutes := float64(res.Duration-start) / 3600
	perMin := func(m map[string]float64) string {
		keys := slices.SortedFunc(maps.Keys(m), func(a, b string) int { return cmp.Compare(m[b], m[a]) })
		var parts []string
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s %.1f", k, m[k]/minutes))
		}
		return strings.Join(parts, ", ")
	}
	fmt.Printf("       one run, per minute from %.0fs:\n", warmup)
	for _, c := range res.Characters {
		actions := map[string]float64{}
		for _, e := range c.ActionEvents {
			if e.Frame >= start {
				actions[e.Action]++
			}
		}
		damage := map[string]float64{}
		for _, e := range c.DamageEvents {
			if e.Frame >= start {
				damage[e.Source] += e.Damage
			}
		}
		reactions := map[string]float64{}
		for _, e := range c.ReactionEvents {
			if e.Frame >= start {
				reactions[e.Reaction]++
			}
		}
		fmt.Printf("       %s\n", c.Name)
		fmt.Printf("         actions:   %s\n", perMin(actions))
		fmt.Printf("         damage:    %s\n", perMin(damage))
		if len(reactions) > 0 {
			fmt.Printf("         reactions: %s\n", perMin(reactions))
		}
	}
}
