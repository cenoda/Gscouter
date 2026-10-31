package web

import (
	"context"
	"math"
	"slices"
	"strconv"
	"testing"

	"github.com/cenoda/gscouter/internal/chart"
	"github.com/cenoda/gscouter/internal/config"
	"github.com/cenoda/gscouter/internal/enka"
	"github.com/cenoda/gscouter/internal/library"
	"github.com/cenoda/gscouter/internal/score"
	"github.com/cenoda/gscouter/teams"
	"github.com/genshinsim/gcsim/pkg/core"
	"github.com/genshinsim/gcsim/pkg/core/action"
	"github.com/genshinsim/gcsim/pkg/core/player/character"
)

// startOf plays a chart for chars with the library team's params, as a
// score plays it (Chart.WithParams), and gives what read reads from the
// character named key as the fight starts.
func startOf(t *testing.T, chars []enka.Character, names, defaults [chart.Slots]string, text, key string, read func(*character.CharWrapper) float64) float64 {
	t.Helper()
	c, err := chart.Parse(text)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Assemble(config.Conditions{Iterations: 1, Duration: 10}, c.WithParams(buildLines(chars), names, defaults), c.Gcsl(names))
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	got, found := 0.0, false
	_, err = score.Play(ctx, cfg, 1, nil, func(core *core.Core, e *action.Eval) {
		if found {
			return
		}
		for _, ch := range core.Player.Chars() {
			if ch.Base.Key.String() == key {
				got, found = read(ch), true
			}
		}
		stop()
	})
	if !found {
		t.Fatalf("%q: no %s in the fight: %v", text, key, err)
	}
	return got
}

// Hu Tao's starting HP, for the library team that sets her
// start_hp=10000: a chart's start_hp% sets it alone, not added to the
// team's start_hp (chart.HPKeys), and a chart's start_hp, or both keys,
// play as they did before.
func TestStartingHP(t *testing.T) {
	lib, err := library.LoadLibrary(teams.FS)
	if err != nil {
		t.Fatal(err)
	}
	team := lib.Find([]string{"hutao", "yelan", "jean", "furina"})
	if team == nil {
		t.Fatal("library team not found")
	}
	chars := hutaoTeam()
	names, defaults := chartSlots(team, chars)
	s := slices.Index(names[:], "hutao")
	if s < 0 || defaults[s] != "start_hp=10000" {
		t.Fatalf("library params %q for %v: want Hu Tao's start_hp=10000", defaults, names)
	}
	maxHP := startOf(t, chars, names, [chart.Slots]string{}, strconv.Itoa(s)+"N", "hutao", func(ch *character.CharWrapper) float64 { return ch.MaxHP() })
	p := strconv.Itoa(s) + "N ; p" + strconv.Itoa(s) + " "
	for _, tc := range []struct {
		name, chart string
		want        float64
	}{
		{"the library's start_hp", strconv.Itoa(s) + "N", 10000},
		{"the chart's start_hp%", p + "start_hp%=50", maxHP * 0.5},
		{"the chart's start_hp% at its least", p + "start_hp%=1", maxHP * 0.01},
		{"an older link with start_hp", p + "start_hp=5000", 5000},
		{"an older link with both keys", p + "start_hp%=50 start_hp=1000", maxHP*0.5 + 1000},
	} {
		got := startOf(t, chars, names, defaults, tc.chart, "hutao", func(ch *character.CharWrapper) float64 { return ch.CurrentHP() })
		if math.Abs(got-tc.want) > 0.01 {
			t.Errorf("%s (%q): Hu Tao starts at %.2f of %.2f HP, want %.2f", tc.name, tc.chart, got, maxHP, tc.want)
		}
	}
}

// Mavuika's start_energy is her Fighting Spirit, 200 with no key and at
// most; the editor's control for it runs to 200 (chart.StartResource).
// Another character's is burst energy, cut to the burst's cost.
func TestStartingResource(t *testing.T) {
	chars := mavuikaTeam()
	names, _ := chartSlots(nil, chars)
	m := slices.Index(names[:], "mavuika")
	b := slices.Index(names[:], "bennett")
	if r := chart.StartResource("mavuika"); r.Kind != "fighting_spirit" || r.Max != 200 {
		t.Errorf("Mavuika's resource %+v", r)
	}
	fs := func(ch *character.CharWrapper) float64 {
		v, err := ch.Condition([]string{"fightingspirit"})
		if err != nil {
			t.Fatal(err)
		}
		return v.(float64)
	}
	note := strconv.Itoa(m) + "N"
	for _, tc := range []struct {
		params string
		want   float64
	}{
		{"", 200},
		{"start_energy=150", 150},
		{"start_energy=200", 200},
		{"start_energy=250", 200},
	} {
		var defaults [chart.Slots]string
		defaults[m] = tc.params
		if got := startOf(t, chars, names, defaults, note, "mavuika", fs); got != tc.want {
			t.Errorf("Mavuika with %q starts at %v Fighting Spirit, want %v", tc.params, got, tc.want)
		}
	}
	energy := func(ch *character.CharWrapper) float64 { return ch.Energy }
	cost := float64(chart.StartResource("bennett").Max)
	var defaults [chart.Slots]string
	defaults[b] = "start_energy=100"
	if got := startOf(t, chars, names, defaults, note, "bennett", energy); cost != 60 || got != cost {
		t.Errorf("Bennett with start_energy=100 starts at %v energy, want his burst's cost %v (60)", got, cost)
	}
}
