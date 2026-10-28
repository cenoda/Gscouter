//go:build !(js && wasm)

package web

import (
	"github.com/cenoda/gscouter/internal/artifact"
	"github.com/cenoda/gscouter/internal/enka"
	"github.com/cenoda/gscouter/internal/library"
)

// scoreArtifacts fills in a prepared library team's artifact scores from
// the weights table. The browser build has no table (it is 5 MB): the
// scores travel to it with the Job.
func scoreArtifacts(tr *teamResult) {
	if tr.Team == nil {
		return
	}
	weights, ok := artifact.Lookup(tr.Team.ID)
	if !ok {
		return
	}
	tr.ArtifactScores = make([]*artifactScore, len(tr.Chars))
	for i, c := range tr.Chars {
		for j := range weights.Characters {
			if w := &weights.Characters[j]; library.Canonical(w.Key) == library.Canonical(c.Key) {
				tr.ArtifactScores[i] = scoreCharacter(w, c)
			}
		}
	}
}

// scoreCharacter is c's artifact score with its weights w: the substats
// are the pieces' (full pieces) or the totals less the main stats
// (stripped pieces); a build without pieces has none.
func scoreCharacter(w *artifact.Character, c enka.Character) *artifactScore {
	if c.PieceDetail == enka.PiecesNone {
		return &artifactScore{Reload: true}
	}
	subs, mains := map[string]float64{}, map[string]string{}
	for _, p := range c.Pieces {
		mains[p.Slot.String()] = p.Main.Key
		for _, s := range p.Subs {
			subs[s.Key] += s.Value
		}
	}
	if c.PieceDetail == enka.PiecesStripped {
		for key := range artifact.MaxRolls {
			subs[key] = c.Stats[key]
		}
		for _, p := range c.Pieces {
			subs[p.Main.Key] -= p.Main.Value
		}
		for key, v := range subs {
			subs[key] = max(v, 0)
		}
	}
	js := w.Judge(mains)
	s := &artifactScore{Subs: w.Score(subs), Penalty: artifact.Penalty(js)}
	for _, j := range js {
		s.Slots = append(s.Slots, slotVerdict{
			Slot: j.Slot, Main: j.Main, Penalty: j.Penalty, Best: j.Best,
			Verdict: [...]string{
				artifact.Unknown: "unknown", artifact.Right: "right", artifact.Wrong: "wrong",
				artifact.LowImpact: "low", artifact.Unplayable: "unplayable",
			}[j.Verdict],
		})
	}
	return s
}
