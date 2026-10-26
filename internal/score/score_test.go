package score

import (
	"context"
	"math"
	"os"
	"testing"

	"github.com/cenoda/gscouter/internal/config"
	"github.com/genshinsim/gcsim/pkg/gcs/ast"
	"github.com/genshinsim/gcsim/pkg/gcs/parser"
)

func testConfig(t *testing.T, duration float64) string {
	t.Helper()
	build, err := os.ReadFile("../../examples/builds/skirk-yelan-furina-escoffier.txt")
	if err != nil {
		t.Fatal(err)
	}
	rotation, err := os.ReadFile("../../teams/skirk-yelan-furina-escoffier/rotation.gcsl")
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
	res, err := simulate(file, simcfg, gcsl, seed)
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
