package score

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
)

// gcsimModule is the module whose version decides the results.
const gcsimModule = "github.com/genshinsim/gcsim"

// cacheFormat names how Iteration is computed from a gcsim run. Change
// it whenever summarize or Iteration changes, so that iterations
// cached before are not used.
const cacheFormat = "score-1"

// Cache keeps the iterations RunIterations computed in a directory, so
// a config run again with the same options and the same gcsim is not
// simulated again. Seeds are fixed, so the result is the same as a new
// run's, to the last bit. A nil *Cache caches nothing.
//
// Entries live under a directory per version: of gcsim (with its
// replace directive), Go and this package's cacheFormat. Removing the
// directory clears the cache.
type Cache struct {
	dir string
}

// Gcsim returns the gcsim module this program is built with, after its
// replace directive if any, and whether it is replaced (a fork, design
// §5.4) rather than upstream.
func Gcsim() (module *debug.Module, replaced bool, err error) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return nil, false, errors.New("no build information to tell the gcsim version")
	}
	for _, m := range info.Deps {
		if m.Path != gcsimModule {
			continue
		}
		if m.Replace != nil {
			return m.Replace, true, nil
		}
		return m, false, nil
	}
	return nil, false, errors.New("not built with gcsim")
}

// OpenCache opens the cache in dir for the gcsim this program is built
// with. It returns a nil *Cache, which caches nothing, with an error
// saying why when that gcsim has no version to key the cache with, as
// with a replace directive to a local directory.
func OpenCache(dir string) (*Cache, error) {
	m, _, err := Gcsim()
	if err != nil {
		return nil, err
	}
	if m.Version == "" || m.Sum == "" {
		return nil, fmt.Errorf("gcsim from %s has no version", m.Path)
	}
	return newCache(dir, m.Path+"@"+m.Version+" "+m.Sum), nil
}

// newCache opens the cache in dir for the given gcsim version.
func newCache(dir, gcsim string) *Cache {
	version := hash(cacheFormat, gcsim, runtime.Version(), runtime.GOARCH)
	return &Cache{dir: filepath.Join(dir, version[:16])}
}

func hash(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		// Length-prefixed, so that no two lists hash alike.
		fmt.Fprintf(h, "%d:%s", len(p), p)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Run is score.Run through the cache.
func (c *Cache) Run(ctx context.Context, cfg string, opts Options) (*Result, error) {
	its, err := c.RunIterations(ctx, cfg, opts)
	if err != nil {
		return nil, err
	}
	return Aggregate(its, opts.Warmup), nil
}

// RunIterations is score.RunIterations through the cache. Only results
// are cached: a run that fails runs again next time.
func (c *Cache) RunIterations(ctx context.Context, cfg string, opts Options) ([]Iteration, error) {
	if c == nil {
		return RunIterations(ctx, cfg, opts)
	}
	// Workers does not change the result.
	key := hash(cfg, fmt.Sprint(opts.Iterations, opts.First, opts.Seed), fmt.Sprintf("%b", opts.Warmup))
	path := filepath.Join(c.dir, key[:2], key+".json")
	if raw, err := os.ReadFile(path); err == nil {
		var its []Iteration
		if json.Unmarshal(raw, &its) == nil && len(its) == opts.Iterations {
			return its, nil
		}
	}
	its, err := RunIterations(ctx, cfg, opts)
	if err != nil {
		return nil, err
	}
	// A cache that cannot be written only costs time.
	_ = c.write(path, its)
	return its, nil
}

// write saves its at path through a temporary file, so that another
// process never reads a partial entry.
func (c *Cache) write(path string, its []Iteration) error {
	raw, err := json.Marshal(its)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), path)
}
