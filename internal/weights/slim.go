package weights

import (
	"maps"
	"slices"

	"github.com/cenoda/gscouter/internal/artifact"
)

// Slim derives the small table the result page reads
// (artifact.Table, docs/artifacts.md §5) from the full one: per team
// with weights and per character, the substat weights above 0 (3
// decimals), and per slot the right main stats, each wrong one's
// penalty (1 decimal), whether the slot has little effect and then its
// best main stat, and the unplayable options without gcsim's errors.
// Gains, DPS, iterations and the skipped teams stay in the full table.
// Penalties are converted again from the DPS with TeamUnit, so a full
// table written before penalties were counted in the team's rolls
// gives the same small table as a new one.
func Slim(t *Table) *artifact.Table {
	out := &artifact.Table{Format: artifact.TableFormat, Gcsim: commonGcsim(t), Teams: map[string]*artifact.Team{}}
	for id, e := range t.Teams {
		team := &artifact.Team{Characters: make([]artifact.Character, 0, len(e.Characters))}
		if e.Gcsim != out.Gcsim {
			team.Gcsim = e.Gcsim
		}
		unit := TeamUnit(e.Characters)
		for _, c := range e.Characters {
			ac := artifact.Character{Key: c.Key, Unplayable: inOrder(substatKeys(), c.Unplayable)}
			for k, w := range c.Weights {
				if w = round(w, 3); w > 0 {
					if ac.Weights == nil {
						ac.Weights = map[string]float64{}
					}
					ac.Weights[k] = w
				}
			}
			slots := [3]**artifact.Slot{&ac.Sands, &ac.Goblet, &ac.Circlet}
			for i, name := range Slots {
				if s := c.Main[name]; s != nil {
					_, penalty := Judge(s.DPS, Options[i], unit)
					*slots[i] = slimSlot(s, penalty, Options[i])
				}
			}
			team.Characters = append(team.Characters, ac)
		}
		out.Teams[id] = team
	}
	return out
}

func slimSlot(s *Slot, penalty map[string]float64, options []string) *artifact.Slot {
	out := &artifact.Slot{LowImpact: s.LowImpact, Unplayable: inOrder(options, s.Unplayable)}
	if s.LowImpact {
		// Every playable option is right: the first with the highest
		// DPS is what the page shows.
		for _, k := range options {
			if d, ok := s.DPS[k]; ok && (out.Best == "" || d > s.DPS[out.Best]) {
				out.Best = k
			}
		}
		return out
	}
	out.Right = slices.Clone(s.Right)
	// The penalty most wrong main stats share, if at least two do, the
	// smallest on a tie, goes to Others.
	count := map[float64]int{}
	for _, p := range penalty {
		count[round(p, 1)]++
	}
	n := 1
	for _, p := range slices.Sorted(maps.Keys(count)) {
		if count[p] > n {
			out.Others, n = p, count[p]
		}
	}
	for k, p := range penalty {
		if p = round(p, 1); n < 2 || p != out.Others {
			if out.Penalty == nil {
				out.Penalty = map[string]float64{}
			}
			out.Penalty[k] = p
		}
	}
	return out
}

// inOrder returns the keys of m in the order of options, nil for none.
func inOrder(options []string, m map[string]string) []string {
	var out []string
	for _, k := range options {
		if _, ok := m[k]; ok {
			out = append(out, k)
		}
	}
	return out
}

func substatKeys() []string {
	keys := make([]string, len(Substats))
	for i, r := range Substats {
		keys[i] = r.Key
	}
	return keys
}

// commonGcsim returns the gcsim module most teams were calculated with,
// the smallest on a tie.
func commonGcsim(t *Table) string {
	n := map[string]int{}
	for _, e := range t.Teams {
		n[e.Gcsim]++
	}
	best := ""
	for _, g := range slices.Sorted(maps.Keys(n)) {
		if best == "" || n[g] > n[best] {
			best = g
		}
	}
	return best
}
