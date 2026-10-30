package score

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/cenoda/gscouter/internal/config"
	"github.com/genshinsim/gcsim/pkg/gcs/ast"
	"github.com/genshinsim/gcsim/pkg/gcs/parser"
	"github.com/genshinsim/gcsim/pkg/stats"
)

func testConfig(t *testing.T, duration float64) string {
	t.Helper()
	build, err := os.ReadFile("testdata/build.txt")
	if err != nil {
		t.Fatal(err)
	}
	rotation, err := os.ReadFile("testdata/rotation.gcsl")
	if err != nil {
		t.Fatal(err)
	}
	return config.Assemble(config.Conditions{Duration: duration, Iterations: 1}, string(build), string(rotation))
}

// totalDamage runs cfg once with seed and returns gcsim's total damage.
func totalDamage(t *testing.T, cfg string, seed int64) float64 {
	t.Helper()
	file := ast.NewFile()
	simcfg, gcsl, err := parser.New(file, cfg).Parse()
	if err != nil {
		t.Fatal(err)
	}
	res, err := simulate(context.Background(), file, simcfg, gcsl, seed)
	if err != nil {
		t.Fatal(err)
	}
	return res.TotalDamage
}

func near(a, b, relTol float64) bool {
	return math.Abs(a-b) <= relTol*math.Max(math.Abs(a), math.Abs(b))
}

// With no warmup the score must equal gcsim's own DPS.
func TestNoWarmupMatchesGcsimDPS(t *testing.T) {
	const duration, seed = 90, 42
	cfg := testConfig(t, duration)
	res, err := Run(context.Background(), cfg, Options{Iterations: 1, Seed: seed})
	if err != nil {
		t.Fatal(err)
	}
	want := totalDamage(t, cfg, seed) / duration
	if !near(res.DPS, want, 1e-9) {
		t.Errorf("DPS = %v, want %v", res.DPS, want)
	}
}

// Cutting the window out of one T2 run must agree with the difference
// method: (damage(T2) - damage(T1)) / (T2 - T1) with the same seed.
func TestWindowMatchesDifferenceMethod(t *testing.T) {
	const t1, t2 = 60, 180
	for _, seed := range []int64{1, 2, 3} {
		res, err := Run(context.Background(), testConfig(t, t2), Options{Iterations: 1, Warmup: t1, Seed: seed})
		if err != nil {
			t.Fatal(err)
		}
		want := (totalDamage(t, testConfig(t, t2), seed) - totalDamage(t, testConfig(t, t1), seed)) / (t2 - t1)
		if !near(res.DPS, want, 1e-9) {
			t.Errorf("seed %d: window DPS = %v, difference method = %v", seed, res.DPS, want)
		}
	}
}

func TestCharacterSharesSumToOne(t *testing.T) {
	res, err := Run(context.Background(), testConfig(t, 120), Options{Iterations: 4, Warmup: 30, Seed: 1})
	if err != nil {
		t.Fatal(err)
	}
	var share, dps float64
	for _, c := range res.Characters {
		share += c.Share
		dps += c.DPS
	}
	if !near(share, 1, 1e-6) || !near(dps, res.DPS, 1e-6) {
		t.Errorf("shares sum to %v and character DPS to %v, want 1 and %v", share, dps, res.DPS)
	}
}

func TestSameSeedIsReproducible(t *testing.T) {
	cfg := testConfig(t, 90)
	opts := Options{Iterations: 8, Warmup: 30, Seed: 7}
	a, err := Run(context.Background(), cfg, opts)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Run(context.Background(), cfg, opts)
	if err != nil {
		t.Fatal(err)
	}
	if a.DPS != b.DPS {
		t.Errorf("same seeds gave %v and %v", a.DPS, b.DPS)
	}
}

// idleConfig is a fight without a duration that deals no damage, so
// neither its action list nor the enemy's HP ever ends it.
func idleConfig(t *testing.T) string {
	t.Helper()
	build, err := os.ReadFile("testdata/build.txt")
	if err != nil {
		t.Fatal(err)
	}
	return config.Assemble(config.Conditions{Iterations: 1}, string(build), "active escoffier;\nwhile 1 { wait(60); }\n")
}

// A fight that would never end stops at maxFightFrames.
func TestFightLengthBounded(t *testing.T) {
	res, err := Once(context.Background(), idleConfig(t), 1)
	if err != nil {
		t.Fatal(err)
	}
	if res.Duration != maxFightFrames {
		t.Errorf("fight ran %d frames, want %d", res.Duration, maxFightFrames)
	}
}

// A cancelled score stops the fights it is running, not only the ones
// it has yet to start.
func TestRunStopsOnCancel(t *testing.T) {
	cfg := idleConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := Run(ctx, cfg, Options{Iterations: 2, Workers: 1})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want %v", err, context.DeadlineExceeded)
	}
	if took := time.Since(start); took > 300*time.Millisecond {
		t.Errorf("returned after %v", took)
	}
}

// A loop of actions that take no time never passes a frame, where a
// cancel is seen, so the fight stops it itself.
func TestRunStopsStillLoop(t *testing.T) {
	build, err := os.ReadFile("testdata/build.txt")
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Assemble(config.Conditions{Iterations: 1}, string(build), "active escoffier;\nwhile 1 { escoffier walk[f=0]; }\n")
	done := make(chan error, 1)
	go func() {
		_, err := Run(context.Background(), cfg, Options{Iterations: 1, Workers: 1})
		done <- err
	}()
	select {
	case err := <-done:
		if !errors.Is(err, ErrStuck) {
			t.Errorf("err = %v, want %v", err, ErrStuck)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("still running after 10 s")
	}
}

// A trace stops when its context is done: the server's timelines hold a
// simulation slot while they run.
func TestTraceStopsOnCancel(t *testing.T) {
	cfg := idleConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, _, err := Trace(ctx, cfg, 1, 0)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want %v", err, context.DeadlineExceeded)
	}
	if took := time.Since(start); took > 300*time.Millisecond {
		t.Errorf("returned after %v", took)
	}
}

// A trace with a limit ends at the first whole second from it, without
// an error.
func TestTraceLimit(t *testing.T) {
	res, actions, err := Trace(context.Background(), idleConfig(t), 1, 90*60+1)
	if err != nil {
		t.Fatal(err)
	}
	if res.Duration != 91*60 {
		t.Errorf("fight ran %d frames, want %d", res.Duration, 91*60)
	}
	if len(actions) != 4 {
		t.Errorf("actions of %d characters, want 4", len(actions))
	}
}

// Iterations run in slices elsewhere, sent as JSON and merged, must give
// exactly the score of one run: browsers split a score across workers.
func TestSlicesMergeToRun(t *testing.T) {
	cfg := testConfig(t, 60)
	opts := Options{Iterations: 7, Warmup: 20, Seed: 3, Workers: 2}
	want, err := Run(context.Background(), cfg, opts)
	if err != nil {
		t.Fatal(err)
	}
	var its []Iteration
	for first := 0; first < opts.Iterations; first += 3 {
		o := opts
		o.First, o.Iterations = first, min(3, opts.Iterations-first)
		part, err := RunIterations(context.Background(), cfg, o)
		if err != nil {
			t.Fatal(err)
		}
		b, err := json.Marshal(part)
		if err != nil {
			t.Fatal(err)
		}
		var back []Iteration
		if err := json.Unmarshal(b, &back); err != nil {
			t.Fatal(err)
		}
		its = append(its, back...)
	}
	if got := Aggregate(its, opts.Warmup); !reflect.DeepEqual(got, want) {
		t.Errorf("merged slices = %+v\nwant %+v", got, want)
	}
}

// talents are the talent levels of Details.
func talents(t *testing.T, cfg string) [][3]int {
	t.Helper()
	ds, err := Details(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var out [][3]int
	for _, d := range ds {
		out = append(out, d.Talents)
	}
	return out
}

func TestTalents(t *testing.T) {
	cfg := testConfig(t, 60)
	// Skirk's passive raises the party's skill by 1 in a Hydro/Cryo
	// party.
	got := talents(t, cfg)
	want := [][3]int{{9, 10, 9}, {1, 10, 9}, {1, 10, 9}, {1, 10, 9}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Talents = %v, want %v", got, want)
	}

	// Skirk's C3 raises her burst by 3, Furina's C3 hers and C5 her
	// skill.
	cfg = strings.Replace(cfg, "skirk char lvl=90/90 cons=0", "skirk char lvl=90/90 cons=3", 1)
	cfg = strings.Replace(cfg, "furina char lvl=90/90 cons=0", "furina char lvl=90/90 cons=6", 1)
	got = talents(t, cfg)
	want = [][3]int{{9, 10, 12}, {1, 10, 9}, {1, 13, 12}, {1, 10, 9}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Talents with constellations = %v, want %v", got, want)
	}
}

// Details' stats are the character screen's: base, ascension and weapon
// stats on top of the artifacts', with always-on set bonuses.
func TestDetailStats(t *testing.T) {
	ds, err := Details(testConfig(t, 60))
	if err != nil {
		t.Fatal(err)
	}
	skirk := ds[0].Stats
	for _, c := range []struct {
		key  string
		want float64
	}{
		{"cr", 0.05 + 0.35 + 0.2205}, // base, artifacts, Azurelight's substat
		{"cd", 0.5 + 1.122 + 0.384},  // base, artifacts, ascension
		{"er", 1 + 0.11},             // the base 100% and the artifacts'
		{"cryo%", 0.466 + 0.15},      // goblet and Finale of the Deep Galleries 2
	} {
		if got := skirk[c.key]; !near(got, c.want, 1e-3) {
			t.Errorf("Skirk %s = %v, want %v", c.key, got, c.want)
		}
	}
	// HP, ATK and DEF are flat totals: base and weapon ATK (about 1,000
	// for a level-90 character with a 5-star weapon) grown by ATK%.
	if atk := skirk["atk"]; atk < 1800 || atk > 2200 {
		t.Errorf("Skirk atk = %v, want about 2,000", atk)
	}
	for _, key := range []string{"atk%", "hp%", "def%", "base_atk", "dmg%"} {
		if v, ok := skirk[key]; ok {
			t.Errorf("Skirk has %s = %v, want it left out", key, v)
		}
	}
	// Emblem of Severed Fate 2 and Favonius Warbow's substat.
	if got, want := ds[1].Stats["er"], 1+0.618+0.2+0.613; !near(got, want, 1e-3) {
		t.Errorf("Yelan er = %v, want %v", got, want)
	}
}

// finite reports whether every number in it is finite: JSON, which the
// browser's workers send iterations in, has no NaN or infinity.
func finite(it Iteration) bool {
	nums := append([]float64{it.DPS, it.FirstHalf, it.SecondHalf}, it.CharDPS...)
	for _, w := range it.Waits {
		for _, v := range w {
			nums = append(nums, v)
		}
	}
	for _, v := range nums {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return true
}

// A one-pass chart shorter than a second, too short to split into two
// halves, or that ends before any time passes, scores the same natively
// and sent as JSON from a browser worker.
func TestShortOnePass(t *testing.T) {
	build, err := os.ReadFile("testdata/build.txt")
	if err != nil {
		t.Fatal(err)
	}
	for rotation, scores := range map[string]bool{"active skirk;\nskirk attack;\n": true, "active skirk;\n": false} {
		cfg := config.Assemble(config.Conditions{Iterations: 1}, string(build), rotation)
		opts := Options{Iterations: 2, Seed: 1, Workers: 1}
		its, err := RunIterations(context.Background(), cfg, opts)
		if err != nil {
			t.Fatal(err)
		}
		for _, it := range its {
			if !finite(it) {
				t.Errorf("%q: iteration %+v is not finite", rotation, it)
			}
		}
		b, err := json.Marshal(its)
		if err != nil {
			t.Fatalf("%q: %v", rotation, err)
		}
		var back []Iteration
		if err := json.Unmarshal(b, &back); err != nil {
			t.Fatal(err)
		}
		want := Aggregate(its, opts.Warmup)
		got := Aggregate(back, opts.Warmup)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%q: from JSON %+v\nwant %+v", rotation, got, want)
		}
		for _, v := range []float64{want.DPS, want.StdDev, want.StdErr, want.Window, want.Drift} {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				t.Errorf("%q: result %+v is not finite", rotation, want)
			}
		}
		if scores && (want.DPS <= 0 || want.Drift != 0) {
			t.Errorf("%q: DPS %v, drift %v; want a positive DPS and no drift", rotation, want.DPS, want.Drift)
		}
	}
}

// Iterations whose fight ended before the warmup, which a one-pass
// rotation can, have no scoring window: they score 0 and stay finite.
func TestNoWindow(t *testing.T) {
	res := &stats.Result{Duration: 40, DamageBuckets: []float64{100, 50}, Characters: []stats.CharacterResult{{
		Name:                    "skirk",
		DamageCumulativeContrib: []float64{1, 1},
		FailedActions:           []stats.ActionFailInterval{{Start: 10, End: 40, Reason: "skill_cd"}},
	}}}
	it := summarize(res, 2) // the window starts at frame 60
	if !finite(it) || it.DPS != 0 || it.CharDPS[0] != 0 || len(it.Waits[0]) != 0 {
		t.Errorf("summarize = %+v, want a finite iteration with no score", it)
	}
	r := Aggregate([]Iteration{it}, 1)
	for _, v := range []float64{r.DPS, r.Window, r.Drift, r.Characters[0].DPS, r.Characters[0].Share} {
		if math.IsNaN(v) || math.IsInf(v, 0) || v != 0 {
			t.Errorf("Aggregate = %+v, want 0s", r)
		}
	}
}
