// Package score computes the steady-state DPS of a gcsim config.
//
// Each iteration is simulated for the full duration, then only the damage
// dealt after the warmup (the opener) is counted. gcsim records damage in
// 0.5 s buckets per iteration, so the window is cut at an exact bucket
// boundary instead of being approximated from two separate runs.
package score

import (
	"context"
	"errors"
	"fmt"
	"math"
	"runtime"
	"sync"

	"github.com/genshinsim/gcsim/pkg/core/action"
	"github.com/genshinsim/gcsim/pkg/core/info"
	"github.com/genshinsim/gcsim/pkg/gcs/ast"
	"github.com/genshinsim/gcsim/pkg/gcs/eval"
	"github.com/genshinsim/gcsim/pkg/gcs/parser"
	"github.com/genshinsim/gcsim/pkg/simulation"
	"github.com/genshinsim/gcsim/pkg/stats"
)

// bucketFrames is the width of gcsim's damage buckets (pkg/stats/damage).
const bucketFrames = 30

type Options struct {
	Iterations int
	Workers    int     // 0 means runtime.NumCPU()
	Warmup     float64 // seconds excluded from the score (T1)
	Seed       int64   // iteration i uses Seed+i, so results are reproducible
}

type Result struct {
	DPS    float64 // mean team DPS over the scoring window
	StdDev float64 // standard deviation of DPS across iterations
	StdErr float64 // standard error of DPS
	Window float64 // length of the scoring window in seconds
	// Drift is (second-half DPS - first-half DPS) / first-half DPS over the
	// scoring window. A team that has reached a steady state has a drift
	// near 0.
	Drift      float64
	Iterations int
	Characters []Character
}

type Character struct {
	Name  string
	DPS   float64 // mean DPS over the scoring window
	Share float64 // fraction of team damage over the scoring window
	// EnergyWait is the mean time, in seconds per minute of the scoring
	// window, that this character's actions were delayed waiting for energy.
	EnergyWait float64
}

// iteration is the part of one gcsim run that the score needs.
type iteration struct {
	dps        float64
	firstHalf  float64 // DPS in the first half of the window
	secondHalf float64 // DPS in the second half of the window
	charDPS    []float64
	energyWait []float64 // seconds within the window
	frames     int
	names      []string
}

// Run simulates cfg and returns its steady-state DPS. The fight duration is
// taken from the config's options line and must be longer than the warmup.
func Run(ctx context.Context, cfg string, opts Options) (*Result, error) {
	if opts.Iterations <= 0 {
		return nil, errors.New("iterations must be positive")
	}
	file := ast.NewFile()
	simcfg, gcsl, err := parser.New(file, cfg).Parse()
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if len(simcfg.Errors) > 0 {
		return nil, fmt.Errorf("invalid config: %w", errors.Join(simcfg.Errors...))
	}
	if simcfg.Settings.Duration <= opts.Warmup {
		return nil, fmt.Errorf("duration %vs must be longer than warmup %vs", simcfg.Settings.Duration, opts.Warmup)
	}
	startBucket := int(opts.Warmup*60) / bucketFrames

	workers := opts.Workers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobs := make(chan int)
	results := make([]iteration, opts.Iterations)
	var (
		wg       sync.WaitGroup
		errOnce  sync.Once
		firstErr error
	)
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				res, err := simulate(file, simcfg, gcsl, opts.Seed+int64(i))
				if err != nil {
					errOnce.Do(func() {
						firstErr = fmt.Errorf("iteration %d (seed %d): %w", i, opts.Seed+int64(i), err)
						cancel()
					})
					continue
				}
				results[i] = summarize(&res, startBucket)
			}
		}()
	}
feed:
	for i := range opts.Iterations {
		select {
		case jobs <- i:
		case <-ctx.Done():
			break feed
		}
	}
	close(jobs)
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return aggregate(results, startBucket), nil
}

func simulate(file *ast.File, simcfg *info.ActionList, gcsl ast.Node, seed int64) (stats.Result, error) {
	cfg := simcfg.Copy()
	c, err := simulation.NewCore(seed, false, cfg)
	if err != nil {
		return stats.Result{}, err
	}
	ev, err := eval.NewEvaluator(file, gcsl.Copy(), c)
	if err != nil {
		return stats.Result{}, err
	}
	sim, err := simulation.New(cfg, ev, c)
	if err != nil {
		return stats.Result{}, err
	}
	return sim.Run()
}

// summarize reduces one run to the damage dealt from startBucket onwards.
func summarize(res *stats.Result, startBucket int) iteration {
	startFrame := startBucket * bucketFrames
	window := float64(res.Duration-startFrame) / 60
	midBucket := (startFrame + res.Duration) / 2 / bucketFrames
	midFrame := midBucket * bucketFrames

	it := iteration{
		charDPS:    make([]float64, len(res.Characters)),
		energyWait: make([]float64, len(res.Characters)),
		frames:     res.Duration,
		names:      make([]string, len(res.Characters)),
	}

	// cumulative[i] is the team damage dealt in buckets 0..i.
	cumulative := make([]float64, len(res.DamageBuckets))
	var sum float64
	for i, d := range res.DamageBuckets {
		sum += d
		cumulative[i] = sum
		switch {
		case i >= midBucket:
			it.secondHalf += d
		case i >= startBucket:
			it.firstHalf += d
		}
	}
	it.dps = (it.firstHalf + it.secondHalf) / window
	it.firstHalf /= float64(midFrame-startFrame) / 60
	it.secondHalf /= float64(res.Duration-midFrame) / 60

	// charCumulative returns a character's damage dealt in buckets 0..i.
	charCumulative := func(c, i int) float64 {
		if i < 0 {
			return 0
		}
		i = min(i, len(cumulative)-1)
		return res.Characters[c].DamageCumulativeContrib[i] * cumulative[i]
	}

	last := len(cumulative) - 1
	for c := range res.Characters {
		it.names[c] = res.Characters[c].Name
		if last >= 0 {
			it.charDPS[c] = (charCumulative(c, last) - charCumulative(c, startBucket-1)) / window
		}
		for _, f := range res.Characters[c].FailedActions {
			if f.Reason != action.InsufficientEnergy.String() {
				continue
			}
			if overlap := min(f.End, res.Duration) - max(f.Start, startFrame); overlap > 0 {
				it.energyWait[c] += float64(overlap) / 60
			}
		}
	}
	return it
}

func aggregate(its []iteration, startBucket int) *Result {
	n := float64(len(its))
	r := &Result{Iterations: len(its)}

	var mean, frames, firstHalf, secondHalf float64
	for _, it := range its {
		mean += it.dps
		frames += float64(it.frames)
		firstHalf += it.firstHalf
		secondHalf += it.secondHalf
	}
	mean /= n
	frames /= n
	var variance float64
	for _, it := range its {
		variance += (it.dps - mean) * (it.dps - mean)
	}
	if len(its) > 1 {
		variance /= n - 1
	}
	r.DPS = mean
	r.StdDev = math.Sqrt(variance)
	r.StdErr = r.StdDev / math.Sqrt(n)
	r.Window = (frames - float64(startBucket*bucketFrames)) / 60
	if firstHalf > 0 {
		r.Drift = (secondHalf - firstHalf) / firstHalf
	}

	minutes := r.Window / 60
	for c, name := range its[0].names {
		ch := Character{Name: name}
		for _, it := range its {
			ch.DPS += it.charDPS[c]
			ch.EnergyWait += it.energyWait[c]
		}
		ch.DPS /= n
		ch.EnergyWait /= n * minutes
		if mean > 0 {
			ch.Share = ch.DPS / mean
		}
		r.Characters = append(r.Characters, ch)
	}
	return r
}
