// Command variant derives alternatives from teams' own rotations
// (library.Variant, docs/cycle.md §7.2): for each team given, it takes a
// member's bursts out of the team's rotation, raises that member's
// constellation in the team's build, and scores both rotations with that
// build as the site does (looped, with fillers). When the changed
// rotation scores at least -min higher, it is written as the team's
// alternative alt/<char>-c<cons>-no-<drop>/ with status draft, for
// cmd/validate -promote to check; otherwise an earlier one of that name
// is removed.
//
// Usage:
//
//	variant -char citlali -cons 6 teams/*citlali*
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/cenoda/gscouter/internal/config"
	"github.com/cenoda/gscouter/internal/library"
	"github.com/cenoda/gscouter/internal/score"
	"github.com/cenoda/gscouter/internal/share"
	"github.com/cenoda/gscouter/internal/steady"
	"github.com/cenoda/gscouter/internal/web"
	"gopkg.in/yaml.v3"
)

func main() {
	var (
		char       = flag.String("char", "", "member whose action is taken out, as gcsim names it")
		cons       = flag.Int("cons", 6, "the member's constellation the variant is for")
		drop       = flag.String("drop", "burst", "the action taken out: "+strings.Join(library.VariantDrops, ", "))
		minGain    = flag.Float64("min", 0.01, "least gain over the team's rotation to write the variant")
		iterations = flag.Int("iter", 200, "iterations of each score")
	)
	flag.Parse()
	if *char == "" || flag.NArg() == 0 || !slices.Contains(library.VariantDrops, *drop) {
		flag.Usage()
		os.Exit(2)
	}
	name := fmt.Sprintf("%s-c%d-no-%s", *char, *cons, *drop)
	written := 0
	for _, dir := range flag.Args() {
		gain, err := derive(dir, name, library.Variant{Drop: *drop, Char: *char}, *cons, *minGain, *iterations)
		switch {
		case err != nil:
			fmt.Printf("%-50s error: %v\n", filepath.Base(dir), err)
		case gain >= *minGain:
			written++
			fmt.Printf("%-50s %+6.1f%%  written\n", filepath.Base(dir), gain*100)
		default:
			fmt.Printf("%-50s %+6.1f%%\n", filepath.Base(dir), gain*100)
		}
	}
	fmt.Printf("\nwrote %d variants (%s) for %d teams\n", written, name, flag.NArg())
}

// derive measures the variant of the team in dir and writes or removes
// it, returning its gain over the team's rotation.
func derive(dir, name string, v library.Variant, cons int, minGain float64, iterations int) (float64, error) {
	out := filepath.Join(dir, library.AltDir, name)
	if err := os.RemoveAll(out); err != nil {
		return 0, err
	}
	team, err := library.Load(dir)
	if err != nil {
		return 0, err
	}
	if team.Status == library.StatusBlocked {
		return 0, fmt.Errorf("blocked")
	}
	if !slices.Contains(team.Members, v.Char) {
		return 0, fmt.Errorf("no member %s", v.Char)
	}
	rotation := config.DropBursts(team.Rotation, v.Char)
	if rotation == team.Rotation {
		return 0, fmt.Errorf("the rotation has no %s of %s", v.Drop, v.Char)
	}
	consRe := regexp.MustCompile(`(?m)^(\s*` + regexp.QuoteMeta(v.Char) + `\s+char\b[^;]*\bcons\s*=\s*)\d+`)
	if !consRe.MatchString(team.Build) {
		return 0, fmt.Errorf("the build has no constellation for %s", v.Char)
	}
	build := consRe.ReplaceAllString(team.Build, fmt.Sprintf("${1}%d", cons))

	scoreWith := func(rotation string) (float64, error) {
		b := build
		for m, p := range team.Params {
			b = config.AddParams(b, m, p)
		}
		res, err := steady.Score(rotation, team.Members, func(looped string) (*score.Result, error) {
			set := web.DefaultSettings
			cfg := config.Assemble(config.Conditions{Iterations: iterations, Duration: set.Duration}, b, looped)
			return score.Run(context.Background(), cfg, score.Options{Iterations: iterations, Warmup: set.Warmup, Seed: 1})
		})
		if err != nil {
			return 0, err
		}
		return res.Result.DPS, nil
	}
	own, err := scoreWith(team.Rotation)
	if err != nil {
		return 0, fmt.Errorf("the team's rotation: %w", err)
	}
	dps, err := scoreWith(rotation)
	if err != nil {
		return 0, fmt.Errorf("the variant: %w", err)
	}
	gain := dps/own - 1
	if gain < minGain {
		return gain, nil
	}

	alt := *team
	alt.Status = library.StatusDraft
	alt.RotationVersion = 1
	alt.GcsimCommit = share.GcsimCommit
	alt.Variant = &v
	alt.Validation = nil
	alt.Reference = nil
	alt.CrossCheck = nil
	alt.Notes = fmt.Sprintf("The team's rotation without %s's %s, for %s C%d (cmd/variant, docs/cycle.md §7.2). With the team's build at %s C%d it scores %.0f DPS against %.0f with the team's rotation (%+.1f%%, looped with fillers, %d iterations, %s).",
		v.Char, v.Drop, v.Char, cons, v.Char, cons, dps, own, gain*100, iterations, time.Now().Format(time.DateOnly))
	meta, err := yaml.Marshal(&alt)
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return 0, err
	}
	files := map[string]string{
		"team.yaml":     string(meta),
		"rotation.gcsl": rotation,
		"build.txt":     fmt.Sprintf("# The team's build with %s at C%d, the build the variant was measured and is validated with.\n\n", v.Char, cons) + stripComments(build),
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(out, name), []byte(content), 0o644); err != nil {
			return 0, err
		}
	}
	return gain, nil
}

// stripComments drops the build's leading comment lines.
func stripComments(build string) string {
	lines := strings.Split(build, "\n")
	for len(lines) > 0 && (strings.HasPrefix(lines[0], "#") || strings.TrimSpace(lines[0]) == "") {
		lines = lines[1:]
	}
	return strings.Join(lines, "\n")
}
