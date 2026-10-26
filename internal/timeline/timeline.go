// Package timeline reduces one gcsim run to what a player reads off a
// rotation: who is on field when, which actions they take, where they
// stand still, and the damage dealt over time and by ability.
package timeline

import (
	"cmp"
	"slices"
	"strings"

	"github.com/genshinsim/gcsim/pkg/stats"
)

// BucketSeconds is the width of the damage-over-time buckets.
const BucketSeconds = 1.0

// minWait is the shortest wait, in seconds, shown on the timeline.
// Shorter ones (a swap cooldown between two actions) are noise.
const minWait = 0.25

type Timeline struct {
	Start, End float64 // seconds of the fight covered
	Characters []Character
	Stints     []Stint
	Damage     float64 // team damage in the window
}

type Character struct {
	Name    string
	Damage  float64
	Actions []Action
	Field   []Span
	Waits   []Wait
	Buckets []float64 // damage landed per BucketSeconds from Start
	Sources []Source  // by damage, highest first
}

type Action struct {
	Time float64
	Kind string // gcsim action name: "skill", "burst", "attack", ...
}

type Span struct{ Start, End float64 }

type Wait struct {
	Span
	Reason string // gcsim failure reason, e.g. "insufficient_energy"
}

// Source is the damage of one ability, with its numbered hits ("Normal
// 0", "Normal 1") merged.
type Source struct {
	Name   string
	Hits   int
	Crits  int
	Damage float64
}

// Stint is one stretch on field, from swapping in to swapping out.
type Stint struct {
	Char       int
	Start, End float64
	Actions    []Run
	// Damage is what each character's attacks created during the stint,
	// by attack source time, so a burst cast in it counts its first hits
	// but not ticks that fire after the swap.
	Damage []float64
	Total  float64
}

// Run is consecutive actions of one kind, e.g. three normal attacks.
type Run struct {
	Kind  string
	Count int
}

// hidden actions are not drawn: swaps show as stints, the rest is idle time.
var hidden = map[string]bool{"swap": true, "wait": true, "delay": true}

// Build reduces res to the frames [from, to). to is clamped to the end of
// the fight.
func Build(res *stats.Result, from, to int) *Timeline {
	to = min(to, res.Duration)
	from = min(from, to)
	sec := func(f int) float64 { return float64(f) / 60 }
	t := &Timeline{Start: sec(from), End: sec(to)}
	inside := func(f int) bool { return f >= from && f < to }

	nb := int((t.End-t.Start)/BucketSeconds + 0.999)
	buckets := cumulativeBuckets(res)
	for ci, cr := range res.Characters {
		c := Character{Name: cr.Name, Buckets: make([]float64, nb)}
		for _, a := range cr.ActionEvents {
			if inside(a.Frame) && !hidden[a.Action] {
				c.Actions = append(c.Actions, Action{Time: sec(a.Frame), Kind: a.Action})
			}
		}
		for _, f := range cr.FailedActions {
			s, e := max(f.Start, from), min(f.End, to)
			if f.Reason != "swap_cd" && sec(e-s) >= minWait {
				c.Waits = append(c.Waits, Wait{Span{sec(s), sec(e)}, f.Reason})
			}
		}
		// Landed damage per bucket, from gcsim's 0.5 s cumulative shares.
		for b := from / 30; b*30 < to && b < len(buckets); b++ {
			d := buckets[b][ci]
			if b > 0 {
				d -= buckets[b-1][ci]
			}
			i := int((sec(b*30) - t.Start) / BucketSeconds)
			if i >= 0 && i < nb {
				c.Buckets[i] += d
			}
		}
		sources := map[string]*Source{}
		for _, d := range cr.DamageEvents {
			if !inside(d.Frame) || d.Damage <= 0 {
				continue
			}
			c.Damage += d.Damage
			name := abilityName(d.Source)
			s := sources[name]
			if s == nil {
				s = &Source{Name: name}
				sources[name] = s
			}
			s.Hits++
			s.Damage += d.Damage
			if d.Crit {
				s.Crits++
			}
		}
		for _, s := range sources {
			c.Sources = append(c.Sources, *s)
		}
		slices.SortFunc(c.Sources, func(a, b Source) int {
			return cmp.Or(cmp.Compare(b.Damage, a.Damage), strings.Compare(a.Name, b.Name))
		})
		t.Damage += c.Damage
		t.Characters = append(t.Characters, c)
	}

	for _, iv := range res.ActiveCharacters {
		s, e := max(iv.Start, from), min(iv.End, to)
		if e <= s || iv.Character < 0 || iv.Character >= len(t.Characters) {
			continue
		}
		t.Characters[iv.Character].Field = append(t.Characters[iv.Character].Field, Span{sec(s), sec(e)})
	}
	// gcsim closes an interval only on a swap, so the last one is open.
	start := 0
	if n := len(res.ActiveCharacters); n > 0 {
		start = res.ActiveCharacters[n-1].End
	}
	if s := max(start, from); s < to {
		if last := activeAfter(res, start); last >= 0 {
			t.Characters[last].Field = append(t.Characters[last].Field, Span{sec(s), sec(to)})
		}
	}
	t.Stints = stints(t, res, from, to)
	return t
}

// activeAfter is the character on field from frame end, the first to act
// from then on, or -1. gcsim logs a swap on the character leaving.
func activeAfter(res *stats.Result, end int) int {
	best, bestFrame := -1, 0
	for ci, c := range res.Characters {
		for _, a := range c.ActionEvents {
			if a.Frame >= end && !hidden[a.Action] && (best < 0 || a.Frame < bestFrame) {
				best, bestFrame = ci, a.Frame
			}
		}
	}
	return best
}

func stints(t *Timeline, res *stats.Result, from, to int) []Stint {
	var out []Stint
	for ci, c := range t.Characters {
		for _, f := range c.Field {
			out = append(out, Stint{Char: ci, Start: f.Start, End: f.End, Damage: make([]float64, len(t.Characters))})
		}
	}
	slices.SortFunc(out, func(a, b Stint) int { return cmp.Compare(a.Start, b.Start) })
	find := func(s float64) int {
		i, _ := slices.BinarySearchFunc(out, s, func(st Stint, s float64) int {
			if st.End <= s {
				return -1
			}
			if st.Start > s {
				return 1
			}
			return 0
		})
		if i < len(out) && out[i].Start <= s && s < out[i].End {
			return i
		}
		return -1
	}
	for _, c := range t.Characters {
		for _, a := range c.Actions {
			if i := find(a.Time); i >= 0 {
				st := &out[i]
				if n := len(st.Actions); n > 0 && st.Actions[n-1].Kind == a.Kind {
					st.Actions[n-1].Count++
				} else {
					st.Actions = append(st.Actions, Run{Kind: a.Kind, Count: 1})
				}
			}
		}
	}
	for ci, cr := range res.Characters {
		for _, d := range cr.DamageEvents {
			if d.Frame < from || d.Frame >= to || d.Damage <= 0 {
				continue
			}
			if i := find(float64(d.Frame) / 60); i >= 0 {
				out[i].Damage[ci] += d.Damage
				out[i].Total += d.Damage
			}
		}
	}
	// A stint without actions or damage is a swap straight through, or
	// the tail of one cut by the window.
	return slices.DeleteFunc(out, func(s Stint) bool { return len(s.Actions) == 0 && s.Total == 0 })
}

// cumulativeBuckets returns each character's cumulative damage per 0.5 s
// bucket.
func cumulativeBuckets(res *stats.Result) [][]float64 {
	out := make([][]float64, len(res.DamageBuckets))
	var sum float64
	for i, d := range res.DamageBuckets {
		sum += d
		out[i] = make([]float64, len(res.Characters))
		for c, cr := range res.Characters {
			if i < len(cr.DamageCumulativeContrib) {
				out[i][c] = cr.DamageCumulativeContrib[i] * sum
			}
		}
	}
	return out
}

// abilityName drops a hit number: "Normal 2" and "Normal (Skill) 4"
// become "Normal" and "Normal (Skill)".
func abilityName(s string) string {
	if i := strings.LastIndexByte(s, ' '); i > 0 {
		if n := s[i+1:]; n != "" && strings.Trim(n, "0123456789") == "" {
			return s[:i]
		}
	}
	return s
}
