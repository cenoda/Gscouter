// Command weights calculates the artifact score's weights for library
// teams (docs/artifacts.md §4.2, §4.3, §5). For each team it scores the
// reference build (build.txt) once, then per character adds one maximum
// 5★ roll of each of the ten substats and puts each possible main stat
// on sands, goblet and circlet in turn, and scores each variant. Every
// variant runs with the reference's seeds and the looped rotation the
// reference scored best with, fillers included.
//
// Usage:
//
//	weights                          # every team under teams/
//	weights teams/<id> ...           # selected teams
//	weights -limit 100               # 100 teams spread evenly over the library
//	weights -shard 2/4 -json b.jsonl # the second of four slices, as JSON lines
//	weights -join teams/weights.json [-base teams/weights.json] a.jsonl b.jsonl ...
//	weights -slim internal/artifact/table.json [teams/weights.json]
//
// Teams run in parallel (-j, one per CPU by default), each in its own
// process, and each simulation on -workers CPUs (1 by default). -json
// writes one weights.Result per team, in team order; -join merges such
// files, the shards of a run, into one table (weights.Table, by default
// at teams/weights.json), replacing the entries of the teams they hold
// in -base, if given.
//
// -slim writes the small table the server embeds (artifact.Table,
// weights.Slim) from the full one: the table -join writes, or else the
// one given as argument (teams/weights.json by default).
//
// A team whose build.txt does not hold, per character, its five main
// stats and then its substat totals on two add stats lines
// (weights.ParseBuild) is skipped with the reason.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/cenoda/gscouter/internal/batch"
	"github.com/cenoda/gscouter/internal/library"
	"github.com/cenoda/gscouter/internal/score"
	"github.com/cenoda/gscouter/internal/weights"
)

func main() {
	weights.ServeChild()
	var (
		root       = flag.String("root", "teams", "team library directory, used when no teams are given")
		iterations = flag.Int("iter", 20, "iterations per simulation")
		warmup     = flag.Float64("warmup", 150, "seconds of the looped fight excluded from the score")
		duration   = flag.Float64("duration", 450, "length of the looped fight in seconds")
		jobs       = flag.Int("j", runtime.NumCPU(), "teams calculated in parallel")
		workers    = flag.Int("workers", 1, "CPUs each simulation runs on")
		shard      = flag.String("shard", "", "k/n: calculate only every n-th team, from the k-th on")
		limit      = flag.Int("limit", 0, "calculate only this many teams, spread evenly over the list (before -shard)")
		jsonOut    = flag.String("json", "", "write each team's result as a JSON line to this file")
		cache      = flag.String("cache", batch.DefaultCache(), `directory where simulation results are kept for later runs with the same gcsim, "" for none`)
		timeout    = flag.Duration("timeout", 30*time.Minute, "time one team may take before it fails")
		verbose    = flag.Bool("v", false, "print per-character weights and main stats")
		join       = flag.String("join", "", "join the JSON line files given as arguments (or glob patterns) into this table file, and calculate nothing")
		base       = flag.String("base", "", "with -join: a table to start from; its teams that the files hold are replaced")
		slim       = flag.String("slim", "", "write the small table derived from the full one to this file, and calculate nothing")
	)
	flag.Parse()

	if *join != "" || *slim != "" {
		if err := tables(*join, *base, *slim, flag.Args()); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(2)
		}
		return
	}

	dirs, err := batch.Dirs(*root, flag.Args())
	if err == nil {
		dirs, err = batch.Shard(spread(dirs, *limit), *shard)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}
	gcsim, fork, err := score.Gcsim()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}
	if !fork {
		n := len(dirs)
		dirs = slices.DeleteFunc(dirs, func(dir string) bool {
			team, err := library.Load(dir)
			return err == nil && team.HasForkOnly()
		})
		if n > len(dirs) {
			fmt.Fprintf(os.Stderr, "skipping %d teams with characters only the gcsim fork has: built with upstream gcsim\n", n-len(dirs))
		}
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

	s := weights.Settings{Iterations: *iterations, Warmup: *warmup, Duration: *duration, Workers: *workers, Cache: *cache, Timeout: *timeout}
	start := time.Now()
	results := batch.Parallel(len(dirs), *jobs, func(i int) *weights.Result {
		return weights.TeamIsolated(dirs[i], s)
	})
	var all []*weights.Result
	for _, ch := range results {
		r := <-ch
		r.Gcsim = gcsim.Path + "@" + gcsim.Version
		all = append(all, r)
		printResult(r, *verbose)
		if enc != nil {
			if err := enc.Encode(r); err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(2)
			}
		}
	}
	_, tot, _ := weights.Join(nil, all)
	fmt.Printf("\n%s\nwall time %.0fs with -j %d -workers %d\n", summary(tot), time.Since(start).Seconds(), *jobs, *workers)
}

// spread returns n of dirs, evenly spaced over the list, so a sample
// covers the whole library rather than its first characters by name;
// n <= 0 or n >= len(dirs) returns dirs.
func spread(dirs []string, n int) []string {
	if n <= 0 || n >= len(dirs) {
		return dirs
	}
	out := make([]string, n)
	for i := range out {
		out[i] = dirs[i*len(dirs)/n]
	}
	return out
}

func summary(t weights.Totals) string {
	per := func(x float64) float64 {
		if t.Computed == 0 {
			return 0
		}
		return x / float64(t.Computed)
	}
	return fmt.Sprintf("%d teams: %d calculated, %d skipped, %d failed; %d simulations (%.1f per calculated team), %.0f team-seconds (%.1f per calculated team)",
		t.Teams, t.Computed, t.Skipped, t.Failed, t.Simulations, per(float64(t.Simulations)), t.Seconds, per(t.Seconds))
}

func printResult(r *weights.Result, verbose bool) {
	switch {
	case r.Error != "":
		fmt.Printf("FAIL %-46s %s\n", r.ID, r.Error)
		return
	case r.Skipped != "":
		fmt.Printf("SKIP %-46s %s\n", r.ID, r.Skipped)
		return
	}
	fmt.Printf("OK   %-46s %7.0f DPS  %3d simulations  %6.1fs\n", r.ID, r.DPS, r.Simulations, r.Seconds)
	if r.Filler != "" && verbose {
		fmt.Printf("       filled: %s\n", r.Filler)
	}
	if !verbose {
		return
	}
	for _, c := range r.Characters {
		var ws []string
		for _, roll := range weights.Substats {
			ws = append(ws, fmt.Sprintf("%s %.2f", roll.Key, c.Weights[roll.Key]))
		}
		fmt.Printf("       %-12s best %-4s (+%.0f DPS)  %s\n", c.Key, c.Best, c.Gains[c.Best], strings.Join(ws, " "))
		for _, slot := range weights.Slots {
			m := c.Main[slot]
			var wrong []string
			for _, k := range slices.Sorted(maps.Keys(m.Penalty)) {
				wrong = append(wrong, fmt.Sprintf("%s −%.0f", k, m.Penalty[k]))
			}
			fmt.Printf("         %-7s build %-8s right %-20s wrong %s\n", slot, m.Build, strings.Join(m.Right, ","), strings.Join(wrong, ", "))
		}
	}
}

// tables runs -join and -slim: it joins the JSON line files matching
// args into the table at join, if given, and writes the small table
// derived from that table, or from the table args name, to slim.
func tables(join, base, slim string, args []string) error {
	var t *weights.Table
	var err error
	switch {
	case join != "":
		if t, err = joinFiles(join, base, args); err != nil {
			return err
		}
	case len(args) > 1:
		return fmt.Errorf("-slim: give one table, not %d", len(args))
	default:
		name := "teams/weights.json"
		if len(args) == 1 {
			name = args[0]
		}
		if t, err = readTable(name); err != nil {
			return err
		}
	}
	if slim == "" {
		return nil
	}
	return writeSlim(slim, t)
}

// readTable reads a full table. An error that is not the file missing
// says to fetch the file through Git LFS, since a checkout without
// LFS holds a pointer in its place.
func readTable(name string) (*weights.Table, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	t, err := weights.ReadTable(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w (a Git LFS pointer? fetch it with git lfs pull)", name, err)
	}
	return t, nil
}

func writeSlim(name string, t *weights.Table) error {
	s := weights.Slim(t)
	f, err := os.Create(name)
	if err != nil {
		return err
	}
	n, err := s.WriteTo(f)
	if err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	fmt.Printf("small table %s: %d teams, %d bytes\n", name, len(s.Teams), n)
	return nil
}

// joinFiles joins the JSON line files matching patterns into the table
// at out, starting from the table at base if given, and returns it.
func joinFiles(out, base string, patterns []string) (*weights.Table, error) {
	var files []string
	for _, p := range patterns {
		m, err := filepath.Glob(p)
		if err != nil {
			return nil, err
		}
		if len(m) == 0 {
			return nil, fmt.Errorf("%s: no such file", p)
		}
		files = append(files, m...)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("-join: no files to join")
	}
	var results []*weights.Result
	for _, name := range files {
		f, err := os.Open(name)
		if err != nil {
			return nil, err
		}
		rs, err := weights.ReadResults(f)
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		results = append(results, rs...)
	}
	var from *weights.Table
	if base != "" {
		var err error
		from, err = readTable(base)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}
	t, tot, err := weights.Join(from, results)
	if err != nil {
		return nil, err
	}
	f, err := os.Create(out)
	if err != nil {
		return nil, err
	}
	if _, err := t.WriteTo(f); err != nil {
		f.Close()
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	fmt.Printf("%s\n%d files joined into %s: %d teams with weights, %d without\n", summary(tot), len(files), out, len(t.Teams), len(t.Skipped))
	return t, nil
}
