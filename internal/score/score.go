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
	"maps"
	"math"
	"runtime"
	"sync"

	"github.com/genshinsim/gcsim/pkg/core"
	"github.com/genshinsim/gcsim/pkg/core/action"
	"github.com/genshinsim/gcsim/pkg/core/attributes"
	"github.com/genshinsim/gcsim/pkg/core/event"
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
				res, err := simulate(ctx, file, simcfg, gcsl, seed)
				if err != nil {
					if ctx.Err() != nil {
						continue // returned below
					}
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

// maxFightFrames is the longest a fight runs. A config without a
// duration runs until its action list ends or the enemy's 999,999,999 HP
// run out, which a rotation stuck in a loop can put off for weeks of
// fight: one waiting for a Favonius proc that never comes attacks until
// the enemy dies, and with a build that deals no damage it never does.
// The library's one-pass rotations end within 53,000 s, or 15 h, except
// for a draft whose loop never ends.
const maxFightFrames = 100_000 * 60

// simulate runs one iteration. It ends the fight early, as at the end of
// its duration, when ctx is done or the fight reaches maxFightFrames;
// the result is then the fight so far, and the error ctx's.
func simulate(ctx context.Context, file *ast.File, simcfg *info.ActionList, gcsl ast.Node, seed int64) (stats.Result, error) {
	return simulateWith(ctx, file, simcfg, gcsl, seed, maxFightFrames, nil, nil)
}

// simulateWith is simulate, ending the fight at limit frames instead of
// maxFightFrames, calling setup with the fight's core before the fight
// is set up, and running the program through wrap when set.
func simulateWith(ctx context.Context, file *ast.File, simcfg *info.ActionList, gcsl ast.Node, seed int64, limit int, setup func(*core.Core), wrap func(*eval.Eval, *core.Core) action.Evaluator) (stats.Result, error) {
	cfg := simcfg.Copy()
	c, err := simulation.NewCore(seed, false, cfg)
	if err != nil {
		return stats.Result{}, err
	}
	if setup != nil {
		setup(c)
	}
	ev, err := eval.NewEvaluator(file, gcsl.Copy(), c)
	if err != nil {
		return stats.Result{}, err
	}
	var run action.Evaluator = ev
	if wrap != nil {
		run = wrap(ev, c)
	}
	sim, err := simulation.New(cfg, run, c)
	if err != nil {
		return stats.Result{}, err
	}
	// gcsim has no way to stop a fight from outside, so this ends it as a
	// fight with a duration ends: its stop check, run after OnTick, ends
	// the fight on the frame of the duration, outside damage mode.
	stopped := false
	c.Events.Subscribe(event.OnTick, func(...any) {
		if stopped || c.F%60 != 0 || c.F < limit && ctx.Err() == nil {
			return
		}
		stopped = true
		c.Combat.DamageMode = false
		cfg.Settings.Duration = (float64(c.F) + 0.5) / 60
	}, "gscouter-stop")
	res, err := sim.Run()
	if err == nil {
		err = ctx.Err()
	}
	return res, err
}

// Once runs cfg a single time and returns gcsim's full result, for
// inspecting what a rotation does. The fight ends early when ctx is
// done, with ctx's error.
func Once(ctx context.Context, cfg string, seed int64) (*stats.Result, error) {
	res, _, err := Trace(ctx, cfg, seed, 0)
	return res, err
}

// Action is an action a character took, with the params the rotation
// gave it (skill[hold=1] has {"hold": 1}).
type Action struct {
	Frame  int
	Action string // gcsim action name: "skill", "charge", ...
	Params map[string]int
}

// Trace is Once, also returning every action each character took, in
// party order: gcsim's result has the actions but not their params.
// When limit is above 0, the fight ends at the first whole second from
// frame limit on, as at the end of a duration, for a caller that reads
// only its start; the result is then the fight so far.
func Trace(ctx context.Context, cfg string, seed int64, limit int) (*stats.Result, [][]Action, error) {
	file := ast.NewFile()
	simcfg, gcsl, err := parser.New(file, cfg).Parse()
	if err != nil {
		return nil, nil, fmt.Errorf("parse config: %w", err)
	}
	if len(simcfg.Errors) > 0 {
		return nil, nil, fmt.Errorf("invalid config: %w", errors.Join(simcfg.Errors...))
	}
	actions := make([][]Action, len(simcfg.Characters))
	if limit <= 0 || limit > maxFightFrames {
		limit = maxFightFrames
	}
	res, err := simulateWith(ctx, file, simcfg, gcsl, seed, limit, func(c *core.Core) {
		c.Events.Subscribe(event.OnActionExec, func(args ...any) {
			char := args[0].(int)
			a := Action{Frame: c.F, Action: args[1].(action.Action).String()}
			if p, _ := args[2].(map[string]int); len(p) > 0 {
				a.Params = maps.Clone(p)
			}
			actions[char] = append(actions[char], a)
		}, "gscouter-trace")
	}, nil)
	if err != nil {
		return nil, nil, err
	}
	return &res, actions, nil
}

// Detail is what gcsim gives a character before the fight starts.
type Detail struct {
	// Talents are the talent levels (normal attack, skill, burst) in
	// the fight: the base levels plus the constellation bonus and
	// passives that raise them, such as Tartaglia's and Skirk's, which
	// depend on the party.
	Talents [3]int
	// Stats are the total stats, as on the character screen, keyed by
	// gcsim's stat names ("hp", "atk", "er", "cr", "pyro%", ...): base
	// stats, ascension, weapon, artifacts and the set bonuses and
	// passives that are always on. HP, ATK and DEF are flat totals, ER
	// includes the base 100%, CRIT the base 5% and 50%. Zero stats are
	// left out.
	Stats map[string]float64
}

// Details returns what gcsim gives cfg's characters before the fight,
// in config order. It sets the fight up without running it, so it costs
// a fraction of one iteration, and does not depend on the seed.
func Details(cfg string) ([]Detail, error) {
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
	// tag the party and always-on bonuses are added.
	sim, err := simulation.New(simcfg, ev, c)
	if err != nil {
		return nil, err
	}
	// CharacterDetails snapshots each character's stats with an attack
	// of no kind, so only bonuses that apply to every attack count.
	snaps := sim.CharacterDetails()
	var out []Detail
	for i, ch := range c.Player.Chars() {
		// gcsim's levels are 0-based indexes into the talent tables.
		d := Detail{
			Talents: [3]int{ch.TalentLvlAttack() + 1, ch.TalentLvlSkill() + 1, ch.TalentLvlBurst() + 1},
			Stats:   map[string]float64{},
		}
		if i < len(snaps) {
			for s, v := range snaps[i].SnapshotStats {
				if name := attributes.Stat(s).String(); v != 0 && screenStats[name] {
					d.Stats[name] = v
				}
			}
		}
		out = append(out, d)
	}
	return out, nil
}

// screenStats are the stats Detail keeps: those the character screen
// shows. gcsim's snapshot also has attack speed, generic DMG bonus and
// base stats, and the percentages of HP, ATK and DEF, which it folds
// into the flat totals.
var screenStats = map[string]bool{
	"hp": true, "atk": true, "def": true, "em": true, "er": true, "cr": true, "cd": true, "heal": true,
	"pyro%": true, "hydro%": true, "anemo%": true, "electro%": true, "dendro%": true, "cryo%": true, "geo%": true, "phys%": true,
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
