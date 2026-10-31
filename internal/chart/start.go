package chart

import (
	"slices"
	"strings"

	"github.com/cenoda/gscouter/internal/library"
	"github.com/genshinsim/gcsim/pkg/catalog"
	"github.com/genshinsim/gcsim/pkg/shortcut"
)

// HPKeys are the two params that together set the HP a character starts
// at. gcsim (pkg/core/player) starts it at max HP × start_hp% / 100 plus
// start_hp when both are positive, at either alone otherwise, and at max
// HP with neither. WithParams treats them as one setting: a chart that
// sets either key sets the starting HP alone, and the team's keys are
// dropped (docs/cycle-swap.md §11.4).
var HPKeys = []string{"start_hp", "start_hp%"}

func isHPKey(p Param) bool { return slices.Contains(HPKeys, p.Key) }

// mergeParams is the params a character plays with: def, the library
// team's, with own, the chart's, in their place. A key own sets replaces
// def's, and either of HPKeys in own replaces both of def's.
func mergeParams(def, own []Param) []Param {
	if slices.ContainsFunc(own, isHPKey) {
		def = slices.DeleteFunc(slices.Clone(def), isHPKey)
	}
	var ps []Param
	for _, p := range def {
		if !slices.ContainsFunc(own, func(q Param) bool { return q.Key == p.Key }) {
			ps = append(ps, p)
		}
	}
	return append(ps, own...)
}

// Resource is what start_energy sets for a character in gcsim, and its
// range: from 0 to Max, which is also where the character starts with
// no start_energy.
type Resource struct {
	// Kind is "energy" (burst energy), or the character's own resource:
	// "fighting_spirit" (Mavuika) or "serpents_subtlety" (Skirk).
	Kind string `json:"kind"`
	Max  int    `json:"max"`
}

// StartResource is name's Resource. Most characters start with their
// burst's energy cost, which gcsim (pkg/core) caps start_energy at; the
// cost is the character data's (pkg/catalog), which matches the
// EnergyMax each character sets. Mavuika and Skirk have no burst energy
// and read start_energy as their own resource, clamped to 0..200
// (internal/characters/mavuika, maxFightingSpirit) and 0..100
// (internal/characters/skirk, maxSerpentsSubtlety). A character the data
// lacks gets 100.
func StartResource(name string) Resource {
	switch library.Canonical(name) {
	case "mavuika":
		return Resource{"fighting_spirit", 200}
	case "skirk":
		return Resource{"serpents_subtlety", 100}
	}
	if k, ok := shortcut.CharNameToKey[strings.ToLower(name)]; ok {
		if d := catalog.CharacterMap[k]; d != nil {
			if cost := int(d.GetSkillDetails().GetBurstEnergyCost()); cost > 0 {
				return Resource{"energy", cost}
			}
		}
	}
	return Resource{"energy", 100}
}
