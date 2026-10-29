// Package steady scores a rotation looped endlessly. gcsim makes a
// character stand still until its burst has the energy or its skill is
// off cooldown; a player would rather fill that time. When the looped
// rotation stands still, Score runs it again with fillers (see
// config.Fill) and keeps the best result, so waiting stays the score
// only where waiting is the best play.
package steady

import (
	"cmp"
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

// maxRuns bounds how many rotations Score simulates in its search (brief's
// runs come on top). At 16 the search stopped before it reached the best
// filler for 12 of 116 searching teams, up to 12% below it; at 24, 6.
const maxRuns = 24

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
	// runnersUp are the other runs of the search that ran (brief's
	// aside), best first.
	// Best confirms the next of them when a full run of this one fails.
	runnersUp []*Outcome
}

// energyFillers are the ways Score tries for each character that waits
// for burst energy: fill the wait, or pass the burst.
var energyFillers = []config.Filler{config.FillSelf, config.FillParty, config.FillSkip}

// allFillers adds FillBrief, which Score only tries on the best run (see
// brief).
var allFillers = append(slices.Clone(energyFillers), config.FillBrief)

// maxBriefRuns bounds the runs brief adds.
const maxBriefRuns = 4

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
	ran := []*Outcome{best}

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
		ran = append(ran, &Outcome{Result: res, Filled: true, Energy: c.energy, Skill: c.skill, Looped: looped})
	}
	best, err = brief(best, rotation, members, run, seen)
	if err != nil {
		return nil, err
	}
	slices.SortStableFunc(ran, func(a, b *Outcome) int { return cmp.Compare(b.Result.DPS, a.Result.DPS) })
	best.runnersUp = slices.DeleteFunc(ran, func(o *Outcome) bool { return o.Result == best.Result })
	return best, nil
}

// Rotation is one of a team's rotations (library.Team.Rotations) as run
// for a build: the script and its members as it names them.
type Rotation struct {
	Rotation string
	Members  []string
}

// Best scores each rotation with Score and returns the index of the one
// that scores best, with its outcome. The best rotation differs with the
// build: one written for higher constellations can score far more with
// them and far less without (docs/cycle.md §3).
//
// run assembles and simulates rotation i's looped script. Only the first
// run of the first rotation, the team's own as written, and the runs
// that confirm a winner are full runs; the others only rank (rank is
// true) and may be cheaper. Ranked outcomes are confirmed with a full
// run from the best down until one beats the full first run, which
// stands otherwise, as fillers ranked on fewer iterations can come out
// below it. A full run that gcsim rejects is replaced by the next-best
// ranked run of the same rotation. A rotation that fails to run is left
// out: an alternative gcsim rejects, or the team's own when a
// constellation breaks it (a C6 Chasca's aim), as long as another
// rotation runs.
func Best(rotations []Rotation, run func(i int, looped string, rank bool) (*score.Result, error)) (int, *Outcome, error) {
	type scored struct {
		i    int
		out  *Outcome
		full bool
	}
	var all []scored
	var first *Outcome
	var firstErr error
	for i, r := range rotations {
		out, err := Score(r.Rotation, r.Members, func(looped string) (*score.Result, error) {
			full := i == 0 && first == nil
			res, err := run(i, looped, !full)
			if err == nil && full {
				first = &Outcome{Result: res, Looped: looped}
			}
			return res, err
		})
		if err != nil {
			if isFatal(err) {
				return 0, nil, err
			}
			if i == 0 {
				firstErr = err
			}
			continue
		}
		all = append(all, scored{i, out, first != nil && out.Result == first.Result})
	}
	if len(all) == 0 {
		return 0, nil, firstErr
	}
	if first != nil && !all[0].full {
		all = append(all, scored{0, first, true})
	}
	slices.SortStableFunc(all, func(a, b scored) int { return cmp.Compare(b.out.Result.DPS, a.out.Result.DPS) })
	for k := 0; k < len(all); k++ {
		s := all[k]
		if s.full {
			return s.i, s.out, nil
		}
		res, err := run(s.i, s.out.Looped, false)
		if err != nil {
			if isFatal(err) {
				return 0, nil, err
			}
			// A filled run can pass on the ranking iterations and fail
			// on one of the full run's seeds (a C6 Skirk's skill in
			// Seven-Phase Flash, docs/characters/skirk.md §4). The
			// rotation's next run then takes its place, ranked among
			// those still to confirm.
			if next := nextRunnerUp(s.out, first); next != nil {
				at := k + 1
				for at < len(all) && all[at].out.Result.DPS >= next.Result.DPS {
					at++
				}
				all = slices.Insert(all, at, scored{s.i, next, false})
			}
			continue
		}
		if first == nil || res.DPS > first.Result.DPS {
			out := *s.out
			out.Result = res
			return s.i, &out, nil
		}
	}
	if first == nil {
		return 0, nil, firstErr
	}
	return 0, first, nil
}

// nextRunnerUp returns the best of out's runners-up, carrying the rest,
// or nil when none is left. The team's own rotation as written (first)
// is already a candidate of Best, so it is passed over.
func nextRunnerUp(out, first *Outcome) *Outcome {
	for i, r := range out.runnersUp {
		if first != nil && r.Result == first.Result {
			continue
		}
		next := *r
		next.runnersUp = out.runnersUp[i+1:]
		return &next
	}
	return nil
}

// brief tries the best run again with FillBrief for the characters whose
// energy it handles: all of them, then each alone. A support that fills
// its wait with attacks stands on field until its burst charges, where a
// player uses its skill, swaps out and bursts next time (Ineffa, two
// skills to a burst). FillBrief is tried only here, after the search,
// so that it cannot crowd the search's own runs out of maxRuns: tried
// in the search, it lowered 58 library teams, one by 19%.
func brief(best *Outcome, rotation string, members []string, run func(string) (*score.Result, error), seen map[string]bool) (*Outcome, error) {
	if !best.Filled || len(best.Energy) == 0 {
		return best, nil
	}
	handled := slices.Sorted(maps.Keys(best.Energy))
	sets := [][]string{handled}
	if len(handled) > 1 {
		for _, c := range handled {
			sets = append(sets, []string{c})
		}
	}
	from := best
	for i, set := range sets {
		if i == maxBriefRuns {
			break
		}
		energy := maps.Clone(from.Energy)
		for _, c := range set {
			energy[c] = config.FillBrief
		}
		filled := config.Fill(rotation, energy, from.Skill, members)
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
			continue
		}
		if res.DPS > best.Result.DPS {
			best = &Outcome{Result: res, Filled: true, Energy: energy, Skill: from.Skill, Looped: looped}
		}
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
		switch g.How {
		case config.FillSkip:
			parts = append(parts, fmt.Sprintf("bursts of %s skipped when short of energy", strings.Join(g.Chars, ", ")))
		case config.FillBrief:
			parts = append(parts, fmt.Sprintf("energy waits of %s filled with skills, bursts skipped when still short", strings.Join(g.Chars, ", ")))
		default:
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
	for _, f := range allFillers {
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
