package chart

import (
	"context"
	"math"
	"strings"
	"testing"

	"github.com/cenoda/gscouter/internal/config"
	"github.com/cenoda/gscouter/internal/library"
	"github.com/cenoda/gscouter/internal/score"
	"github.com/cenoda/gscouter/teams"
	"github.com/genshinsim/gcsim/pkg/core"
	"github.com/genshinsim/gcsim/pkg/core/action"
)

// The two starting-HP keys are one setting: a chart that sets either
// replaces both of the team's, and a chart's own pair stays as it is.
func TestWithParamsStartHP(t *testing.T) {
	build := "hutao char lvl=90/90 cons=0 talent=9,9,9;\n"
	names := [Slots]string{"hutao"}
	for _, tc := range []struct{ chart, defaults, want string }{
		{"0E", "start_hp=10000", "[start_hp=10000]"},
		{"0E ; p0 start_hp%=50", "start_hp=10000", "[start_hp%=50]"},
		{"0E ; p0 start_hp=5000", "start_hp=10000, start_energy=20", "[start_energy=20, start_hp=5000]"},
		{"0E ; p0 start_hp=5000", "start_hp%=50", "[start_hp=5000]"},
		{"0E ; p0 start_hp%=50 start_hp=1000", "start_hp=10000", "[start_hp%=50, start_hp=1000]"},
		{"0E ; p0 start_energy=0", "start_hp=10000", "[start_hp=10000, start_energy=0]"},
	} {
		c, err := Parse(tc.chart)
		if err != nil {
			t.Fatal(err)
		}
		got := c.WithParams(build, names, [Slots]string{tc.defaults})
		if want := "hutao char lvl=90/90 cons=0 talent=9,9,9 +params=" + tc.want + ";"; !strings.Contains(got, want) {
			t.Errorf("WithParams(%q, %q) =\n%s\nwant a line %q", tc.chart, tc.defaults, got, want)
		}
	}
}

// Hu Tao's starting HP in the fight, for the library team that sets her
// start_hp=10000, as the preview reports it (Lane.HP): the team's alone,
// a chart's start_hp% alone (not added to the team's start_hp), a
// chart's start_hp as it was, and a chart's pair added as gcsim adds it.
func TestPreviewStartHP(t *testing.T) {
	team, err := library.LoadFS(teams.FS, "hutao-yelan-jean-furina")
	if err != nil {
		t.Fatal(err)
	}
	if team.Params["hutao"] != "start_hp=10000" {
		t.Fatalf("team params %v: want Hu Tao's start_hp=10000", team.Params)
	}
	names := [Slots]string{"furina", "hutao", "jean", "yelan"}
	defaults := [Slots]string{"", team.Params["hutao"]}
	for _, tc := range []struct {
		chart string
		want  func(max float64) float64
	}{
		{"1N ~60", func(float64) float64 { return 10000 }},
		{"1N ~60 ; p1 start_hp%=50", func(max float64) float64 { return max * 0.5 }},
		{"1N ~60 ; p1 start_hp%=1", func(max float64) float64 { return max * 0.01 }},
		{"1N ~60 ; p1 start_hp=5000", func(float64) float64 { return 5000 }},
		{"1N ~60 ; p1 start_hp%=50 start_hp=1000", func(max float64) float64 { return max*0.5 + 1000 }},
	} {
		c, err := Parse(tc.chart)
		if err != nil {
			t.Fatal(err)
		}
		p, err := c.Preview(c.WithParams(team.Build, names, defaults), names, nil)
		if err != nil || p.Error != nil {
			t.Fatalf("%q: preview %v %+v", tc.chart, err, p)
		}
		hp := p.Lanes[1].HP
		if hp[1] < 20000 {
			t.Fatalf("%q: max HP %.2f", tc.chart, hp[1])
		}
		if want := tc.want(hp[1]); math.Abs(hp[0]-want) > 0.01 {
			t.Errorf("%q: Hu Tao starts at %.2f of %.2f HP, want %.2f", tc.chart, hp[0], hp[1], want)
		}
	}
}

// StartResource is the range of start_energy as the fight reads it:
// each character starts at Max with no start_energy, a larger value is
// cut to Max, and Mavuika's is her Fighting Spirit, up to 200.
func TestStartResource(t *testing.T) {
	for _, tc := range []struct {
		team  string
		names [Slots]string
	}{
		{"mavuika-citlali-xilonen-bennett", [Slots]string{"bennett", "citlali", "mavuika", "xilonen"}},
		{"hutao-yelan-jean-furina", [Slots]string{"furina", "hutao", "jean", "yelan"}},
		{"raidenshogun-furina-xilonen-yelan", [Slots]string{"furina", "raidenshogun", "xilonen", "yelan"}},
	} {
		team, err := library.LoadFS(teams.FS, tc.team)
		if err != nil {
			t.Fatal(err)
		}
		for _, energy := range []string{"", "start_energy=150", "start_energy=300"} {
			var params [Slots]string
			for i := range params {
				params[i] = energy
			}
			c, err := Parse("0N ~60")
			if err != nil {
				t.Fatal(err)
			}
			cfg := config.Assemble(config.Conditions{Iterations: 1, Duration: 10}, c.WithParams(team.Build, tc.names, params), c.Gcsl(tc.names))
			ctx, stop := context.WithCancel(context.Background())
			done := false
			_, err = score.Play(ctx, cfg, 1, nil, func(core *core.Core, _ *action.Eval) {
				if done {
					return
				}
				done = true
				defer stop()
				for _, ch := range core.Player.Chars() {
					name := ch.Base.Key.String()
					r := StartResource(name)
					want := float64(r.Max)
					if energy == "start_energy=150" {
						want = min(150, want)
					}
					got := ch.Energy
					if r.Kind == "fighting_spirit" {
						v, err := ch.Condition([]string{"fightingspirit"})
						if err != nil {
							t.Fatal(err)
						}
						got = v.(float64)
					} else if ch.EnergyMax != float64(r.Max) {
						t.Errorf("%s: StartResource max %d, burst cost %v", name, r.Max, ch.EnergyMax)
					}
					if got != want {
						t.Errorf("%s with %q: starts at %v, want %v (%+v)", name, energy, got, want, r)
					}
				}
			})
			stop()
			if !done {
				t.Fatalf("%s: the fight did not start: %v", tc.team, err)
			}
		}
	}
	if r := StartResource("mavuika"); r != (Resource{"fighting_spirit", 200}) {
		t.Errorf("Mavuika: %+v", r)
	}
	if r := StartResource("skirk"); r != (Resource{"serpents_subtlety", 100}) {
		t.Errorf("Skirk: %+v", r)
	}
}
