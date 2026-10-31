package main

import (
	"errors"
	"os"
	"path/filepath"
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
