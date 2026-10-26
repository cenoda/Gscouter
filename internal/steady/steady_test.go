package steady

import (
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
