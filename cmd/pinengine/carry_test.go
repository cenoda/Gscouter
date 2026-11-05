package main

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cenoda/gscouter/internal/engine"
	"github.com/cenoda/gscouter/internal/score"
	"github.com/cenoda/gscouter/teams"
)

// fakeEngine scores every iteration alike: team DPS dps, split between
// two characters.
type fakeEngine struct {
	dps   float64
	calls *[]score.Options
}

func (f fakeEngine) ID() string { return "fake" }

func (f fakeEngine) RunIterations(_ context.Context, cfg string, opts score.Options) ([]score.Iteration, error) {
	*f.calls = append(*f.calls, opts)
	its := make([]score.Iteration, opts.Iterations)
	for i := range its {
		its[i] = score.Iteration{DPS: f.dps, FirstHalf: f.dps, SecondHalf: f.dps, CharDPS: []float64{f.dps * 0.75, f.dps * 0.25}, Names: []string{"a", "b"}, Frames: 600 * 60, Waits: []map[string]float64{{}, {}}}
	}
	return its, nil
}

const (
	oldModule = "0123456789abcdef"
	newModule = "fedcba9876543210"
	team      = "durin-nicole-xilonen-bennett" // any library team
)

func releases(prevDPS, nextDPS float64, calls *[]score.Options) (*release, *release) {
	mk := func(prefix string, dps float64) *release {
		r := &release{module: []byte(prefix), library: teams.FS, runner: fakeEngine{dps, calls}}
		r.manifest.Engine.Commit = "e-" + prefix
		r.manifest.Engine.WasmSHA256 = prefix + strings.Repeat("0", 48)
		r.manifest.Gscouter = "g-" + prefix
		r.manifest.Teams = map[string]engine.ManifestTeam{team: {Verified: true, Group: "B", Scores: []engine.ManifestScore{{Build: "library", Iterations: 40}}}}
		return r
	}
	prev, next := mk(oldModule, prevDPS), mk(newModule, nextDPS)
	// carry checks the pinned module against its manifest
	prev.module = []byte("old module")
	prev.manifest.Engine.WasmSHA256 = sha(prev.module)
	return prev, next
}

func sha(b []byte) string { return hashOf(b) }

func TestCarryIdentical(t *testing.T) {
	var calls []score.Options
	prev, next := releases(66587.4288375, 66587.4288375, &calls)
	a := engine.Approval{Team: team, Date: "2026-11-04", Release: prev.prefix()}
	carried, dropped, err := carry(context.Background(), []engine.Approval{a, {Team: "group-a", Date: "2026-11-04"}}, prev, next, "2026-11-06")
	if err != nil {
		t.Fatal(err)
	}
	got, ok := carried[team]
	want := team + " 2026-11-04 release=" + newModule + " carried=" + prev.prefix() + ":identical:2026-11-06"
	if !ok || got.String() != want || len(dropped) != 0 || len(carried) != 1 {
		t.Fatalf("carried %v, dropped %v; want %s", carried, dropped, want)
	}
	// both modules ran the verdict's iterations, seed and warmup
	if len(calls) != 2 || calls[0] != calls[1] || calls[0].Iterations != 40 || calls[0].Seed != 1 || calls[0].Warmup != 150 {
		t.Errorf("runs %+v", calls)
	}

	// The list's line is rewritten, the owner's date kept, and it loads.
	dir := t.TempDir()
	list := filepath.Join(dir, "approved.txt")
	os.WriteFile(list, []byte("# header\n"+a.String()+"\n"), 0o644)
	if err := dropApprovals(list, next.manifest, "2026-11-06", carried, dropped); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(list)
	if string(b) != "# header\n"+want+"\n" {
		t.Errorf("list:\n%s", b)
	}
	l, err := engine.ParseApproved(string(b))
	if err != nil || engine.Dropped(next.manifest, l[0]) != "" {
		t.Errorf("carried approval does not hold: %v %v", l, err)
	}
}

func TestCarryDifferentDrops(t *testing.T) {
	var calls []score.Options
	prev, next := releases(66587.4288375, math.Nextafter(66587.4288375, math.Inf(1)), &calls)
	a := engine.Approval{Team: team, Date: "2026-11-04", Release: prev.prefix()}
	carried, dropped, err := carry(context.Background(), []engine.Approval{a}, prev, next, "2026-11-06")
	if err != nil {
		t.Fatal(err)
	}
	if len(carried) != 0 || !strings.Contains(dropped[team], "scores it differently") {
		t.Fatalf("carried %v, dropped %v", carried, dropped)
	}
	dir := t.TempDir()
	list := filepath.Join(dir, "approved.txt")
	os.WriteFile(list, []byte(a.String()+"\n"), 0o644)
	if err := dropApprovals(list, next.manifest, "2026-11-06", carried, dropped); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(list)
	l, _ := engine.ParseApproved(string(b))
	if len(l) != 0 || !strings.Contains(string(b), "not carried: module "+newModule+" scores it differently") {
		t.Errorf("list:\n%s", b)
	}
}

func TestCarryRefusesWithoutOldModule(t *testing.T) {
	var calls []score.Options
	prev, next := releases(1, 1, &calls)
	a := engine.Approval{Team: team, Date: "2026-11-04", Release: prev.prefix()}
	for name, p := range map[string]*release{"no release": nil, "no module": {manifest: prev.manifest}} {
		if _, _, err := carry(context.Background(), []engine.Approval{a}, p, next, "2026-11-06"); err == nil {
			t.Errorf("%s: carried without the old module", name)
		}
	}
	// a module that is not the one the approval was for
	other := *prev
	other.module = []byte("another module")
	other.manifest.Engine.WasmSHA256 = sha(other.module)
	if _, _, err := carry(context.Background(), []engine.Approval{a}, &other, next, "2026-11-06"); err == nil {
		t.Error("carried from another module than the approval's")
	}
	if len(calls) != 0 {
		t.Errorf("ran %d times", len(calls))
	}
	// nothing to carry needs no old module
	if c, d, err := carry(context.Background(), []engine.Approval{{Team: "group-a", Date: "2026-11-04"}}, nil, next, "2026-11-06"); err != nil || len(c)+len(d) != 0 {
		t.Errorf("group A only: %v %v %v", c, d, err)
	}
}
