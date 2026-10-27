// Command validate runs every library team with its reference build. It
// checks that one pass of the rotation under the source's own conditions
// (source.txt) reproduces the DPS its source config reported, then scores
// the looped rotation as the site does, under the common conditions, and
// reports energy waits and members with almost no damage. A rotation
// written from a guide (an experimental team without a source DPS) skips
// the comparison and gets the looped check.
//
// Usage:
//
//	validate                      # every team under teams/
//	validate teams/<id> ...       # selected teams
//	validate -promote             # also record each result and the status it earns
//	validate -shard 2/4 -json b.jsonl  # the second of four slices, reports as JSON
//
// Each team is checked in its own process (-j at a time), so a rotation
// that never ends cannot slow down the others. It exits with status 1 if
// any team fails.
//
// With -promote, it records each result in the team's team.yaml
// (validation: the looped score, how much of one pass it keeps and how
// far one pass is from the source) and sets status: supported on teams
// that meet the criteria for it (validate.Report.Unpromotable), draft on
// the others. It never promotes an experimental team, which a person
// reviews first. Blocked teams are left alone. A looped score that moved
// more than 5% from the recorded one is a warning.
//
// Simulation results are kept in -cache, by default under the user's
// cache directory, and reused when the same config runs again with the
// same gcsim (score.Cache): a rerun simulates only the teams that
// changed. Seeds are fixed, so a cached result is the one a new run
// would give; -cache "" turns it off, as when looking for
// nondeterminism (design §5.4).
//
// -shard k/n checks every n-th team from the k-th on, so one run can be
// split across machines or sessions; -json writes each report as one
// JSON line, in team order, for cmd/compare to compare runs (the
// regression run, design §5.4) or join shards.
package main

import (
	"cmp"
	"encoding/json"
	"flag"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cenoda/gscouter/internal/library"
	"github.com/cenoda/gscouter/internal/score"
	"github.com/cenoda/gscouter/internal/share"
	"github.com/cenoda/gscouter/internal/validate"
)

func main() {
	validate.ServeChild()
	var (
		root       = flag.String("root", "teams", "team library directory, used when no teams are given")
		iterations = flag.Int("iter", 100, "number of iterations")
		warmup     = flag.Float64("warmup", 150, "seconds of the looped fight excluded from the score")
		duration   = flag.Float64("duration", 450, "length of the looped fight in seconds")
		verbose    = flag.Bool("v", false, "print per-character results")
		detail     = flag.Bool("detail", false, "print what each character did in one run: actions, damage by source, reactions")
		jobs       = flag.Int("j", 1, "teams checked in parallel")
		promote    = flag.Bool("promote", false, "record each result in team.yaml and set status: supported on teams that meet the criteria, draft on the others")
		shard      = flag.String("shard", "", "k/n: check only every n-th team, from the k-th on")
		jsonOut    = flag.String("json", "", "write each report as a JSON line to this file")
		cache      = flag.String("cache", defaultCache(), `directory where simulation results are kept for later runs with the same gcsim, "" for none`)
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

	if *shard != "" {
		var k, n int
		if _, err := fmt.Sscanf(*shard, "%d/%d", &k, &n); err != nil || n < 1 || k < 1 || k > n {
			fmt.Fprintf(os.Stderr, "error: -shard %q: want k/n with 1 <= k <= n\n", *shard)
			os.Exit(2)
		}
		var picked []string
		for i := k - 1; i < len(dirs); i += n {
			picked = append(picked, dirs[i])
		}
		dirs = picked
	}
	var enc *json.Encoder
	if *jsonOut != "" {
		f, err := os.Create(*jsonOut)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(2)
		}
		defer f.Close()
		enc = json.NewEncoder(f)
	}

	if *cache != "" {
		if _, err := score.OpenCache(*cache); err != nil {
			fmt.Fprintln(os.Stderr, "not caching simulation results:", err)
		}
	}
	s := validate.Settings{Iterations: *iterations, Warmup: *warmup, Duration: *duration, Cache: *cache}
	// Each team is checked in its own process, so a rotation stuck
	// inside gcsim cannot slow down the others.
	reports := make([]chan *validate.Report, len(dirs))
	for i := range reports {
		reports[i] = make(chan *validate.Report, 1)
	}
	next := make(chan int)
	go func() {
		for i := range dirs {
			next <- i
		}
		close(next)
	}()
	for range max(*jobs, 1) {
		go func() {
			for i := range next {
				reports[i] <- validate.CheckIsolated(dirs[i], s)
			}
		}()
	}
	var failed, warned int
	for _, ch := range reports {
		r := <-ch
		switch {
		case len(r.Failures) > 0:
			failed++
		case len(r.Warnings) > 0:
			warned++
		}
		printReport(r, *verbose)
		if enc != nil {
			if err := enc.Encode(r); err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(2)
			}
		}
		if *promote {
			if err := record(r, *iterations); err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(2)
			}
		}
		if *detail && r.Cfg != "" {
			printDetail(r.Cfg)
		}
	}
	fmt.Printf("\n%d teams, %d failed, %d with warnings\n", len(dirs), failed, warned)
	if failed > 0 {
		os.Exit(1)
	}
}

// defaultCache is the directory for cached simulation results under the
// user's cache directory, "" if there is none.
func defaultCache() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "gscouter", "validate")
}

// record writes the check's result to team.yaml with the status it
// earns, and says so when the status changes.
func record(r *validate.Report, iterations int) error {
	if r.Team == nil {
		// A check that timed out reports no team.
		team, err := library.Load(r.Dir)
		if err != nil {
			return nil
		}
		r.Team = team
	}
	if r.Team.Status == library.StatusBlocked {
		return nil
	}
	v := r.Validation(iterations)
	want := library.StatusSupported
	switch {
	case v.Draft != "":
		want = library.StatusDraft
	case r.Team.Tier == library.TierExperimental && r.Team.Status != library.StatusSupported:
		// A person reviews an experimental team before it is
		// supported (design §5.4); the criteria can still return it to
		// draft.
		want = library.StatusDraft
		v.Draft = "experimental: a person reviews it before it is supported"
	}
	if err := library.Record(r.Dir, want, share.GcsimCommit, v); err != nil {
		return err
	}
	switch {
	case want == r.Team.Status:
	case want == library.StatusSupported:
		fmt.Printf("       promoted to supported\n")
	default:
		fmt.Printf("       returned to draft: %s\n", v.Draft)
	}
	return nil
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
		fmt.Printf("%-4s %-46s looped %7.0f  one rotation %7.0f (%4.1fs, source %s)  kept %3.0f%%\n", verdict, name, r.Res.DPS, r.Once.DPS, r.Once.Window, ref, r.Kept()*100)
	} else {
		fmt.Printf("%-4s %s\n", verdict, name)
	}
	if r.Filler != "" {
		fmt.Printf("       filled: %s\n", r.Filler)
	}
	if r.Skipped != "" {
		fmt.Printf("       %s\n", r.Skipped)
	}
	if r.ReferenceSkipped != "" {
		fmt.Printf("       reference check skipped: %s\n", r.ReferenceSkipped)
		if cc := r.Team.CrossCheck; cc != nil && cc.DPS > 0 {
			fmt.Printf("       cross-check %.0f DPS (%s): one rotation %+.1f%%\n", cc.DPS, cc.Source, (r.Once.DPS-cc.DPS)/cc.DPS*100)
		}
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
