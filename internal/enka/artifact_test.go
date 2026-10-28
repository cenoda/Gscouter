package enka

import (
	"bytes"
	"errors"
	"maps"
	"os"
	"reflect"
	"testing"
)

func parseSkirk(t *testing.T, edit func([]byte) []byte) Character {
	t.Helper()
	raw, err := os.ReadFile("testdata/skirk.json")
	if err != nil {
		t.Fatal(err)
	}
	if edit != nil {
		raw = edit(raw)
	}
	p, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return p.Characters[0]
}

// pct divides at run time, as the converter does, so the sums below are
// bit for bit the converter's.
func pct(v float64) float64 { return v / 100 }

// Keeping pieces must not change the totals the gcsim config is built
// from, with or without the slots Enka reports.
func TestStatsUnchanged(t *testing.T) {
	wantStats := map[string]float64{
		"hp": 4780, "atk": 19, "cr": pct(10.5) + pct(3.9), "cd": pct(21), "cryo%": 0.466,
	}
	wantSets := map[string]int{"finaleofthedeepgalleries": 2}
	noSlots := func(raw []byte) []byte {
		raw = bytes.ReplaceAll(raw, []byte(`"equipType": "EQUIP_BRACER", `), nil)
		return bytes.ReplaceAll(raw, []byte(`"equipType": "EQUIP_RING", `), nil)
	}
	for name, edit := range map[string]func([]byte) []byte{"with slots": nil, "without slots": noSlots} {
		c := parseSkirk(t, edit)
		if !reflect.DeepEqual(c.Stats, wantStats) || !reflect.DeepEqual(c.Sets, wantSets) || c.UnknownSets != 0 {
			t.Errorf("%s: stats %v sets %v unknown %d", name, c.Stats, c.Sets, c.UnknownSets)
		}
	}
	// Without slots the pieces cannot be placed, so there are none.
	if c := parseSkirk(t, noSlots); c.PieceDetail != PiecesNone || c.Pieces != nil {
		t.Errorf("without slots: %v %+v", c.PieceDetail, c.Pieces)
	}
}

func TestParsePieces(t *testing.T) {
	c := parseSkirk(t, nil)
	want := []Artifact{
		{Slot: Flower, Set: "finaleofthedeepgalleries", Rarity: 5, Level: 20, Main: StatValue{"hp", 4780},
			Subs: []StatValue{{"cr", pct(10.5)}, {"cd", pct(21)}}},
		{Slot: Goblet, Set: "finaleofthedeepgalleries", Rarity: 5, Level: 20, Main: StatValue{"cryo%", 0.466},
			Subs: []StatValue{{"cr", pct(3.9)}, {"atk", 19}}},
	}
	if c.PieceDetail != PiecesFull || !reflect.DeepEqual(c.Pieces, want) {
		t.Errorf("pieces %v %+v, want %+v", c.PieceDetail, c.Pieces, want)
	}
}

func TestPackPieces(t *testing.T) {
	full := parseSkirk(t, nil)
	stripped := full.Stripped()
	if stripped.PieceDetail != PiecesStripped || stripped.Pieces[1].Subs != nil || stripped.Pieces[1].Set != "" ||
		stripped.Pieces[1].Main != full.Pieces[1].Main || full.Pieces[1].Subs == nil {
		t.Fatalf("stripped %+v, full %+v", stripped.Pieces, full.Pieces)
	}
	none := full
	none.Pieces, none.PieceDetail = nil, PiecesNone
	for _, c := range []Character{full, stripped, none} {
		got, err := Unpack(Pack([]Character{c}))
		if err != nil {
			t.Fatalf("%v: %v", c.PieceDetail, err)
		}
		if got[0].Config() != c.Config() {
			t.Errorf("%v: config %q, want %q", c.PieceDetail, got[0].Config(), c.Config())
		}
		got[0].Stats, c.Stats = nil, nil
		if !reflect.DeepEqual(got[0], c) {
			t.Errorf("%v: got %+v, want %+v", c.PieceDetail, got[0], c)
		}
	}

	// A version 1 build is a version 2 one with no pieces byte.
	v2 := Pack([]Character{none})
	v1 := append([]byte{1}, v2[1:len(v2)-1]...)
	got, err := Unpack(v1)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].PieceDetail != PiecesNone || got[0].Pieces != nil || got[0].Config() != full.Config() {
		t.Errorf("version 1: %+v", got[0])
	}
	if _, err := Unpack(Pack([]Character{full})[:len(v2)]); err == nil {
		t.Error("version 2 without pieces read")
	}
}

// The showcase holds up to 12 characters (design §4.1).
func TestPackTwelve(t *testing.T) {
	c := parseSkirk(t, nil)
	chars := make([]Character, 12)
	for i := range chars {
		chars[i] = c
	}
	got, err := Unpack(Pack(chars))
	if err != nil || len(got) != 12 {
		t.Fatalf("got %d characters, err %v", len(got), err)
	}
}

func TestUnpackRejectsPieces(t *testing.T) {
	good := parseSkirk(t, nil)
	clone := func(c Character) Character {
		c.Stats = maps.Clone(c.Stats)
		c.Pieces = append([]Artifact(nil), c.Pieces...)
		for i := range c.Pieces {
			c.Pieces[i].Subs = append([]StatValue(nil), c.Pieces[i].Subs...)
		}
		return c
	}
	sub := func(c *Character, s StatValue) { c.Pieces[0].Subs = append(c.Pieces[0].Subs, s) }
	for name, edit := range map[string]func(*Character){
		"rarity 6":        func(c *Character) { c.Pieces[0].Rarity = 6 },
		"rarity 0":        func(c *Character) { c.Pieces[0].Rarity = 0 },
		"level 21":        func(c *Character) { c.Pieces[0].Level = 21 },
		"level 17 at 4★":  func(c *Character) { c.Pieces[0].Rarity = 4; c.Pieces[0].Level = 17 },
		"slot 5":          func(c *Character) { c.Pieces[1].Slot = 5 },
		"two flowers":     func(c *Character) { c.Pieces[1].Slot = Flower },
		"out of order":    func(c *Character) { c.Pieces[0], c.Pieces[1] = c.Pieces[1], c.Pieces[0] },
		"cr flower":       func(c *Character) { c.Pieces[0].Main.Key = "cr" },
		"main as substat": func(c *Character) { sub(c, StatValue{"hp", 100}) },
		"substat twice":   func(c *Character) { sub(c, StatValue{"cr", 0.039}) },
		"elemental sub":   func(c *Character) { sub(c, StatValue{"pyro%", 0.05}) },
		"five substats": func(c *Character) {
			sub(c, StatValue{"em", 20})
			sub(c, StatValue{"er", 0.05})
			sub(c, StatValue{"atk", 19})
		},
		"huge substat":      func(c *Character) { c.Pieces[0].Subs[0].Value = 0.3 },
		"huge at level 0":   func(c *Character) { c.Pieces[0].Level = 0; c.Pieces[1].Level = 0 },
		"zero substat":      func(c *Character) { c.Pieces[0].Subs[0].Value = 0 },
		"unknown set piece": func(c *Character) { c.Pieces[0].Set = "" },
		"pieces over sets":  func(c *Character) { c.Sets = map[string]int{"finaleofthedeepgalleries": 1} },
		"stripped short":    func(c *Character) { *c = c.Stripped(); c.Pieces = c.Pieces[:1] },
		// Pieces must agree with the totals the score is calculated from.
		"off the totals":   func(c *Character) { c.Stats["cd"] += 0.01 },
		"total missing":    func(c *Character) { delete(c.Stats, "atk") },
		"stripped too low": func(c *Character) { *c = c.Stripped(); c.Stats["cryo%"] = 0.3 },
		"stripped extra":   func(c *Character) { *c = c.Stripped(); c.Stats["heal"] = 0.1 },
		"detail 3":         func(c *Character) { c.PieceDetail = 3 },
	} {
		c := clone(good)
		edit(&c)
		b := Pack([]Character{c})
		if name == "detail 3" {
			// Pack never writes it; patch the byte.
			b = Pack([]Character{clone(good).Stripped()})
			b[len(b)-1-3*len(good.Pieces)-1] = 3
		}
		if _, err := Unpack(b); !errors.Is(err, errPacked) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

// Totals within the substats' one-decimal rounding of their pieces
// read; so do stripped pieces under totals with substats on top.
func TestUnpackTotalsRounding(t *testing.T) {
	c := parseSkirk(t, nil)
	c.Stats = maps.Clone(c.Stats)
	c.Stats["cr"] += 0.0009 // two CRIT Rate substats: up to 0.1% off
	c.Stats["atk"] -= 0.04
	if _, err := Unpack(Pack([]Character{c})); err != nil {
		t.Errorf("full: %v", err)
	}
	s := c.Stripped()
	s.Stats["em"] = 40
	if _, err := Unpack(Pack([]Character{s})); err != nil {
		t.Errorf("stripped: %v", err)
	}
}

func TestFingerprint(t *testing.T) {
	c := parseSkirk(t, nil)
	got, _ := Unpack(Pack([]Character{c}))
	for i := range c.Pieces {
		if a, b := c.Pieces[i].Fingerprint(), got[0].Pieces[i].Fingerprint(); a != b {
			t.Errorf("piece %d: %q after packing, %q before", i, b, a)
		}
	}
	other := c.Pieces[0]
	other.Subs = []StatValue{{"cr", pct(10.9)}, {"cd", pct(21)}}
	if other.Fingerprint() == c.Pieces[0].Fingerprint() {
		t.Error("different substats, same fingerprint")
	}
	if c.Pieces[0].Fingerprint() == c.Pieces[1].Fingerprint() {
		t.Error("different slots, same fingerprint")
	}
}
