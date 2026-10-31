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
// draft, with the result of its check (validation in team.yaml).
//
// Teams that fail every entry are recorded in <root>/harvest-failed.txt
// and skipped by the next run unless -retry-failed is set.
//
// Each team harvested also gets its alternatives (library.AltDir): for
// every set of constellations above those of the team's entry that the
// database has, the entry chosen the same way among those written for
// it, kept when its rotation differs from the team's and it meets the
// criteria for supported. The score tries them besides the team's own
// rotation and keeps the best (design §5.2). -alternatives only
// harvests them again, for the teams already in the library, and adds
// no team. An earlier alternative is replaced or removed only once its
// constellations' entries were measured; when none of them passes the
// checks (gcsim cannot parse them, the simulation fails or times out) it
// is kept, and the log names it.
//
// An experimental team whose rotation was written from a guide
// (library.Team.Written) is replaced, without -force, once the database
// has an entry for it that passes; until then it is kept as it is.
//
// Usage:
//
//	harvest                                # the 64 most common teams into teams/
//	harvest -n 10000 -cache kqm.json       # every team, reusing a saved download
//	harvest -n 100 -force                  # replace existing teams
//	harvest -n 10000 -cache kqm.json -alternatives  # redo every team's alternatives
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

	"github.com/cenoda/gscouter/internal/batch"
	"github.com/cenoda/gscouter/internal/config"
	"github.com/cenoda/gscouter/internal/kqm"
	"github.com/cenoda/gscouter/internal/library"
	"github.com/cenoda/gscouter/internal/share"
	"github.com/cenoda/gscouter/internal/validate"
)

// failedFile lists, under the library root, the teams that failed.
const failedFile = "harvest-failed.txt"

type options struct {
	root         string
	n            int
	minEntries   int
	tries        int
	choose       int
	iterations   int
	jobs         int
	cache        string
	force        bool
	retryFailed  bool
	alternatives bool
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
	flag.BoolVar(&o.alternatives, "alternatives", false, "only harvest again the alternatives of the teams already in the library")
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
	// replaces is the directory of the team's written rotation, removed
	// when an entry passes.
	replaces string
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
	// alts are the teams whose alternatives are harvested afterwards.
	var alts []altJob
	held, skippedFailed := 0, 0
	for _, g := range groups {
		if held+len(todo) >= o.n || len(g.candidates) < o.minEntries {
			break
		}
		if o.alternatives {
			if ex, ok := existing[g.key]; ok && !ex.team.Written() {
				alts = append(alts, altJob{ex.dir, g})
			}
			continue
		}
		if ex, ok := existing[g.key]; ok {
			// A written rotation gives way to the database's entry, but
			// only once one passes: harvest removes it then.
			if ex.team.Written() {
				g.replaces = ex.dir
			} else if !o.force {
				if err := backfillSource(ex.dir, byID); err != nil {
					return err
				}
				held++
				continue
			} else if err := os.RemoveAll(ex.dir); err != nil {
				return err
			}
		}
		if _, ok := failed[g.key]; ok && !o.retryFailed {
			if g.replaces != "" {
				held++
			}
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

				dir, reason, log := harvest(o.root, g, o.tries, o.choose, settings)
				mu.Lock()
				fmt.Print(log)
				if dir != "" {
					added++
					delete(failed, g.key)
					alts = append(alts, altJob{dir, g})
				} else {
					if g.replaces != "" {
						held++ // the written rotation stays
					}
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

	altStart := time.Now()
	slices.SortFunc(alts, func(a, b altJob) int { return cmp.Compare(a.dir, b.dir) })
	written, withAlts, kept := 0, 0, 0
	for _, ch := range batch.Parallel(len(alts), o.jobs, func(i int) altResult {
		return alternatives(alts[i].dir, alts[i].g, byID, o.choose, settings)
	}) {
		r := <-ch
		fmt.Print(r.log)
		written += r.written
		kept += r.kept
		if r.written > 0 {
			withAlts++
		}
	}
	if len(alts) > 0 {
		fmt.Printf("\nwrote %d alternatives for %d of %d teams in %v.\n", written, withAlts, len(alts), time.Since(altStart).Round(time.Second))
		if kept > 0 {
			fmt.Printf("kept %d earlier alternatives as they were: their entries could not be measured again or written (see the lines above).\n", kept)
		}
	}
	return nil
}

// harvest imports the team's entry that loops best and returns its
// directory, "" for a team that fails. It tries the
// entries from the lowest investment up: an entry must pass the
// reference check and then the full check (the looped rotation must
// not stall), and of the first choose entries that pass both it keeps
// the one whose score keeps most of its one pass (validate.Report.Kept,
// counted up to 100%; a lower investment wins within 2%). It returns, for a team that
// fails, the reason the first entry failed, and what to print.
func harvest(root string, g *group, tries, choose int, s validate.Settings) (string, string, string) {
	cs := slices.Clone(g.candidates)
	sortCandidates(cs)
	if tries > 0 {
		cs = cs[:min(tries, len(cs))]
	}
	tmp, err := os.MkdirTemp("", "harvest-")
	if err != nil {
		return "", err.Error(), "error: " + err.Error() + "\n"
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
		r := check(dir, s)
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
		if g.replaces != "" {
			// The entry's directory may be named in another member order.
			if err := os.RemoveAll(g.replaces); err != nil {
				return "", err.Error(), "error: " + err.Error() + "\n"
			}
		}
		best.team.Validation = best.r.Validation(s.Iterations)
		if best.team.Validation.Draft == "" {
			best.team.Status = library.StatusSupported
		}
		dir, err := best.split.Write(root, best.team, true)
		if err != nil {
			return "", err.Error(), "error: " + err.Error() + "\n"
		}
		r := best.r
		return dir, "", fmt.Sprintf("add  %-46s looped %8.0f  one rotation %8.0f  kept %3.0f%% (source %8.0f, %d of %d entries passed, %s)\n",
			best.team.ID, r.Res.DPS, r.Once.DPS, r.Kept()*100, best.team.Reference.DPS, passed, len(g.candidates), best.entry.ID)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "fail %-46s (%d entries)\n", g.key, len(g.candidates))
	for _, f := range failures {
		fmt.Fprintf(&b, "       %s\n", f)
	}
	reason := fmt.Sprintf("%d entries tried; first: %s", len(cs), strings.Join(strings.Fields(failures[0]), " "))
	return "", reason[:min(len(reason), 300)], b.String()
}

// sortCandidates orders entries from the lowest investment up, the most
// recent first within one.
func sortCandidates(cs []candidate) {
	slices.SortFunc(cs, func(a, b candidate) int {
		return cmp.Or(
			cmp.Compare(a.entry.Investment(), b.entry.Investment()),
			cmp.Compare(b.entry.LastUpdate, a.entry.LastUpdate),
			cmp.Compare(a.entry.ID, b.entry.ID),
		)
	})
}

// altJob is a team whose alternatives are harvested.
type altJob struct {
	dir string
	g   *group
}

type altResult struct {
	written int
	// kept counts the earlier alternatives left as they were because
	// their constellations' entries could not be measured again.
	kept int
	log  string
}

// check runs the reference check and then the full one on the team in
// dir, each in a child process. It is a variable so that tests can stand
// in for the simulation.
var check = func(dir string, s validate.Settings) *validate.Report {
	r := validate.CheckReferenceIsolated(dir, s)
	if len(r.Failures) == 0 {
		r = validate.CheckIsolated(dir, s)
	}
	return r
}

// writeAlt writes an alternative's files into the staged alternatives.
// It is a variable so that tests can make the write fail.
var writeAlt = func(sp share.Split, dir string, t *library.Team) error { return sp.WriteDir(dir, t) }

// earlierAlt is an alternative the team has from an earlier harvest.
type earlierAlt struct {
	name    string // its directory under alt/
	profile string // the constellations of its entry now, "" if the database no longer has it as one
}

// profileOf names the constellations of an entry, as alternatives
// groups entries.
func profileOf(e *kqm.Entry) string {
	cons := entryCons(e)
	var profile []string
	for _, k := range slices.Sorted(maps.Keys(cons)) {
		profile = append(profile, fmt.Sprintf("%s C%d", k, cons[k]))
	}
	return strings.Join(profile, ", ")
}

// alternatives replaces the team's alternatives (library.AltDir) with
// those the database has: the entries with a member at a higher
// constellation than in the entry the team's rotation came from, grouped
// by their constellations. Each group's entries are tried as harvest
// tries a team's, at most choose passing, and the one that keeps most
// of its one pass is written, if its rotation is not one the team has
// already and it meets the criteria for supported.
//
// An earlier alternative is removed only on a decision: its group's
// entries were measured and another won, or every group was measured
// and it is obsolete (none is kept for its constellations: the same
// rotation as the team's, not supported, experimental; or the database
// no longer has its entry among the team's higher constellations). When
// none of some group's entries passes the checks (gcsim cannot parse
// them, the simulation fails or times out), every earlier alternative
// no pick replaces is kept as it is, and so is every alternative of a
// team whose own entry is not in the database. The log names those kept
// and those removed.
// The new set is written in full beside alt/ and swapped in only then,
// so a write failure leaves the earlier set as it was. Variants
// (library.Variant, cmd/variant) are always kept.
func alternatives(dir string, g *group, byID map[string]candidate, choose int, s validate.Settings) altResult {
	team, err := library.Load(dir)
	if err != nil {
		return altResult{log: fmt.Sprintf("alt  %-46s error: %v; its alternatives are left as they were\n", filepath.Base(dir), err)}
	}
	altRoot := filepath.Join(dir, library.AltDir)
	var variants []string
	var earlier []earlierAlt
	sources := map[string]string{} // the earlier alternatives' entries by name
	dirs, err := os.ReadDir(altRoot)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return altResult{log: fmt.Sprintf("alt  %-46s error: %v; its alternatives are left as they were\n", team.ID, err)}
	}
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		alt, err := library.Load(filepath.Join(altRoot, d.Name()))
		if err != nil {
			return altResult{log: fmt.Sprintf("alt  %-46s error: %v; its alternatives are left as they were\n", team.ID, err)}
		}
		if alt.Variant != nil {
			variants = append(variants, d.Name())
			continue
		}
		earlier = append(earlier, earlierAlt{name: d.Name()})
		sources[d.Name()] = d.Name()
		if len(alt.Sources) > 0 {
			sources[d.Name()], _ = strings.CutPrefix(alt.Sources[0], "https://db.kqm.gg/db/")
		}
	}
	earlierNames := func() string {
		var names []string
		for _, e := range earlier {
			names = append(names, e.name)
		}
		return strings.Join(names, ", ")
	}
	fail := func(err error) altResult {
		msg := fmt.Sprintf("alt  %-46s error: %v", team.ID, err)
		if len(earlier) > 0 {
			msg += "; kept " + earlierNames()
		}
		return altResult{kept: len(earlier), log: msg + "\n"}
	}
	keepAll := func(why string) altResult {
		if len(earlier) == 0 {
			return altResult{}
		}
		return altResult{kept: len(earlier), log: fmt.Sprintf("alt  %-46s %s; kept %s\n", team.ID, why, earlierNames())}
	}
	if len(team.Sources) == 0 {
		return keepAll("no source entry")
	}
	id, _ := strings.CutPrefix(team.Sources[0], "https://db.kqm.gg/db/")
	base, ok := byID[id]
	if !ok {
		return keepAll("its entry " + id + " is not in the database")
	}
	baseCons := entryCons(base.entry)
	groups := map[string][]candidate{}
	for _, c := range g.candidates {
		cons := entryCons(c.entry)
		if c.entry.ID == id || !slices.ContainsFunc(slices.Collect(maps.Keys(cons)), func(k string) bool { return cons[k] > baseCons[k] }) {
			continue
		}
		key := profileOf(c.entry)
		groups[key] = append(groups[key], c)
	}
	for i, e := range earlier {
		eid := sources[e.name]
		if c, ok := byID[eid]; ok {
			if p := profileOf(c.entry); slices.ContainsFunc(groups[p], func(x candidate) bool { return x.entry.ID == eid }) {
				earlier[i].profile = p
			}
		}
	}

	tmp, err := os.MkdirTemp("", "harvest-alt-")
	if err != nil {
		return fail(err)
	}
	defer os.RemoveAll(tmp)

	type staged struct {
		split share.Split
		team  *library.Team
		entry *kqm.Entry
	}
	var picks []staged
	// undecided are the groups none of whose entries could be measured.
	undecided := map[string]bool{}
	picked := map[string]bool{} // the groups with a pick
	rotations := map[string]bool{normalize(team.Rotation): true}
	var out altResult
	var b strings.Builder
	for n, profile := range slices.Sorted(maps.Keys(groups)) {
		cs := groups[profile]
		sortCandidates(cs)
		type pick struct {
			split share.Split
			team  *library.Team
			entry *kqm.Entry
			r     *validate.Report
		}
		var best *pick
		var failures []string
		passed, same := 0, 0
		for i, c := range cs {
			if choose > 0 && passed >= choose {
				break
			}
			if rotations[normalize(c.split.Rotation)] {
				same++
				continue
			}
			notes := fmt.Sprintf("Harvested from the KQM gcsim database as an alternative for higher constellations (%s): %d entries are at these constellations; of those that reproduce their DPS, this one keeps the most of it when looped, at the lowest refinements.", profile, len(cs))
			alt, err := c.split.Team([]string{c.entry.Link()}, reference(c.entry), notes)
			if err != nil {
				failures = append(failures, c.entry.ID+": "+err.Error())
				continue
			}
			alt.ID, alt.Name = team.ID, team.Name
			tdir := filepath.Join(tmp, fmt.Sprintf("%d-%d", n, i), team.ID, library.AltDir, c.entry.ID)
			if err := c.split.WriteDir(tdir, alt); err != nil {
				return fail(err)
			}
			r := check(tdir, s)
			if len(r.Failures) > 0 {
				failures = append(failures, c.entry.ID+": "+strings.Join(r.Failures, "; "))
				continue
			}
			passed++
			if best == nil || min(r.Kept(), 1) > min(best.r.Kept(), 1)+0.02 {
				best = &pick{c.split, alt, c.entry, r}
			}
		}
		switch {
		case best == nil && same == len(cs):
			fmt.Fprintf(&b, "alt  %-46s %s: same rotation as the team's\n", team.ID, profile)
			continue
		case best == nil:
			undecided[profile] = true
			fmt.Fprintf(&b, "alt  %-46s %s: none of %d entries passed\n", team.ID, profile, len(cs))
			for _, f := range failures {
				fmt.Fprintf(&b, "       %s\n", f)
			}
			continue
		}
		best.team.Validation = best.r.Validation(s.Iterations)
		if d := best.team.Validation.Draft; d != "" {
			fmt.Fprintf(&b, "alt  %-46s %s: %s not kept, %s\n", team.ID, profile, best.entry.ID, d)
			continue
		}
		if best.team.Tier == library.TierExperimental {
			// A person reviews an experimental team before it is
			// supported (design §5.4); an alternative is never reviewed.
			fmt.Fprintf(&b, "alt  %-46s %s: %s not kept, experimental\n", team.ID, profile, best.entry.ID)
			continue
		}
		best.team.Status = library.StatusSupported
		picks = append(picks, staged{best.split, best.team, best.entry})
		picked[profile] = true
		rotations[normalize(best.split.Rotation)] = true
		r := best.r
		fmt.Fprintf(&b, "alt  %-46s %s: %s looped %8.0f  one rotation %8.0f  kept %3.0f%% (%d of %d entries passed)\n",
			team.ID, profile, best.entry.ID, r.Res.DPS, r.Once.DPS, r.Kept()*100, passed, len(cs))
	}

	// The next set: the variants, the picks, and the earlier
	// alternatives no pick replaces when some group could not be
	// measured. An earlier one whose group has a pick lost to it; the
	// others are obsolete (none kept for their constellations, or their
	// entry gone from the database), but only once every group was
	// measured: one that was not may be where an earlier one belongs.
	var kept, lost, obsolete []string
	for _, e := range earlier {
		switch {
		case slices.ContainsFunc(picks, func(p staged) bool { return p.entry.ID == e.name }):
			// written again
		case e.profile != "" && picked[e.profile]:
			lost = append(lost, e.name)
		case len(undecided) > 0:
			kept = append(kept, e.name)
		default:
			obsolete = append(obsolete, e.name)
		}
	}
	if len(picks) > 0 || len(lost) > 0 || len(obsolete) > 0 {
		files := map[string]func(string) error{}
		for _, name := range append(slices.Clone(variants), kept...) {
			files[name] = func(to string) error { return copyDir(filepath.Join(altRoot, name), to) }
		}
		for _, p := range picks {
			files[p.entry.ID] = func(to string) error { return writeAlt(p.split, to, p.team) }
		}
		if err := replaceAlts(altRoot, files); err != nil {
			fmt.Fprintf(&b, "alt  %-46s error: %v; its alternatives are left as they were (%s)\n", team.ID, err, earlierNames())
			return altResult{kept: len(earlier), log: b.String()}
		}
		out.written = len(picks)
	}
	out.kept = len(kept)
	if len(kept) > 0 {
		fmt.Fprintf(&b, "alt  %-46s kept %s: not every group of entries could be measured\n", team.ID, strings.Join(kept, ", "))
	}
	if len(lost) > 0 {
		fmt.Fprintf(&b, "alt  %-46s removed %s: another entry at its constellations was picked\n", team.ID, strings.Join(lost, ", "))
	}
	if len(obsolete) > 0 {
		fmt.Fprintf(&b, "alt  %-46s removed %s: none is kept for its constellations, or the database no longer has its entry\n", team.ID, strings.Join(obsolete, ", "))
	}
	out.log = b.String()
	return out
}

// replaceAlts makes altRoot hold exactly the directories files names,
// each filled by its function. It fills them in a staging directory
// beside altRoot and swaps it in only once every one succeeded, so a
// failure leaves altRoot as it was. An empty set removes altRoot.
func replaceAlts(altRoot string, files map[string]func(dir string) error) error {
	parent := filepath.Dir(altRoot)
	stage, err := os.MkdirTemp(parent, "."+filepath.Base(altRoot)+"-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage) // a no-op once it is swapped in
	if err := os.Chmod(stage, 0o755); err != nil {
		return err
	}
	for _, name := range slices.Sorted(maps.Keys(files)) {
		if err := files[name](filepath.Join(stage, name)); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	old := ""
	if _, err := os.Lstat(altRoot); err == nil {
		old = stage + "-old"
		if err := os.Rename(altRoot, old); err != nil {
			return err
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if len(files) > 0 {
		if err := os.Rename(stage, altRoot); err != nil {
			if old != "" {
				if rerr := os.Rename(old, altRoot); rerr != nil {
					return fmt.Errorf("%w (the earlier alternatives are left in %s: %v)", err, old, rerr)
				}
			}
			return err
		}
	}
	if old != "" {
		return os.RemoveAll(old)
	}
	return nil
}

// copyDir copies the files of the directory from into a new directory to.
func copyDir(from, to string) error {
	entries, err := os.ReadDir(from)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(to, 0o755); err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			if err := copyDir(filepath.Join(from, e.Name()), filepath.Join(to, e.Name())); err != nil {
				return err
			}
			continue
		}
		raw, err := os.ReadFile(filepath.Join(from, e.Name()))
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(to, e.Name()), raw, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// entryCons returns an entry's constellations by canonical name.
func entryCons(e *kqm.Entry) map[string]int {
	out := map[string]int{}
	for _, c := range e.Summary.Team {
		out[library.Canonical(c.Name)] = c.Cons
	}
	return out
}

// normalize collapses a rotation's whitespace, to compare rotations.
func normalize(rotation string) string {
	return strings.Join(strings.Fields(rotation), " ")
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

type existingTeam struct {
	dir  string
	team *library.Team
}

// existingTeams maps the library's teams by member set.
func existingTeams(root string) (map[string]existingTeam, error) {
	out := map[string]existingTeam{}
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
		out[teamKey(t.Members)] = existingTeam{dir, t}
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
	b.WriteString("# A rotation written from a guide for the team stays in the library.\n")
	b.WriteString("# The next harvest skips them unless run with -retry-failed.\n")
	for _, key := range slices.Sorted(maps.Keys(failed)) {
		fmt.Fprintf(&b, "%s\t%s\n", key, failed[key])
	}
	return os.WriteFile(filepath.Join(root, failedFile), []byte(b.String()), 0o644)
}
