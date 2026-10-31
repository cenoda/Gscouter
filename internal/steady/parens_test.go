package steady

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/cenoda/gscouter/internal/config"
	"github.com/cenoda/gscouter/internal/score"
)

// A parameter written in parentheses scores as the plain one through the
// whole filler search, with the real simulation: Fill and skillForms
// read "skill [counter=(1)]" as "skill [counter=1]". Before they did,
// the parenthesized statement was passed over (Itto / Bennett / Fischl /
// Beidou scored 1.78% lower with it).
func TestScoreParenthesizedParam(t *testing.T) {
	dir := "../../teams/aratakiitto-bennett-fischl-beidou/"
	build, err := os.ReadFile(dir + "build.txt")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(dir + "rotation.gcsl")
	if err != nil {
		t.Fatal(err)
	}
	plain := string(raw)
	parens := strings.Replace(plain, "beidou skill [counter=1], burst;", "beidou skill [counter=(1)], burst;", 1)
	if parens == plain {
		t.Fatal("the team no longer writes Beidou's skill as expected")
	}
	members := []string{"itto", "bennett", "fischl", "beidou"}
	run := func(looped string) (*score.Result, error) {
		cfg := config.Assemble(config.Conditions{Iterations: 3, Duration: 300}, string(build), looped)
		return score.Run(context.Background(), cfg, score.Options{Iterations: 3, Warmup: 100, Seed: 1})
	}
	a, err := Score(plain, members, run)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Score(parens, members, run)
	if err != nil {
		t.Fatal(err)
	}
	if a.Result.DPS != b.Result.DPS || a.Filled != b.Filled || len(a.Energy) != len(b.Energy) {
		t.Errorf("counter=1: %.2f DPS, filled %v %v; counter=(1): %.2f DPS, filled %v %v",
			a.Result.DPS, a.Filled, a.Energy, b.Result.DPS, b.Filled, b.Energy)
	}
	for c, how := range a.Energy {
		if b.Energy[c] != how {
			t.Errorf("%s: %v with counter=1, %v with counter=(1)", c, how, b.Energy[c])
		}
	}
	if strings.ReplaceAll(b.Looped, "counter=(1)", "counter=1") != a.Looped {
		t.Errorf("the best runs differ beyond the parameter:\n%s\n---\n%s", a.Looped, b.Looped)
	}
	t.Logf("%.2f DPS, filled %v %v", a.Result.DPS, a.Filled, a.Energy)
}
