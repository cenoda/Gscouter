// Command wazbench times one config on gcsim and on the engine's module
// through the server's runner (wazero), on one share and on four, and
// wazero's compiling (compiler and interpreter) and instantiating
// (docs/engine.md §4). Best of three.
//
// Usage:
//
//	wazbench -wasm gse_wasm.wasm -cfg <config> [-iter 50]
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/tetratelabs/wazero"

	"github.com/cenoda/gscouter/internal/engine"
	"github.com/cenoda/gscouter/internal/score"
	"github.com/cenoda/gscouter/teams"
)

func main() {
	wasm := flag.String("wasm", "", "engine module")
	cfgPath := flag.String("cfg", "", "config")
	n := flag.Int("iter", 50, "iterations")
	flag.Parse()
	mod, _ := os.ReadFile(*wasm)
	raw, _ := os.ReadFile(*cfgPath)
	cfg := string(raw)
	sum := sha256.Sum256(mod)
	e, _, err := engine.New(mod, fmt.Appendf(nil, `{"engine":{"wasm_sha256":%q},"teams":{}}`, hex.EncodeToString(sum[:])), nil, teams.FS, nil)
	if err != nil {
		panic(err)
	}
	ctx := context.Background()
	t := func(name string, f func()) {
		best := time.Hour
		for k := 0; k < 3; k++ {
			s := time.Now()
			f()
			best = min(best, time.Since(s))
		}
		fmt.Printf("%-34s %8.3fs\n", name, best.Seconds())
	}
	rt := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfigCompiler())
	s := time.Now()
	cm, err := rt.CompileModule(ctx, mod)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%-34s %8.3fs\n", "compile (compiler)", time.Since(s).Seconds())
	t("instantiate", func() {
		m, err := rt.InstantiateModule(ctx, cm, wazero.NewModuleConfig().WithName(""))
		if err != nil {
			panic(err)
		}
		m.Close(ctx)
	})
	rti := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfigInterpreter())
	s = time.Now()
	if _, err := rti.CompileModule(ctx, mod); err != nil {
		panic(err)
	}
	fmt.Printf("%-34s %8.3fs\n", "compile (interpreter)", time.Since(s).Seconds())
	r := e.Runner()
	r.(interface{ Compile() error }).Compile()
	for _, w := range []int{1, 4} {
		o := score.Options{Iterations: *n, Warmup: 150, Seed: 1, Workers: w}
		t(fmt.Sprintf("engine %d its, %d shares", *n, w), func() {
			if _, err := r.RunIterations(ctx, cfg, o); err != nil {
				panic(err)
			}
		})
		t(fmt.Sprintf("gcsim %d its, %d workers", *n, w), func() {
			if _, err := score.RunIterations(ctx, cfg, o); err != nil {
				panic(err)
			}
		})
	}
}
