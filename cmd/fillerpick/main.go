// Command fillerpick runs the result page's rotation and filler search
// (steady.Best, with the server's settings: 50 iterations, ranking runs
// on 10, seed 1) on gcsim and on a gscouter-engine module for library
// teams' library builds, and writes where the two pick differently, the
// number of runs each search made, and, where the picks differ, both
// picks on the engine at 500 iterations. docs/engine.md §5 has its
// measurement of 2026-11-04.
//
// Usage:
//
//	fillerpick -wasm gse_wasm.wasm [-sample 400] [-j 4] [-out picks.json]
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"hash/fnv"
	"io/fs"
	"math"
	"os"
	"sort"
	"sync"

	"github.com/cenoda/gscouter/internal/config"
	"github.com/cenoda/gscouter/internal/engine"
	"github.com/cenoda/gscouter/internal/library"
	"github.com/cenoda/gscouter/internal/score"
	"github.com/cenoda/gscouter/internal/steady"
	"github.com/cenoda/gscouter/teams"
)

type refusing struct {
	score.Engine
	mu      sync.Mutex
	refused map[string]string
}

func (r *refusing) RunIterations(ctx context.Context, cfg string, o score.Options) ([]score.Iteration, error) {
	it, err := r.Engine.RunIterations(ctx, cfg, o)
	if err != nil {
		r.mu.Lock()
		r.refused[cfg] = err.Error()
		r.mu.Unlock()
	}
	return it, err
}

type pick struct {
	Team             string
	GIdx, EIdx       int
	GLooped, ELooped string
	GDPS, EDPS       float64 // each engine's winner, on its own engine
	GFilled, EFilled bool
	G500, E500       float64
	GRuns, ERuns     int
	Refused          string
	Err              string
}

// exploreEngine is the ranking runs' iterations on the engine (10 as the
// site ranks; -explore-engine).
var exploreEngine = flag.Int("explore-engine", 10, "iterations of the engine's ranking runs")

func cross(t *library.Team, build string, idx int, looped string, eng score.Engine) float64 {
	return crossN(t, build, idx, looped, eng, 500)
}

// crossN is the DPS of rotation idx's looped script on eng (gcsim for
// nil) at iters iterations.
func crossN(t *library.Team, build string, idx int, looped string, eng score.Engine, iters int) float64 {
	rotations := t.RotationsFor(library.BuildConstellations(build))
	b := build
	for m, p := range rotations[idx].Params {
		b = config.AddParams(b, m, p)
	}
	cfg := config.Assemble(config.Conditions{Iterations: iters, Duration: 450}, b, looped)
	res, err := score.Run(context.Background(), cfg, score.Options{Iterations: iters, Warmup: 150, Seed: 1, Engine: eng})
	if err != nil {
		return -1
	}
	return res.DPS
}

func search(t *library.Team, build string, eng score.Engine, n *int) (int, *steady.Outcome, error) {
	const iters = 50
	explore := 10
	if eng != nil {
		explore = *exploreEngine
	}
	rotations := t.RotationsFor(library.BuildConstellations(build))
	builds := make([]string, len(rotations))
	runs := make([]steady.Rotation, len(rotations))
	for i, r := range rotations {
		builds[i] = build
		for m, p := range r.Params {
			builds[i] = config.AddParams(builds[i], m, p)
		}
		runs[i] = steady.Rotation{Rotation: r.Rotation, Members: r.Members}
	}
	return steady.Best(runs, func(i int, looped string, rank bool) (*score.Result, error) {
		*n++
		n := iters
		if rank {
			n = explore
		}
		cfg := config.Assemble(config.Conditions{Iterations: n, Duration: 450}, builds[i], looped)
		return score.Run(context.Background(), cfg, score.Options{Iterations: n, Warmup: 150, Seed: 1, Engine: eng})
	})
}

func main() {
	wasm := flag.String("wasm", "", "engine module")
	sample := flag.Int("sample", 0, "teams to run (by hash), 0 for all")
	workers := flag.Int("j", 4, "teams at once")
	out := flag.String("out", "picks.json", "")
	judge := flag.String("judge", "", "a picks file: score both picks of each build whose picks differ by more than 1% on the engine (G500/E500) on gcsim at -judge-iter iterations, and print them")
	judgeIter := flag.Int("judge-iter", 4000, "iterations of -judge")
	flag.Parse()
	if *judge != "" {
		judgePicks(*judge, *judgeIter)
		return
	}
	mod, err := os.ReadFile(*wasm)
	if err != nil {
		panic(err)
	}
	sum := sha256.Sum256(mod)
	man := fmt.Sprintf(`{"engine":{"commit":"local","wasm_sha256":%q},"teams":{}}`, hex.EncodeToString(sum[:]))
	e, _, err := engine.New(mod, []byte(man), nil, teams.FS, nil)
	if err != nil {
		panic(err)
	}
	lib, err := library.LoadLibrary(teams.FS)
	if err != nil {
		panic(err)
	}
	var ids []string
	for _, t := range lib.Teams {
		if t.Status == library.StatusBlocked {
			continue
		}
		ids = append(ids, t.ID)
	}
	sort.Slice(ids, func(i, j int) bool {
		hi, hj := fnv.New32a(), fnv.New32a()
		hi.Write([]byte(ids[i]))
		hj.Write([]byte(ids[j]))
		return hi.Sum32() < hj.Sum32()
	})
	if *sample > 0 && *sample < len(ids) {
		ids = ids[:*sample]
	}
	byID := map[string]*library.Team{}
	for _, t := range lib.Teams {
		byID[t.ID] = t
	}
	jobs := make(chan string)
	var mu sync.Mutex
	var picks []pick
	var wg sync.WaitGroup
	for w := 0; w < *workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range jobs {
				t := byID[id]
				p := pick{Team: id}
				raw, err := fs.ReadFile(teams.FS, id+"/build.txt")
				if err != nil {
					p.Err = err.Error()
				} else {
					r := &refusing{Engine: e.Runner(), refused: map[string]string{}}
					gi, go_, gerr := search(t, string(raw), nil, &p.GRuns)
					ei, eo, eerr := search(t, string(raw), r, &p.ERuns)
					switch {
					case gerr != nil || eerr != nil:
						p.Err = fmt.Sprint(gerr, " / ", eerr)
					default:
						p.GIdx, p.EIdx, p.GLooped, p.ELooped = gi, ei, go_.Looped, eo.Looped
						p.GDPS, p.EDPS, p.GFilled, p.EFilled = go_.Result.DPS, eo.Result.DPS, go_.Filled, eo.Filled
						if len(r.refused) == 0 && (gi != ei || go_.Looped != eo.Looped) {
							p.G500 = cross(t, string(raw), gi, go_.Looped, r)
							p.E500 = cross(t, string(raw), ei, eo.Looped, r)
						}
					}
					for _, v := range r.refused {
						p.Refused = v
						break
					}
				}
				mu.Lock()
				picks = append(picks, p)
				n := len(picks)
				mu.Unlock()
				if n%50 == 0 {
					fmt.Fprintln(os.Stderr, n, "teams")
				}
			}
		}()
	}
	for _, id := range ids {
		jobs <- id
	}
	close(jobs)
	wg.Wait()
	b, _ := json.MarshalIndent(picks, "", " ")
	os.WriteFile(*out, b, 0o644)
}

// judgePicks scores on gcsim both picks of each build whose two picks
// differ by more than 1% on the engine.
func judgePicks(path string, iters int) {
	raw, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	var picks []pick
	if err := json.Unmarshal(raw, &picks); err != nil {
		panic(err)
	}
	lib, err := library.LoadLibrary(teams.FS)
	if err != nil {
		panic(err)
	}
	byID := map[string]*library.Team{}
	for _, t := range lib.Teams {
		byID[t.ID] = t
	}
	fmt.Printf("%-48s %10s %10s %8s %8s\n", "team", "gcsim pick", "engine pick", "engine", "±95%")
	for _, p := range picks {
		if p.G500 <= 0 || p.E500 <= 0 || math.Abs(p.E500/p.G500-1) <= 0.01 {
			continue
		}
		t := byID[p.Team]
		build, _ := fs.ReadFile(teams.FS, p.Team+"/build.txt")
		g, gse := crossNSE(t, string(build), p.GIdx, p.GLooped, iters)
		e, ese := crossNSE(t, string(build), p.EIdx, p.ELooped, iters)
		d := e/g - 1
		half := 1.96 * math.Sqrt(gse*gse+ese*ese) / g
		fmt.Printf("%-48s %10.1f %10.1f %+7.2f%% %7.2f%%   (engine at 500: %+.2f%%)\n", p.Team, g, e, 100*d, 100*half, 100*(p.E500/p.G500-1))
	}
}

// crossNSE is crossN on gcsim with the standard error of the mean.
func crossNSE(t *library.Team, build string, idx int, looped string, iters int) (float64, float64) {
	rotations := t.RotationsFor(library.BuildConstellations(build))
	b := build
	for m, p := range rotations[idx].Params {
		b = config.AddParams(b, m, p)
	}
	cfg := config.Assemble(config.Conditions{Iterations: iters, Duration: 450}, b, looped)
	res, err := score.Run(context.Background(), cfg, score.Options{Iterations: iters, Warmup: 150, Seed: 1})
	if err != nil {
		return -1, 0
	}
	return res.DPS, res.StdErr
}
