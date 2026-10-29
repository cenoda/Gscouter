package weights

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/cenoda/gscouter/internal/batch"
	"github.com/cenoda/gscouter/internal/config"
	"github.com/cenoda/gscouter/internal/library"
	"github.com/cenoda/gscouter/internal/score"
	"github.com/cenoda/gscouter/internal/steady"
)

type Settings struct {
	Iterations       int // 20 (docs/artifacts.md §5)
	Warmup, Duration float64
	Workers          int // simulations run in parallel per variant; 0 means one per CPU
	// Cache is a directory where simulation results are kept
	// (score.Cache), "" for none.
	Cache string
	// Timeout is how long one team may take before it fails; 0 means
	// 30 minutes.
	Timeout time.Duration
}

func (s Settings) timeout() time.Duration {
	if s.Timeout > 0 {
		return s.Timeout
	}
	return 30 * time.Minute
}

// Result is what cmd/weights writes for one team, one JSON line per
// team.
type Result struct {
	ID  string `json:"id"`
	Dir string `json:"dir"`
	// Skipped says why the team has no weights: blocked, or a build.txt
	// that does not write main stats and substats as ParseBuild needs.
	Skipped string `json:"skipped,omitempty"`
	// Error is a failure: a simulation that failed or ran out of time.
	Error      string `json:"error,omitempty"`
	Iterations int    `json:"iterations,omitempty"`
	// DPS is the reference build's looped score, Filler how it fills
	// waits ("" for the rotation as written); every variant runs that
	// same rotation.
	DPS    float64 `json:"dps,omitempty"`
	Filler string  `json:"filler,omitempty"`
	// Simulations counts the runs: the reference's (one, or more with
	// fillers) and one per variant. Seconds is the team's wall time.
	Simulations int         `json:"simulations"`
	Seconds     float64     `json:"seconds"`
	Characters  []Character `json:"characters,omitempty"`
	// Gcsim is the gcsim module the run was built with, path@version;
	// cmd/weights sets it.
	Gcsim string `json:"gcsim,omitempty"`
}

// Team calculates the weights of the team in dir. Every simulation
// uses seed 1 and the same iterations, so the variants share the
// reference's luck, and runs the looped rotation the reference scored
// best with, fillers included (design §3.1), so that a small change
// cannot switch fillers.
func Team(ctx context.Context, dir string, s Settings) *Result {
	start := time.Now()
	r := &Result{ID: filepath.Base(dir), Dir: dir}
	defer func() { r.Seconds = round(time.Since(start).Seconds(), 1) }()
	team, err := library.Load(dir)
	if err != nil {
		r.Error = err.Error()
		return r
	}
	r.ID = team.ID
	if team.Status == library.StatusBlocked {
		r.Skipped = "blocked"
		return r
	}
	raw, err := os.ReadFile(filepath.Join(dir, "build.txt"))
	if err != nil {
		r.Error = "reference build: " + err.Error()
		return r
	}
	build, err := ParseBuild(string(raw))
	if err != nil {
		r.Skipped = "build.txt: " + err.Error()
		return r
	}
	withParams := func(b string) string {
		for _, m := range slices.Sorted(maps.Keys(team.Params)) {
			b = config.AddParams(b, m, team.Params[m])
		}
		return b
	}
	var cache *score.Cache
	if s.Cache != "" {
		cache, _ = score.OpenCache(s.Cache)
	}
	opts := score.Options{Iterations: s.Iterations, Workers: s.Workers, Warmup: s.Warmup, Seed: 1}
	run := func(b, looped string) (*score.Result, error) {
		r.Simulations++
		cfg := config.Assemble(config.Conditions{Iterations: s.Iterations, Duration: s.Duration}, withParams(b), looped)
		return cache.Run(ctx, cfg, opts)
	}

	ref := build.String()
	out, err := steady.Score(team.Rotation, team.Members, func(looped string) (*score.Result, error) {
		return run(ref, looped)
	})
	if err != nil {
		r.Error = "reference: " + err.Error()
		return r
	}
	r.Iterations = s.Iterations
	r.DPS = round(out.Result.DPS, 1)
	r.Filler = out.String()
	refDPS := out.Result.DPS

	for i, c := range build.Chars {
		withRoll := map[string]float64{}
		var rollUnplayable map[string]string
		for _, roll := range Substats {
			res, err := run(build.WithSub(i, roll.Key, roll.Value), out.Looped)
			switch {
			case err == nil:
				withRoll[roll.Key] = res.DPS
			case ctx.Err() != nil:
				r.Error = fmt.Sprintf("%s with a %s roll: %v", c.Key, roll.Key, err)
				return r
			default:
				// As for main stats below: the roll shifts the timing
				// so that the rotation cannot be played. That substat
				// gets no weight.
				if rollUnplayable == nil {
					rollUnplayable = map[string]string{}
				}
				rollUnplayable[roll.Key] = err.Error()
			}
		}
		var current [3]string
		var withMain [3]map[string]float64
		var unplayable [3]map[string]string
		for slot, options := range Options {
			current[slot] = c.Main[slot].Key
			withMain[slot] = map[string]float64{}
			for _, key := range options {
				if key == current[slot] {
					// The reference build itself.
					withMain[slot][key] = refDPS
					continue
				}
				b, err := build.WithMain(i, slot, key)
				if err != nil {
					r.Error = fmt.Sprintf("%s with a %s %s: %v", c.Key, key, Slots[slot], err)
					return r
				}
				res, err := run(b, out.Looped)
				switch {
				case err == nil:
					withMain[slot][key] = res.DPS
				case ctx.Err() != nil:
					r.Error = fmt.Sprintf("%s with a %s %s: %v", c.Key, key, Slots[slot], err)
					return r
				default:
					// The rotation cannot be played with this main
					// stat (gcsim rejects an action, say for lack of
					// energy). Only this option is left out.
					if unplayable[slot] == nil {
						unplayable[slot] = map[string]string{}
					}
					unplayable[slot][key] = err.Error()
				}
			}
		}
		ch := character(c.Key, c.Reordered, refDPS, withRoll, current, withMain)
		ch.Unplayable = rollUnplayable
		for slot, name := range Slots {
			ch.Main[name].Unplayable = unplayable[slot]
			if len(unplayable[slot]) > 0 {
				// A main stat that breaks the rotation matters.
				ch.Main[name].LowImpact = false
			}
		}
		r.Characters = append(r.Characters, ch)
	}
	TeamPenalties(r.Characters)
	return r
}

// childEnv carries a request to a child process.
const childEnv = "GSCOUTER_WEIGHTS_CHILD"

type childRequest struct {
	Dir      string
	Settings Settings
}

// ServeChild must be called first thing in main by every command that
// uses TeamIsolated.
func ServeChild() {
	batch.Serve(childEnv, func(body []byte) any {
		var req childRequest
		if err := json.Unmarshal(body, &req); err != nil {
			fmt.Fprintln(os.Stderr, "bad request:", err)
			os.Exit(2)
		}
		ctx, cancel := context.WithTimeout(context.Background(), req.Settings.timeout())
		defer cancel()
		done := make(chan *Result, 1)
		go func() { done <- Team(ctx, req.Dir, req.Settings) }()
		select {
		case r := <-done:
			return r
		case <-ctx.Done():
			return &Result{ID: filepath.Base(req.Dir), Dir: req.Dir, Error: fmt.Sprintf("no result after %v", req.Settings.timeout()), Seconds: req.Settings.timeout().Seconds()}
		}
	})
}

// TeamIsolated runs Team in a child process of the running command, so
// that a rotation stuck inside gcsim cannot keep running.
func TeamIsolated(dir string, s Settings) *Result {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout()+30*time.Second)
	defer cancel()
	var r Result
	if err := batch.Isolated(ctx, childEnv, childRequest{dir, s}, &r); err != nil {
		return &Result{ID: filepath.Base(dir), Dir: dir, Error: err.Error()}
	}
	return &r
}
