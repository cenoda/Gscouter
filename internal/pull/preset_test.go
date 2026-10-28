package pull

import (
	"math"
	"testing"
	"time"

	"github.com/cenoda/gscouter/internal/enka"
	"github.com/cenoda/gscouter/internal/library"
	"github.com/cenoda/gscouter/teams"
)

func TestParse(t *testing.T) {
	// The forms build.txt files write: an alias, a constant expression,
	// a stat repeated on one line, weapon and set params, the Traveler.
	chars, err := Parse(`kazuha char lvl=90/90 cons=0 talent=9,9,9;
kazuha add weapon="favsword" refine=3 lvl=90/90 +params=[stacks=5];
kazuha add set="vv" count=4 +params=[stacks=1];
kazuha add stats hp=4780 atk=311 em=187 em=187 em=187;
kazuha add stats def%=0.062*2 er=0.1102 cr=0.331;
travelercryo char lvl=80/90 cons=6 talent=1,8,10;
travelercryo add weapon="mistsplitter" refine=1 lvl=90/90;
travelercryo add set="bs" count=2;
travelercryo add set="no" count=2;
travelercryo add stats atk%=0.466;
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(chars) != 2 {
		t.Fatalf("%d characters, want 2", len(chars))
	}
	k := chars[0]
	if k.Key != "kaedeharakazuha" || k.Level != 90 || k.MaxLevel != 90 || k.Cons != 0 || k.Talents != [3]int{9, 9, 9} {
		t.Errorf("Kazuha: %+v", k)
	}
	if k.Weapon != (enka.Weapon{Key: "favoniussword", Refine: 3, Level: 90, MaxLevel: 90}) {
		t.Errorf("Kazuha's weapon: %+v", k.Weapon)
	}
	if len(k.Sets) != 1 || k.Sets["viridescentvenerer"] != 4 {
		t.Errorf("Kazuha's sets: %v", k.Sets)
	}
	for stat, want := range map[string]float64{"hp": 4780, "atk": 311, "em": 561, "def%": 0.124, "er": 0.1102, "cr": 0.331} {
		if got := k.Stats[stat]; math.Abs(got-want) > 1e-9 {
			t.Errorf("Kazuha's %s: %v, want %v", stat, got, want)
		}
	}
	if len(k.Stats) != 6 {
		t.Errorf("Kazuha's stats: %v", k.Stats)
	}
	tr := chars[1]
	if tr.Key != "aethercryo" || tr.Level != 80 || tr.Talents != [3]int{1, 8, 10} || len(tr.Sets) != 2 {
		t.Errorf("Traveler: %+v", tr)
	}
}

// TestLibraryPresets reads every build in the library: each member of
// a team with a build gets a preset, the preset survives a packed link
// (Unpack checks it against the game data), and it writes the same
// stats as its build.txt.
func TestLibraryPresets(t *testing.T) {
	lib, err := library.LoadLibrary(teams.FS)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	p, err := NewPresets(lib)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("NewPresets: %v", time.Since(start))
	n := 0
	for _, team := range lib.Teams {
		hasBuild := p.byTeam[team.ID] != nil
		for _, m := range team.Members {
			c, ok := p.For(team, m)
			if !ok {
				// Only a character no library build has may lack one.
				if hasBuild || p.common[library.Canonical(m)].Key != "" {
					t.Errorf("%s: no preset for %s", team.ID, m)
				} else {
					t.Logf("%s: no library build has %s", team.ID, m)
				}
				continue
			}
			if hasBuild && c.Config() != p.byTeam[team.ID][library.Canonical(m)].Config() {
				t.Errorf("%s: %s's preset is not the team's build", team.ID, m)
			}
			back, err := enka.Unpack(enka.Pack([]enka.Character{c}))
			if err != nil {
				t.Errorf("%s: %s's preset does not unpack: %v", team.ID, m, err)
				continue
			}
			if back[0].Config() != c.Config() {
				t.Errorf("%s: %s's preset changes when packed:\n%s\n%s", team.ID, m, c.Config(), back[0].Config())
			}
			again, err := Parse(c.Config())
			if err != nil || len(again) != 1 || again[0].Config() != c.Config() {
				t.Errorf("%s: %s's preset does not read back: %v", team.ID, m, err)
			}
			n++
		}
	}
	t.Logf("%d presets for %d teams, %d characters", n, len(lib.Teams), len(p.common))
}

func TestForCopies(t *testing.T) {
	lib, err := library.LoadLibrary(teams.FS)
	if err != nil {
		t.Fatal(err)
	}
	p, err := NewPresets(lib)
	if err != nil {
		t.Fatal(err)
	}
	team := lib.Teams[0]
	c, _ := p.For(team, team.Members[0])
	c.Stats["atk"] += 1000
	c.Sets["x"] = 1
	d, _ := p.For(team, team.Members[0])
	if d.Config() == c.Config() || d.Sets["x"] != 0 {
		t.Error("changing a preset changed the stored one")
	}
}
