// Command variant derives alternatives from teams' own rotations
// (library.Variant, docs/cycle.md §7.2): for each team given, it takes a
// member's bursts out of the team's rotation, raises that member's
// constellation in the team's build, and scores both rotations with that
// build as the site does (looped, with fillers). When the changed
// rotation scores at least -min higher, it is written as the team's
// alternative alt/<char>-c<cons>-no-<drop>/ with status draft, for
// cmd/validate -promote to check; otherwise an earlier one of that name
// is removed. A team that cannot be measured keeps its earlier one.
//
// Usage:
//
//	variant -char citlali -cons 6 teams/*citlali*
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
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
		char       = flag.String("char", "", "member whose action is taken out, under any of gcsim's names for it")
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
	written := 0
	for _, dir := range flag.Args() {
		name, gain, err := derive(dir, library.Variant{Drop: *drop, Char: *char}, *cons, *minGain, *iterations)
		switch {
		case err != nil:
			fmt.Printf("%-50s error: %v\n", filepath.Base(dir), err)
		case gain >= *minGain:
			written++
			fmt.Printf("%-50s %+6.1f%%  written (%s)\n", filepath.Base(dir), gain*100, name)
		default:
			fmt.Printf("%-50s %+6.1f%%\n", filepath.Base(dir), gain*100)
		}
	}
	fmt.Printf("\nwrote %d variants (%s C%d without %s) for %d teams\n", written, *char, *cons, *drop, flag.NArg())
}

// derive measures the variant of the team in dir and writes or removes
// it, returning its name and its gain over the team's rotation. v.Char
// may be any of gcsim's names for the member ("amy" for "fischl"): the
// member, the rotation's statements and the build's character are all
// matched by gcsim key, and the variant is written under the team's name
// for the member (library loading checks Variant.Char against the
// team's members), as alt/<member>-c<cons>-no-<drop>. An earlier variant of
// that name is left as it is until the measurement succeeds: it is
// removed only when the gain falls short of minGain, and replaced only
// once the new one is written in full (replaceDir).
func derive(dir string, v library.Variant, cons int, minGain float64, iterations int) (name string, gain float64, err error) {
	team, err := library.Load(dir)
	if err != nil {
		return "", 0, err
	}
	if team.Status == library.StatusBlocked {
		return "", 0, fmt.Errorf("blocked")
	}
	i := slices.IndexFunc(team.Members, func(m string) bool { return library.Canonical(m) == library.Canonical(v.Char) })
	if i < 0 {
		return "", 0, fmt.Errorf("no member %s", v.Char)
	}
	v.Char = team.Members[i]
	name = fmt.Sprintf("%s-c%d-no-%s", v.Char, cons, v.Drop)
	out := filepath.Join(dir, library.AltDir, name)
	rotation := config.DropBursts(team.Rotation, v.Char)
	if rotation == team.Rotation {
		return name, 0, fmt.Errorf("the rotation has no %s of %s", v.Drop, v.Char)
	}
	build, ok := raiseCons(team.Build, v.Char, cons)
	if !ok {
		return name, 0, fmt.Errorf("the build has no constellation for %s", v.Char)
	}

	own, err := measure(team, build, team.Rotation, iterations)
	if err != nil {
		return name, 0, fmt.Errorf("the team's rotation: %w", err)
	}
	dps, err := measure(team, build, rotation, iterations)
	if err != nil {
		return name, 0, fmt.Errorf("the variant: %w", err)
	}
	gain = dps/own - 1
	if gain < minGain {
		return name, gain, os.RemoveAll(out)
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
		return name, 0, err
	}
	files := map[string]string{
		"team.yaml":     string(meta),
		"rotation.gcsl": rotation,
		"build.txt":     fmt.Sprintf("# The team's build with %s at C%d, the build the variant was measured and is validated with.\n\n", v.Char, cons) + stripComments(build),
	}
	if err := replaceDir(out, files); err != nil {
		return name, 0, err
	}
	return name, gain, nil
}

// consRe matches a character line of a build up to its constellation's
// number: "fischl char lvl=90/90 cons=" of "... cons=6 talent=9,9,9;".
var consRe = regexp.MustCompile(`(?m)^(\s*)(\w+)(\s+char\b[^;]*\bcons\s*=\s*)\d+`)

// raiseCons sets the constellation of char in build to cons, matching
// the build's character lines by gcsim key, and reports whether the
// build has one for char.
func raiseCons(build, char string, cons int) (string, bool) {
	found := false
	out := consRe.ReplaceAllStringFunc(build, func(line string) string {
		m := consRe.FindStringSubmatch(line)
		if library.Canonical(m[2]) != library.Canonical(char) {
			return line
		}
		found = true
		return m[1] + m[2] + m[3] + strconv.Itoa(cons)
	})
	return out, found
}

// measure scores rotation on build for the team as the site does: looped,
// with fillers. It is a variable so that tests can stand in for the
// simulation.
var measure = simulate

func simulate(team *library.Team, build, rotation string, iterations int) (float64, error) {
	for m, p := range team.Params {
		build = config.AddParams(build, m, p)
	}
	res, err := steady.Score(rotation, team.Members, func(looped string) (*score.Result, error) {
		set := web.DefaultSettings
		cfg := config.Assemble(config.Conditions{Iterations: iterations, Duration: set.Duration}, build, looped)
		return score.Run(context.Background(), cfg, score.Options{Iterations: iterations, Warmup: set.Warmup, Seed: 1})
	})
	if err != nil {
		return 0, err
	}
	return res.Result.DPS, nil
}

// replaceDir makes dir hold exactly files. It writes them into a staging
// directory beside dir and swaps it in only once every write succeeded,
// so a failure leaves dir as it was.
func replaceDir(dir string, files map[string]string) error {
	parent := filepath.Dir(dir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(parent, "."+filepath.Base(dir)+"-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage) // a no-op once it is swapped in
	if err := os.Chmod(stage, 0o755); err != nil {
		return err
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(stage, name), []byte(content), 0o644); err != nil {
			return err
		}
	}
	old := ""
	if _, err := os.Lstat(dir); err == nil {
		old = stage + "-old"
		if err := os.Rename(dir, old); err != nil {
			return err
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := os.Rename(stage, dir); err != nil {
		if old != "" {
			if rerr := os.Rename(old, dir); rerr != nil {
				return fmt.Errorf("%w (the earlier %s is left in %s: %v)", err, filepath.Base(dir), old, rerr)
			}
		}
		return err
	}
	if old != "" {
		return os.RemoveAll(old)
	}
	return nil
}

// stripComments drops the build's leading comment lines.
func stripComments(build string) string {
	lines := strings.Split(build, "\n")
	for len(lines) > 0 && (strings.HasPrefix(lines[0], "#") || strings.TrimSpace(lines[0]) == "") {
		lines = lines[1:]
	}
	return strings.Join(lines, "\n")
}
