package pull

import (
	"errors"
	"io/fs"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/genshinsim/gcsim/pkg/core/attributes"
	"github.com/genshinsim/gcsim/pkg/gcs/ast"
	"github.com/genshinsim/gcsim/pkg/gcs/parser"

	"github.com/cenoda/gscouter/internal/config"
	"github.com/cenoda/gscouter/internal/enka"
	"github.com/cenoda/gscouter/internal/gse"
	"github.com/cenoda/gscouter/internal/library"
	"github.com/cenoda/gscouter/internal/steady"
	"github.com/cenoda/gscouter/teams"
)

// gcsim's parser stays only here, until step 5 of gscouter-engine's
// docs/independence.md: these are Parse and config.Characters as they
// were on gcsim's parser, to check that the engine's parse API reads
// every build and library config alike.

func gcsimParse(build string) ([]enka.Character, error) {
	res, _, err := parser.New(ast.NewFile(), build).Parse()
	if err != nil {
		return nil, err
	}
	chars := make([]enka.Character, 0, len(res.Characters))
	for _, p := range res.Characters {
		c := enka.Character{
			Key:      enkaKey(p.Base.Key.String()),
			Level:    p.Base.Level,
			MaxLevel: p.Base.MaxLevel,
			Cons:     p.Base.Cons,
			Talents:  [3]int{p.Talents.Attack, p.Talents.Skill, p.Talents.Burst},
			Weapon: enka.Weapon{
				Key:      p.Weapon.Key.String(),
				Refine:   p.Weapon.Refine,
				Level:    p.Weapon.Level,
				MaxLevel: p.Weapon.MaxLevel,
			},
			Sets:  map[string]int{},
			Stats: map[string]float64{},
		}
		for set, n := range p.Sets {
			c.Sets[set.String()] = n
		}
		for i, v := range p.Stats {
			if v != 0 && i != int(attributes.NoStat) {
				c.Stats[attributes.Stat(i).String()] = v
			}
		}
		chars = append(chars, c)
	}
	return chars, nil
}

func gcsimCharacters(cfg string) ([]string, error) {
	simcfg, _, err := parser.New(ast.NewFile(), cfg).Parse()
	if err != nil {
		return nil, err
	}
	if len(simcfg.Errors) > 0 {
		return nil, errors.Join(simcfg.Errors...)
	}
	chars := make([]string, len(simcfg.Characters))
	for i, p := range simcfg.Characters {
		chars[i] = p.Base.Key.String()
	}
	return chars, nil
}

// Every build.txt of the library, alternatives included, reads the same
// on the engine's parse API as on gcsim's parser.
func TestParseAgreesOnBuilds(t *testing.T) {
	needParse(t)
	n := 0
	err := fs.WalkDir(os.DirFS("../../teams"), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.Name() != "build.txt" {
			return err
		}
		raw, err := os.ReadFile("../../teams/" + path)
		if err != nil {
			return err
		}
		got, err1 := Parse(string(raw))
		want, err2 := gcsimParse(string(raw))
		switch {
		case (err1 == nil) != (err2 == nil):
			t.Errorf("%s: engine %v, gcsim %v", path, err1, err2)
		case err1 == nil && !reflect.DeepEqual(got, want):
			t.Errorf("%s:\nengine %+v\ngcsim  %+v", path, got, want)
		}
		n++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if n < 4000 {
		t.Errorf("read %d builds, want the library's", n)
	}
}

// Every library config, as gscouter -dump assembles it, gives the same
// characters, and the same refusal, on both.
func TestCharactersAgreeOnLibraryConfigs(t *testing.T) {
	needParse(t)
	lib, err := library.LoadLibrary(teams.FS)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, team := range lib.Teams {
		for _, r := range team.Rotations() {
			if team.Build == "" {
				continue
			}
			cfg, err := steady.LibraryConfig(r, team.Build, 1000, 450)
			if err != nil {
				continue // a rotation steady cannot loop is no config
			}
			got, err1 := config.Characters(cfg)
			want, err2 := gcsimCharacters(cfg)
			if (err1 == nil) != (err2 == nil) || !slices.Equal(got, want) {
				t.Errorf("%s: engine %v %v, gcsim %v %v", r.ID, got, err1, want, err2)
			}
			n++
		}
	}
	if n < 4000 {
		t.Errorf("checked %d configs, want the library's", n)
	}
}

// Malformed setup lines are refused by both, and add stats random, the
// one declared difference, is refused by the engine with a clear error.
func TestParseAgreesOnMalformed(t *testing.T) {
	needParse(t)
	for name, build := range map[string]string{
		"unknown character": "nobody char lvl=90/90 cons=0 talent=9,9,9;\n",
		"unknown weapon":    "hutao char lvl=90/90 cons=0 talent=9,9,9;\nhutao add weapon=\"nope\" refine=1 lvl=90/90;\n",
		"unknown set":       "hutao char lvl=90/90 cons=0 talent=9,9,9;\nhutao add set=\"nope\" count=4;\n",
		"unterminated":      "hutao char lvl=90/90 cons=0 talent=9,9,9\n",
		"bad talents":       "hutao char lvl=90/90 cons=0 talent=9,9;\n",
		"stat not constant": "hutao char lvl=90/90 cons=0 talent=9,9,9;\nhutao add stats hp=x;\n",
	} {
		_, err1 := Parse(build)
		_, err2 := gcsimParse(build)
		if err1 == nil || err2 == nil {
			t.Errorf("%s: engine %v, gcsim %v: want both to refuse", name, err1, err2)
		}
	}
	random := "hutao char lvl=90/90 cons=0 talent=9,9,9;\nhutao add stats random rarity=5 sand=hp% goblet=pyro% circlet=cd;\n"
	if _, err := gcsimParse(random); err != nil {
		t.Errorf("gcsim refuses add stats random: %v", err)
	}
	_, err := Parse(random)
	var pe *gse.ParseError
	if !errors.As(err, &pe) || pe.Class != "unsupported" || !strings.Contains(err.Error(), "not supported by Gscouter's parser") {
		t.Errorf("add stats random: %v, want a clear unsupported error", err)
	}
}
