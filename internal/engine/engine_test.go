package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/cenoda/gscouter/internal/score"
)

func TestParseApproved(t *testing.T) {
	got, err := ParseApproved("# comment\n\nhutao-xingqiu-yelan-zhongli 2026-11-02\n  raiden-national 2026-11-03  \nb-team 2026-11-04 release=0123456789abcdef\n")
	if err != nil {
		t.Fatal(err)
	}
	want := []Approval{{"hutao-xingqiu-yelan-zhongli", "2026-11-02", ""}, {"raiden-national", "2026-11-03", ""}, {"b-team", "2026-11-04", "0123456789abcdef"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	for name, text := range map[string]string{
		"no date":       "hutao-xingqiu-yelan-zhongli\n",
		"bad date":      "hutao-xingqiu-yelan-zhongli 2026-13-01\n",
		"extra word":    "hutao-xingqiu-yelan-zhongli 2026-11-02 yes\n",
		"twice":         "a 2026-11-02\na 2026-11-03\n",
		"short release": "a 2026-11-02 release=0123\n",
		"bad release":   "a 2026-11-02 release=0123456789abcdeg\n",
	} {
		if _, err := ParseApproved(text); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

// The committed list and release load. The list is the owner's to fill
// (docs/engine.md §2): an entry needs no change here.
func TestCommittedFilesLoad(t *testing.T) {
	b, err := os.ReadFile("approved.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseApproved(string(b)); err != nil {
		t.Fatal(err)
	}
	e, skipped, err := Release()
	if err != nil {
		t.Fatal(err)
	}
	if len(skipped) > 0 {
		t.Errorf("approved teams the release leaves on gcsim: %v", skipped)
	}
	if e != nil {
		t.Logf("engine %s at %s: %d teams", e.Hash()[:16], e.Commit(), len(e.Teams()))
	}
}

// fake is testdata/fake.wasm, a stand-in for the engine's module with
// the same calls (testdata/fake/lib.rs).
func fake(t testing.TB) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/fake.wasm")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func manifest(module []byte, teams map[string]bool) []byte {
	sum := sha256.Sum256(module)
	var parts []string
	for id, v := range teams {
		group := ""
		if v {
			group = "A"
		}
		parts = append(parts, fmt.Sprintf("%q: {\"verified\": %v, \"group\": %q}", id, v, group))
	}
	return fmt.Appendf(nil, `{"engine": {"commit": "abc", "wasm_sha256": %q}, "gscouter_commit": "g1", "teams": {%s}}`, hex.EncodeToString(sum[:]), strings.Join(parts, ","))
}

// library is a team library with each team's files, as teams/ has them.
func library(ids ...string) fstest.MapFS {
	l := fstest.MapFS{}
	for _, id := range ids {
		l[id+"/team.yaml"] = &fstest.MapFile{Data: []byte("id: " + id + "\n")}
		l[id+"/rotation.gcsl"] = &fstest.MapFile{Data: []byte("attack;\n")}
	}
	return l
}

// pinned is release/teams.txt for library's teams as they are.
func pinned(t *testing.T, l fs.FS, ids ...string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, id := range ids {
		h, err := TeamHash(l, id)
		if err != nil {
			t.Fatal(err)
		}
		out[id] = h
	}
	return out
}

func TestRouting(t *testing.T) {
	mod := fake(t)
	m := manifest(mod, map[string]bool{"verified-approved": true, "verified": true, "unverified-approved": false, "changed-approved": true, "unpinned-approved": true})
	l := library("verified-approved", "verified", "unverified-approved", "changed-approved", "unpinned-approved")
	pins := pinned(t, l, "verified-approved", "verified", "changed-approved")
	l["changed-approved/alt/x/build.txt"] = &fstest.MapFile{Data: []byte("changed\n")}
	e, skipped, err := New(mod, m, pins, l, []Approval{{Team: "verified-approved", Date: "2026-11-02"}, {Team: "unverified-approved", Date: "2026-11-02"}, {Team: "absent-approved", Date: "2026-11-02"}, {Team: "changed-approved", Date: "2026-11-02"}, {Team: "unpinned-approved", Date: "2026-11-02"}})
	if err != nil {
		t.Fatal(err)
	}
	for team, want := range map[string]bool{"verified-approved": true, "verified": false, "unverified-approved": false, "absent-approved": false, "changed-approved": false, "unpinned-approved": false, "other": false} {
		if got := e.Use(team); got != want {
			t.Errorf("Use(%s) = %v, want %v", team, got, want)
		}
	}
	if want := []string{"unverified-approved: not verified", "absent-approved: not in the manifest", "changed-approved: teams/changed-approved changed since Gscouter g1, which the release verified", "unpinned-approved: no pinned library files (release/teams.txt)"}; !reflect.DeepEqual(skipped, want) {
		t.Errorf("skipped %v, want %v", skipped, want)
	}
	if got := e.Teams(); !reflect.DeepEqual(got, []string{"verified-approved"}) {
		t.Errorf("Teams() = %v", got)
	}
	var none *Engine
	if none.Use("verified-approved") || none.Runner() != nil || none.Teams() != nil {
		t.Error("a nil engine runs teams")
	}
	// An empty list runs nothing on the engine.
	e, _, err = New(mod, m, pins, l, nil)
	if err != nil || e.Use("verified-approved") {
		t.Errorf("empty list: Use = true, err %v", err)
	}
}

func TestModuleMustMatchManifest(t *testing.T) {
	mod := fake(t)
	other := manifest([]byte("another module"), nil)
	if _, _, err := New(mod, other, nil, nil, nil); err == nil {
		t.Error("a module that is not the manifest's loaded")
	}
}

const cfg = "hutao char lvl=90/90 cons=0 talent=9,9,9;\nxingqiu char lvl=90/90 cons=6 talent=9,9,9;\n"

func fakeEngine(t *testing.T) *Engine {
	mod := fake(t)
	e, _, err := New(mod, manifest(mod, nil), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

// The runner splits iterations among instances and merges them in
// order: iteration i has the fake's seed + i DPS however it is split.
func TestRunnerSlices(t *testing.T) {
	e := fakeEngine(t)
	for _, workers := range []int{1, 3, 7, 50} {
		its, err := e.Runner().RunIterations(context.Background(), cfg, score.Options{Iterations: 7, First: 5, Seed: 100, Workers: workers})
		if err != nil {
			t.Fatal(err)
		}
		if len(its) != 7 {
			t.Fatalf("workers %d: %d iterations", workers, len(its))
		}
		for j, it := range its {
			if want := float64(100 + 5 + j); it.DPS != want || !reflect.DeepEqual(it.Names, []string{"hutao", "xingqiu"}) || it.Frames != 6000 {
				t.Errorf("workers %d: iteration %d = %+v, want DPS %v", workers, j, it, want)
			}
		}
	}
}

func TestRunnerRefuses(t *testing.T) {
	e := fakeEngine(t)
	ctx := context.Background()
	if _, err := e.Runner().RunIterations(ctx, cfg+"# refuse\n", score.Options{Iterations: 4, Seed: 1}); err == nil || !strings.Contains(err.Error(), "unsupported: refuse") {
		t.Errorf("refused config: err %v", err)
	}
	// A fight failing in one share fails the run.
	if _, err := e.Runner().RunIterations(ctx, cfg+"# fail at 6\n", score.Options{Iterations: 8, Seed: 1, Workers: 4}); err == nil || !strings.Contains(err.Error(), "iteration 6") {
		t.Errorf("failing fight: err %v", err)
	}
	missing, err := e.Check(ctx, cfg+"# refuse\n")
	if err != nil || !reflect.DeepEqual(missing, []string{"unsupported: refuse"}) {
		t.Errorf("Check = %v, %v", missing, err)
	}
	if missing, err := e.Check(ctx, cfg); err != nil || len(missing) != 0 {
		t.Errorf("Check = %v, %v", missing, err)
	}
}

// A config the engine refuses runs on gcsim (score.RunIterations), all
// of it.
func TestRefusedConfigRunsOnGcsim(t *testing.T) {
	e := fakeEngine(t)
	gcsimCfg := "options iteration=1 duration=20;\ntarget lvl=100 resist=0.1;\nenergy every interval=480,720 amount=1;\n" +
		"xingqiu char lvl=90/90 cons=6 talent=9,9,9;\nxingqiu add weapon=\"sacrificialsword\" refine=1 lvl=90/90;\n" +
		"xingqiu add stats hp=4780 atk=311 atk%=0.466 hydro%=0.466 cr=0.311;\nactive xingqiu;\nwhile 1 { xingqiu attack; }\n"
	opts := score.Options{Iterations: 3, Seed: 1, Workers: 1}
	want, err := score.RunIterations(context.Background(), gcsimCfg, opts)
	if err != nil {
		t.Fatal(err)
	}
	opts.Engine = e.Runner()
	got, err := score.RunIterations(context.Background(), gcsimCfg+"# refuse\n", opts)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].DPS != want[0].DPS || got[0].DPS < 10 {
		t.Errorf("refused config: DPS %v, gcsim's %v", got[0].DPS, want[0].DPS)
	}
	// Accepted, the same config runs on the engine.
	got, err = score.RunIterations(context.Background(), gcsimCfg, opts)
	if err != nil || got[0].DPS != 1 {
		t.Errorf("accepted config: %v, %v; want the fake's DPS 1", got, err)
	}
}

func TestCancel(t *testing.T) {
	e := fakeEngine(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := e.Runner().RunIterations(ctx, cfg, score.Options{Iterations: 4, Seed: 1}); err == nil {
		t.Error("a cancelled run returned results")
	}
}

func TestPinnedFile(t *testing.T) {
	hashes := map[string]string{"b": strings.Repeat("1", 64), "a": strings.Repeat("2", 64)}
	text := FormatPinned("abc123", hashes)
	got, err := ParsePinned(text)
	if err != nil || !reflect.DeepEqual(got, hashes) {
		t.Errorf("ParsePinned(FormatPinned) = %v, %v\n%s", got, err, text)
	}
	if _, err := ParsePinned("a deadbeef\n"); err == nil {
		t.Error("a short hash parsed")
	}
}

// TeamHash covers the files the server embeds and nothing else.
func TestTeamHash(t *testing.T) {
	l := library("a")
	want, _ := TeamHash(l, "a")
	l["a/source.txt"] = &fstest.MapFile{Data: []byte("not scored")}
	l["a/notes/build.txt"] = &fstest.MapFile{Data: []byte("not embedded")}
	if got, _ := TeamHash(l, "a"); got != want {
		t.Error("a file the server does not embed changed the hash")
	}
	for _, name := range []string{"a/build.txt", "a/alt/x/team.yaml"} {
		l := library("a")
		l[name] = &fstest.MapFile{Data: []byte("x")}
		if got, _ := TeamHash(l, "a"); got == want {
			t.Errorf("%s did not change the hash", name)
		}
	}
	if _, err := TeamHash(l, "none"); err == nil {
		t.Error("a team without files hashed")
	}
}

// An approval from group A holds while the team stays in group A; one
// from group B, for its release only.
func TestDropped(t *testing.T) {
	m := Manifest{Teams: map[string]ManifestTeam{
		"a":  {Verified: true, Group: "A"},
		"b":  {Verified: true, Group: "B", GroupWhy: []string{"library +0.70%"}},
		"u":  {Verified: false},
		"no": {Verified: true},
	}}
	m.Engine.WasmSHA256 = "0123456789abcdef" + strings.Repeat("0", 48)
	for _, c := range []struct {
		a    Approval
		want string
	}{
		{Approval{Team: "a"}, ""},
		{Approval{Team: "b"}, "in group B (library +0.70%)"},
		{Approval{Team: "b", Release: "0123456789abcdef"}, ""},
		{Approval{Team: "b", Release: "fedcba9876543210"}, "approved from group B for module fedcba9876543210 only"},
		{Approval{Team: "a", Release: "fedcba9876543210"}, "approved from group B for module fedcba9876543210 only"},
		{Approval{Team: "u"}, "not verified"},
		{Approval{Team: "x"}, "not in the manifest"},
		{Approval{Team: "no"}, "not in group A"},
	} {
		if got := Dropped(m, c.a); got != c.want {
			t.Errorf("%+v: %q, want %q", c.a, got, c.want)
		}
	}
}

// A library-only release's team (verified on its library build alone)
// runs on the engine like any verified team in group A.
func TestLibraryOnlyRelease(t *testing.T) {
	mod := fake(t)
	sum := sha256.Sum256(mod)
	m := fmt.Appendf(nil, `{"engine": {"commit": "abc", "wasm_sha256": %q}, "gscouter_commit": "g1", "kinds": ["library"], "teams": {"a": {"verified": true, "verified_kinds": ["library"], "group": "A"}}}`, hex.EncodeToString(sum[:]))
	l := library("a")
	e, skipped, err := New(mod, m, pinned(t, l, "a"), l, []Approval{{Team: "a", Date: "2026-11-04"}})
	if err != nil || !e.Use("a") || len(skipped) > 0 {
		t.Errorf("library-only: Use = %v, skipped %v, err %v", e.Use("a"), skipped, err)
	}
}

// A config the check accepts but the engine fails on while it runs, by
// an error or by a trap (a panic in the engine, which aborts the module:
// the instance traps, the server does not), runs on gcsim, all of it;
// the runner goes on running other configs.
func TestRuntimeFailureRunsOnGcsim(t *testing.T) {
	e := fakeEngine(t)
	ctx := context.Background()
	gcsimCfg := "options iteration=1 duration=20;\ntarget lvl=100 resist=0.1;\nenergy every interval=480,720 amount=1;\n" +
		"xingqiu char lvl=90/90 cons=6 talent=9,9,9;\nxingqiu add weapon=\"sacrificialsword\" refine=1 lvl=90/90;\n" +
		"xingqiu add stats hp=4780 atk=311 atk%=0.466 hydro%=0.466 cr=0.311;\nactive xingqiu;\nwhile 1 { xingqiu attack; }\n"
	opts := score.Options{Iterations: 4, Seed: 1, Workers: 2}
	want, err := score.RunIterations(ctx, gcsimCfg, opts)
	if err != nil {
		t.Fatal(err)
	}
	opts.Engine = e.Runner()
	for _, marker := range []string{"# fail at 2\n", "# panic at 3\n"} {
		cfg := gcsimCfg + marker
		if missing, err := e.Check(ctx, cfg); err != nil || len(missing) != 0 {
			t.Fatalf("%q: the check refuses it (%v, %v)", marker, missing, err)
		}
		if _, err := e.Runner().RunIterations(ctx, cfg, opts); err == nil {
			t.Errorf("%q: the engine ran it", marker)
		}
		got, err := score.RunIterations(ctx, cfg, opts)
		if err != nil {
			t.Fatalf("%q: %v", marker, err)
		}
		for i := range got {
			if got[i].DPS != want[i].DPS {
				t.Errorf("%q: iteration %d DPS %v, gcsim's %v", marker, i, got[i].DPS, want[i].DPS)
			}
		}
	}
	// After a trap, the engine still runs.
	got, err := score.RunIterations(ctx, gcsimCfg, opts)
	if err != nil || got[0].DPS != 1 {
		t.Errorf("after a trap: %v, %v; want the fake's DPS 1", got, err)
	}
}
