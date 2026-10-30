package timeline

import (
	"context"
	"errors"
	"math"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/cenoda/gscouter/internal/config"
	"github.com/cenoda/gscouter/internal/score"
)

// testConfig is the score package's test team, looped for 90 s.
func testConfig(t *testing.T) string {
	t.Helper()
	build, err := os.ReadFile("../score/testdata/build.txt")
	if err != nil {
		t.Fatal(err)
	}
	rotation, err := os.ReadFile("../score/testdata/rotation.gcsl")
	if err != nil {
		t.Fatal(err)
	}
	looped, err := config.Loop(string(rotation))
	if err != nil {
		t.Fatal(err)
	}
	return config.Assemble(config.Conditions{Iterations: 1, Duration: 90}, string(build), looped)
}

func TestBuild(t *testing.T) {
	res, actions, err := score.Trace(context.Background(), testConfig(t), 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	tl := Build(res, actions, 30*60, 60*60)
	if tl.Start != 30 || tl.End != 60 {
		t.Fatalf("window %v-%v, want 30-60", tl.Start, tl.End)
	}
	if len(tl.Characters) != 4 || len(tl.Stints) == 0 {
		t.Fatalf("%d characters, %d stints", len(tl.Characters), len(tl.Stints))
	}

	// The window's damage adds up the same by character, by ability and
	// by stint (stints cover the whole window).
	var bySource, byStint float64
	for _, c := range tl.Characters {
		for _, s := range c.Sources {
			bySource += s.Damage
		}
		if len(c.Buckets) != 30 {
			t.Errorf("%s: %d buckets, want 30", c.Name, len(c.Buckets))
		}
	}
	for _, s := range tl.Stints {
		byStint += s.Total
	}
	if math.Abs(bySource-tl.Damage) > 1 || math.Abs(byStint-tl.Damage) > 1 {
		t.Errorf("damage %v, by ability %v, by stint %v", tl.Damage, bySource, byStint)
	}
	// Exactly one character is on field at any time.
	var field float64
	for _, c := range tl.Characters {
		for _, f := range c.Field {
			field += f.End - f.Start
		}
	}
	if math.Abs(field-30) > 0.01 {
		t.Errorf("on-field time %v s, want 30", field)
	}
	for _, s := range tl.Stints {
		t.Logf("%5.1f-%5.1f %-10s %v %.0f", s.Start, s.End, tl.Characters[s.Char].Name, s.Actions, s.Total)
	}
	for _, c := range tl.Characters {
		t.Logf("%s %.0f waits=%v sources=%v", c.Name, c.Damage, c.Waits, c.Sources)
	}
}

// Simulate stops the fight after the window, and gives the timeline of
// the whole fight.
func TestSimulate(t *testing.T) {
	cfg := testConfig(t)
	res, actions, err := score.Trace(context.Background(), cfg, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range [][2]int{{0, 20 * 60}, {10 * 60, 40 * 60}, {30*60 + 7, 55*60 + 13}} {
		got, err := Simulate(context.Background(), cfg, w[0], w[1])
		if err != nil {
			t.Fatal(err)
		}
		if want := Build(res, actions, w[0], w[1]); !reflect.DeepEqual(got, want) {
			t.Errorf("window %v: timeline differs from the whole fight's", w)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Simulate(ctx, cfg, 0, 20*60); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled: err = %v, want %v", err, context.Canceled)
	}
}

func TestAbilityName(t *testing.T) {
	for in, want := range map[string]string{
		"Normal 2":                  "Normal",
		"Normal (Skill) 4":          "Normal (Skill)",
		"Exquisite Throw":           "Exquisite Throw",
		"Surging Blade (escoffier)": "Surging Blade (escoffier)",
		"Lv2":                       "Lv2",
	} {
		if got := abilityName(in); got != want {
			t.Errorf("abilityName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAddAction(t *testing.T) {
	a := func(name string, params map[string]int) score.Action {
		return score.Action{Action: name, Params: params}
	}
	// Mavuika's stint on Mavuika / Citlali / Xilonen / Nicole.
	stint := []score.Action{
		a("skill", map[string]int{"recast": 1}), a("charge", nil), a("dash", nil), a("attack", nil), a("charge", nil),
		a("burst", nil),
		a("charge", nil), a("charge", nil), a("charge", nil), a("charge", map[string]int{"hold": 6}), a("charge", map[string]int{"final": 1}), a("dash", nil),
		a("charge", nil), a("charge", nil), a("charge", nil), a("charge", map[string]int{"hold": 6}), a("charge", map[string]int{"final": 1}), a("dash", nil),
		a("attack", nil), a("skill", nil), a("attack", nil),
	}
	var got []Action
	for i, act := range stint {
		got = addAction(got, "mavuika", act, float64(i))
	}
	var forms []string
	for _, g := range got {
		forms = append(forms, strings.TrimSuffix(g.Kind+" "+g.Form, " "))
	}
	want := []string{
		"skill recast", "charge", "dash", "attack", "charge", "burst",
		"charge hold", "dash", "charge hold", "dash",
		"attack", "skill", "attack",
	}
	if !slices.Equal(forms, want) {
		t.Errorf("got %q, want %q", forms, want)
	}
	// A held charged attack starts with its first charge.
	if got[6].Time != 6 || got[8].Time != 12 {
		t.Errorf("held charged attacks at %v and %v, want 6 and 12", got[6].Time, got[8].Time)
	}
	// Held skills are told apart for everyone, held charges only for
	// Mavuika.
	if g := addAction(nil, "nahida", a("skill", map[string]int{"hold": 1}), 0); g[0].Form != "hold" {
		t.Errorf("held skill: form %q", g[0].Form)
	}
	if g := addAction([]Action{{Kind: "charge"}}, "arlecchino", a("charge", map[string]int{"final": 1}), 1); len(g) != 2 || g[1].Form != "" {
		t.Errorf("another character's charges folded: %v", g)
	}
}
