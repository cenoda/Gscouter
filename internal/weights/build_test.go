package weights

import (
	"math"
	"strings"
	"testing"

	"github.com/genshinsim/gcsim/pkg/core/attributes"
	"github.com/genshinsim/gcsim/pkg/gcs/ast"
	"github.com/genshinsim/gcsim/pkg/gcs/parser"

	"github.com/cenoda/gscouter/internal/config"
)

const sample = `# Build from the source config, used to validate the rotation.

xingqiu char lvl=90/90 cons=6 talent=9,9,9;
xingqiu add weapon="sacrificialsword" refine=5 lvl=90/90;
xingqiu add set="eosf" count=4;
xingqiu add stats hp=4780 atk=311 atk%=0.466 hydro%=0.466 cr=0.311;
xingqiu add stats def%=0.124 def=39.36 hp=507.88 hp%=0.0992 atk=33.08 atk%=0.0992 er=0.3306 em=39.64 cr=0.331 cd=0.662;

sucrose char lvl=90/90 cons=6 talent=9,9,9;
sucrose add weapon="sacrificialfragments" refine=5 lvl=90/90;
sucrose add set="vv" count=4;
sucrose add stats hp=4780 atk=311 em=187 em = 187 em=186.5;
sucrose add stats def%=0.062*2 def=39.36 hp=507.88 hp%= 0.0992 atk=33.08 atk%=0.0992 er=0.6612 em=118.92 cr=0.2317 cd=0.1986;

bennett char lvl=90/90 cons=6 talent=9,9,9;
bennett add weapon="mistsplitterreforged" refine=1 lvl=90/90;
bennett add set="no" count=4;
bennett add stats hp=4780 atk=311 er=0.518 cr=0.311 pyro%=0.466;
bennett add stats def%=0.124 def=39.36 hp=507.88 hp%=0.0992 atk=33.08 atk%=0.0992 er=0.6612 em=39.64 cr=0.2317 cd=0.1986;

thoma char lvl=90/90 cons=6 talent=9,9,9;
thoma add weapon="favoniuslance" refine=5 lvl=90/90;
thoma add set="instructor" count=4;
thoma add stats hp=3571 atk=232 er=0.387 cr= 0.232 pyro%=0.348;
thoma add stats def%=0.124 def=39.36 hp=507.88 hp%=0.0992 atk=33.08 atk%=0.0992 er=0.6612 em=39.64 cr=0.2317 cd=0.1986;
`

func TestParseBuild(t *testing.T) {
	b, err := ParseBuild(sample)
	if err != nil {
		t.Fatal(err)
	}
	if b.String() != sample {
		t.Error("String does not give the build back")
	}
	want := []struct {
		key       string
		main      [3]string
		rarity    [3]int
		reordered bool
	}{
		{"xingqiu", [3]string{"atk%", "hydro%", "cr"}, [3]int{5, 5, 5}, false},
		{"sucrose", [3]string{"em", "em", "em"}, [3]int{5, 5, 5}, false},
		// Written sands, circlet, goblet.
		{"bennett", [3]string{"er", "pyro%", "cr"}, [3]int{5, 5, 5}, true},
		{"thoma", [3]string{"er", "pyro%", "cr"}, [3]int{4, 4, 4}, true},
	}
	if len(b.Chars) != len(want) {
		t.Fatalf("%d characters, want %d", len(b.Chars), len(want))
	}
	for i, w := range want {
		c := b.Chars[i]
		if c.Key != w.key || c.Reordered != w.reordered || c.Rarity != w.rarity {
			t.Errorf("character %d: %s reordered %v rarity %v, want %s %v %v", i, c.Key, c.Reordered, c.Rarity, w.key, w.reordered, w.rarity)
		}
		for slot := range 3 {
			if c.Main[slot].Key != w.main[slot] {
				t.Errorf("%s %s: %s, want %s", c.Key, Slots[slot], c.Main[slot].Key, w.main[slot])
			}
		}
		if len(c.Subs) != 10 {
			t.Errorf("%s: %d substats, want 10", c.Key, len(c.Subs))
		}
	}
	if s := b.Chars[1].Subs[0]; s.Key != "def%" || math.Abs(s.Value-0.124) > 1e-12 {
		t.Errorf("sucrose def%%=0.062*2 read as %v", s)
	}
}

func TestParseBuildRejects(t *testing.T) {
	for name, tc := range map[string]struct{ lines, want string }{
		"totals on one line": {
			"xingqiu add stats hp=5287.88 atk=344.08 atk%=0.5652 cr=0.642;\n",
			"1 add stats lines",
		},
		"main stats summed": {
			"kazuha add stats hp=4780 atk=311 em=561;\nkazuha add stats em=40;\n",
			"3 stats, want 5",
		},
		"not a main stat value": {
			"yelan add stats hp=4780 atk=311 hp%=0.466 hydro%=0.466 cr=0.331;\nyelan add stats em=40;\n",
			"cr=0.331 is not the main stat",
		},
		"no slot for it": {
			"yelan add stats hp=4780 atk=311 er=0.518 er=0.518 cr=0.311;\nyelan add stats em=40;\n",
			"er, er, cr cannot be",
		},
		"not levelled": {
			"aether add stats hp=3824 atk=248.8 em=149.6 dendro%=0.466 cr=0.2488;\naether add stats em=40;\n",
			"hp=3824 is not the main stat",
		},
		"a main stat among the substats": {
			"xingqiu add stats hp=4780 atk=311 atk%=0.466 hydro%=0.466 cr=0.311;\nxingqiu add stats pyro%=0.15 cr=0.1;\n",
			"pyro% is not a substat",
		},
		"a label": {
			"xingqiu add stats hp=4780 atk=311 atk%=0.466 hydro%=0.466 cr=0.311;\nxingqiu add stats label=x cr=0.1;\n",
			"is not key=value",
		},
	} {
		_, err := ParseBuild(tc.lines)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error %v, want one with %q", name, err, tc.want)
		}
	}
}

// stats parses a build with gcsim and returns each character's stat
// totals, by gcsim key.
func stats(t *testing.T, build string) map[string][]float64 {
	t.Helper()
	cfg := config.Assemble(config.Conditions{Iterations: 1, Duration: 10}, build, "active xingqiu;\nxingqiu attack;")
	simcfg, _, err := parser.New(ast.NewFile(), cfg).Parse()
	if err != nil {
		t.Fatal(err)
	}
	if len(simcfg.Errors) > 0 {
		t.Fatal(simcfg.Errors)
	}
	out := map[string][]float64{}
	for _, c := range simcfg.Characters {
		out[c.Base.Key.String()] = c.Stats
	}
	return out
}

func TestVariants(t *testing.T) {
	b, err := ParseBuild(sample)
	if err != nil {
		t.Fatal(err)
	}
	ref := stats(t, b.String())
	near := func(a, b float64) bool { return math.Abs(a-b) < 1e-9 }
	diff := func(variant string) map[string]map[attributes.Stat]float64 {
		got := stats(t, variant)
		out := map[string]map[attributes.Stat]float64{}
		for char, s := range got {
			for i, v := range s {
				if d := v - ref[char][i]; !near(d, 0) {
					if out[char] == nil {
						out[char] = map[attributes.Stat]float64{}
					}
					out[char][attributes.Stat(i)] = d
				}
			}
		}
		return out
	}

	// A CRIT DMG roll on Sucrose changes her CRIT DMG only.
	d := diff(b.WithSub(1, "cd", 0.0777))
	if len(d) != 1 || len(d["sucrose"]) != 1 || !near(d["sucrose"][attributes.CD], 0.0777) {
		t.Errorf("cd roll on sucrose: %v", d)
	}

	// Sucrose's goblet, the second of three EM pieces, to Anemo DMG:
	// one EM main stat goes, the others stay.
	v, err := b.WithMain(1, 1, "anemo%")
	if err != nil {
		t.Fatal(err)
	}
	d = diff(v)
	if len(d) != 1 || !near(d["sucrose"][attributes.EM], -187) || !near(d["sucrose"][attributes.AnemoP], 0.466) || len(d["sucrose"]) != 2 {
		t.Errorf("anemo goblet on sucrose: %v", d)
	}
	if !strings.Contains(v, "sucrose add stats hp=4780 atk=311 em=187 anemo%=0.466 em=186.5;") {
		t.Errorf("anemo goblet on sucrose: line not rewritten as expected:\n%s", v)
	}

	// Bennett writes his circlet before his goblet: the goblet is the
	// Pyro DMG piece.
	v, _ = b.WithMain(2, 1, "hp%")
	d = diff(v)
	if !near(d["bennett"][attributes.PyroP], -0.466) || !near(d["bennett"][attributes.HPP], 0.466) || len(d["bennett"]) != 2 {
		t.Errorf("hp%% goblet on bennett: %v", d)
	}

	// Thoma's pieces are 4★: a CRIT DMG circlet is a 4★ one.
	v, _ = b.WithMain(3, 2, "cd")
	d = diff(v)
	if !near(d["thoma"][attributes.CR], -0.232) || !near(d["thoma"][attributes.CD], 0.464) {
		t.Errorf("cd circlet on thoma: %v", d)
	}
}

// TestOptions checks the main stat lists against the game: sands 5,
// goblet 12, circlet 7, each with a 5★ level-20 value in the main stat
// table.
func TestOptions(t *testing.T) {
	for i, n := range []int{5, 12, 7} {
		if len(Options[i]) != n {
			t.Errorf("%s: %d options, want %d", Slots[i], len(Options[i]), n)
		}
		for _, k := range Options[i] {
			if _, ok := MainValue(k, 5); !ok {
				t.Errorf("%s: no 5★ value for %s", Slots[i], k)
			}
		}
	}
	for k, want := range map[string]float64{"atk%": 0.466, "em": 186.5, "er": 0.518, "def%": 0.583, "phys%": 0.583, "cr": 0.311, "cd": 0.622, "heal": 0.359, "hp": 4780, "atk": 311} {
		if got, _ := MainValue(k, 5); got != want {
			t.Errorf("5★ %s = %v, want %v", k, got, want)
		}
	}
}
