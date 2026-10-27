// Command compare compares two validation runs team by team, as the
// regression run does (design §5.4): every report field at full
// precision, so the difference must be exactly 0.
//
// Usage:
//
//	compare a.jsonl b.jsonl
//	compare 'upstream-*.jsonl' 'fork-*.jsonl'
//
// Each side is a file `validate -json` wrote, or a glob of them, such
// as the shards of one run (`validate -shard k/n`); a team found twice on
// one side is an error. It prints each team that differs with the
// fields that do, and the teams only one side has, and exits with
// status 1 if there are any.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
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
		fmt.Fprintln(os.Stderr, "usage: compare <a.jsonl or glob> <b.jsonl or glob>")
		os.Exit(2)
	}
	a, err := load(os.Args[1])
	if err == nil {
		var b map[string]*validate.Report
		if b, err = load(os.Args[2]); err == nil {
			os.Exit(compare(a, b))
		}
	}
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(2)
}

// load reads the reports in the files pattern matches, by team.
func load(pattern string) (map[string]*validate.Report, error) {
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no file matches %s", pattern)
	}
	reports := map[string]*validate.Report{}
	for _, name := range files {
		f, err := os.Open(name)
		if err != nil {
			return nil, err
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(nil, 64<<20)
		for sc.Scan() {
			var r validate.Report
			if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
				f.Close()
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			team := filepath.Base(r.Dir)
			if _, dup := reports[team]; dup {
				f.Close()
				return nil, fmt.Errorf("%s: %s is already in %s", name, team, pattern)
			}
			reports[team] = &r
		}
		f.Close()
		if err := sc.Err(); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
	}
	return reports, nil
}

func compare(a, b map[string]*validate.Report) int {
	var teams []string
	for t := range a {
		teams = append(teams, t)
	}
	for t := range b {
		if a[t] == nil {
			teams = append(teams, t)
		}
	}
	slices.Sort(teams)
	var differ, missing int
	for _, t := range teams {
		ra, rb := a[t], b[t]
		switch {
		case rb == nil:
			missing++
			fmt.Printf("%s: only in the first run\n", t)
			continue
		case ra == nil:
			missing++
			fmt.Printf("%s: only in the second run\n", t)
			continue
		}
		if lines := diff(ra, rb); len(lines) > 0 {
			differ++
			fmt.Printf("%s:\n  %s\n", t, strings.Join(lines, "\n  "))
		}
	}
	fmt.Printf("%d teams, %d differ, %d in one run only\n", len(teams), differ, missing)
	if differ+missing > 0 {
		return 1
	}
	return 0
}

// diff lists the fields of two reports that differ, with the scores'
// DPS, per character where those differ.
func diff(a, b *validate.Report) []string {
	var lines []string
	va, vb := reflect.ValueOf(*a), reflect.ValueOf(*b)
	for i := range va.NumField() {
		f := va.Type().Field(i)
		if !f.IsExported() || f.Name == "Dir" {
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
