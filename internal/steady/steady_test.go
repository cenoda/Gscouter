package steady

import (
	"errors"
	"strings"
	"testing"

	"github.com/cenoda/gscouter/internal/config"
	"github.com/cenoda/gscouter/internal/score"
)

// A rotation where Hu Tao should pass her burst, which only pays once
// Furina fills her own energy wait instead: no filler applied to both
// at once finds it.
func TestScoreMixesFillers(t *testing.T) {
	rot := "active furina;\nfurina skill, burst;\nhutao skill, attack:2, burst;\n"
	members := []string{"hutao", "furina"}
	skipHutao := "if .hutao.burst.ready { hutao burst; }"
	fillFurina := "while !.furina.burst.ready"
	run := func(looped string) (*score.Result, error) {
		skip, fill := strings.Contains(looped, skipHutao), strings.Contains(looped, fillFurina)
		dps, hutaoWait, furinaWait := 100.0, 30.0, 0.0
		switch {
		case skip && fill:
			dps, hutaoWait = 300, 0
		case skip:
			// Passing makes the rotation shorter: Furina now waits.
			dps, hutaoWait, furinaWait = 90, 0, 20
		case fill:
			dps = 110
		case strings.Contains(looped, "while !.hutao.burst.ready"):
			dps, hutaoWait = 150, 0
		}
		return &score.Result{DPS: dps, Characters: []score.Character{
			{Name: "hutao", Waits: map[string]float64{"insufficient_energy": hutaoWait}},
			{Name: "furina", Waits: map[string]float64{"insufficient_energy": furinaWait}},
		}}, nil
	}
	out, err := Score(rot, members, run)
	if err != nil {
		t.Fatal(err)
	}
	if out.Result.DPS != 300 || out.Energy["hutao"] != config.FillSkip || out.Energy["furina"] == config.FillSkip {
		t.Errorf("got %.0f DPS with %v, want 300 with hutao passing and furina filling", out.Result.DPS, out.Energy)
	}
}

func TestScoreKeepsRotationAsWritten(t *testing.T) {
	runs := 0
	run := func(string) (*score.Result, error) {
		runs++
		return &score.Result{DPS: 100, Characters: []score.Character{{Name: "a"}}}, nil
	}
	out, err := Score("active a;\na skill, burst;\n", []string{"a"}, run)
	if err != nil {
		t.Fatal(err)
	}
	if out.Filled || runs != 1 {
		t.Errorf("filled %v after %d runs; want the rotation as written after 1", out.Filled, runs)
	}
}

// Ineffa fills her energy wait with attacks; after the search, the best
// run is tried again with her burst filled briefly and then passed.
func TestScoreTriesBriefOnBest(t *testing.T) {
	rot := "active ineffa;\nineffa skill, burst;\nayato skill, attack:15;\n"
	brief := "gs_fill1a < 2"
	run := func(looped string) (*score.Result, error) {
		dps, wait := 100.0, 30.0
		switch {
		case strings.Contains(looped, brief):
			dps, wait = 130, 0
		case strings.Contains(looped, "while !.ineffa.burst.ready"):
			dps, wait = 120, 0
		}
		return &score.Result{DPS: dps, Characters: []score.Character{
			{Name: "ineffa", Waits: map[string]float64{"insufficient_energy": wait}},
			{Name: "ayato"},
		}}, nil
	}
	out, err := Score(rot, []string{"ineffa", "ayato"}, run)
	if err != nil {
		t.Fatal(err)
	}
	if out.Result.DPS != 130 || out.Energy["ineffa"] != config.FillBrief {
		t.Errorf("got %.0f DPS with %v, want 130 with ineffa filled briefly", out.Result.DPS, out.Energy)
	}
}

// A filled run that ranks best but fails in its full run gives way to
// the rotation's next run, not to the rotation as written, which stands
// still and scores 0 (a C6 Skirk, docs/characters/skirk.md §4).
func TestBestConfirmsNextRunAfterFailure(t *testing.T) {
	rotations := []Rotation{{"active a;\na skill, burst;\n", []string{"a"}}}
	run := func(_ int, looped string, rank bool) (*score.Result, error) {
		dps, wait := 0.0, 30.0
		switch {
		case strings.Contains(looped, "gs_fill1a < 2"):
			dps, wait = 100, 0
		case strings.Contains(looped, "if .a.burst.ready"):
			if !rank {
				return nil, errors.New("iteration 11 (seed 12): rejected action")
			}
			dps, wait = 160, 0
		case strings.Contains(looped, "while !.a.burst.ready"):
			dps, wait = 150, 0
		}
		return &score.Result{DPS: dps, Characters: []score.Character{
			{Name: "a", Waits: map[string]float64{"insufficient_energy": wait}},
		}}, nil
	}
	i, out, err := Best(rotations, run)
	if err != nil {
		t.Fatal(err)
	}
	if i != 0 || out.Result.DPS != 150 || out.Energy["a"] == config.FillSkip {
		t.Errorf("got rotation %d at %.0f DPS with %v; want 0 at 150 with a filling", i, out.Result.DPS, out.Energy)
	}
}

// bestRun scores rotation i at dps[i] when ranked and full[i] in a full
// run, and counts the full runs.
func bestRun(dps, full []float64, fullRuns *int) func(int, string, bool) (*score.Result, error) {
	return func(i int, _ string, rank bool) (*score.Result, error) {
		d := dps[i]
		if !rank {
			*fullRuns++
			d = full[i]
		}
		return &score.Result{DPS: d, Characters: []score.Character{{Name: "a"}}}, nil
	}
}

func TestBestKeepsTheBestRotation(t *testing.T) {
	rotations := []Rotation{
		{"active a;\na skill;\n", []string{"a"}},
		{"active a;\na burst;\n", []string{"a"}},
		{"active a;\na attack;\n", []string{"a"}},
	}
	fullRuns := 0
	i, out, err := Best(rotations, bestRun([]float64{100, 150, 120}, []float64{100, 140, 120}, &fullRuns))
	if err != nil {
		t.Fatal(err)
	}
	// The team's own rotation runs in full once, the winner once more.
	if i != 1 || out.Result.DPS != 140 || fullRuns != 2 {
		t.Errorf("got rotation %d at %.0f DPS after %d full runs; want 1 at 140 after 2", i, out.Result.DPS, fullRuns)
	}
}

// A rotation that ranks best but scores below the team's own in a full
// run gives way to the next, and the team's own stands when none beats it.
func TestBestConfirmsWithFullRuns(t *testing.T) {
	rotations := []Rotation{
		{"active a;\na skill;\n", []string{"a"}},
		{"active a;\na burst;\n", []string{"a"}},
		{"active a;\na attack;\n", []string{"a"}},
	}
	fullRuns := 0
	i, out, err := Best(rotations, bestRun([]float64{100, 150, 120}, []float64{100, 95, 110}, &fullRuns))
	if err != nil {
		t.Fatal(err)
	}
	if i != 2 || out.Result.DPS != 110 {
		t.Errorf("got rotation %d at %.0f DPS; want 2 at 110", i, out.Result.DPS)
	}
	i, out, err = Best(rotations, bestRun([]float64{100, 150, 120}, []float64{100, 95, 90}, &fullRuns))
	if err != nil {
		t.Fatal(err)
	}
	if i != 0 || out.Result.DPS != 100 {
		t.Errorf("got rotation %d at %.0f DPS; want 0 at 100", i, out.Result.DPS)
	}
}

// A rotation gcsim rejects is left out, the team's own included; the
// score fails only when none runs.
func TestBestSkipsFailedAlternative(t *testing.T) {
	rotations := []Rotation{
		{"active a;\na skill;\n", []string{"a"}},
		{"active a;\na burst;\n", []string{"a"}},
	}
	fail := func(bad int) func(int, string, bool) (*score.Result, error) {
		return func(i int, _ string, _ bool) (*score.Result, error) {
			if i == bad {
				return nil, errors.New("invalid config")
			}
			return &score.Result{DPS: 100, Characters: []score.Character{{Name: "a"}}}, nil
		}
	}
	if i, _, err := Best(rotations, fail(1)); err != nil || i != 0 {
		t.Errorf("got rotation %d, error %v; want 0 without error", i, err)
	}
	if i, out, err := Best(rotations, fail(0)); err != nil || i != 1 || out.Result.DPS != 100 {
		t.Errorf("got rotation %d, error %v; want 1 without error", i, err)
	}
	if _, _, err := Best(rotations[:1], fail(0)); err == nil {
		t.Error("want an error when no rotation runs")
	}
}
