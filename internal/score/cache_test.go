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
