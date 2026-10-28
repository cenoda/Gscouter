package weights

import (
	"bytes"
	"maps"
	"os"
	"reflect"
	"slices"
	"testing"

	"github.com/cenoda/gscouter/internal/artifact"
)

// The artifact package keeps its own copy of the rolls and options, so
// that it needs nothing of this one.
func TestArtifactConstants(t *testing.T) {
	if len(artifact.MaxRolls) != len(Substats) {
		t.Errorf("artifact has %d rolls, want %d", len(artifact.MaxRolls), len(Substats))
	}
	for _, r := range Substats {
		if artifact.MaxRolls[r.Key] != r.Value {
			t.Errorf("artifact roll %s = %v, want %v", r.Key, artifact.MaxRolls[r.Key], r.Value)
		}
	}
	if !slices.Equal(artifact.Slots, Slots) || !reflect.DeepEqual(artifact.Options, Options) || artifact.Points != Points {
		t.Error("artifact slots, options or points differ from weights'")
	}
}

func TestSlim(t *testing.T) {
	full := &Table{Format: TableFormat, Teams: map[string]*Entry{
		"a": {Gcsim: "g@1", Iterations: 20, DPS: 1000, Characters: []Character{{
			Key:        "xiangling",
			Weights:    map[string]float64{"cr": 1, "cd": 0.98765, "atk%": 0.4, "hp": 0, "def": 0.0004},
			Gains:      map[string]float64{"cr": 100},
			Best:       "cr",
			Unplayable: map[string]string{"er": "an error"},
			Main: map[string]*Slot{
				"sands": {Build: "er", DPS: map[string]float64{"er": 1000, "atk%": 990, "em": 995, "hp%": 800, "def%": 800},
					Right: []string{"atk%", "em", "er"}, Penalty: map[string]float64{"hp%": 15.54, "def%": 15.54}},
				"goblet": {Build: "pyro%", DPS: map[string]float64{"pyro%": 1000, "em": 999, "atk%": 1001},
					Right: []string{"atk%", "em", "pyro%"}, LowImpact: true},
				"circlet": {Build: "cr", DPS: map[string]float64{"cr": 1000, "cd": 1000, "atk%": 900},
					Right: []string{"cr", "cd"}, Penalty: map[string]float64{"atk%": 7.77}, Unplayable: map[string]string{"heal": "x", "em": "y"}},
			},
		}}},
		"b": {Gcsim: "g@2", Characters: []Character{{Key: "qiqi", Weights: map[string]float64{"cr": 0}, Main: map[string]*Slot{
			"sands": {Right: []string{"hp%"}, DPS: map[string]float64{"hp%": 1, "atk%": 0.5}},
		}}}},
		"c": {Gcsim: "g@1"},
	}, Skipped: map[string]string{"d": "build.txt: no"}}
	got := Slim(full)
	want := &artifact.Table{Format: artifact.TableFormat, Gcsim: "g@1", Teams: map[string]*artifact.Team{
		"a": {Characters: []artifact.Character{{
			Key:        "xiangling",
			Weights:    map[string]float64{"cr": 1, "cd": 0.988, "atk%": 0.4}, // zeros and what rounds to 0 dropped
			Unplayable: []string{"er"},
			Sands:      &artifact.Slot{Right: []string{"atk%", "em", "er"}, Others: 15.5},
			Goblet:     &artifact.Slot{LowImpact: true, Best: "atk%"},
			Circlet:    &artifact.Slot{Right: []string{"cr", "cd"}, Penalty: map[string]float64{"atk%": 7.8}, Unplayable: []string{"em", "heal"}},
		}}},
		"b": {Gcsim: "g@2", Characters: []artifact.Character{{Key: "qiqi", Sands: &artifact.Slot{Right: []string{"hp%"}}}}},
		"c": {Characters: []artifact.Character{}},
	}}
	if !reflect.DeepEqual(got, want) {
		var g, w bytes.Buffer
		got.WriteTo(&g)
		want.WriteTo(&w)
		t.Errorf("slim:\n%s\nwant:\n%s", g.String(), w.String())
	}
}

func TestSlimOthers(t *testing.T) {
	s := slimSlot(&Slot{Right: []string{"pyro%"}}, map[string]float64{
		"hp%": 35.5, "def%": 35.5, "hydro%": 35.5, "em": 30.1, "atk%": 12, "phys%": 33.7,
	}, Options[1])
	if s.Others != 35.5 || !maps.Equal(s.Penalty, map[string]float64{"em": 30.1, "atk%": 12, "phys%": 33.7}) {
		t.Errorf("others %v, penalty %v", s.Others, s.Penalty)
	}
	// Ties go to the smallest; one of each stays listed.
	s = slimSlot(&Slot{}, map[string]float64{"hp%": 2, "def%": 2, "em": 1, "atk%": 1}, Options[0])
	if s.Others != 1 || !maps.Equal(s.Penalty, map[string]float64{"hp%": 2, "def%": 2}) {
		t.Errorf("tie: others %v, penalty %v", s.Others, s.Penalty)
	}
	s = slimSlot(&Slot{}, map[string]float64{"hp%": 2, "em": 1}, Options[0])
	if s.Others != 0 || len(s.Penalty) != 2 {
		t.Errorf("all different: others %v, penalty %v", s.Others, s.Penalty)
	}
	// What the page computes from the slim slot is the full penalty.
	c := &artifact.Character{Goblet: slimSlot(&Slot{Right: []string{"pyro%"}}, map[string]float64{"hp%": 35.5, "def%": 35.5, "em": 30.1}, Options[1])}
	for k, want := range map[string]float64{"hp%": 35.5, "def%": 35.5, "em": 30.1} {
		if j := c.Judge(map[string]string{"goblet": k})[1]; j.Verdict != artifact.Wrong || j.Penalty != want {
			t.Errorf("goblet %s: %v %v, want wrong %v", k, j.Verdict, j.Penalty, want)
		}
	}
}

// readFullTable reads teams/weights.json, skipping the test when the
// checkout has only its Git LFS pointer.
func readFullTable(t *testing.T) *Table {
	t.Helper()
	data, err := os.ReadFile("../../teams/weights.json")
	if os.IsNotExist(err) {
		t.Skip("no teams/weights.json")
	}
	if err != nil {
		t.Fatal(err)
	}
	if bytes.HasPrefix(data, []byte("version https://git-lfs.github.com/spec/")) {
		t.Skip("teams/weights.json is a Git LFS pointer; fetch it with git lfs pull")
	}
	full, err := ReadTable(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return full
}

// TestSlimTableCurrent checks that the embedded small table is the one
// cmd/weights -slim derives from the full table.
func TestSlimTableCurrent(t *testing.T) {
	full := readFullTable(t)
	var b bytes.Buffer
	if _, err := Slim(full).WriteTo(&b); err != nil {
		t.Fatal(err)
	}
	committed, err := os.ReadFile("../artifact/table.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b.Bytes(), committed) {
		t.Error("internal/artifact/table.json is not derived from teams/weights.json: run go run ./cmd/weights -slim internal/artifact/table.json")
	}
}
