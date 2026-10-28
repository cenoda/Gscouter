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

// collectors are the gcsim stats a score is computed with. summarize
// reads damage (buckets and contributions) and action (failed actions).
// shield is not read but takes part in the fight: on every hit on the
// player it queues a task, which reorders other tasks due the same
// frame, so leaving it out changes the score of teams that are hit
// (source conditions with hurt) and hold a shield, such as Dehya /
// Furina / Albedo / Xilonen. The others only observe the fight, so
// leaving them out changes no result, and saves their time and
// allocations.
var collectors = []string{"damage", "action", "shield"}

type Options struct {
	Iterations int
	Workers    int     // 0 means runtime.NumCPU()
	Warmup     float64 // seconds excluded from the score (T1)
	Seed       int64   // iteration i uses Seed+i, so results are reproducible
	// First is the index of the first iteration to run, for running a
	// share of them elsewhere (RunIterations): iterations First to
	// First+Iterations-1 run.
	First int
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
	// Waits is the same measure for every reason gcsim delays an action
	// (e.g. "insufficient_energy", "insufficient_stamina", "skill_cd").
	Waits map[string]float64
}

// Iteration is the part of one gcsim run that the score needs. Iterations
// run in separate places (browser workers) are merged by Aggregate.
type Iteration struct {
	DPS        float64              `json:"dps"`
	FirstHalf  float64              `json:"first"`  // DPS in the first half of the window
	SecondHalf float64              `json:"second"` // DPS in the second half of the window
	CharDPS    []float64            `json:"chars"`
	Waits      []map[string]float64 `json:"waits"` // seconds within the window, by reason
	Frames     int                  `json:"frames"`
	Names      []string             `json:"names"`
}

// Run simulates cfg and returns its steady-state DPS. The fight duration is
// taken from the config's options line and must be longer than the warmup.
func Run(ctx context.Context, cfg string, opts Options) (*Result, error) {
	its, err := RunIterations(ctx, cfg, opts)
	if err != nil {
		return nil, err
	}
	return Aggregate(its, opts.Warmup), nil
}

// RunIterations simulates opts.Iterations iterations of cfg, from
// opts.First on, and returns each one's share of the score in order.
func RunIterations(ctx context.Context, cfg string, opts Options) ([]Iteration, error) {
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
	simcfg.Settings.CollectStats = collectors
	// A config without a duration runs until its action list ends.
	if simcfg.Settings.Duration > 0 && simcfg.Settings.Duration <= opts.Warmup {
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
	results := make([]Iteration, opts.Iterations)
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
				seed := opts.Seed + int64(opts.First+i)
				res, err := simulate(file, simcfg, gcsl, seed)
				if err != nil {
					errOnce.Do(func() {
						firstErr = fmt.Errorf("iteration %d (seed %d): %w", opts.First+i, seed, err)
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
	return results, nil
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

// Once runs cfg a single time and returns gcsim's full result, for
// inspecting what a rotation does.
func Once(cfg string, seed int64) (*stats.Result, error) {
	file := ast.NewFile()
	simcfg, gcsl, err := parser.New(file, cfg).Parse()
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if len(simcfg.Errors) > 0 {
		return nil, fmt.Errorf("invalid config: %w", errors.Join(simcfg.Errors...))
	}
	res, err := simulate(file, simcfg, gcsl, seed)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

// Talents returns the talent levels (normal attack, skill, burst) gcsim
// gives cfg's characters in a fight, in config order: the base levels
// plus the constellation bonus and passives that raise them, such as
// Tartaglia's and Skirk's, which depend on the party.
func Talents(cfg string) ([][3]int, error) {
	file := ast.NewFile()
	simcfg, gcsl, err := parser.New(file, cfg).Parse()
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if len(simcfg.Errors) > 0 {
		return nil, fmt.Errorf("invalid config: %w", errors.Join(simcfg.Errors...))
	}
	c, err := simulation.NewCore(1, false, simcfg)
	if err != nil {
		return nil, err
	}
	ev, err := eval.NewEvaluator(file, gcsl, c)
	if err != nil {
		return nil, err
	}
	// New sets the characters up and runs their Init, where passives
	// tag the party.
	if _, err := simulation.New(simcfg, ev, c); err != nil {
		return nil, err
	}
	var out [][3]int
	for _, ch := range c.Player.Chars() {
		// gcsim's levels are 0-based indexes into the talent tables.
		out = append(out, [3]int{ch.TalentLvlAttack() + 1, ch.TalentLvlSkill() + 1, ch.TalentLvlBurst() + 1})
	}
	return out, nil
}

// summarize reduces one run to the damage dealt from startBucket onwards.
func summarize(res *stats.Result, startBucket int) Iteration {
	startFrame := startBucket * bucketFrames
	window := float64(res.Duration-startFrame) / 60
	midBucket := (startFrame + res.Duration) / 2 / bucketFrames
	midFrame := midBucket * bucketFrames

	it := Iteration{
		CharDPS: make([]float64, len(res.Characters)),
		Waits:   make([]map[string]float64, len(res.Characters)),
		Frames:  res.Duration,
		Names:   make([]string, len(res.Characters)),
	}

	// cumulative[i] is the team damage dealt in buckets 0..i.
	cumulative := make([]float64, len(res.DamageBuckets))
	var sum float64
	for i, d := range res.DamageBuckets {
		sum += d
		cumulative[i] = sum
		switch {
		case i >= midBucket:
			it.SecondHalf += d
		case i >= startBucket:
			it.FirstHalf += d
		}
	}
	it.DPS = (it.FirstHalf + it.SecondHalf) / window
	it.FirstHalf /= float64(midFrame-startFrame) / 60
	it.SecondHalf /= float64(res.Duration-midFrame) / 60

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
		it.Names[c] = res.Characters[c].Name
		if last >= 0 {
			it.CharDPS[c] = (charCumulative(c, last) - charCumulative(c, startBucket-1)) / window
		}
		it.Waits[c] = map[string]float64{}
		for _, f := range res.Characters[c].FailedActions {
			if overlap := min(f.End, res.Duration) - max(f.Start, startFrame); overlap > 0 {
				it.Waits[c][f.Reason] += float64(overlap) / 60
			}
		}
	}
	return it
}

// Aggregate merges iterations, in iteration order, into a score; warmup
// is the one they were run with.
func Aggregate(its []Iteration, warmup float64) *Result {
	startBucket := int(warmup*60) / bucketFrames
	if len(its) == 0 {
		return &Result{}
	}
	n := float64(len(its))
	r := &Result{Iterations: len(its)}

	var mean, frames, firstHalf, secondHalf float64
	for _, it := range its {
		mean += it.DPS
		frames += float64(it.Frames)
		firstHalf += it.FirstHalf
		secondHalf += it.SecondHalf
	}
	mean /= n
	frames /= n
	var variance float64
	for _, it := range its {
		variance += (it.DPS - mean) * (it.DPS - mean)
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
	for c, name := range its[0].Names {
		ch := Character{Name: name, Waits: map[string]float64{}}
		for _, it := range its {
			ch.DPS += it.CharDPS[c]
			for reason, w := range it.Waits[c] {
				ch.Waits[reason] += w / (n * minutes)
			}
		}
		ch.DPS /= n
		ch.EnergyWait = ch.Waits[action.InsufficientEnergy.String()]
		if mean > 0 {
			ch.Share = ch.DPS / mean
		}
		r.Characters = append(r.Characters, ch)
	}
	return r
}
