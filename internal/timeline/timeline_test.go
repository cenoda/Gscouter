package timeline

import (
	"math"
	"os"
	"testing"

	"github.com/cenoda/gscouter/internal/config"
	"github.com/cenoda/gscouter/internal/score"
)

func TestBuild(t *testing.T) {
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
	cfg := config.Assemble(config.Conditions{Iterations: 1, Duration: 90}, string(build), looped)
	res, err := score.Once(cfg, 1)
	if err != nil {
		t.Fatal(err)
	}
	tl := Build(res, 30*60, 60*60)
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
