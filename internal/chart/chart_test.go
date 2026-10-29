package chart

import (
	"strings"
	"testing"

	"github.com/cenoda/gscouter/internal/config"
	"github.com/cenoda/gscouter/internal/library"
	"github.com/cenoda/gscouter/internal/score"
	"github.com/cenoda/gscouter/teams"
)

func TestRoundTrip(t *testing.T) {
	for _, s := range []string{
		"2E(hold=1) 3E 3Q? 3N 1Q? 1E 0E 0N 0C 0D [0N 0C 0D]3 0Q?",
		"0E ~30 1N [~12 1C 1D]2 2Q ; p0 start_hp=1 ; p3 start_energy=40 breakthrough=1",
		"[0N]2 [1N 1N]3",
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
		strings.Repeat("0N ", MaxNotes+1),
	} {
		if _, err := Parse(s); err == nil {
			t.Errorf("Parse(%q) accepted", s)
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
