// Package gamekeys maps every name gcsl accepts for a character, weapon
// or artifact set (keys and aliases) to its key, from gscouter-engine's
// key catalog (catalog.json; README.md). It replaces gcsim's
// pkg/shortcut and pkg/core/keys on Gscouter's input path (step 2 of
// gscouter-engine's docs/independence.md). The keys are the strings
// gcsim's keys print, so they are interchangeable with gcsim's.
package gamekeys

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
)

// EngineCommit is the gscouter-engine commit whose key table catalog.json
// exports.
const EngineCommit = "8a3c5800a4e6df3dc6f39bc5cf1c8e236f0ce06d"

// CatalogVersion is the catalog_version this package reads. A catalog
// that removes or renames a field raises it.
const CatalogVersion = 1

//go:embed catalog.json
var catalogJSON []byte

// Entry is one key of the catalog.
type Entry struct {
	Key         string   `json:"key"`
	Aliases     []string `json:"aliases"`
	Implemented bool     `json:"implemented"`
	// Rarity, Element, WeaponClass and BurstEnergy are empty for content
	// without base data in the engine.
	Rarity      int     `json:"rarity"`
	Element     string  `json:"element"`
	WeaponClass string  `json:"weapon_class"`
	BurstEnergy float64 `json:"burst_energy"`
}

// Catalog is the decoded catalog.
type Catalog struct {
	Version    int     `json:"catalog_version"`
	Characters []Entry `json:"characters"`
	Weapons    []Entry `json:"weapons"`
	Sets       []Entry `json:"sets"`
}

var (
	catalog  Catalog
	chars    map[string]string
	weapons  map[string]string
	sets     map[string]string
	charData map[string]Entry
)

func init() {
	c, err := Decode(catalogJSON)
	if err != nil {
		panic(err)
	}
	catalog = c
	chars, weapons, sets = byName(c.Characters), byName(c.Weapons), byName(c.Sets)
	charData = map[string]Entry{}
	for _, e := range c.Characters {
		charData[e.Key] = e
	}
}

// Decode reads a catalog and checks that it is of CatalogVersion and
// that no name resolves to two keys.
func Decode(raw []byte) (Catalog, error) {
	var c Catalog
	if err := json.Unmarshal(raw, &c); err != nil {
		return c, fmt.Errorf("gamekeys: decode catalog: %w", err)
	}
	if c.Version != CatalogVersion {
		return c, fmt.Errorf("gamekeys: catalog_version %d, want %d", c.Version, CatalogVersion)
	}
	for what, es := range map[string][]Entry{"character": c.Characters, "weapon": c.Weapons, "set": c.Sets} {
		seen := map[string]string{}
		for _, e := range es {
			for _, n := range append([]string{e.Key}, e.Aliases...) {
				if n != strings.ToLower(n) {
					return c, fmt.Errorf("gamekeys: %s name %q is not lowercase", what, n)
				}
				if k, ok := seen[n]; ok {
					return c, fmt.Errorf("gamekeys: %s name %q is %s and %s", what, n, k, e.Key)
				}
				seen[n] = e.Key
			}
		}
	}
	return c, nil
}

func byName(es []Entry) map[string]string {
	m := map[string]string{}
	for _, e := range es {
		m[e.Key] = e.Key
		for _, a := range e.Aliases {
			m[a] = e.Key
		}
	}
	return m
}

// Char returns the key of a character name or alias, which must be
// lowercase, and whether the catalog has it.
func Char(name string) (string, bool) { k, ok := chars[name]; return k, ok }

// Weapon returns the key of a weapon name or alias, which must be
// lowercase, and whether the catalog has it.
func Weapon(name string) (string, bool) { k, ok := weapons[name]; return k, ok }

// Set returns the key of an artifact set name or alias, which must be
// lowercase, and whether the catalog has it.
func Set(name string) (string, bool) { k, ok := sets[name]; return k, ok }

// Character returns a character key's entry.
func Character(key string) (Entry, bool) { e, ok := charData[key]; return e, ok }

// All returns the catalog.
func All() Catalog { return catalog }
