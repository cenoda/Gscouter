//go:build !js

package engine

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"runtime"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"

	"github.com/cenoda/gscouter/internal/score"
)

// runner runs the module on wazero, pure Go, as the browser's helper
// workers run it: gse_run_slice on a share of the iterations each,
// merged in order. Iteration i uses seed Seed+i wherever it runs, so the
// server's numbers are the browser's.
type runner struct {
	e    *Engine
	once sync.Once
	rt   wazero.Runtime
	mod  wazero.CompiledModule
	err  error
}

func newRunner(e *Engine) score.Engine { return &runner{e: e} }

func (r *runner) ID() string { return r.e.hash }

// Compile compiles the module ahead of the first run.
func (r *runner) Compile() error { return r.compile() }

// step is how many iterations one call runs. A call cannot be stopped,
// so a cancelled run stops after the calls under way, a few tenths of a
// second. wazero could stop calls (WithCloseOnContextDone), but its
// checks made the engine 3.4 times slower (docs/engine.md §4).
const step = 5

// compile compiles the module once, about a second: on the first run, or
// when the server starts with teams on the engine (web.Server.SetEngine).
func (r *runner) compile() error {
	r.once.Do(func() {
		ctx := context.Background()
		r.rt = wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig())
		r.mod, r.err = r.rt.CompileModule(ctx, r.e.module)
	})
	return r.err
}

// RunIterations runs opts' iterations of cfg in up to opts.Workers
// shares (0 for one per CPU), each on its own instance of the module.
// Any share's error is the run's.
func (r *runner) RunIterations(ctx context.Context, cfg string, opts score.Options) ([]score.Iteration, error) {
	if err := r.compile(); err != nil {
		return nil, fmt.Errorf("engine: compile: %w", err)
	}
	n := opts.Workers
	if n <= 0 {
		n = runtime.NumCPU()
	}
	n = max(1, min(n, opts.Iterations))
	parts := make([][]score.Iteration, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	size, extra := opts.Iterations/n, opts.Iterations%n
	first := opts.First
	for k := range n {
		count := size
		if k < extra {
			count++
		}
		wg.Add(1)
		go func(k, first, count int) {
			defer wg.Done()
			parts[k], errs[k] = r.slice(ctx, cfg, opts.Warmup, opts.Seed, first, count)
		}(k, first, count)
		first += count
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// The shares refuse a config alike: the first error is the run's.
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	out := make([]score.Iteration, 0, opts.Iterations)
	for _, p := range parts {
		out = append(out, p...)
	}
	return out, nil
}

// slice runs iterations first to first+count-1 on a new instance, step
// at a time, and stops between calls when ctx is done.
func (r *runner) slice(ctx context.Context, cfg string, warmup float64, seed int64, first, count int) ([]score.Iteration, error) {
	m, err := r.rt.InstantiateModule(ctx, r.mod, wazero.NewModuleConfig().WithName(""))
	if err != nil {
		return nil, fmt.Errorf("engine: instantiate: %w", err)
	}
	defer m.Close(context.Background())
	out := make([]score.Iteration, 0, count)
	for i := 0; i < count; i += step {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		n := min(step, count-i)
		body, err := call(ctx, m, "gse_run_slice", cfg, api.EncodeF64(warmup), uint64(seed), uint64(uint32(first+i)), uint64(uint32(n)))
		if err != nil {
			return nil, err
		}
		var its []score.Iteration
		if err := json.Unmarshal(body, &its); err != nil {
			return nil, fmt.Errorf("engine: %w", err)
		}
		if len(its) != n {
			return nil, fmt.Errorf("engine: %d iterations, want %d", len(its), n)
		}
		out = append(out, its...)
	}
	return out, nil
}

// Check is gse_check: what cfg needs that the engine lacks, empty when
// the engine can run it.
func (e *Engine) Check(ctx context.Context, cfg string) ([]string, error) {
	r := e.runner.(*runner)
	if err := r.compile(); err != nil {
		return nil, fmt.Errorf("engine: compile: %w", err)
	}
	m, err := r.rt.InstantiateModule(ctx, r.mod, wazero.NewModuleConfig().WithName(""))
	if err != nil {
		return nil, fmt.Errorf("engine: instantiate: %w", err)
	}
	defer m.Close(context.Background())
	body, err := call(ctx, m, "gse_check", cfg)
	if err != nil {
		return nil, err
	}
	var missing []string
	if err := json.Unmarshal(body, &missing); err != nil {
		return nil, fmt.Errorf("engine: %w", err)
	}
	return missing, nil
}

// call is one call of the module (wasm/lib.rs in gscouter-engine): cfg
// goes in through gse_alloc, fn is called with its pointer, its length and
// args, and the result comes back as a u32 status (0 ok), a u32 length
// and that many bytes, the body or the error.
func call(ctx context.Context, m api.Module, fn string, cfg string, args ...uint64) ([]byte, error) {
	alloc, free, freeResult, f := m.ExportedFunction("gse_alloc"), m.ExportedFunction("gse_free"), m.ExportedFunction("gse_free_result"), m.ExportedFunction(fn)
	if alloc == nil || free == nil || freeResult == nil || f == nil {
		return nil, fmt.Errorf("engine: the module does not export %s", fn)
	}
	if len(cfg) > math.MaxUint32 {
		return nil, errors.New("engine: config too long")
	}
	res, err := alloc.Call(ctx, uint64(len(cfg)))
	if err != nil {
		return nil, fmt.Errorf("engine: %w", err)
	}
	ptr := uint32(res[0])
	if !m.Memory().WriteString(ptr, cfg) {
		return nil, errors.New("engine: config out of the module's memory")
	}
	res, err = f.Call(ctx, append([]uint64{uint64(ptr), uint64(len(cfg))}, args...)...)
	if err != nil {
		return nil, fmt.Errorf("engine: %w", err)
	}
	out := uint32(res[0])
	head, ok := m.Memory().Read(out, 8)
	if !ok {
		return nil, errors.New("engine: result out of the module's memory")
	}
	status, n := binary.LittleEndian.Uint32(head), binary.LittleEndian.Uint32(head[4:])
	b, ok := m.Memory().Read(out+8, n)
	if !ok {
		return nil, errors.New("engine: result out of the module's memory")
	}
	body := append([]byte(nil), b...)
	if _, err := freeResult.Call(ctx, uint64(out)); err != nil {
		return nil, fmt.Errorf("engine: %w", err)
	}
	if _, err := free.Call(ctx, uint64(ptr), uint64(len(cfg))); err != nil {
		return nil, fmt.Errorf("engine: %w", err)
	}
	if status != 0 {
		return nil, errors.New(string(body))
	}
	return body, nil
}
