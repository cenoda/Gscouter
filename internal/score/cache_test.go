package score

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// A cached run gives exactly what a new run gives, and is read back
// instead of simulated again.
func TestCacheRepeatsRun(t *testing.T) {
	cfg := testConfig(t, 60)
	opts := Options{Iterations: 3, Warmup: 20, Seed: 1}
	want, err := Run(context.Background(), cfg, opts)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	c := newCache(dir, "gcsim@v1 h1:x")
	for i := range 2 {
		got, err := c.Run(context.Background(), cfg, opts)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("run %d through the cache: %+v, want %+v", i, got, want)
		}
	}
	entries, _ := filepath.Glob(filepath.Join(dir, "*", "*", "*.json"))
	if len(entries) != 1 {
		t.Fatalf("%d cache entries, want 1", len(entries))
	}
	// An entry is used as it is: change it to see it read back.
	if err := os.WriteFile(entries[0], []byte(`[{"dps":1},{"dps":2},{"dps":3}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, _ := c.Run(context.Background(), cfg, opts); got.DPS != 2 {
		t.Errorf("DPS %v after changing the entry, want 2 read from it", got.DPS)
	}
	// Other options, another gcsim or no cache simulate again.
	other := opts
	other.First = 1
	for name, run := range map[string]func() (*Result, error){
		"other options": func() (*Result, error) { return c.Run(context.Background(), cfg, other) },
		"other gcsim":   func() (*Result, error) { return newCache(dir, "gcsim@v2 h1:y").Run(context.Background(), cfg, opts) },
		"no cache":      func() (*Result, error) { return (*Cache)(nil).Run(context.Background(), cfg, opts) },
	} {
		switch got, err := run(); {
		case err != nil:
			t.Errorf("%s: %v", name, err)
		case got.DPS == 2:
			t.Errorf("%s: read the changed entry, want a new simulation", name)
		}
	}
}

// constEngine is an engine whose every iteration deals dps.
type constEngine struct {
	id  string
	dps float64
}

func (e constEngine) ID() string { return e.id }

func (e constEngine) RunIterations(_ context.Context, _ string, opts Options) ([]Iteration, error) {
	its := make([]Iteration, opts.Iterations)
	for i := range its {
		its[i].DPS = e.dps
	}
	return its, nil
}

// A run on an engine is cached under the engine's module: neither
// gcsim's entry nor another module's is read for it, and gcsim's key is
// as it was.
func TestCacheKeysOnEngine(t *testing.T) {
	cfg := testConfig(t, 60)
	opts := Options{Iterations: 3, Warmup: 20, Seed: 1}
	c := newCache(t.TempDir(), "gcsim@v1 h1:x")
	ctx := context.Background()
	want, err := c.Run(ctx, cfg, opts)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range []constEngine{{"module-a", 1}, {"module-b", 2}, {"module-a", 3}} {
		o := opts
		o.Engine = e
		got, err := c.Run(ctx, cfg, o)
		if err != nil {
			t.Fatal(err)
		}
		// module-a's first result is kept and read back for the second.
		if wantDPS := map[string]float64{"module-a": 1, "module-b": 2}[e.id]; got.DPS != wantDPS {
			t.Errorf("engine %s: DPS %v, want %v", e.id, got.DPS, wantDPS)
		}
	}
	if got, _ := c.Run(ctx, cfg, opts); !reflect.DeepEqual(got, want) {
		t.Errorf("gcsim after the engines: %+v, want %+v", got, want)
	}
}

// WithLabel names a context's runs.
func TestLabel(t *testing.T) {
	if LabelOf(WithLabel(context.Background(), "a-b-c-d")) != "a-b-c-d" || LabelOf(context.Background()) != "" {
		t.Error("labels")
	}
}
