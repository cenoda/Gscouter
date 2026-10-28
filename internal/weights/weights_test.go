package weights

import (
	"bytes"
	"encoding/json"
	"math"
	"slices"
	"strings"
	"testing"
)

func TestWeights(t *testing.T) {
	w, best := Weights(map[string]float64{"cr": 100, "cd": 95, "atk%": 50, "hp": -3, "em": 0})
	want := map[string]float64{"cr": 1, "cd": 0.95, "atk%": 0.5, "hp": 0, "em": 0}
	if best != "cr" {
		t.Errorf("best %q, want cr", best)
	}
	for k, v := range want {
		if math.Abs(w[k]-v) > 1e-12 {
			t.Errorf("%s weight %v, want %v", k, w[k], v)
		}
	}
	w, best = Weights(map[string]float64{"cr": -1, "cd": 0})
	if best != "" || w["cr"] != 0 || w["cd"] != 0 {
		t.Errorf("no gain: weights %v best %q, want all 0 and none", w, best)
	}
}

func TestJudge(t *testing.T) {
	// docs/artifacts.md §4.3: a goblet that loses 18% of the DPS where
	// one roll gains 2% costs 9 rolls, about 70 points.
	dps := map[string]float64{"pyro%": 1000, "atk%": 985, "em": 980.01, "dendro%": 960, "hp%": 820, "phys%": 700}
	right, penalty := Judge(dps, Options[1], 20)
	if !slices.Equal(right, []string{"atk%", "em", "pyro%"}) {
		t.Errorf("right %v, want atk%%, em, pyro%% in option order", right)
	}
	if got := penalty["dendro%"]; math.Abs(got-2*Points) > 1e-9 {
		t.Errorf("dendro%% penalty %v, want %v", got, 2*Points)
	}
	if got := penalty["hp%"]; math.Abs(got-9*Points) > 1e-9 {
		t.Errorf("hp%% penalty %v, want %v", got, 9*Points)
	}
	if got := penalty["phys%"]; math.Abs(got-15*Points) > 1e-9 {
		t.Errorf("phys%% penalty %v, want %v", got, 15*Points)
	}
	if len(penalty) != 3 {
		t.Errorf("penalties %v, want only the wrong ones", penalty)
	}
	if _, p := Judge(dps, Options[1], 0); p != nil {
		t.Errorf("penalties without a gain: %v, want none", p)
	}
}

func TestCharacter(t *testing.T) {
	rolls := map[string]float64{}
	for _, r := range Substats {
		rolls[r.Key] = 1000
	}
	rolls["cr"], rolls["cd"], rolls["atk%"] = 1020, 1019, 1010
	withMain := [3]map[string]float64{
		{"atk%": 1000, "em": 990, "hp%": 900, "def%": 890, "er": 950},
		{"pyro%": 1000, "atk%": 970},
		{"cr": 1000, "cd": 1001, "heal": 800},
	}
	c := character("xiangling", false, 1000, rolls, [3]string{"atk%", "pyro%", "cr"}, withMain)
	if c.Best != "cr" || c.Weights["cr"] != 1 || c.Weights["cd"] != 0.95 || c.Weights["atk%"] != 0.5 || c.Weights["hp"] != 0 {
		t.Errorf("weights %v best %s", c.Weights, c.Best)
	}
	if got := c.Main["sands"].Penalty["hp%"]; math.Abs(got-100.0/20*Points) > 0.05+1e-9 {
		t.Errorf("hp%% sands penalty %v", got)
	}
	if got := c.Main["circlet"].Right; !slices.Equal(got, []string{"cr", "cd"}) {
		t.Errorf("right circlets %v", got)
	}
	for _, s := range Slots {
		if c.Main[s].LowImpact {
			t.Errorf("%s low impact with a wrong main stat", s)
		}
	}
	withMain[1] = map[string]float64{"pyro%": 1000, "atk%": 985}
	if c := character("xiangling", false, 1000, rolls, [3]string{"atk%", "pyro%", "cr"}, withMain); !c.Main["goblet"].LowImpact || len(c.Main["goblet"].Penalty) != 0 {
		t.Errorf("goblet within 2%%: low impact %v, penalties %v", c.Main["goblet"].LowImpact, c.Main["goblet"].Penalty)
	}
	if c.Main["goblet"].Build != "pyro%" {
		t.Errorf("goblet build %q", c.Main["goblet"].Build)
	}
}

func result(id string, dps float64) *Result {
	return &Result{ID: id, Dir: "teams/" + id, Iterations: 20, DPS: dps, Simulations: 130, Seconds: 60, Gcsim: "g@v",
		Characters: []Character{{Key: "xiangling", Weights: map[string]float64{"cr": 1}, Gains: map[string]float64{"cr": 10}, Best: "cr", Main: map[string]*Slot{}}}}
}

func TestJoin(t *testing.T) {
	shard1 := []*Result{result("a", 100), {ID: "b", Skipped: "build.txt: x"}}
	shard2 := []*Result{result("c", 300), {ID: "d", Error: "no result after 30m0s", Seconds: 1800}}
	var lines bytes.Buffer
	for _, r := range append(shard1, shard2...) {
		lines.WriteString(mustJSON(t, r) + "\n")
	}
	results, err := ReadResults(&lines)
	if err != nil || len(results) != 4 {
		t.Fatalf("ReadResults: %d results, %v", len(results), err)
	}
	table, tot, err := Join(nil, results)
	if err != nil {
		t.Fatal(err)
	}
	if tot != (Totals{Teams: 4, Computed: 2, Skipped: 1, Failed: 1, Simulations: 260, Seconds: 1920}) {
		t.Errorf("totals %+v", tot)
	}
	if len(table.Teams) != 2 || table.Teams["a"].DPS != 100 || table.Skipped["b"] != "build.txt: x" || !strings.HasPrefix(table.Skipped["d"], "error: ") {
		t.Errorf("table %+v", table)
	}

	var file bytes.Buffer
	if _, err := table.WriteTo(&file); err != nil {
		t.Fatal(err)
	}
	// One team per line.
	for _, id := range []string{"a", "c"} {
		n := 0
		for _, line := range strings.Split(file.String(), "\n") {
			if strings.HasPrefix(line, `"`+id+`": {`) {
				n++
			}
		}
		if n != 1 {
			t.Errorf("team %s on %d lines of\n%s", id, n, file.String())
		}
	}
	base, err := ReadTable(&file)
	if err != nil {
		t.Fatal(err)
	}

	// A later run: a is calculated again, c now fails, b now passes.
	again, _, err := Join(base, []*Result{result("a", 110), {ID: "c", Error: "x"}, result("b", 200)})
	if err != nil {
		t.Fatal(err)
	}
	if again.Teams["a"].DPS != 110 || again.Teams["b"].DPS != 200 || again.Teams["c"] != nil || again.Skipped["c"] != "error: x" || again.Skipped["b"] != "" || !strings.HasPrefix(again.Skipped["d"], "error: ") {
		t.Errorf("rejoined table: teams %v skipped %v", again.Teams, again.Skipped)
	}

	if _, _, err := Join(nil, []*Result{result("a", 1), result("a", 2)}); err == nil {
		t.Error("a team in two shards joined without an error")
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
