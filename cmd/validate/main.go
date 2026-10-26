// Command validate runs every library team with its reference build. It
// checks that one pass of the rotation under the source's own conditions
// (source.txt) reproduces the DPS its source config reported, then scores
// the looped rotation as the site does, under the common conditions, and
// reports energy waits and members with almost no damage.
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
	"flag"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cenoda/gscouter/internal/score"
	"github.com/cenoda/gscouter/internal/validate"
)

func main() {
	var (
		root       = flag.String("root", "teams", "team library directory, used when no teams are given")
		iterations = flag.Int("iter", 100, "number of iterations")
		warmup     = flag.Float64("warmup", 150, "seconds of the looped fight excluded from the score")
		duration   = flag.Float64("duration", 450, "length of the looped fight in seconds")
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

	s := validate.Settings{Iterations: *iterations, Warmup: *warmup, Duration: *duration}
	var failed, warned int
	for _, dir := range dirs {
		r := validate.Check(dir, s)
		switch {
		case len(r.Failures) > 0:
			failed++
		case len(r.Warnings) > 0:
			warned++
		}
		printReport(r, *verbose)
		if *detail && r.Cfg != "" {
			printDetail(r.Cfg)
		}
	}
	fmt.Printf("\n%d teams, %d failed, %d with warnings\n", len(dirs), failed, warned)
	if failed > 0 {
		os.Exit(1)
	}
}

func printReport(r *validate.Report, verbose bool) {
	verdict := "OK"
	switch {
	case len(r.Failures) > 0:
		verdict = "FAIL"
	case r.Skipped != "":
		verdict = "SKIP"
	case len(r.Warnings) > 0:
		verdict = "WARN"
	}
	name := filepath.Base(r.Dir)
	if r.Res != nil {
		ref := "      -"
		if r.Team.Reference != nil {
			ref = fmt.Sprintf("%7.0f", r.Team.Reference.DPS)
		}
		fmt.Printf("%-4s %-46s looped %7.0f  one rotation %7.0f (%4.1fs, source %s)\n", verdict, name, r.Res.DPS, r.Once.DPS, r.Once.Window, ref)
	} else {
		fmt.Printf("%-4s %s\n", verdict, name)
	}
	if r.Filler != "" {
		fmt.Printf("       filled: %s\n", r.Filler)
	}
	if r.Skipped != "" {
		fmt.Printf("       %s\n", r.Skipped)
	}
	for _, msg := range r.Failures {
		fmt.Printf("       fail: %s\n", msg)
	}
	for _, msg := range r.Warnings {
		fmt.Printf("       warn: %s\n", msg)
	}
	if verbose && r.Res != nil {
		for _, c := range r.Res.Characters {
			var waits []string
			for _, reason := range slices.Sorted(maps.Keys(c.Waits)) {
				waits = append(waits, fmt.Sprintf("%s %.2fs/min", reason, c.Waits[reason]))
			}
			fmt.Printf("       %-18s %8.0f DPS %6.1f%%  %s\n", c.Name, c.DPS, c.Share*100, strings.Join(waits, ", "))
		}
	}
}

// printDetail runs the looped config once and prints, per character, the
// actions taken, damage by source and reactions triggered over the whole
// fight.
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
