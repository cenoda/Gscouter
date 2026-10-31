package main

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cenoda/gscouter/internal/kqm"
	"github.com/cenoda/gscouter/internal/library"
	"github.com/cenoda/gscouter/internal/score"
	"github.com/cenoda/gscouter/internal/share"
	"github.com/cenoda/gscouter/internal/validate"
)

// TestMain serves the checks alternatives runs in child processes,
// which run the test binary.
func TestMain(m *testing.M) {
	validate.ServeChild()
	os.Exit(m.Run())
}

const (
	teamSrc  = "../../teams/hutao-xingqiu-zhongli-yelan"
	baseID   = "B7cTMcwpmp7F" // the team's own entry
	earlyAlt = "fjPG7F9FpwDQ" // its alternative from the database
)

// teamDir copies the committed Hu Tao team, with its alternative, into a
// temporary directory.
func teamDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "hutao-xingqiu-zhongli-yelan")
	if err := copyDir(teamSrc, dir); err != nil {
		t.Fatal(err)
	}
	return dir
}

// configOf rebuilds a gcsim config from a team directory's files.
func configOf(t *testing.T, dir string) string {
	t.Helper()
	var b strings.Builder
	for _, name := range []string{"build.txt", "source.txt", "rotation.gcsl"} {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		b.Write(raw)
		b.WriteString("\n")
	}
	return b.String()
}

// entry makes a database entry of cfg with the given constellations.
func entry(id, cfg string, dps float64, cons map[string]int) candidate {
	e := &kqm.Entry{ID: id, Config: cfg}
	e.Summary.DPS = dps
	e.Summary.TargetCount = 1
	for _, name := range []string{"hutao", "xingqiu", "zhongli", "yelan"} {
		e.Summary.Team = append(e.Summary.Team, struct {
			Name   string `json:"name"`
			Cons   int    `json:"cons"`
			Weapon struct {
				Name   string `json:"name"`
				Refine int    `json:"refine"`
			} `json:"weapon"`
		}{Name: name, Cons: cons[name]})
	}
	return candidate{e, share.SplitConfig(cfg)}
}

// db makes the database of the given entries, with the team's own.
func db(t *testing.T, dir string, cs ...candidate) (*group, map[string]candidate) {
	t.Helper()
	all := append([]candidate{entry(baseID, configOf(t, dir), 68025, map[string]int{"hutao": 1, "xingqiu": 6})}, cs...)
	byID := map[string]candidate{}
	for _, c := range all {
		byID[c.entry.ID] = c
	}
	return &group{key: teamKey(all[0].split.Members), candidates: all}, byID
}

var earlyCons = map[string]int{"hutao": 1, "xingqiu": 6, "yelan": 2, "zhongli": 1}

// earlyEntry is the earlier alternative's entry.
func earlyEntry(t *testing.T, dir string) candidate {
	return entry(earlyAlt, configOf(t, filepath.Join(dir, library.AltDir, earlyAlt)), 103148, earlyCons)
}

// newEntry is an entry at the earlier alternative's constellations with
// another rotation.
func newEntry(t *testing.T, dir string) candidate {
	cfg := configOf(t, filepath.Join(dir, library.AltDir, earlyAlt)) + "hutao attack;\n"
	return entry("NewEntry0001", cfg, 100000, earlyCons)
}

// passing stands in for the checks: every team passes, keeping kept of
// its one pass, the most for the new entry.
func passing(t *testing.T) {
	t.Helper()
	prev := check
	check = func(dir string, _ validate.Settings) *validate.Report {
		team, err := library.Load(dir)
		if err != nil {
			t.Fatal(err)
		}
		kept := 0.9
		if strings.Contains(team.Sources[0], "NewEntry0001") {
			kept = 1.0
		}
		return &validate.Report{
			Dir: dir, Team: team,
			Once:   &score.Result{DPS: team.Reference.DPS},
			Common: &score.Result{DPS: 1000},
			Res:    &score.Result{DPS: 1000 * kept},
		}
	}
	t.Cleanup(func() { check = prev })
}

func read(t *testing.T, dir string) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, e := range entries {
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		files[e.Name()] = string(raw)
	}
	return files
}

// altNames lists alt/ and checks that nothing else was left in the team.
func altNames(t *testing.T, dir string) []string {
	t.Helper()
	var names []string
	entries, _ := os.ReadDir(filepath.Join(dir, library.AltDir))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	all, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range all {
		if strings.HasPrefix(e.Name(), ".") {
			t.Errorf("staging left behind: %s", e.Name())
		}
	}
	return names
}

// unchanged checks that the earlier alternative is still there as it was.
func unchanged(t *testing.T, dir string, want map[string]string) {
	t.Helper()
	got := read(t, filepath.Join(dir, library.AltDir, earlyAlt))
	if len(got) != len(want) {
		t.Fatalf("the alternative holds %d files, want %d", len(got), len(want))
	}
	for name, content := range want {
		if got[name] != content {
			t.Errorf("%s changed", name)
		}
	}
}

// The audit's reproduction: the only new candidate does not parse, with
// the real checks. The earlier alternative stays.
func TestAlternativesKeepsEarlierOnParseError(t *testing.T) {
	dir := teamDir(t)
	want := read(t, filepath.Join(dir, library.AltDir, earlyAlt))
	cfg := strings.Replace(configOf(t, dir), "hutao char lvl=90/90 cons=1", "hutao char lvl=90/90 cons=6", 1)
	cfg = strings.Replace(cfg, "active zhongli;", "active hutao;\nhutao audit_invalid_action;", 1)
	g, byID := db(t, dir, entry("BadEntry0001", cfg, 80000, map[string]int{"hutao": 6, "xingqiu": 6}))

	r := alternatives(dir, g, byID, 5, validate.Settings{Iterations: 1, Warmup: 150, Duration: 450})
	t.Log(r.log)
	if r.written != 0 || r.kept != 1 {
		t.Errorf("written %d, kept %d; want 0, 1", r.written, r.kept)
	}
	for _, s := range []string{"none of 1 entries passed", "audit_invalid_action", "kept " + earlyAlt} {
		if !strings.Contains(r.log, s) {
			t.Errorf("the log does not say %q", s)
		}
	}
	unchanged(t, dir, want)
	if names := altNames(t, dir); len(names) != 1 {
		t.Errorf("alt holds %v", names)
	}
}

// A failed simulation or a timeout in the new candidates' checks, the
// earlier alternative's own entry among them, keeps it.
func TestAlternativesKeepsEarlierOnCheckFailure(t *testing.T) {
	for _, failure := range []string{
		"simulation failed: boom",
		"no result after 2m0s: the rotation probably never ends (an unconditional loop?)",
	} {
		t.Run(failure, func(t *testing.T) {
			dir := teamDir(t)
			want := read(t, filepath.Join(dir, library.AltDir, earlyAlt))
			g, byID := db(t, dir, earlyEntry(t, dir), newEntry(t, dir))
			prev := check
			check = func(dir string, _ validate.Settings) *validate.Report {
				return &validate.Report{Dir: dir, Failures: []string{failure}}
			}
			t.Cleanup(func() { check = prev })

			r := alternatives(dir, g, byID, 5, validate.Settings{Iterations: 1})
			t.Log(r.log)
			if r.written != 0 || r.kept != 1 {
				t.Errorf("written %d, kept %d; want 0, 1", r.written, r.kept)
			}
			if !strings.Contains(r.log, "none of 2 entries passed") || !strings.Contains(r.log, failure) || !strings.Contains(r.log, "kept "+earlyAlt) {
				t.Errorf("the log does not report the failure and what was kept")
			}
			unchanged(t, dir, want)
		})
	}
}

// A new entry that wins at the earlier alternative's constellations
// replaces it; a variant stays.
func TestAlternativesReplaces(t *testing.T) {
	dir := teamDir(t)
	variant := filepath.Join(dir, library.AltDir, "hutao-c6-no-burst")
	if err := copyDir(filepath.Join(dir, library.AltDir, earlyAlt), variant); err != nil {
		t.Fatal(err)
	}
	meta, err := os.ReadFile(filepath.Join(variant, "team.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(variant, "team.yaml"), append(meta, "variant: {drop: burst, char: hutao}\n"...), 0o644); err != nil {
		t.Fatal(err)
	}
	g, byID := db(t, dir, earlyEntry(t, dir), newEntry(t, dir))
	passing(t)

	r := alternatives(dir, g, byID, 5, validate.Settings{Iterations: 1})
	t.Log(r.log)
	if r.written != 1 || r.kept != 0 {
		t.Errorf("written %d, kept %d; want 1, 0", r.written, r.kept)
	}
	if !strings.Contains(r.log, "removed "+earlyAlt) {
		t.Errorf("the log does not name the alternative removed")
	}
	names := altNames(t, dir)
	if strings.Join(names, " ") != "NewEntry0001 hutao-c6-no-burst" {
		t.Errorf("alt holds %v, want the new entry and the variant", names)
	}
	team, err := library.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(team.Alternatives) != 2 {
		t.Fatalf("the team loads with %d alternatives", len(team.Alternatives))
	}
	for _, alt := range team.Alternatives {
		if alt.Variant == nil && (alt.Status != library.StatusSupported || alt.Sources[0] != "https://db.kqm.gg/db/NewEntry0001") {
			t.Errorf("the new alternative: status %s, sources %v", alt.Status, alt.Sources)
		}
	}
}

// Once every group was measured, an earlier alternative whose entry the
// database no longer has is removed.
func TestAlternativesRemovesObsolete(t *testing.T) {
	dir := teamDir(t)
	g, byID := db(t, dir, newEntry(t, dir))
	passing(t)
	r := alternatives(dir, g, byID, 5, validate.Settings{Iterations: 1})
	t.Log(r.log)
	if names := altNames(t, dir); r.written != 1 || strings.Join(names, " ") != "NewEntry0001" {
		t.Errorf("written %d, alt holds %v", r.written, names)
	}
}

func TestAlternativesKeepsEarlierOnWriteFailure(t *testing.T) {
	dir := teamDir(t)
	want := read(t, filepath.Join(dir, library.AltDir, earlyAlt))
	g, byID := db(t, dir, earlyEntry(t, dir), newEntry(t, dir))
	passing(t)
	prev := writeAlt
	writeAlt = func(sp share.Split, to string, team *library.Team) error {
		if err := sp.WriteDir(to, team); err != nil {
			return err
		}
		return errors.New("disk full")
	}
	t.Cleanup(func() { writeAlt = prev })

	r := alternatives(dir, g, byID, 5, validate.Settings{Iterations: 1})
	t.Log(r.log)
	if r.written != 0 || r.kept != 1 {
		t.Errorf("written %d, kept %d; want 0, 1", r.written, r.kept)
	}
	if !strings.Contains(r.log, "disk full") || !strings.Contains(r.log, "left as they were ("+earlyAlt+")") {
		t.Errorf("the log does not report the failure and what was kept")
	}
	unchanged(t, dir, want)
	if names := altNames(t, dir); len(names) != 1 {
		t.Errorf("alt holds %v", names)
	}
}

func TestReplaceAltsKeepsDirOnFailure(t *testing.T) {
	root := filepath.Join(t.TempDir(), library.AltDir)
	write := func(content string) func(string) error {
		return func(dir string) error {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(dir, "f"), []byte(content), 0o644)
		}
	}
	if err := replaceAlts(root, map[string]func(string) error{"a": write("old")}); err != nil {
		t.Fatal(err)
	}
	err := replaceAlts(root, map[string]func(string) error{"a": write("new"), "b": func(string) error { return errors.New("fail") }})
	if err == nil {
		t.Fatal("no error")
	}
	if got := read(t, filepath.Join(root, "a")); got["f"] != "old" {
		t.Errorf("a holds %v, want the old file", got)
	}
	if entries, _ := os.ReadDir(filepath.Dir(root)); len(entries) != 1 {
		t.Errorf("staging left behind: %d entries", len(entries))
	}
	if err := replaceAlts(root, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("an empty set leaves alt/ (%v)", err)
	}
}

// libraryRoot copies the committed Hu Tao team, with its alternative,
// into a temporary library root, and returns the root and the team.
func libraryRoot(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "hutao-xingqiu-zhongli-yelan")
	if err := copyDir(teamSrc, dir); err != nil {
		t.Fatal(err)
	}
	return root, dir
}

// tree reads every file under dir by its path there.
func tree(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		files[rel] = string(raw)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// same checks that dir holds exactly the files want, byte for byte.
func same(t *testing.T, dir string, want map[string]string) {
	t.Helper()
	got := tree(t, dir)
	for name, content := range want {
		if c, ok := got[name]; !ok {
			t.Errorf("%s is gone", name)
		} else if c != content {
			t.Errorf("%s changed", name)
		}
	}
	for name := range got {
		if _, ok := want[name]; !ok {
			t.Errorf("%s was added", name)
		}
	}
}

// noStaging checks that no staging directory was left in the root.
func noStaging(t *testing.T, root string) {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			t.Errorf("staging left behind: %s", e.Name())
		}
	}
}

// harvestForced runs harvest with -force on the root, the database
// being the given entries, read from a local cache.
func harvestForced(t *testing.T, root string, cs ...candidate) {
	t.Helper()
	var entries []kqm.Entry
	for _, c := range cs {
		entries = append(entries, *c.entry)
	}
	raw, err := json.Marshal(entries)
	if err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(t.TempDir(), "kqm.json")
	if err := os.WriteFile(cache, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	o := options{root: root, n: 64, minEntries: 1, choose: 5, iterations: 1, jobs: 1, cache: cache, force: true}
	if err := run(o); err != nil {
		t.Fatal(err)
	}
}

var (
	baseCons  = map[string]int{"hutao": 1, "xingqiu": 6}
	teamKeyHT = teamKey([]string{"hutao", "xingqiu", "zhongli", "yelan"})
)

// newRoot is an entry at the team's constellations with another
// rotation.
func newRoot(t *testing.T, dir string) candidate {
	return entry("NewRoot00001", configOf(t, dir)+"hutao attack;\n", 68025, baseCons)
}

// The audit's reproduction: with -force, the only entry for the team
// does not parse, with the real checks. The team stays as it was, its
// alternatives too.
func TestForceKeepsTeamOnParseError(t *testing.T) {
	root, dir := libraryRoot(t)
	want := tree(t, dir)
	cfg := configOf(t, dir) + "hutao audit_invalid_action;\n"
	harvestForced(t, root, entry("BadEntry0001", cfg, 68025, baseCons))
	same(t, dir, want)
	noStaging(t, root)
	failed, err := readFailed(root)
	if err != nil {
		t.Fatal(err)
	}
	if r := failed[teamKeyHT]; !strings.Contains(r, "audit_invalid_action") {
		t.Errorf("harvest-failed.txt gives %q", r)
	}
}

// A team listed as failed before is skipped before anything is
// measured or changed.
func TestForceSkipsFailedBeforeChanging(t *testing.T) {
	root, dir := libraryRoot(t)
	want := tree(t, dir)
	if err := os.WriteFile(filepath.Join(root, failedFile), []byte(teamKeyHT+"\tearlier failure\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	checks := 0
	prev := check
	check = func(dir string, _ validate.Settings) *validate.Report {
		checks++
		return &validate.Report{Dir: dir, Failures: []string{"must not run"}}
	}
	t.Cleanup(func() { check = prev })
	harvestForced(t, root, entry(baseID, configOf(t, dir), 68025, baseCons))
	if checks != 0 {
		t.Errorf("%d checks ran", checks)
	}
	same(t, dir, want)
}

// A failure to write the new team leaves the old one as it was.
func TestForceKeepsTeamOnWriteFailure(t *testing.T) {
	root, dir := libraryRoot(t)
	want := tree(t, dir)
	passing(t)
	prev := writeTeam
	writeTeam = func(sp share.Split, to string, team *library.Team) error {
		if err := sp.WriteDir(to, team); err != nil {
			return err
		}
		return errors.New("disk full")
	}
	t.Cleanup(func() { writeTeam = prev })
	harvestForced(t, root, newRoot(t, dir))
	same(t, dir, want)
	noStaging(t, root)
}

// A failure to move the staged team in moves the old one back.
func TestReplaceTeamRollsBack(t *testing.T) {
	root, dir := libraryRoot(t)
	want := tree(t, dir)
	team, err := library.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	sp := share.SplitConfig(configOf(t, dir) + "hutao attack;\n")
	prev := rename
	rename = func(from, to string) error {
		// Moving the staged directory in fails; moving the old one
		// back does not.
		if filepath.Base(to) == team.ID && !strings.Contains(from, "-old") {
			return errors.New("rename failed")
		}
		return os.Rename(from, to)
	}
	t.Cleanup(func() { rename = prev })
	if _, err := replaceTeam(root, dir, true, sp, team); err == nil || !strings.Contains(err.Error(), "rename failed") {
		t.Fatalf("error %v", err)
	}
	same(t, dir, want)
	noStaging(t, root)
}

// A passing entry replaces the team: its files are the new entry's, the
// files the old team had besides are gone, and its alternative is
// harvested again from the database.
func TestForceReplaces(t *testing.T) {
	root, dir := libraryRoot(t)
	for _, f := range []string{"obsolete.txt", filepath.Join(library.AltDir, earlyAlt, "obsolete.txt")} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("old"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	early := earlyEntry(t, dir)
	passing(t)
	harvestForced(t, root, newRoot(t, dir), early)
	noStaging(t, root)
	got := tree(t, dir)
	var names []string
	for name := range got {
		names = append(names, filepath.ToSlash(name))
	}
	slices.Sort(names)
	alt := "alt/" + earlyAlt + "/"
	wantNames := []string{alt + "build.txt", alt + "rotation.gcsl", alt + "source.txt", alt + "team.yaml", "build.txt", "rotation.gcsl", "source.txt", "team.yaml"}
	if !slices.Equal(names, wantNames) {
		t.Errorf("the team holds %v, want %v", names, wantNames)
	}
	if !strings.Contains(got["rotation.gcsl"], "hutao attack;") {
		t.Errorf("the rotation is not the new entry's")
	}
	team, err := library.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if team.Sources[0] != "https://db.kqm.gg/db/NewRoot00001" || team.Status != library.StatusSupported {
		t.Errorf("the team: sources %v, status %s", team.Sources, team.Status)
	}
}

// When the replaced team's alternatives cannot be measured again, those
// carried over stay as they were.
func TestForceKeepsAlternativesItCannotMeasure(t *testing.T) {
	root, dir := libraryRoot(t)
	want := read(t, filepath.Join(dir, library.AltDir, earlyAlt))
	early := earlyEntry(t, dir)
	passing(t)
	pass := check
	check = func(dir string, s validate.Settings) *validate.Report {
		if strings.Contains(dir, earlyAlt) {
			return &validate.Report{Dir: dir, Failures: []string{"simulation failed: boom"}}
		}
		return pass(dir, s)
	}
	harvestForced(t, root, newRoot(t, dir), early)
	noStaging(t, root)
	unchanged(t, dir, want)
	raw, err := os.ReadFile(filepath.Join(dir, "rotation.gcsl"))
	if err != nil || !strings.Contains(string(raw), "hutao attack;") {
		t.Errorf("the team was not replaced (%v)", err)
	}
}
