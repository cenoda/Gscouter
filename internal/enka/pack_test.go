package enka

import (
	"errors"
	"os"
	"reflect"
	"testing"
)

func TestPackRoundTrip(t *testing.T) {
	raw, err := os.ReadFile("testdata/skirk.json")
	if err != nil {
		t.Fatal(err)
	}
	p, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	c := p.Characters[0]
	c.UnknownSets = 1
	got, err := Unpack(Pack([]Character{c, c}))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d characters, want 2", len(got))
	}
	// Stats come back at Config's precision, so the config is the same.
	if got[0].Config() != c.Config() {
		t.Errorf("config:\n%s\nwant:\n%s", got[0].Config(), c.Config())
	}
	got[0].Stats, c.Stats = nil, nil
	if !reflect.DeepEqual(got[0], c) {
		t.Errorf("got %+v, want %+v", got[0], c)
	}
}

func TestUnpackRejects(t *testing.T) {
	good := Character{
		Key: "skirk", Level: 90, MaxLevel: 90, Cons: 2, Talents: [3]int{9, 10, 8},
		Weapon: Weapon{Key: "azurelight", Refine: 1, Level: 90, MaxLevel: 90},
		Sets:   map[string]int{"finaleofthedeepgalleries": 4},
		Stats:  map[string]float64{"cr": 0.5},
	}
	if _, err := Unpack(Pack([]Character{good})); err != nil {
		t.Fatalf("good build: %v", err)
	}
	for name, edit := range map[string]func(*Character){
		"unknown character": func(c *Character) { c.Key = "nobody" },
		// A key that is not a gcsim key could write config lines.
		"injected key":    func(c *Character) { c.Key = "skirk char lvl=1/1;\nskirk" },
		"unknown weapon":  func(c *Character) { c.Weapon.Key = "stick" },
		"unknown set":     func(c *Character) { c.Sets = map[string]int{"socks": 4} },
		"six artifacts":   func(c *Character) { c.Sets["gladiatorsfinale"] = 2 },
		"constellation 7": func(c *Character) { c.Cons = 7 },
		"refinement 0":    func(c *Character) { c.Weapon.Refine = 0 },
		"above its cap":   func(c *Character) { c.MaxLevel = 80 },
		"huge stat":       func(c *Character) { c.Stats["atk"] = 1e7 },
	} {
		c := good
		c.Sets = map[string]int{"finaleofthedeepgalleries": 4}
		c.Stats = map[string]float64{"cr": 0.5}
		edit(&c)
		if _, err := Unpack(Pack([]Character{c})); !errors.Is(err, errPacked) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	b := Pack([]Character{good})
	for name, b := range map[string][]byte{
		"empty":     nil,
		"version 9": append([]byte{9}, b[1:]...),
		"truncated": b[:len(b)-1],
		"trailing":  append(b, 0),
	} {
		if _, err := Unpack(b); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}
