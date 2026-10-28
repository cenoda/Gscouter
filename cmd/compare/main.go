// Command compare compares validation runs team by team, as the
// regression run does (docs/fork.md §2): every report field at full
// precision, so the difference must be exactly 0.
//
// Usage:
//
//	compare a.jsonl b.jsonl
//	compare 'upstream-*.jsonl' 'fork-*.jsonl'
//	compare 'up1-*.jsonl,up2-*.jsonl' 'fork1-*.jsonl,fork2-*.jsonl'
//
// Each side is one run or several, separated by commas; a run is a file
// `validate -json` wrote, or a glob of them, such as the shards of one
// run (`validate -shard k/n`). A team found twice in one run is an
// error. build.sh builds cmd/validate on the fork and on upstream.
//
// Upstream gcsim, very rarely, does not repeat an iteration exactly, so
// a team that differs is classified by how often each result appears:
//
//	DIFF              a side has one run: rerun the team on both sides
//	                  (it lists them), with -cache "" so that the runs
//	                  repeat the simulation
//	REGRESSION        a result appears in two runs or more of one side
//	                  and in none of the other
//	NONDETERMINISTIC  the sides differ only by results seen once
//
// It prints each team that differs, with the fields that do, and the
// teams only one side has, and exits with status 1 if there is a DIFF, a
// REGRESSION or a missing team.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/cenoda/gscouter/internal/score"
	"github.com/cenoda/gscouter/internal/validate"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: compare <runs of side a> <runs of side b>")
		os.Exit(2)
	}
	a, err := loadSide(os.Args[1])
	if err == nil {
		var b *side
		if b, err = loadSide(os.Args[2]); err == nil {
			os.Exit(compare(a, b))
		}
	}
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(2)
}

// side is the runs of one build.
type side struct {
	name   string
	runs   int
	gcsim  map[string]bool
	cached bool
	// results counts each team's results by their JSON, which holds
	// every report field but the team's directory and files.
	results map[string]map[string]int
	reports map[string]*validate.Report // by result JSON
}

func loadSide(arg string) (*side, error) {
	s := &side{name: arg, gcsim: map[string]bool{}, results: map[string]map[string]int{}, reports: map[string]*validate.Report{}}
	for _, pattern := range strings.Split(arg, ",") {
		if err := s.load(pattern); err != nil {
			return nil, err
		}
		s.runs++
	}
	return s, nil
}

// load reads one run, the files pattern matches.
func (s *side) load(pattern string) error {
	files, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no file matches %s", pattern)
	}
	seen := map[string]bool{}
	for _, name := range files {
		if err := s.loadFile(name, pattern, seen); err != nil {
			return err
		}
	}
	return nil
}

func (s *side) loadFile(name, pattern string, seen map[string]bool) error {
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(nil, 64<<20)
	for sc.Scan() {
		var l validate.Line
		if err := json.Unmarshal(sc.Bytes(), &l); err != nil || l.Report == nil {
			return fmt.Errorf("%s: not a report: %v", name, err)
		}
		team := filepath.Base(l.Dir)
		if seen[team] {
			return fmt.Errorf("%s: %s is already in %s", name, team, pattern)
		}
		seen[team] = true
		s.gcsim[l.Gcsim] = true
		s.cached = s.cached || l.Cached
		r := *l.Report
		r.Dir, r.Team = "", nil
		raw, err := json.Marshal(r)
		if err != nil {
			return err
		}
		if s.results[team] == nil {
			s.results[team] = map[string]int{}
		}
		s.results[team][string(raw)]++
		s.reports[string(raw)] = &r
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

type verdict string

const (
	same             verdict = ""
	diff             verdict = "DIFF"
	regression       verdict = "REGRESSION"
	nondeterministic verdict = "NONDETERMINISTIC"
)

// classify compares one team's results on two sides, each counted by
// the runs it appeared in.
func classify(a, b map[string]int, runsA, runsB int) verdict {
	all := maps.Clone(a)
	maps.Copy(all, b)
	repeats := func(x, y map[string]int) bool {
		for res, n := range x {
			if n >= 2 && y[res] == 0 {
				return true
			}
		}
		return false
	}
	switch {
	case len(all) == 1:
		return same
	case runsA == 1 || runsB == 1:
		return diff
	case repeats(a, b) || repeats(b, a):
		return regression
	}
	return nondeterministic
}

func compare(a, b *side) int {
	for _, s := range []*side{a, b} {
		if len(s.gcsim) > 1 {
			fmt.Printf("warning: the runs of %s were built with different gcsim: %v\n", s.name, slices.Sorted(maps.Keys(s.gcsim)))
		}
		if s.runs > 1 && s.cached {
			fmt.Printf("warning: the runs of %s used the cache, so they repeat the first run's results: run them with -cache \"\"\n", s.name)
		}
	}
	if maps.Equal(a.gcsim, b.gcsim) {
		fmt.Printf("warning: both sides were built with the same gcsim: %v\n", slices.Sorted(maps.Keys(a.gcsim)))
	}
	teams := slices.Sorted(maps.Keys(a.results))
	for t := range b.results {
		if a.results[t] == nil {
			teams = append(teams, t)
		}
	}
	slices.Sort(teams)
	count := map[verdict]int{}
	var missing int
	var rerun []string
	for _, t := range teams {
		ra, rb := a.results[t], b.results[t]
		if ra == nil || rb == nil {
			missing++
			fmt.Printf("%s: in one side only\n", t)
			continue
		}
		v := classify(ra, rb, a.runs, b.runs)
		count[v]++
		switch v {
		case same:
			continue
		case diff:
			rerun = append(rerun, t)
		}
		fmt.Printf("%s %s: %s vs %s\n", v, t, counts(ra), counts(rb))
		lines := reportDiff(a.reports[mostCommon(ra)], b.reports[mostCommon(rb)])
		fmt.Printf("  %s\n", strings.Join(lines, "\n  "))
	}
	fmt.Printf("%d teams: %d differ (%d DIFF, %d REGRESSION, %d NONDETERMINISTIC), %d in one side only\n",
		len(teams), count[diff]+count[regression]+count[nondeterministic], count[diff], count[regression], count[nondeterministic], missing)
	if len(rerun) > 0 {
		fmt.Printf("rerun on both sides, with -cache \"\":\n  %s\n", strings.Join(rerun, " "))
	}
	if count[diff]+count[regression]+missing > 0 {
		return 1
	}
	return 0
}

// counts shows how many runs gave each result, most common first.
func counts(c map[string]int) string {
	n := slices.SortedFunc(maps.Values(c), func(x, y int) int { return y - x })
	parts := make([]string, len(n))
	for i, x := range n {
		parts[i] = fmt.Sprint(x)
	}
	return strings.Join(parts, "+") + " runs"
}

// mostCommon is the result most runs gave.
func mostCommon(c map[string]int) string {
	best := ""
	for res, n := range c {
		if best == "" || n > c[best] || n == c[best] && res < best {
			best = res
		}
	}
	return best
}

// reportDiff lists the fields of two reports that differ, with the
// scores' fields, per character where those differ.
func reportDiff(a, b *validate.Report) []string {
	var lines []string
	va, vb := reflect.ValueOf(*a), reflect.ValueOf(*b)
	for i := range va.NumField() {
		f := va.Type().Field(i)
		if !f.IsExported() {
			continue
		}
		x, y := va.Field(i).Interface(), vb.Field(i).Interface()
		if reflect.DeepEqual(x, y) {
			continue
		}
		ra, okA := x.(*score.Result)
		rb, okB := y.(*score.Result)
		if okA && okB && ra != nil && rb != nil {
			lines = append(lines, resultDiff(f.Name, ra, rb)...)
			continue
		}
		lines = append(lines, fmt.Sprintf("%s: %v\n    vs %v", f.Name, show(x), show(y)))
	}
	return lines
}

func resultDiff(field string, a, b *score.Result) []string {
	var parts []string
	va, vb := reflect.ValueOf(*a), reflect.ValueOf(*b)
	for i := range va.NumField() {
		name := va.Type().Field(i).Name
		if x, y := va.Field(i).Interface(), vb.Field(i).Interface(); name != "Characters" && !reflect.DeepEqual(x, y) {
			parts = append(parts, fmt.Sprintf("%s %v vs %v", name, x, y))
		}
	}
	lines := []string{field + ": " + strings.Join(parts, ", ")}
	if len(parts) == 0 {
		lines[0] = field + ": per character"
	}
	for i := range max(len(a.Characters), len(b.Characters)) {
		var ca, cb score.Character
		if i < len(a.Characters) {
			ca = a.Characters[i]
		}
		if i < len(b.Characters) {
			cb = b.Characters[i]
		}
		if !reflect.DeepEqual(ca, cb) {
			lines = append(lines, fmt.Sprintf("  %s: %s\n      vs %s", ca.Name, show(ca), show(cb)))
		}
	}
	return lines
}

func show(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(raw)
}
