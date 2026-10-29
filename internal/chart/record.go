package chart

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/cenoda/gscouter/internal/config"
	"github.com/cenoda/gscouter/internal/library"
	"github.com/cenoda/gscouter/internal/score"
)

// recordFight is long enough for two passes of any library rotation.
const recordFight = 400 // seconds

// The stretch of a rotation that never ends its pass that Record takes,
// in seconds.
const windowFrom, windowTo = 30, 60

// Record writes a rotation as a chart, to start the editor from a
// library team's rotation (docs/cycle-swap.md §3). The chart is what the
// rotation's second pass asks for when looped, played with build: every
// if and while already decided, as the build's energy and cooldowns
// decided them, and waits kept. Runs of repeated notes become repeats.
// names are the members' gcsim keys by slot, as in build.
func Record(build, rotation string, names [Slots]string) (*Chart, error) {
	setup, body, err := config.Split(rotation)
	if err != nil {
		return nil, err
	}
	// Each pass runs in a block of its own, as in config.Loop, so that
	// its let statements can run twice.
	mark := fmt.Sprintf("delay(%d);\n", score.Marker)
	pass := "for let gscouter_pass = 0; gscouter_pass < 1; gscouter_pass = gscouter_pass + 1 {\n" + strings.Join(body, "\n") + "\n}\n"
	src := strings.Join(setup, "\n") + "\n" + mark + pass + mark + pass + mark
	reqs, err := score.Requests(config.Assemble(config.Conditions{Iterations: 1, Duration: recordFight}, build, src), 1)
	if err != nil {
		return nil, err
	}
	var marks []int
	for i, r := range reqs {
		if r.Action == "mark" {
			marks = append(marks, i)
		}
	}
	var played []score.Request
	switch {
	case len(marks) >= 3:
		played = reqs[marks[1]+1 : marks[2]]
	case len(marks) == 2:
		// The second pass did not finish within the fight: the first,
		// played from full energy, is the one there is.
		played = reqs[marks[0]+1 : marks[1]]
	case len(marks) == 1:
		// A rotation that never ends its pass (an action priority list,
		// for { switch ... }): a stretch of it after its opener.
		for _, r := range reqs[marks[0]+1:] {
			if r.Frame >= windowFrom*60 && r.Frame < windowTo*60 {
				played = append(played, r)
			}
		}
	}
	if len(played) == 0 {
		return nil, errors.New("the rotation asked for no actions")
	}
	slot := map[string]int{}
	for i, n := range names {
		slot[library.Canonical(n)] = i
	}
	var notes []Note
	wait := 0
	for _, r := range played {
		switch r.Action {
		case "wait", "delay":
			wait += r.Params["f"]
			continue
		}
		s, ok := slot[library.Canonical(r.Char)]
		if _, known := codeOf[r.Action]; !ok || !known {
			continue // swaps are implicit; a character outside the team cannot act
		}
		notes = append(notes, Note{Slot: s, Action: r.Action, Params: params(r.Params), Wait: min(wait, MaxWait)})
		wait = 0
	}
	c := compress(notes)
	// A rotation that picks up crystallize shards gets its chart picking
	// them up: the script's calls are not actions, so they are not
	// recorded (docs/cycle-swap.md §11.7).
	c.Pickup = strings.Contains(rotation, "pick_up_crystallize")
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return c, nil
}

// params are an action's params in a fixed order, those a chart can
// carry.
func params(m map[string]int) []Param {
	var ps []Param
	for k, v := range m {
		if keyRe.MatchString(k) {
			ps = append(ps, Param{k, v})
		}
	}
	slices.SortFunc(ps, func(a, b Param) int { return strings.Compare(a.Key, b.Key) })
	return ps[:min(len(ps), MaxParams)]
}

// maxBlock is the longest block of notes compress looks for.
const maxBlock = 16

// compress turns runs of a repeated block of notes into repeats, taking
// at each note the block that saves the most notes.
func compress(notes []Note) *Chart {
	c := &Chart{}
	for i := 0; i < len(notes); {
		bestL, bestK := 0, 0
		for l := 1; l <= maxBlock && i+2*l <= len(notes); l++ {
			k := 1
			for i+(k+1)*l <= len(notes) && k < MaxTimes && sameBlock(notes[i:i+l], notes[i+k*l:i+(k+1)*l]) {
				k++
			}
			if k >= 2 && l*(k-1) > bestL*(bestK-1) {
				bestL, bestK = l, k
			}
		}
		if bestK < 2 {
			c.Notes = append(c.Notes, notes[i])
			i++
			continue
		}
		from := len(c.Notes)
		c.Notes = append(c.Notes, notes[i:i+bestL]...)
		c.Repeats = append(c.Repeats, Repeat{From: from, To: len(c.Notes) - 1, Times: bestK})
		i += bestL * bestK
	}
	return c
}

func sameBlock(a, b []Note) bool {
	return slices.EqualFunc(a, b, func(x, y Note) bool {
		return x.Slot == y.Slot && x.Action == y.Action && x.Ready == y.Ready && x.Wait == y.Wait && slices.Equal(x.Params, y.Params)
	})
}
