// Package artifact holds what the result page needs for the artifact
// score (docs/artifacts.md §4): the small weights table, embedded in
// the binary, and the arithmetic of the substat score and the main
// stat judgement. It depends on the standard library only, so the
// server and the browser build can use it without the simulation code
// the table is calculated with (internal/weights, cmd/weights).
package artifact

import "slices"

// MaxRolls are the ten substats' maximum 5★ rolls by gcsim key, as
// fractions for percentages (docs/artifacts.md §4.1).
var MaxRolls = map[string]float64{
	"hp":   298.75,
	"atk":  19.45,
	"def":  23.15,
	"hp%":  0.0583,
	"atk%": 0.0583,
	"def%": 0.0729,
	"em":   23.31,
	"er":   0.0648,
	"cr":   0.0389,
	"cd":   0.0777,
}

// Points is what one roll of a substat with weight 1 is worth: a
// maximum CRIT DMG roll, 7.77, as in CV.
const Points = 7.77

// Slots are the pieces with a main stat to choose, in order.
var Slots = []string{"sands", "goblet", "circlet"}

// Options are the main stats each slot can have in the game, by gcsim
// key (Slots order).
var Options = [3][]string{
	{"hp%", "atk%", "def%", "em", "er"},
	{"hp%", "atk%", "def%", "em", "pyro%", "hydro%", "electro%", "cryo%", "anemo%", "geo%", "dendro%", "phys%"},
	{"hp%", "atk%", "def%", "em", "cr", "cd", "heal"},
}

// Score is the substat score of substat totals (gcsim key → value,
// percentages as fractions) with the given weights: Σ value ÷ maximum
// roll × weight × Points. Keys that are not substats, and substats
// without a weight, count nothing.
func Score(weights, subs map[string]float64) float64 {
	s := 0.0
	for k, v := range subs {
		if roll, ok := MaxRolls[k]; ok {
			s += v / roll * weights[k] * Points
		}
	}
	return s
}

// Verdict is how a main stat is judged (docs/artifacts.md §4.3).
type Verdict int

const (
	// Unknown: no main stat given, or one the slot cannot have.
	Unknown Verdict = iota
	// Right: within 2% of the best main stat's team DPS.
	Right
	// Wrong: costs Judgement.Penalty points.
	Wrong
	// LowImpact: every main stat of the slot is right; the page shows
	// the best one (Judgement.Best) and no penalty.
	LowImpact
	// Unplayable: the rotation cannot be played with this main stat,
	// so it is neither right nor wrong.
	Unplayable
)

func (v Verdict) String() string {
	return [...]string{"unknown", "right", "wrong", "low impact", "unplayable"}[v]
}

// Judgement is one slot's main stat, judged.
type Judgement struct {
	Slot    string // sands, goblet or circlet
	Main    string // the main stat judged, gcsim key
	Verdict Verdict
	// Penalty is a wrong main stat's cost in score points; 0 otherwise,
	// and for a wrong main stat of a character whose substats gain the
	// team nothing (no weights to convert the loss with).
	Penalty float64
	Best    string // the best-scoring main stat, on a LowImpact slot
}

// Judge judges a character's main stats, given by slot (sands, goblet,
// circlet → gcsim key), in Slots order. A slot the table has no
// judgement for, or a key the slot cannot have, is Unknown.
func (c *Character) Judge(main map[string]string) []Judgement {
	out := make([]Judgement, len(Slots))
	for i, name := range Slots {
		k := main[name]
		j := Judgement{Slot: name, Main: k}
		s := c.Slot(name)
		switch {
		case s == nil || !slices.Contains(Options[i], k):
		case slices.Contains(s.Unplayable, k):
			j.Verdict = Unplayable
		case s.LowImpact:
			j.Verdict, j.Best = LowImpact, s.Best
		case slices.Contains(s.Right, k):
			j.Verdict = Right
		default:
			j.Verdict, j.Penalty = Wrong, s.Others
			if p, ok := s.Penalty[k]; ok {
				j.Penalty = p
			}
		}
		out[i] = j
	}
	return out
}

// Penalty sums the judgements' penalties.
func Penalty(js []Judgement) float64 {
	p := 0.0
	for _, j := range js {
		p += j.Penalty
	}
	return p
}

// Score is the character's substat score for the given substat totals.
func (c *Character) Score(subs map[string]float64) float64 {
	return Score(c.Weights, subs)
}

// Slot returns the judgement data of a slot by name, nil if none.
func (c *Character) Slot(name string) *Slot {
	switch name {
	case "sands":
		return c.Sands
	case "goblet":
		return c.Goblet
	case "circlet":
		return c.Circlet
	}
	return nil
}
