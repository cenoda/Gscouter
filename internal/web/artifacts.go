package web

import (
	"cmp"
	"fmt"
	"html/template"
	"maps"
	"slices"
	"strings"

	"github.com/cenoda/gscouter/internal/enka"
)

// Result pages show each character's artifact totals, and keep the
// pieces folded until the visitor opens them (docs/artifacts.md §2).

// flatStats are the stats shown as whole numbers; the rest are
// percentages.
var flatStats = map[string]bool{"hp": true, "atk": true, "def": true, "em": true}

// totalOrder is the order totals are shown in, the character screen's.
var totalOrder = []string{
	"hp", "atk", "def", "hp%", "atk%", "def%", "em", "er", "cr", "cd", "heal",
	"pyro%", "hydro%", "anemo%", "electro%", "dendro%", "cryo%", "geo%", "phys%",
}

// statText is a stat with its value as the game shows it, e.g. "CRIT
// Rate 10.5%" or "HP 4,780".
func (l localizer) statText(s enka.StatValue) string {
	name := cmp.Or(enka.StatName(s.Key, l.Lang), s.Key)
	if flatStats[s.Key] {
		return name + " " + thousands(s.Value)
	}
	return fmt.Sprintf("%s %.1f%%", name, s.Value*100)
}

// setName is a set's in-game name, or its gcsim key.
func (l localizer) setName(key string) string {
	return cmp.Or(enka.ArtifactName(key, l.Lang), key)
}

// totals describes a character's artifacts: the sets, then each total,
// e.g. "Crimson Witch of Flames 4 · HP 4,780 · ATK 311 · ...".
func (l localizer) totals(c enka.Character) string {
	var parts []string
	for _, set := range slices.Sorted(maps.Keys(c.Sets)) {
		parts = append(parts, fmt.Sprintf("%s %d", l.setName(set), c.Sets[set]))
	}
	for _, key := range totalOrder {
		if v := c.Stats[key]; v != 0 {
			parts = append(parts, l.statText(enka.StatValue{Key: key, Value: v}))
		}
	}
	return strings.Join(parts, " · ")
}

// pieceText describes a piece below its slot: set, rarity and level.
func (l localizer) pieceText(p enka.Artifact) string {
	s := fmt.Sprintf("%d★ +%d", p.Rarity, p.Level)
	if p.Set != "" {
		s = l.setName(p.Set) + " · " + s
	}
	return s
}

// subsText lists a piece's substats.
func (l localizer) subsText(p enka.Artifact) string {
	parts := make([]string, len(p.Subs))
	for i, s := range p.Subs {
		parts[i] = l.statText(s)
	}
	return strings.Join(parts, " · ")
}

func hasFullPieces(chars []enka.Character) bool {
	return slices.ContainsFunc(chars, func(c enka.Character) bool { return c.PieceDetail == enka.PiecesFull })
}

func hasPieces(chars []enka.Character) bool {
	return slices.ContainsFunc(chars, func(c enka.Character) bool { return c.PieceDetail != enka.PiecesNone })
}

// Stripped is the team's selection with its pieces left out, for a link
// whose sharer chose so: the score is the same (docs/artifacts.md §2).
func (tr teamResult) Stripped() string {
	sel := tr.Selection
	if sel.Builds == nil {
		return sel.String()
	}
	sel.Builds = make([]enka.Character, len(tr.Selection.Builds))
	for i, c := range tr.Selection.Builds {
		sel.Builds[i] = c.Stripped()
	}
	return sel.String()
}

// artifactFuncs are the template functions for artifacts in l's
// language.
func artifactFuncs(l localizer) template.FuncMap {
	return template.FuncMap{
		"totals":    l.totals,
		"stat":      l.statText,
		"pieceText": l.pieceText,
		"subsText":  l.subsText,
		"slotName":  func(s enka.Slot) string { return l.T("art.slot." + s.String()) },
		"hasPieces": hasPieces,
		"hasFull":   hasFullPieces,
		"full":      func(c enka.Character) bool { return c.PieceDetail == enka.PiecesFull },
		"none":      func(c enka.Character) bool { return c.PieceDetail == enka.PiecesNone },
	}
}
