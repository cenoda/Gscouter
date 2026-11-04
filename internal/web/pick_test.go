package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"strings"
	"testing"

	"github.com/cenoda/gscouter/internal/score"
	"github.com/cenoda/gscouter/internal/timeline"
)

// noisySim stands in for the engine: gcsim's result scaled by a factor
// drawn from the config, as if the engine's noise ranked the search's
// runs otherwise. It records the configs of the engine's runs and of the
// timelines.
type noisySim struct {
	serverSim
	gcsim         []string
	fallbackAt    int // engine call that falls back, zero for none
	fallbackError bool
	engine        []string
	timelines     []string
}

func factor(cfg string) float64 {
	h := fnv.New32a()
	h.Write([]byte(cfg))
	return 0.85 + 0.3*float64(h.Sum32()%1000)/1000
}

func (s *noisySim) Score(ctx context.Context, cfg string, warmup float64, iterations int, engine bool) (*score.Result, error) {
	res, err := s.serverSim.Score(ctx, cfg, warmup, iterations, false)
	if !engine {
		s.gcsim = append(s.gcsim, cfg)
	}
	if err != nil || !engine {
		return res, err
	}
	s.engine = append(s.engine, cfg)
	out := *res
	if len(s.engine) == s.fallbackAt {
		if s.fallbackError {
			return nil, errors.Join(errors.New("retry failed"), score.ErrGcsimFallback)
		}
		out.GcsimFallback = true
		return &out, nil
	}
	out.DPS *= factor(cfg)
	return &out, nil
}

func (s *noisySim) Timeline(ctx context.Context, cfg string, from, to float64) (*timeline.Timeline, error) {
	s.timelines = append(s.timelines, cfg)
	return s.serverSim.Timeline(ctx, cfg, from, to)
}

// On the engine, a team whose search goes past the rotation as written
// keeps gcsim's pick, scored on the engine (docs/engine.md §5).
func TestEngineKeepsGcsimPick(t *testing.T) {
	srv := newTestSite(t)
	ctx := context.Background()
	searched := 0
	for _, sel := range []selection{hutaoSel, {UID: "800000002", Chars: hutaoSel.Chars}, {UID: "800000005", Chars: []string{"lohen", "durin", "nicole", "citlali"}}, {UID: "800000006", Chars: []string{"skirk", "furina", "citlali", "escoffier"}}} {
		gcsim := &noisySim{serverSim: serverSim{srv}}
		g := srv.prepare(ctx, english, sel, nil)
		if g.Problem != "" {
			t.Fatal(g.Problem)
		}
		compute(ctx, english, gcsim, srv.settings, &g)

		eng := &noisySim{serverSim: serverSim{srv}}
		e := srv.prepare(ctx, english, sel, nil)
		e.Engine = true
		compute(ctx, english, eng, srv.settings, &e)
		if g.Problem != "" || e.Problem != "" {
			t.Fatalf("%s: %q / %q", sel.UID, g.Problem, e.Problem)
		}
		if len(gcsim.timelines) < 2 || len(eng.timelines) < 2 {
			t.Fatalf("%s: no timelines", sel.UID)
		}
		// the looped config the page shows, gcsim's pick either way
		gLooped, eLooped := gcsim.timelines[1], eng.timelines[1]
		if gLooped != eLooped || g.Rotation != e.Rotation {
			t.Errorf("%s: the engine's page shows another pick than gcsim's", sel.UID)
		}
		// the score is the engine's run of that pick, with the full
		// iterations
		full := strings.Replace(eLooped, "iteration=1 ", fmt.Sprintf("iteration=%d ", srv.settings.Iterations), 1)
		res, err := srv.score(ctx, full, srv.settings.Warmup, srv.settings.Iterations, false)
		if err != nil {
			t.Fatal(err)
		}
		if want := res.DPS * factor(full); e.Result.DPS != want {
			t.Errorf("%s: engine DPS %v, want gcsim's pick on the engine, %v", sel.UID, e.Result.DPS, want)
		}
		// A search past the rotation as written stops on the engine at
		// its second run: the engine runs the first run, gcsim's pick and
		// the one pass, three in all; one that stops at the first, two.
		switch len(eng.engine) {
		case 3:
			searched++
		case 2:
		default:
			t.Errorf("%s: %d runs on the engine, want 2 or 3", sel.UID, len(eng.engine))
		}
		t.Logf("%s: %d runs on the engine, DPS %.1f on gcsim, %.1f on the engine", sel.UID, len(eng.engine), g.Result.DPS, e.Result.DPS)
	}
	if searched == 0 {
		t.Error("no showcase searched: the rule is not exercised")
	}
}

// The setting travels in Job.Settings, and RunJob uses the same compute
// policy as the server. A fallback at any stage restarts the gcsim search.
func TestEngineOwnSearch(t *testing.T) {
	if DefaultSettings.EngineOwnSearch {
		t.Fatal("engine own search must default off")
	}
	srv := newTestSite(t)
	ctx := context.Background()
	sel := selection{UID: "800000002", Chars: hutaoSel.Chars}
	baseline := srv.prepare(ctx, english, sel, nil)
	gcsim := &noisySim{serverSim: serverSim{srv}}
	compute(ctx, english, gcsim, srv.settings, &baseline)
	if baseline.Problem != "" || len(gcsim.timelines) < 2 || len(gcsim.gcsim) <= 3 {
		t.Fatalf("baseline did not search: %s, %d runs", baseline.Problem, len(gcsim.gcsim))
	}
	set := srv.settings
	set.EngineOwnSearch = true
	run := func(fallbackAt int, fallbackError bool) (*noisySim, teamResult) {
		t.Helper()
		tr := srv.prepare(ctx, english, sel, nil)
		tr.Engine = true
		// Round-trip the job as the browser receives it, including the setting.
		data, err := json.Marshal(Job{Page: "score", Lang: "en", Settings: set, Teams: []teamResult{tr}})
		if err != nil {
			t.Fatal(err)
		}
		var job Job
		if err := json.Unmarshal(data, &job); err != nil {
			t.Fatal(err)
		}
		if !job.Settings.EngineOwnSearch {
			t.Fatal("job lost setting")
		}
		sim := &noisySim{serverSim: serverSim{srv}, fallbackAt: fallbackAt, fallbackError: fallbackError}
		if _, _, err := RunJob(ctx, job, sim); err != nil {
			t.Fatal(err)
		}
		got := job.Teams[0]
		if got.Problem != "" || len(sim.timelines) < 2 {
			t.Fatalf("compute: %s", got.Problem)
		}
		return sim, got
	}
	own, _ := run(0, false)
	if len(own.engine) <= 3 {
		t.Fatalf("engine did not finish search: %d runs", len(own.engine))
	}
	if len(own.gcsim) != 0 {
		t.Fatalf("own search ran %d gcsim scores", len(own.gcsim))
	}
	// First run, a ranked filler, and the final search run (before one pass).
	for _, at := range []int{1, 2, len(own.engine) - 1} {
		t.Run(fmt.Sprintf("fallback-at-%d", at), func(t *testing.T) {
			sim, got := run(at, false)
			if len(sim.gcsim) != len(gcsim.gcsim)-1 { // one pass still requests the engine
				t.Errorf("fallback ran %d gcsim scores, want full search's %d", len(sim.gcsim), len(gcsim.gcsim)-1)
			}
			if sim.timelines[1] != gcsim.timelines[1] || got.Rotation != baseline.Rotation || got.Filler != baseline.Filler {
				t.Error("fallback did not keep gcsim's pick")
			}
		})
	}
	sim, got := run(2, true)
	if len(sim.gcsim) == 0 || sim.timelines[1] != gcsim.timelines[1] || got.Filler != baseline.Filler {
		t.Error("failed gcsim retry did not restart search with gcsim's pick")
	}
}
