// Package steady scores a rotation looped endlessly. gcsim makes a
// character stand still until its burst has the energy or its skill is
// off cooldown; a player would rather fill that time. When the looped
// rotation stands still, Score runs it again with fillers (see
// config.Fill) and keeps the best result, so waiting stays the score
// only where waiting is the best play.
package steady

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/cenoda/gscouter/internal/config"
	"github.com/cenoda/gscouter/internal/library"
	"github.com/cenoda/gscouter/internal/score"
)

// minWait is how long, in seconds per minute, a character must stand
// still before it gets a filler.
const minWait = 1.0

// maxRuns bounds how many rotations Score simulates.
const maxRuns = 16

type Outcome struct {
	Result *score.Result
	// Filled is false when the rotation as written scored best.
	// Otherwise Energy maps the characters whose energy waits were
	// handled to how, and Skill lists those whose skill cooldowns were
	// filled.
	Filled bool
	Energy map[string]config.Filler
	Skill  []string
	// Looped is the rotation that scored best, as run.
	Looped string
}

// energyFillers are the ways Score tries for each character that waits
// for burst energy: fill the wait, or pass the burst.
var energyFillers = []config.Filler{config.FillSelf, config.FillParty, config.FillSkip, config.FillBrief}

// Score loops rotation and scores it with run, which assembles and
// simulates a looped rotation. members are the characters as the
// rotation names them.
//
// Whether to fill a wait for burst energy or pass the burst differs per
// member (Hu Tao passes hers, Yelan fills for hers) and depends on the
// others: once Hu Tao passes, the rotation is shorter and Furina runs
// short. So every run that leaves members standing still is followed by
// runs that handle them, each way: all of them one way, and, when there
// are several, each one alone. Follow-ups that handle all of them go
// first, then those of better runs, and at most maxRuns rotations run in
// all.
func Score(rotation string, members []string, run func(looped string) (*score.Result, error)) (*Outcome, error) {
	looped, err := config.Loop(rotation)
	if err != nil {
		return nil, err
	}
	res, err := run(looped)
	if err != nil {
		return nil, err
	}
	best := &Outcome{Result: res, Looped: looped}

	type candidate struct {
		energy   map[string]config.Filler
		skill    []string
		single   bool    // handles one of several members
		priority float64 // DPS of the run it follows up
	}
	var queue []candidate
	followUp := func(energy map[string]config.Filler, skill []string, res *score.Result) {
		handled := slices.Sorted(maps.Keys(energy))
		e, sk := addStarved(res, members, handled, skill)
		added := e[len(handled):]
		if len(added) == 0 && len(sk) == len(skill) {
			return
		}
		with := func(f config.Filler, chars ...string) map[string]config.Filler {
			out := maps.Clone(energy)
			if out == nil {
				out = map[string]config.Filler{}
			}
			for _, c := range chars {
				out[c] = f
			}
			return out
		}
		if len(added) == 0 {
			queue = append(queue, candidate{energy, sk, false, res.DPS})
			return
		}
		for _, f := range energyFillers {
			queue = append(queue, candidate{with(f, added...), sk, false, res.DPS})
		}
		if len(added) > 1 {
			for _, c := range added {
				for _, f := range energyFillers {
					queue = append(queue, candidate{with(f, c), sk, true, res.DPS})
				}
			}
		}
	}
	followUp(nil, nil, res)

	seen := map[string]bool{rotation: true}
	for len(queue) > 0 && len(seen) < maxRuns {
		i := 0
		for j, c := range queue {
			if c.single != queue[i].single && !c.single || c.single == queue[i].single && c.priority > queue[i].priority {
				i = j
			}
		}
		c := queue[i]
		queue = slices.Delete(queue, i, i+1)
		filled := config.Fill(rotation, c.energy, c.skill, members)
		if seen[filled] {
			continue
		}
		seen[filled] = true
		looped, err := config.Loop(filled)
		if err != nil {
			return nil, err
		}
		res, err := run(looped)
		if err != nil {
			if isFatal(err) {
				return nil, err
			}
			// A filler can lead a rotation into actions gcsim rejects;
			// that filler is then not an option.
			continue
		}
		if res.DPS > best.Result.DPS {
			best = &Outcome{Result: res, Filled: true, Energy: c.energy, Skill: c.skill, Looped: looped}
		}
		followUp(c.energy, c.skill, res)
	}
	return best, nil
}

// addStarved returns energy and skill extended with the members that
// stand still in res for burst energy or a skill cooldown.
func addStarved(res *score.Result, members, energy, skill []string) ([]string, []string) {
	energy, skill = slices.Clone(energy), slices.Clone(skill)
	for _, c := range res.Characters {
		for _, m := range members {
			if library.Canonical(m) != library.Canonical(c.Name) {
				continue
			}
			if c.Waits["insufficient_energy"] >= minWait && !slices.Contains(energy, m) {
				energy = append(energy, m)
			}
			if c.Waits["skill_cd"] >= minWait && !slices.Contains(skill, m) {
				skill = append(skill, m)
			}
		}
	}
	return energy, skill
}

// String describes the fillers in English, "" when there are none.
func (o *Outcome) String() string {
	if !o.Filled {
		return ""
	}
	var parts []string
	for _, g := range o.ByFiller() {
		if g.How == config.FillSkip {
			parts = append(parts, fmt.Sprintf("bursts of %s skipped when short of energy", strings.Join(g.Chars, ", ")))
		} else {
			parts = append(parts, fmt.Sprintf("energy waits of %s filled with %s", strings.Join(g.Chars, ", "), g.How))
		}
	}
	if len(o.Skill) > 0 {
		parts = append(parts, fmt.Sprintf("skill cooldowns of %s filled with attacks", strings.Join(o.Skill, ", ")))
	}
	return strings.Join(parts, "; ")
}

// Group lists the characters whose energy waits are handled one way.
type Group struct {
	How   config.Filler
	Chars []string
}

// ByFiller groups o.Energy by filler, in filler order, characters sorted.
func (o *Outcome) ByFiller() []Group {
	var groups []Group
	for _, f := range energyFillers {
		var chars []string
		for c, how := range o.Energy {
			if how == f {
				chars = append(chars, c)
			}
		}
		if len(chars) > 0 {
			slices.Sort(chars)
			groups = append(groups, Group{How: f, Chars: chars})
		}
	}
	return groups
}

// Fatal marks an error from run that must stop Score, such as a full
// queue, rather than rule out one filler.
func Fatal(err error) error { return fatalError{err} }

type fatalError struct{ err error }

func (e fatalError) Error() string { return e.err.Error() }
func (e fatalError) Unwrap() error { return e.err }

func isFatal(err error) bool {
	var f fatalError
	return errors.As(err, &f) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
