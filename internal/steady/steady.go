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
	"strings"

	"github.com/cenoda/gscouter/internal/config"
	"github.com/cenoda/gscouter/internal/library"
	"github.com/cenoda/gscouter/internal/score"
)

// minWait is how long, in seconds per minute, a character must stand
// still before it gets a filler.
const minWait = 1.0

type Outcome struct {
	Result *score.Result
	// Filled is false when the rotation as written scored best.
	// Otherwise Energy and Skill list the characters whose energy waits
	// and skill cooldowns were filled, energy waits as How says.
	Filled        bool
	How           config.Filler
	Energy, Skill []string
}

// Score loops rotation and scores it with run, which assembles and
// simulates a looped rotation. members are the characters as the
// rotation names them.
func Score(rotation string, members []string, run func(looped string) (*score.Result, error)) (*Outcome, error) {
	looped, err := config.Loop(rotation)
	if err != nil {
		return nil, err
	}
	res, err := run(looped)
	if err != nil {
		return nil, err
	}
	best := &Outcome{Result: res}

	var energy, skill []string
	for _, c := range res.Characters {
		for _, m := range members {
			if library.Canonical(m) != library.Canonical(c.Name) {
				continue
			}
			if c.Waits["insufficient_energy"] >= minWait {
				energy = append(energy, m)
			}
			if c.Waits["skill_cd"] >= minWait {
				skill = append(skill, m)
			}
		}
	}
	if len(energy) == 0 && len(skill) == 0 {
		return best, nil
	}
	fillers := []config.Filler{config.FillSelf, config.FillParty}
	if len(energy) == 0 {
		fillers = fillers[:1] // skill waits fill the same way either way
	}
	for _, f := range fillers {
		looped, err := config.Loop(config.Fill(rotation, energy, skill, members, f))
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
			best = &Outcome{Result: res, Filled: true, How: f, Energy: energy, Skill: skill}
		}
	}
	return best, nil
}

// String describes the fillers in English, "" when there are none.
func (o *Outcome) String() string {
	if !o.Filled {
		return ""
	}
	var parts []string
	if len(o.Energy) > 0 {
		parts = append(parts, fmt.Sprintf("energy waits of %s filled with %s", strings.Join(o.Energy, ", "), o.How))
	}
	if len(o.Skill) > 0 {
		parts = append(parts, fmt.Sprintf("skill cooldowns of %s filled with attacks", strings.Join(o.Skill, ", ")))
	}
	return strings.Join(parts, "; ")
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
