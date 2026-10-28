// Package pull fills a team slot with a character the player does not
// own (docs/pull.md). This file holds the presets a slot starts from
// (§2.1): builds from the library's build.txt files, as the build
// type the rest of the site uses.
package pull

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/cenoda/gscouter/internal/enka"
	"github.com/cenoda/gscouter/internal/library"
	"github.com/genshinsim/gcsim/pkg/core/attributes"
	"github.com/genshinsim/gcsim/pkg/gcs/ast"
	"github.com/genshinsim/gcsim/pkg/gcs/parser"
)

// Parse reads the characters of gcsim setup lines, such as a library
// build.txt, in the order they appear. gcsim's own parser reads them, so
// every form a build.txt writes (aliases, constant expressions, params)
// reads as gcsim reads it. Weapon, set and character params are dropped,
// as they are for showcase builds (enka.Character has none): a preset
// runs with gcsim's defaults, like a player's build.
func Parse(build string) ([]enka.Character, error) {
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

// enkaKey returns the key the game data (and so enka.Character) uses for
// a gcsim key. They are the same except for the Traveler, which gcsim
// may call travelercryo and the game data calls aethercryo; the gender
// does not change the kit.
func enkaKey(key string) string {
	if elem, ok := strings.CutPrefix(library.Canonical(key), "traveler"); ok {
		return "aether" + elem
	}
	return key
}

// Presets holds a preset for every character of every library team that
// has a build.
type Presets struct {
	byTeam map[string]map[string]enka.Character // team ID → canonical key → build
	common map[string]enka.Character            // canonical key → most common build
}

// NewPresets reads the builds of every team in lib. A team without a
// build.txt is skipped; a build.txt that does not parse is an error.
func NewPresets(lib *library.Library) (*Presets, error) {
	p := &Presets{byTeam: map[string]map[string]enka.Character{}, common: map[string]enka.Character{}}
	type tally struct {
		c enka.Character
		n int
	}
	counts := map[string]map[string]*tally{} // canonical key → config → tally
	for _, t := range lib.Teams {
		if strings.TrimSpace(t.Build) == "" {
			continue
		}
		chars, err := Parse(t.Build)
		if err != nil {
			return nil, fmt.Errorf("%s: build.txt: %w", t.ID, err)
		}
		team := map[string]enka.Character{}
		for _, c := range chars {
			key := library.Canonical(c.Key)
			team[key] = c
			if counts[key] == nil {
				counts[key] = map[string]*tally{}
			}
			cfg := c.Config()
			if counts[key][cfg] == nil {
				counts[key][cfg] = &tally{c: c}
			}
			counts[key][cfg].n++
		}
		p.byTeam[t.ID] = team
	}
	for key, builds := range counts {
		// The build most teams use; on a tie the lower constellation,
		// then the lower refinement, then the config, so the choice
		// does not depend on map order.
		best := slices.MinFunc(slices.Collect(maps.Keys(builds)), func(a, b string) int {
			x, y := builds[a], builds[b]
			return cmp.Or(
				cmp.Compare(y.n, x.n),
				cmp.Compare(x.c.Cons, y.c.Cons),
				cmp.Compare(x.c.Weapon.Refine, y.c.Weapon.Refine),
				cmp.Compare(a, b),
			)
		})
		p.common[key] = builds[best].c
	}
	return p, nil
}

// For returns the preset for character key in team t (docs/pull.md
// §2.1): the character's build in the team's build.txt, the build the
// rotation was validated with. A team without one gets the build most
// library teams give the character. ok is false when no team has the
// character. The result is a copy the caller may change.
func (p *Presets) For(t *library.Team, key string) (c enka.Character, ok bool) {
	key = library.Canonical(key)
	if c, ok = p.byTeam[t.ID][key]; !ok {
		c, ok = p.common[key]
	}
	if ok {
		c.Sets = maps.Clone(c.Sets)
		c.Stats = maps.Clone(c.Stats)
	}
	return c, ok
}
