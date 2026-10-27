// Command harvest fills the team library from the community gcsim
// database at db.kqm.gg.
//
// It ranks teams by how many database entries use the same four
// characters, and takes the most common ones. An entry is usable unless
// it does not have four characters or its rotation never ends (an
// unconditional while loop); its enemies, resistances and energy may
// differ from Gscouter's, because a team is accepted by running one pass
// of the rotation under the entry's own conditions and comparing the DPS
// with the database's (validate.CheckReference). For each team it tries
// the entries from the lowest constellations and refinements up, closest
// to an ordinary account. An entry must pass that check and the full one
// (validate.Check: the looped rotation must not stall); of the first
// -choose entries that pass, it keeps the one whose looped score keeps
// most of its one pass under the same conditions. Checks run in child
// processes, because gcsim cannot cancel a rotation stuck in a loop.
// A team that meets the criteria for supported
// (validate.Report.Unpromotable) is written as supported, any other as
// draft.
//
// Teams that fail every entry are recorded in <root>/harvest-failed.txt
// and skipped by the next run unless -retry-failed is set.
//
// Usage:
//
//	harvest                                # the 64 most common teams into teams/
//	harvest -n 10000 -cache kqm.json       # every team, reusing a saved download
//	harvest -n 100 -force                  # replace existing teams
package main

import (
	"bufio"
	"cmp"
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cenoda/gscouter/internal/config"
	"github.com/cenoda/gscouter/internal/kqm"
	"github.com/cenoda/gscouter/internal/library"
	"github.com/cenoda/gscouter/internal/share"
	"github.com/cenoda/gscouter/internal/validate"
)

// failedFile lists, under the library root, the teams that failed.
const failedFile = "harvest-failed.txt"

type options struct {
	root        string
	n           int
	minEntries  int
	tries       int
	choose      int
	iterations  int
	jobs        int
	cache       string
	force       bool
	retryFailed bool
}

func main() {
	validate.ServeChild()
	var o options
	flag.StringVar(&o.root, "root", "teams", "team library directory")
	flag.IntVar(&o.n, "n", 64, "number of teams the library should hold")
	flag.IntVar(&o.minEntries, "min", 1, "least database entries a team needs")
	flag.IntVar(&o.tries, "tries", 0, "entries to try per team before giving up; 0 tries all")
	flag.IntVar(&o.choose, "choose", 5, "entries that pass validation to compare per team; 0 compares all")
	flag.IntVar(&o.iterations, "iter", 20, "iterations per validation run")
	flag.IntVar(&o.jobs, "j", runtime.NumCPU(), "teams harvested in parallel")
	flag.StringVar(&o.cache, "cache", "", "file to save the database download to, and to read it from when it exists")
	flag.BoolVar(&o.force, "force", false, "replace teams already in the library")
	flag.BoolVar(&o.retryFailed, "retry-failed", false, "try again the teams listed in "+failedFile)
	flag.Parse()
	if err := run(o); err != nil {
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

func run(o options) error {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	var entries []kqm.Entry
	var err error
	if o.cache != "" {
		entries, err = kqm.Cached(ctx, o.cache)
	} else {
		entries, err = kqm.FetchAll(ctx)
	}
	if err != nil {
		return err
	}
	fmt.Printf("database read in %v\n", time.Since(start).Round(time.Second))

	byTeam := map[string]*group{}
	byID := map[string]candidate{}
	rejected := map[string]int{}
	for i := range entries {
		e := &entries[i]
		sp := share.SplitConfig(e.Config)
		if reason := usable(sp); reason != "" {
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
		byID[e.ID] = candidate{e, sp}
	}
	fmt.Printf("%d database entries, %d teams usable\n", len(entries), len(byTeam))
	for _, reason := range slices.Sorted(maps.Keys(rejected)) {
		fmt.Printf("  skipped %4d: %s\n", rejected[reason], reason)
	}

	groups := slices.Collect(maps.Values(byTeam))
	slices.SortFunc(groups, func(a, b *group) int {
		return cmp.Or(cmp.Compare(len(b.candidates), len(a.candidates)), cmp.Compare(a.key, b.key))
	})

	existing, err := existingTeams(o.root)
	if err != nil {
		return err
	}
	failed, err := readFailed(o.root)
	if err != nil {
		return err
	}

	// Choose the teams to harvest in rank order.
	var todo []*group
	held, skippedFailed := 0, 0
	for _, g := range groups {
		if held+len(todo) >= o.n || len(g.candidates) < o.minEntries {
			break
		}
		if dir, ok := existing[g.key]; ok {
			if !o.force {
				if err := backfillSource(dir, byID); err != nil {
					return err
				}
				held++
				continue
			}
			if err := os.RemoveAll(dir); err != nil {
				return err
			}
		}
		if _, ok := failed[g.key]; ok && !o.retryFailed {
			skippedFailed++
			continue
		}
		todo = append(todo, g)
	}
	fmt.Printf("%d teams kept, %d skipped as failed before (-retry-failed), %d to harvest with %d jobs\n", held, skippedFailed, len(todo), o.jobs)

	settings := validate.Settings{Iterations: o.iterations, Warmup: 150, Duration: 450}
	if o.jobs > 1 {
		// Teams run in parallel; each simulation runs on one CPU.
		settings.Workers = 1
	}
	var (
		mu    sync.Mutex
		next  int
		added int
		wg    sync.WaitGroup
	)
	harvestStart := time.Now()
	for range max(o.jobs, 1) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				mu.Lock()
				if next == len(todo) {
					mu.Unlock()
					return
				}
				g := todo[next]
				next++
				mu.Unlock()

				ok, reason, log := harvest(o.root, g, o.tries, o.choose, settings)
				mu.Lock()
				fmt.Print(log)
				if ok {
					added++
					delete(failed, g.key)
				} else {
					failed[g.key] = reason
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if err := writeFailed(o.root, failed); err != nil {
		return err
	}
	fmt.Printf("\nharvested %d of %d teams in %v; the library holds %d teams. %d failed teams are listed in %s.\n",
		added, len(todo), time.Since(harvestStart).Round(time.Second), held+added, len(failed), filepath.Join(o.root, failedFile))
	return nil
}

// harvest imports the team's entry that loops best. It tries the
// entries from the lowest investment up: an entry must pass the
// reference check and then the full check (the looped rotation must
// not stall), and of the first choose entries that pass both it keeps
// the one whose score keeps most of its one pass (validate.Report.Kept,
// counted up to 100%; a lower investment wins within 2%). It returns, for a team that
// fails, the reason the first entry failed, and what to print.
func harvest(root string, g *group, tries, choose int, s validate.Settings) (bool, string, string) {
	cs := slices.Clone(g.candidates)
	slices.SortFunc(cs, func(a, b candidate) int {
		return cmp.Or(
			cmp.Compare(a.entry.Investment(), b.entry.Investment()),
			cmp.Compare(b.entry.LastUpdate, a.entry.LastUpdate),
			cmp.Compare(a.entry.ID, b.entry.ID),
		)
	})
	if tries > 0 {
		cs = cs[:min(tries, len(cs))]
	}
	tmp, err := os.MkdirTemp("", "harvest-")
	if err != nil {
		return false, err.Error(), "error: " + err.Error() + "\n"
	}
	defer os.RemoveAll(tmp)

	type pick struct {
		split share.Split
		team  *library.Team
		entry *kqm.Entry
		r     *validate.Report
	}
	var best *pick
	var failures []string
	passed := 0
	for i, c := range cs {
		if choose > 0 && passed >= choose {
			break
		}
		e := c.entry
		notes := fmt.Sprintf("Harvested from the KQM gcsim database: %d entries use this team; of those that reproduce their DPS, this one keeps the most of it when looped, at the lowest constellations and refinements.", len(g.candidates))
		team, err := c.split.Team([]string{e.Link()}, reference(e), notes)
		if err != nil {
			failures = append(failures, e.ID+": "+err.Error())
			continue
		}
		dir, err := c.split.Write(filepath.Join(tmp, strconv.Itoa(i)), team, true)
		if err != nil {
			failures = append(failures, e.ID+": "+err.Error())
			continue
		}
		r := validate.CheckReferenceIsolated(dir, s)
		if len(r.Failures) == 0 {
			r = validate.CheckIsolated(dir, s)
		}
		if len(r.Failures) > 0 {
			failures = append(failures, e.ID+": "+strings.Join(r.Failures, "; "))
			continue
		}
		passed++
		// Keeping all of the one pass is enough; beyond that, the lower
		// investment wins.
		if best == nil || min(r.Kept(), 1) > min(best.r.Kept(), 1)+0.02 {
			best = &pick{c.split, team, e, r}
		}
	}
	if best != nil {
		if best.r.Unpromotable() == "" {
			best.team.Status = library.StatusSupported
		}
		if _, err := best.split.Write(root, best.team, true); err != nil {
			return false, err.Error(), "error: " + err.Error() + "\n"
		}
		r := best.r
		return true, "", fmt.Sprintf("add  %-46s looped %8.0f  one rotation %8.0f  kept %3.0f%% (source %8.0f, %d of %d entries passed, %s)\n",
			best.team.ID, r.Res.DPS, r.Once.DPS, r.Kept()*100, best.team.Reference.DPS, passed, len(g.candidates), best.entry.ID)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "fail %-46s (%d entries)\n", g.key, len(g.candidates))
	for _, f := range failures {
		fmt.Fprintf(&b, "       %s\n", f)
	}
	reason := fmt.Sprintf("%d entries tried; first: %s", len(cs), strings.Join(strings.Fields(failures[0]), " "))
	return false, reason[:min(len(reason), 300)], b.String()
}

// reference is the entry's team DPS. The database reports DPS per
// target; Gscouter's score is the team's damage on all targets.
func reference(e *kqm.Entry) *library.Reference {
	dps := e.Summary.DPS * float64(max(e.Summary.TargetCount, 1))
	return &library.Reference{DPS: float64(int64(dps + 0.5)), Computed: e.Updated()}
}

var foreverRe = regexp.MustCompile(`\bwhile\s+(1|true)\b`)

// usable says why an entry cannot be used, or "" if it can. Conditions
// that differ from Gscouter's are fine: the reference check runs the
// entry under its own.
func usable(sp share.Split) string {
	switch {
	case len(sp.Members) != 4:
		return "not four characters"
	case foreverRe.MatchString(sp.Rotation):
		return "rotation loops forever"
	}
	// Gscouter loops the rotation itself, which needs its setup.
	if _, err := config.Loop(sp.Rotation); err != nil {
		return "rotation does not start with active"
	}
	return ""
}

// backfillSource writes source.txt for a kept team harvested before the
// file existed, from its database entry.
func backfillSource(dir string, byID map[string]candidate) error {
	path := filepath.Join(dir, "source.txt")
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	t, err := library.Load(dir)
	if err != nil {
		return err
	}
	for _, src := range t.Sources {
		id, ok := strings.CutPrefix(src, "https://db.kqm.gg/db/")
		if c, found := byID[id]; ok && found {
			return os.WriteFile(path, []byte(c.split.SourceConditions()), 0o644)
		}
	}
	return nil
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

// readFailed reads the failed teams: one per line, the team key, a tab
// and the reason.
func readFailed(root string) (map[string]string, error) {
	out := map[string]string{}
	f, err := os.Open(filepath.Join(root, failedFile))
	if errors.Is(err, fs.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, reason, _ := strings.Cut(line, "\t")
		out[key] = reason
	}
	return out, sc.Err()
}

func writeFailed(root string, failed map[string]string) error {
	var b strings.Builder
	b.WriteString("# Teams cmd/harvest could not import: no database entry reproduced its DPS,\n")
	b.WriteString("# or the imported team failed the full validation (cmd/validate) and was removed.\n")
	b.WriteString("# The next harvest skips them unless run with -retry-failed.\n")
	for _, key := range slices.Sorted(maps.Keys(failed)) {
		fmt.Fprintf(&b, "%s\t%s\n", key, failed[key])
	}
	return os.WriteFile(filepath.Join(root, failedFile), []byte(b.String()), 0o644)
}
