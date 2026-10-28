// Package weights calculates the artifact score's weights for a library
// team (docs/artifacts.md §4.2, §4.3, §5): with the team's reference
// build (build.txt), how much one maximum roll of each substat raises
// the team's DPS, and how the team scores with each possible main stat
// on sands, goblet and circlet.
package weights

import (
	"math"
	"slices"
)

// Roll is a substat and its maximum 5★ roll.
type Roll struct {
	Key   string
	Value float64
}

// Substats are the ten substats with their maximum 5★ rolls
// (docs/artifacts.md §4.1), in the order tables list them.
var Substats = []Roll{
	{"hp", 298.75},
	{"atk", 19.45},
	{"def", 23.15},
	{"hp%", 0.0583},
	{"atk%", 0.0583},
	{"def%", 0.0729},
	{"em", 23.31},
	{"er", 0.0648},
	{"cr", 0.0389},
	{"cd", 0.0777},
}

const (
	// RightWithin is how close to the best main stat's DPS a main stat
	// must score to be right. A slot whose every main stat is right has
	// little effect on the team's DPS (Slot.LowImpact).
	RightWithin = 0.02
	// Points is what one roll of the best substat is worth in the
	// score: a maximum CRIT DMG roll, 7.77, as in CV.
	Points = 7.77
)

// Weights returns each substat's weight from the team DPS one roll of
// it gains: the gain over the best stat's gain, at least 0. best is the
// substat with the largest gain; with no gain above 0 every weight is
// 0 and best is "".
func Weights(gains map[string]float64) (weights map[string]float64, best string) {
	weights = map[string]float64{}
	top := 0.0
	for _, r := range Substats {
		if g, ok := gains[r.Key]; ok && g > top {
			top, best = g, r.Key
		}
	}
	for _, r := range Substats {
		g, ok := gains[r.Key]
		if !ok {
			continue
		}
		w := 0.0
		if top > 0 {
			w = max(g/top, 0)
		}
		weights[r.Key] = w
	}
	return weights, best
}

// Judge returns, from the team DPS with each main stat of a slot, the
// right main stats (within RightWithin of the best, in option order)
// and the penalty of each wrong one: the DPS it loses against the best,
// in rolls of the best substat (bestGain, the DPS one such roll gains),
// times Points. Without a gain above 0 the loss cannot be converted
// and penalty is nil.
func Judge(dps map[string]float64, options []string, bestGain float64) (right []string, penalty map[string]float64) {
	top := math.Inf(-1)
	for _, k := range options {
		if d, ok := dps[k]; ok && d > top {
			top = d
		}
	}
	if math.IsInf(top, -1) {
		return nil, nil
	}
	if bestGain > 0 {
		penalty = map[string]float64{}
	}
	for _, k := range options {
		d, ok := dps[k]
		if !ok {
			continue
		}
		if d >= top*(1-RightWithin) {
			right = append(right, k)
			continue
		}
		if penalty != nil {
			penalty[k] = (top - d) / bestGain * Points
		}
	}
	return right, penalty
}

// Character is what the weights give for one character in a team.
type Character struct {
	Key string `json:"key"` // gcsim key, as build.txt writes it
	// Weights are the substats' weights, between 0 and 1.
	Weights map[string]float64 `json:"weights"`
	// Gains are the team DPS one maximum roll of each substat gains.
	Gains map[string]float64 `json:"gains"`
	// Best is the substat whose roll gains the most, "" when none
	// gains anything (every weight is then 0 and there are no
	// penalties).
	Best string `json:"best"`
	// Reordered is true when build.txt does not write the main stats in
	// the order flower, plume, sands, goblet, circlet.
	Reordered bool `json:"reordered,omitempty"`
	// Main is, per slot (sands, goblet, circlet), how the team scores
	// with each main stat there.
	Main map[string]*Slot `json:"main"`
}

// Slot is how the team scores with each main stat of one slot.
type Slot struct {
	Build string             `json:"build"` // the reference build's main stat
	DPS   map[string]float64 `json:"dps"`   // team DPS by main stat
	Right []string           `json:"right"` // within RightWithin of the best
	// LowImpact is true when every main stat is right: the slot barely
	// changes the team's DPS, so the page says so instead of ✓ or ✗.
	LowImpact bool `json:"lowImpact,omitempty"`
	// Penalty is each wrong main stat's cost in score points.
	Penalty map[string]float64 `json:"penalty,omitempty"`
}

// round rounds x to the given number of decimals.
func round(x float64, digits int) float64 {
	p := math.Pow(10, float64(digits))
	x = math.Round(x*p) / p
	if x == 0 {
		return 0 // not -0
	}
	return x
}

func roundAll(m map[string]float64, digits int) {
	for k, v := range m {
		m[k] = round(v, digits)
	}
}

// character assembles a Character from the reference DPS, the team DPS
// with each substat roll added and with each main stat per slot.
func character(key string, reordered bool, ref float64, withRoll map[string]float64, current [3]string, withMain [3]map[string]float64) Character {
	c := Character{Key: key, Reordered: reordered, Gains: map[string]float64{}, Main: map[string]*Slot{}}
	for k, d := range withRoll {
		c.Gains[k] = d - ref
	}
	c.Weights, c.Best = Weights(c.Gains)
	bestGain := c.Gains[c.Best]
	for i, name := range Slots {
		dps := withMain[i]
		right, penalty := Judge(dps, Options[i], bestGain)
		roundAll(dps, 1)
		roundAll(penalty, 1)
		low := len(dps) > 0 && len(right) == len(dps)
		c.Main[name] = &Slot{Build: current[i], DPS: dps, Right: slices.Clip(right), LowImpact: low, Penalty: penalty}
	}
	roundAll(c.Gains, 2)
	roundAll(c.Weights, 3)
	return c
}
