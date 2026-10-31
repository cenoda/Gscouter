package web

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/cenoda/gscouter/internal/chart"
	"github.com/cenoda/gscouter/internal/config"
	"github.com/cenoda/gscouter/internal/library"
	"github.com/cenoda/gscouter/internal/score"
	"github.com/cenoda/gscouter/teams"
	"github.com/genshinsim/gcsim/pkg/core"
	"github.com/genshinsim/gcsim/pkg/core/action"
)

// Skirk's starting Serpent's Subtlety, as the fight starts it: gcsim
// reads the legacy start_serpents_subtlety, then start_energy over it,
// and starts her at 100 with neither. A chart's value, under either key,
// replaces the library team's (chart.FoldResource), and an older link
// with only the legacy key plays as it did.
func TestSkirkStartingResource(t *testing.T) {
	lib, err := library.LoadLibrary(teams.FS)
	if err != nil {
		t.Fatal(err)
	}
	team := lib.Find([]string{"skirk", "furina", "citlali", "escoffier"})
	if team == nil {
		t.Fatal("library team not found")
	}
	chars := skirkTeam()
	names, teamParams := chartSlots(team, chars)
	skirk := slices.Index(names[:], "skirk")
	if skirk < 0 || !strings.Contains(teamParams[skirk], chart.LegacyResource+"=0") {
		t.Fatalf("library params %q for %v: want Skirk's legacy key at 0", teamParams, names)
	}
	start := func(text string, defaults [chart.Slots]string) any {
		t.Helper()
		c, err := chart.Parse(text)
		if err != nil {
			t.Fatal(err)
		}
		cfg := config.Assemble(config.Conditions{Iterations: 1, Duration: 10}, c.WithParams(buildLines(chars), names, defaults), c.Gcsl(names))
		ctx, stop := context.WithCancel(context.Background())
		defer stop()
		var got any
		var cerr error
		_, err = score.Play(ctx, cfg, 1, nil, func(core *core.Core, e *action.Eval) {
			if got != nil || cerr != nil {
				return
			}
			for _, ch := range core.Player.Chars() {
				if ch.Base.Key.String() == "skirk" {
					got, cerr = ch.Condition([]string{"serpents_subtlety"})
				}
			}
			stop()
		})
		if cerr != nil || got == nil {
			t.Fatalf("%q: condition %v, %v; play %v", text, got, cerr, err)
		}
		return got
	}
	p := " ; p" + strconv.Itoa(skirk) + " "
	note := strconv.Itoa(skirk) + "E"
	var teamEnergy [chart.Slots]string
	teamEnergy[skirk] = "start_energy=60"
	for _, tc := range []struct {
		name, chart string
		defaults    [chart.Slots]string
		want        float64
	}{
		{"neither key", note, [chart.Slots]string{}, 100},
		{"the library's legacy key", note, teamParams, 0},
		{"the chart's start_energy over the library's legacy key", note + p + "start_energy=40", teamParams, 40},
		{"an older link with the legacy key", note + p + chart.LegacyResource + "=20", teamParams, 20},
		{"an older link with both keys", note + p + "start_energy=20 " + chart.LegacyResource + "=100", teamParams, 20},
		{"the chart's legacy key over a team's start_energy", note + p + chart.LegacyResource + "=30", teamEnergy, 30},
	} {
		if got := start(tc.chart, tc.defaults); got != tc.want {
			t.Errorf("%s (%q): Skirk starts at %v (%T), want %v", tc.name, tc.chart, got, got, tc.want)
		}
	}
}
