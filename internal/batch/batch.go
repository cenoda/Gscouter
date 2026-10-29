// Package batch holds what the commands that run over the whole team
// library share (cmd/validate, cmd/weights): listing the team
// directories, splitting them into shards, running them in parallel
// with results in order, and running each one in a child process.
package batch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/cenoda/gscouter/internal/library"
)

// Dirs returns dirs when any are given, else every directory directly
// under root, in name order.
func Dirs(root string, dirs []string) ([]string, error) {
	if len(dirs) > 0 {
		return dirs, nil
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, filepath.Join(root, e.Name()))
		}
	}
	return dirs, nil
}

// WithAlternatives returns dirs with, after each team directory, those of
// its alternatives (library.AltDir), in name order.
func WithAlternatives(dirs []string) ([]string, error) {
	var out []string
	for _, dir := range dirs {
		out = append(out, dir)
		entries, err := os.ReadDir(filepath.Join(dir, library.AltDir))
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		for _, e := range entries {
			if e.IsDir() {
				out = append(out, filepath.Join(dir, library.AltDir, e.Name()))
			}
		}
	}
	return out, nil
}

// Shard returns every n-th of dirs from the k-th on, for spec "k/n"; an
// empty spec returns dirs unchanged.
func Shard(dirs []string, spec string) ([]string, error) {
	if spec == "" {
		return dirs, nil
	}
	var k, n int
	if _, err := fmt.Sscanf(spec, "%d/%d", &k, &n); err != nil || n < 1 || k < 1 || k > n {
		return nil, fmt.Errorf("shard %q: want k/n with 1 <= k <= n", spec)
	}
	var picked []string
	for i := k - 1; i < len(dirs); i += n {
		picked = append(picked, dirs[i])
	}
	return picked, nil
}

// Parallel runs fn for every index below n, jobs at a time, and returns
// one channel per index that receives its result, so results can be
// read in order while later ones still run.
func Parallel[T any](n, jobs int, fn func(i int) T) []chan T {
	out := make([]chan T, n)
	for i := range out {
		out[i] = make(chan T, 1)
	}
	next := make(chan int)
	go func() {
		for i := range n {
			next <- i
		}
		close(next)
	}()
	for range max(jobs, 1) {
		go func() {
			for i := range next {
				out[i] <- fn(i)
			}
		}()
	}
	return out
}

// DefaultCache is the directory for cached simulation results
// (score.Cache) under the user's cache directory, "" if there is none.
// The library commands share it: a result is keyed by its config and
// options, so a run reuses whatever another command already simulated.
func DefaultCache() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "gscouter", "validate")
}

// Serve must be called first thing in main by every command that runs
// work in child processes with Isolated under the environment variable
// env. In such a child it decodes the request, calls handle, writes the
// result to stdout as JSON and exits; otherwise it returns at once.
func Serve(env string, handle func(req []byte) any) {
	v := os.Getenv(env)
	if v == "" {
		return
	}
	TuneGC()
	out := handle([]byte(v))
	if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
		os.Exit(2)
	}
	// Exit at once: a stuck simulation may still be spinning.
	os.Exit(0)
}

// Isolated runs req in a child process of the running command, which
// serves it with Serve under env, and decodes its result into out. A
// rotation that never advances spins inside gcsim, which cannot be
// cancelled; the child's exit at ctx's end stops it, where in this
// process it would keep a CPU busy until the end.
func Isolated(ctx context.Context, env string, req, out any) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, exe)
	cmd.Env = append(os.Environ(), env+"="+string(body))
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	runErr := cmd.Run()
	if err := json.Unmarshal(stdout.Bytes(), out); err != nil {
		return fmt.Errorf("check did not finish: %v %s", runErr, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// TuneGC makes the garbage collector run about a tenth as often as by
// default, unless GOGC or GOMEMLIMIT says otherwise. A fight allocates
// fast but keeps little alive, so with the default the collector takes a
// third of a check's time; this way a check takes 40% less time, for a
// few hundred MB per process (measured 2026-10-27). The soft limit keeps
// a heavy team from growing much beyond that. The collector changes no
// result.
func TuneGC() {
	if os.Getenv("GOGC") == "" {
		debug.SetGCPercent(1000)
	}
	if os.Getenv("GOMEMLIMIT") == "" {
		debug.SetMemoryLimit(1 << 30)
	}
}
