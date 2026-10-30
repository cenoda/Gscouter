package chart

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/cenoda/gscouter/internal/config"
	"github.com/cenoda/gscouter/internal/library"
	"github.com/cenoda/gscouter/internal/score"
	"github.com/cenoda/gscouter/internal/timeline"
	"github.com/cenoda/gscouter/teams"
)

func TestRoundTrip(t *testing.T) {
	for _, s := range []string{
		"2E(hold=1) 3E 3Q? 3N 1Q? 1E 0E 0N 0C 0D [0N 0C 0D]3 0Q?",
		"0E ~30 1N [~12 1C 1D]2 2Q ; p0 start_hp=1 ; p3 start_energy=40 breakthrough=1",
		"[0N]2 [1N 1N]3",
		"0E 1N ; p2 start_energy=0 ; pickup",
	} {
		c, err := Parse(s)
		if err != nil {
			t.Fatalf("Parse(%q): %v", s, err)
		}
		if got := c.String(); got != s {
			t.Errorf("String() = %q, want %q", got, s)
		}
		d, err := Decode(c.Encode())
		if err != nil {
			t.Fatalf("Decode: %v", err)
		}
		if got := d.String(); got != s {
			t.Errorf("Decode(Encode()) = %q, want %q", got, s)
		}
	}
}

func TestParseRejects(t *testing.T) {
	for _, s := range []string{
		"",
		"4E",
		"0X",
		"0N?",
		"0E(hold)",
		"0E(Hold=1)",
		"0E(a=1,a=2)",
		"[0N [1N]2]2",
		"[0N",
		"0N]2",
		"[0N]",
		"[0N]1",
		"0N ~30",
		"0N ~601 1N",
		"0N ; p4 a=1",
		"0N ; p0 a=1 ; p0 b=2",
		"0E(x=1);0N",
		"0N ; pickup ; pickup",
		strings.Repeat("0N ", MaxNotes+1),
	} {
		if _, err := Parse(s); err == nil {
			t.Errorf("Parse(%q) accepted", s)
		}
	}
}

// A chart made only of notes played when ready would loop on one frame
// forever once none is ready; one note that always plays, or a wait,
// moves it on.
func TestParseRejectsIdle(t *testing.T) {
	for _, s := range []string{"0Q?", "0E? 1Q?", "[0E? 1Q?]3", "0E? [2Q?]2 ; pickup"} {
		if _, err := Parse(s); !errors.Is(err, ErrIdle) {
			t.Errorf("Parse(%q): %v, want %v", s, err, ErrIdle)
		}
	}
	for _, s := range []string{"0Q? 1N", "0Q? ~30 1E?", "[0E? 1Q?]3 0D"} {
		if _, err := Parse(s); err != nil {
			t.Errorf("Parse(%q): %v", s, err)
		}
	}
}

func TestGcsl(t *testing.T) {
	c, err := Parse("2E(hold=1) ~30 3Q? [0N 0C]4 1E ; p0 start_hp=1")
	if err != nil {
		t.Fatal(err)
	}
	got := c.Gcsl([Slots]string{"hutao", "xingqiu", "zhongli", "yelan"})
	want := `active zhongli;
zhongli skill[hold=1];
wait(30);
if .yelan.burst.ready { yelan burst; }
for let r0 = 0; r0 < 4; r0 = r0 + 1 {
  hutao attack;
  hutao charge;
}
xingqiu skill;
`
	if got != want {
		t.Errorf("Gcsl() =\n%s\nwant\n%s", got, want)
	}
	if p := c.CharParams(0); p != "start_hp=1" {
		t.Errorf("CharParams(0) = %q", p)
	}
}

func TestRecord(t *testing.T) {
	team, err := library.LoadFS(teams.FS, "hutao-xingqiu-zhongli-yelan")
	if err != nil {
		t.Fatal(err)
	}
	names := [Slots]string{"hutao", "xingqiu", "yelan", "zhongli"}
	build := config.AddParams(team.Build, "hutao", team.Params["hutao"])
	c, err := Record(build, team.Rotation, names)
	if err != nil {
		t.Fatal(err)
	}
	// The rotation's four cycles each play Hu Tao's ten attack, charge,
	// dash strings, which come back as repeats.
	if len(c.Repeats) == 0 {
		t.Errorf("no repeats in %s", c)
	}
	if !strings.Contains(c.String(), "3E(hold=1)") {
		t.Errorf("Zhongli's held skill lost: %s", c)
	}
	d, err := Decode(c.Encode())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := score.Once(config.Assemble(config.Conditions{Iterations: 1}, build, d.Gcsl(names)), 1); err != nil {
		t.Errorf("recorded chart does not run: %v", err)
	}
}

func TestPreview(t *testing.T) {
	team, err := library.LoadFS(teams.FS, "hutao-xingqiu-zhongli-yelan")
	if err != nil {
		t.Fatal(err)
	}
	names := [Slots]string{"hutao", "xingqiu", "yelan", "zhongli"}
	c, err := Parse("3E(hold=1) ~20 2E 2Q? 1Q? 1E [0N 0C]2 0E 0Q 2N")
	if err != nil {
		t.Fatal(err)
	}
	build := c.WithParams(team.Build, names, [Slots]string{"start_hp=1"})
	p, err := c.Preview(build, names, nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.Error != nil {
		t.Fatalf("error at note %d: %s", p.Error.Note, p.Error.Message)
	}
	if len(p.Passes) != PreviewPasses {
		t.Fatalf("passes %v", p.Passes)
	}
	// Pass 0 plays every note once, the repeated two twice.
	var first []int
	for _, n := range p.Notes {
		if n.Pass == 0 {
			first = append(first, n.Note)
			if n.Start < n.Asked {
				t.Errorf("note %d ran at %v before it was asked at %v", n.Note, n.Start, n.Asked)
			}
		}
	}
	want := []int{0, 1, 2, 3, 4, 5, 6, 5, 6, 7, 8, 9}
	if !slices.Equal(first, want) {
		t.Errorf("first pass notes %v, want %v", first, want)
	}
	// The wait before note 1 holds it back at least a third of a second.
	if n := p.Notes[1]; n.Start-n.Asked < 20.0/60 {
		t.Errorf("note 1 asked %v ran %v", n.Asked, n.Start)
	}
	if len(p.Lanes[3].Cooldown) == 0 || len(p.Lanes[0].Energy) == 0 || p.DPS <= 0 {
		t.Errorf("lanes or DPS missing: %+v DPS %v", p.Lanes[3], p.DPS)
	}

	bad, _ := Parse("3E 2H")
	p, err = bad.Preview(build, names, nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.Error == nil || p.Error.Note != 1 {
		t.Errorf("a plunge from the ground: error %+v, want at note 1", p.Error)
	}
}

// A chart that asks for a burst it lacks the energy for stands still in
// gcsim, but the score fills that wait, and so does the preview's DPS:
// it comes out near the chart that passes the burst when not ready.
func TestPreviewFilled(t *testing.T) {
	team, err := library.LoadFS(teams.FS, "flins-ineffa-aino-sucrose")
	if err != nil {
		t.Fatal(err)
	}
	names := [Slots]string{"aino", "flins", "ineffa", "sucrose"}
	dps := func(s string) (float64, *Preview) {
		c, err := Parse(s)
		if err != nil {
			t.Fatal(err)
		}
		p, err := c.Preview(c.WithParams(team.Build, names, [Slots]string{}), names, nil)
		if err != nil || p.Error != nil {
			t.Fatalf("Preview(%q): %v %+v", s, err, p.Error)
		}
		return p.DPS, p
	}
	stuck, p := dps("2E 2N 0E 0Q 3E 3N 1E 1E 1Q [1N 1N 1N 1N 1D]2 1E 1Q [1N 1N 1N 1N 1D]2 1E 1Q [1N 1N 1N 1N 1D]2")
	passed, _ := dps("2E 2N 0E 0Q 3E 3N 1E 1E 1Q [1N 1N 1N 1N 1D]2 1E 1Q? [1N 1N 1N 1N 1D]2 1E 1Q? [1N 1N 1N 1N 1D]2")
	// The lanes still show the chart as written, standing still.
	if !slices.ContainsFunc(p.Lanes[1].Waits, func(w timeline.Wait) bool { return w.Reason == "insufficient_energy" }) {
		t.Errorf("Flins' wait for energy not shown: %+v", p.Lanes[1].Waits)
	}
	if stuck < passed*0.9 {
		t.Errorf("preview DPS %.0f with the wait, %.0f passing the burst", stuck, passed)
	}
}

func TestWithParams(t *testing.T) {
	c, err := Parse("0E ; p0 start_energy=20 ; p2 breakthrough=0")
	if err != nil {
		t.Fatal(err)
	}
	build := "hutao char lvl=90/90 cons=0 talent=9,9,9;\nxingqiu char lvl=90/90 cons=0 talent=9,9,9;\nyelan char lvl=90/90 cons=0 talent=9,9,9;\n"
	got := c.WithParams(build, [Slots]string{"hutao", "xingqiu", "yelan", "zhongli"}, [Slots]string{"start_hp=1, start_energy=60", "", ""})
	for _, want := range []string{"hutao char lvl=90/90 cons=0 talent=9,9,9 +params=[start_hp=1, start_energy=20];", "xingqiu char lvl=90/90 cons=0 talent=9,9,9;", "yelan char lvl=90/90 cons=0 talent=9,9,9 +params=[breakthrough=0];"} {
		if !strings.Contains(got, want) {
			t.Errorf("WithParams() =\n%s\nwant a line %q", got, want)
		}
	}
}

func TestRemap(t *testing.T) {
	c, err := Parse("0E 1Q [2N 1N]3 [2C]2 3E ; p1 start_hp=1 ; p2 start_energy=0")
	if err != nil {
		t.Fatal(err)
	}
	// Slot 2 leaves, 1 becomes 0, 0 becomes 1.
	got := c.Remap([Slots]int{1, 0, -1, 3}).String()
	if want := "1E 0Q [0N]3 3E ; p0 start_hp=1"; got != want {
		t.Errorf("Remap() = %q, want %q", got, want)
	}
}

// A rotation that picks up crystallize shards records into a chart that
// picks them up, and the chart scores as the rotation does.
func TestRecordPickup(t *testing.T) {
	team, err := library.LoadFS(teams.FS, "arlecchino-kaeya-yunjin-xilonen")
	if err != nil {
		t.Fatal(err)
	}
	names := [Slots]string{"arlecchino", "kaeya", "xilonen", "yunjin"}
	c, err := Record(team.Build, team.Rotation, names)
	if err != nil {
		t.Fatal(err)
	}
	if !c.Pickup || !strings.HasSuffix(c.String(), "; pickup") {
		t.Fatalf("no pickup in %s", c)
	}
	looped := func(rot string) float64 {
		l, err := config.Loop(rot)
		if err != nil {
			t.Fatal(err)
		}
		res, err := score.Run(context.Background(), config.Assemble(config.Conditions{Iterations: 4, Duration: 300}, team.Build, l), score.Options{Iterations: 4, Warmup: 100, Seed: 1})
		if err != nil {
			t.Fatal(err)
		}
		return res.DPS
	}
	with := looped(c.Gcsl(names))
	c.Pickup = false
	without := looped(c.Gcsl(names))
	// Yun Jin's Archaic Petra: +35% Pyro DMG for Arlecchino.
	if with < without*1.05 {
		t.Errorf("picking up shards: %.0f DPS, without: %.0f", with, without)
	}
	c.Pickup = true
	p, err := c.Preview(team.Build, names, nil)
	if err != nil || p.Error != nil {
		t.Fatalf("preview: %v %+v", err, p.Error)
	}
}
