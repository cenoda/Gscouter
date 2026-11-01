package library

import (
	"strings"

	"github.com/cenoda/gscouter/internal/enka"
)

// reactions are the reactions two elements trigger, in the order a label
// lists them. Anemo and Geo are left out: Swirl and Crystallize carry
// another element, which names the team.
var reactions = []struct{ a, b, name string }{
	{"pyro", "hydro", "Vaporize"},
	{"pyro", "cryo", "Melt"},
	{"dendro", "electro", "Quicken"},
	{"dendro", "hydro", "Bloom"},
	{"dendro", "pyro", "Burning"},
	{"hydro", "electro", "Electro-Charged"},
	{"hydro", "cryo", "Freeze"},
	{"electro", "pyro", "Overload"},
	{"electro", "cryo", "Superconduct"},
}

// elementOrder is the order a label names elements in.
var elementOrder = []string{"pyro", "hydro", "electro", "cryo", "dendro", "anemo", "geo"}

// ArchetypeLabel returns the team's archetype: the one written in
// team.yaml when it has one, otherwise one derived from the members'
// elements (DeriveArchetype).
func (t *Team) ArchetypeLabel() string {
	if t.Archetype != "" {
		return t.Archetype
	}
	return DeriveArchetype(t.Members)
}

// DeriveArchetype names a team by its members' elements: the reactions
// they can trigger ("Vaporize, Overload"), or, when they trigger none,
// the elements that deal the damage ("Hydro"; "Geo" for a team of Anemo
// and Geo only). It returns "" when no member's element is known.
func DeriveArchetype(members []string) string {
	has := map[string]bool{}
	for _, m := range members {
		if e := memberElement(m); e != "" {
			has[e] = true
		}
	}
	var names []string
	for _, r := range reactions {
		if has[r.a] && has[r.b] {
			names = append(names, r.name)
		}
	}
	if len(names) > 0 {
		return strings.Join(names, ", ")
	}
	var reacting, other []string
	for _, e := range elementOrder {
		if !has[e] {
			continue
		}
		name := strings.ToUpper(e[:1]) + e[1:]
		if e == "anemo" || e == "geo" {
			other = append(other, name)
		} else {
			reacting = append(reacting, name)
		}
	}
	if len(reacting) == 0 {
		reacting = other
	}
	return strings.Join(reacting, ", ")
}

// memberElement returns a member's element by gcsim key, the Traveler's
// from the key ("travelercryo").
func memberElement(key string) string {
	key = Canonical(key)
	if e, ok := strings.CutPrefix(key, "traveler"); ok {
		return e
	}
	return enka.Element(key)
}
