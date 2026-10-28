package artifact

import (
	"bytes"
	"maps"
	"math"
	"reflect"
	"runtime"
	"slices"
	"testing"
	"time"
)

func TestScoreIsCV(t *testing.T) {
	// docs/artifacts.md §4.1: with CRIT Rate and CRIT DMG at weight 1
	// and the rest at 0, the score is CV (CRIT Rate × 2 + CRIT DMG, in
	// percent). The rolls as displayed, 3.89% and 7.77%, are not quite
	// 1:2, so CRIT Rate counts 7.77 ÷ 3.89 = 1.997 times, not 2: CRIT
	// DMG alone is CV exactly, CRIT Rate within 0.13%.
	w := map[string]float64{"cr": 1, "cd": 1}
	if got := Score(w, map[string]float64{"cd": 0.218, "atk%": 0.152}); math.Abs(got-21.8) > 1e-9 {
		t.Errorf("CRIT DMG 21.8%%: score %v, want 21.8", got)
	}
	subs := map[string]float64{"cr": 0.105, "cd": 0.218, "atk%": 0.152, "em": 40, "hp": 538}
	got := Score(w, subs)
	cv := (subs["cr"]*2 + subs["cd"]) * 100
	if math.Abs(got-cv) > cv*0.0013 {
		t.Errorf("score %v, want CV %v", got, cv)
	}
}

func TestScore(t *testing.T) {
	// One maximum roll is worth weight × 7.77; keys that are not
	// substats count nothing.
	w := map[string]float64{"atk%": 0.5, "em": 0.25, "pyro%": 1}
	subs := map[string]float64{"atk%": 0.0583 * 2, "em": 23.31, "pyro%": 0.466, "er": 0.0648}
	want := 2*0.5*Points + 0.25*Points
	if got := Score(w, subs); math.Abs(got-want) > 1e-9 {
		t.Errorf("score %v, want %v", got, want)
	}
	if got := Score(nil, subs); got != 0 {
		t.Errorf("no weights: score %v, want 0", got)
	}
}

func TestJudge(t *testing.T) {
	c := &Character{
		Key:     "fischl",
		Sands:   &Slot{Right: []string{"atk%"}, Penalty: map[string]float64{"em": 30}, Others: 35.4, Unplayable: []string{"er"}},
		Goblet:  &Slot{LowImpact: true, Best: "electro%"},
		Circlet: &Slot{Right: []string{"cr", "cd"}, Penalty: map[string]float64{"em": 39}, Others: 43},
	}
	for _, tc := range []struct {
		main    map[string]string
		verdict [3]Verdict
		penalty float64
	}{
		{map[string]string{"sands": "atk%", "goblet": "phys%", "circlet": "cd"}, [3]Verdict{Right, LowImpact, Right}, 0},
		{map[string]string{"sands": "em", "goblet": "em", "circlet": "hp%"}, [3]Verdict{Wrong, LowImpact, Wrong}, 30 + 43},
		{map[string]string{"sands": "hp%", "circlet": "em"}, [3]Verdict{Wrong, Unknown, Wrong}, 35.4 + 39},
		{map[string]string{"sands": "er", "goblet": "cr", "circlet": "pyro%"}, [3]Verdict{Unplayable, Unknown, Unknown}, 0},
	} {
		js := c.Judge(tc.main)
		var got [3]Verdict
		for i, j := range js {
			got[i] = j.Verdict
			if j.Slot != Slots[i] || j.Main != tc.main[Slots[i]] {
				t.Errorf("%v: judgement %d is %s %s", tc.main, i, j.Slot, j.Main)
			}
		}
		if got != tc.verdict {
			t.Errorf("%v: verdicts %v, want %v", tc.main, got, tc.verdict)
		}
		if p := Penalty(js); math.Abs(p-tc.penalty) > 1e-9 {
			t.Errorf("%v: penalty %v, want %v", tc.main, p, tc.penalty)
		}
		if js[1].Verdict == LowImpact && js[1].Best != "electro%" {
			t.Errorf("%v: low impact goblet best %q, want electro%%", tc.main, js[1].Best)
		}
	}
	// No judgement for a slot: Unknown.
	if js := (&Character{}).Judge(map[string]string{"sands": "atk%"}); js[0].Verdict != Unknown {
		t.Errorf("no slot data: %v, want unknown", js[0].Verdict)
	}
}

func TestTableRoundTrip(t *testing.T) {
	tab := &Table{Format: TableFormat, Gcsim: "g@v1", Teams: map[string]*Team{
		"b-team": {Characters: []Character{{Key: "x", Weights: map[string]float64{"cr": 1}, Sands: &Slot{Right: []string{"atk%"}, Others: 12.5}}}},
		"a-team": {Gcsim: "g@v2", Characters: []Character{{Key: "y", Unplayable: []string{"er"}, Goblet: &Slot{LowImpact: true, Best: "em"}}}},
	}}
	var b bytes.Buffer
	if _, err := tab.WriteTo(&b); err != nil {
		t.Fatal(err)
	}
	// One team per line, in id order.
	lines := bytes.Split(bytes.TrimSpace(b.Bytes()), []byte("\n"))
	if len(lines) != 8 || !bytes.HasPrefix(lines[4], []byte(`"a-team": `)) || !bytes.HasPrefix(lines[5], []byte(`"b-team": `)) {
		t.Errorf("layout:\n%s", b.Bytes())
	}
	back, err := ReadTable(bytes.NewReader(b.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back, tab) {
		t.Errorf("read back %+v, want %+v", back, tab)
	}
	ix, err := newIndex(b.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if ix.gcsim != "g@v1" || len(ix.teams) != 2 {
		t.Errorf("index: gcsim %q, %d teams", ix.gcsim, len(ix.teams))
	}
	a, err := ix.lookup("a-team")
	if err != nil || a == nil || a.Gcsim != "g@v2" || a.Characters[0].Goblet.Best != "em" {
		t.Errorf("lookup a-team: %+v, %v", a, err)
	}
	bt, err := ix.lookup("b-team")
	if err != nil || bt == nil || bt.Gcsim != "g@v1" || bt.Character("x").Sands.Others != 12.5 {
		t.Errorf("lookup b-team: %+v, %v (want the table's gcsim filled in)", bt, err)
	}
	if c, err := ix.lookup("c-team"); c != nil || err != nil {
		t.Errorf("lookup c-team: %+v, %v, want nothing", c, err)
	}
	if _, err := newIndex([]byte(`{"format": 2, "teams": {}}`)); err == nil {
		t.Error("format 2 read without an error")
	}
}

// TestEmbedded checks table.json: the index finds every team, each
// parses to what parsing the whole table gives, and the data is sane.
func TestEmbedded(t *testing.T) {
	whole, err := ReadTable(bytes.NewReader(embedded))
	if err != nil {
		t.Fatal(err)
	}
	if Len() != len(whole.Teams) || Len() == 0 {
		t.Fatalf("index has %d teams, table %d", Len(), len(whole.Teams))
	}
	if f, g := Version(); f != TableFormat || g != whole.Gcsim || g == "" {
		t.Errorf("version %d %q", f, g)
	}
	for id, want := range whole.Teams {
		got, ok := Lookup(id)
		if !ok {
			t.Fatalf("%s not found", id)
		}
		if want.Gcsim == "" {
			want.Gcsim = whole.Gcsim
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: lookup %+v, want %+v", id, got, want)
		}
		for _, c := range got.Characters {
			for k, w := range c.Weights {
				if _, ok := MaxRolls[k]; !ok || w <= 0 || w > 1 {
					t.Errorf("%s %s: weight %s %v", id, c.Key, k, w)
				}
			}
			for i, name := range Slots {
				s := c.Slot(name)
				if s == nil {
					t.Errorf("%s %s: no %s", id, c.Key, name)
					continue
				}
				for _, k := range slices.Concat(s.Right, s.Unplayable, slices.Collect(maps.Keys(s.Penalty))) {
					if !slices.Contains(Options[i], k) {
						t.Errorf("%s %s %s: %s is not an option", id, c.Key, name, k)
					}
				}
				if s.LowImpact != (s.Best != "") || (s.LowImpact && len(s.Right) > 0) {
					t.Errorf("%s %s %s: low impact %v, best %q, right %v", id, c.Key, name, s.LowImpact, s.Best, s.Right)
				}
			}
		}
	}
	if _, ok := Lookup("no-such-team"); ok {
		t.Error("no-such-team found")
	}
}

// TestEmbeddedCost measures what the embedded table costs the server:
// the heap the index keeps and the time it takes to build, against
// parsing the whole table. The index is to stay small.
func TestEmbeddedCost(t *testing.T) {
	heap := func() uint64 {
		runtime.GC()
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		return m.HeapAlloc
	}
	before := heap()
	start := time.Now()
	ix, err := newIndex(embedded)
	took := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	indexHeap := heap() - before
	runtime.KeepAlive(ix)

	before = heap()
	start = time.Now()
	whole, err := ReadTable(bytes.NewReader(embedded))
	tookWhole := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	wholeHeap := heap() - before
	runtime.KeepAlive(whole)

	start = time.Now()
	n := 0
	for id := range ix.teams {
		if _, err := ix.lookup(id); err != nil {
			t.Fatal(err)
		}
		n++
	}
	perLookup := time.Since(start) / time.Duration(max(n, 1))
	t.Logf("table.json %d bytes, %d teams: index %.2f MB heap in %v; whole table parsed %.2f MB heap in %v; lookup %v",
		len(embedded), len(ix.teams), float64(indexHeap)/1e6, took, float64(wholeHeap)/1e6, tookWhole, perLookup)
	if indexHeap > 4<<20 {
		t.Errorf("index keeps %d bytes on the heap", indexHeap)
	}
}
