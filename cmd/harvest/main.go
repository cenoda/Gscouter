// Command harvest fills the team library from the community gcsim
// database at db.kqm.gg.
//
// It ranks teams by how many accepted database entries use the same four
// characters, and takes the most common ones. Only entries that run under
// Gscouter's conditions are counted: one Lv.100 target with 10%
// resistance, a rotation (not random delays or an action priority list)
// that starts with active and ends. For each team it imports the entry
// with the lowest constellations and refinements, closest to an ordinary
// account, and validates it as cmd/validate does; when the entry fails,
// it tries the next one.
//
// Usage:
//
//	harvest                    # the 64 most common teams into teams/
//	harvest -n 100 -force      # more teams, replacing existing ones
package main

import (
	"cmp"
	"context"
	"flag"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/cenoda/gscouter/internal/config"
	"github.com/cenoda/gscouter/internal/kqm"
	"github.com/cenoda/gscouter/internal/library"
	"github.com/cenoda/gscouter/internal/share"
	"github.com/cenoda/gscouter/internal/validate"
)

func main() {
	var (
		root       = flag.String("root", "teams", "team library directory")
		n          = flag.Int("n", 64, "number of teams the library should hold")
		minEntries = flag.Int("min", 2, "least database entries a team needs")
		tries      = flag.Int("tries", 3, "entries to try per team before giving up")
		iterations = flag.Int("iter", 50, "iterations per validation run")
		force      = flag.Bool("force", false, "replace teams already in the library")
	)
	flag.Parse()
	if err := run(*root, *n, *minEntries, *tries, *iterations, *force); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// candidate is a usable database entry.
type candidate struct {
	entry *kqm.Entry
	split share.Split
}

type group struct {
	key        string // sorted canonical member names
	candidates []candidate
}

func run(root string, n, minEntries, tries, iterations int, force bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	entries, err := kqm.FetchAll(ctx)
	if err != nil {
		return err
	}

	byTeam := map[string]*group{}
	rejected := map[string]int{}
	for i := range entries {
		e := &entries[i]
		sp := share.SplitConfig(e.Config)
		if reason := usable(e, sp); reason != "" {
			rejected[reason]++
			continue
		}
		key := teamKey(sp.Members)
		g := byTeam[key]
		if g == nil {
			g = &group{key: key}
			byTeam[key] = g
		}
		g.candidates = append(g.candidates, candidate{e, sp})
	}
	fmt.Printf("%d database entries, %d teams usable\n", len(entries), len(byTeam))
	for _, reason := range slices.Sorted(maps.Keys(rejected)) {
		fmt.Printf("  skipped %4d: %s\n", rejected[reason], reason)
	}

	groups := slices.Collect(maps.Values(byTeam))
	slices.SortFunc(groups, func(a, b *group) int {
		return cmp.Or(cmp.Compare(len(b.candidates), len(a.candidates)), cmp.Compare(a.key, b.key))
	})

	existing, err := existingTeams(root)
	if err != nil {
		return err
	}
	settings := validate.Settings{Iterations: iterations, Warmup: 150, Duration: 450}
	held := 0
	for _, g := range groups {
		if held >= n || len(g.candidates) < minEntries {
			break
		}
		if dir, ok := existing[g.key]; ok {
			if !force {
				fmt.Printf("keep %-46s (%d entries)\n", filepath.Base(dir), len(g.candidates))
				held++
				continue
			}
			if err := os.RemoveAll(dir); err != nil {
				return err
			}
		}
		if harvest(root, g, tries, settings) {
			held++
		}
	}
	fmt.Printf("\nThe library holds %d harvested or kept teams. Run go run ./cmd/validate -v for the full report.\n", held)
	return nil
}

// harvest imports the team's best candidate that passes validation.
func harvest(root string, g *group, tries int, s validate.Settings) bool {
	cs := slices.Clone(g.candidates)
	slices.SortFunc(cs, func(a, b candidate) int {
		return cmp.Or(
			cmp.Compare(a.entry.Investment(), b.entry.Investment()),
			cmp.Compare(b.entry.LastUpdate, a.entry.LastUpdate),
			cmp.Compare(a.entry.ID, b.entry.ID),
		)
	})
	var failures []string
	for _, c := range cs[:min(tries, len(cs))] {
		e := c.entry
		notes := fmt.Sprintf("Harvested from the KQM gcsim database: %d accepted entries use this team; this one has the lowest constellations and refinements.", len(g.candidates))
		team, err := c.split.Team([]string{e.Link()}, &library.Reference{DPS: float64(int64(e.Summary.DPS + 0.5)), Computed: e.Updated()}, notes)
		if err != nil {
			failures = append(failures, e.ID+": "+err.Error())
			continue
		}
		dir, err := c.split.Write(root, team, true)
		if err != nil {
			failures = append(failures, e.ID+": "+err.Error())
			continue
		}
		r := validate.Check(dir, s)
		if len(r.Failures) == 0 {
			fmt.Printf("add  %-46s looped %7.0f  one rotation %7.0f (source %7.0f, %d entries, %s)\n",
				team.ID, r.Res.DPS, r.Once.DPS, team.Reference.DPS, len(g.candidates), e.ID)
			return true
		}
		failures = append(failures, e.ID+": "+strings.Join(r.Failures, "; "))
		if err := os.RemoveAll(dir); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
		}
	}
	fmt.Printf("fail %-46s (%d entries)\n", g.key, len(g.candidates))
	for _, f := range failures {
		fmt.Printf("       %s\n", f)
	}
	return false
}

var (
	lvlRe     = regexp.MustCompile(`\blvl=(\d+)`)
	resistRe  = regexp.MustCompile(`\bresist=([\d.]+)`)
	foreverRe = regexp.MustCompile(`\bwhile\s+(1|true)\b`)
)

// usable says why an entry cannot be used, or "" if it can.
func usable(e *kqm.Entry, sp share.Split) string {
	switch {
	case !e.HasTag(kqm.TagGcsim) && !e.HasTag(kqm.TagGuides):
		return "not accepted by gcsim or a guide"
	case e.HasTag(kqm.TagRandomDelays) || e.HasTag(kqm.TagAPL):
		return "random delays or action priority list"
	case e.Summary.Mode != kqm.ModeTTK:
		return "not a rotation that ends"
	case e.Summary.TargetCount != 1:
		return "more than one target"
	case len(sp.Members) != 4:
		return "not four characters"
	case !standardTarget(sp.Dropped):
		return "enemy other than Lv.100 with 10% resistance"
	case foreverRe.MatchString(sp.Rotation):
		return "rotation loops forever"
	}
	if _, err := config.Loop(sp.Rotation); err != nil {
		return "rotation does not start with active"
	}
	return ""
}

// standardTarget reports whether the config has one Lv.100 target with
// 10% resistance, the common conditions.
func standardTarget(header []string) bool {
	targets := 0
	for _, h := range header {
		if !strings.HasPrefix(h, "target") {
			continue
		}
		targets++
		lvl, res := lvlRe.FindStringSubmatch(h), resistRe.FindStringSubmatch(h)
		if lvl == nil || lvl[1] != "100" || res == nil || !isTenth(res[1]) {
			return false
		}
	}
	return targets == 1
}

func isTenth(s string) bool {
	switch strings.TrimRight(s, "0") {
	case "0.1", ".1":
		return true
	}
	return false
}

func teamKey(members []string) string {
	keys := make([]string, len(members))
	for i, m := range members {
		keys[i] = library.Canonical(m)
	}
	slices.Sort(keys)
	return strings.Join(keys, " ")
}

// existingTeams maps the library's teams by member set to their directory.
func existingTeams(root string) (map[string]string, error) {
	out := map[string]string{}
	dirs, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		dir := filepath.Join(root, d.Name())
		t, err := library.Load(dir)
		if err != nil {
			return nil, err
		}
		out[teamKey(t.Members)] = dir
	}
	return out, nil
}
