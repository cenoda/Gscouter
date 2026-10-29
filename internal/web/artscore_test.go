//go:build !(js && wasm)

package web

import (
	"context"
	"encoding/json"
	"math"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/cenoda/gscouter/internal/artifact"
)

// The artifact score needs the weights table, which only the server has.

func TestScoreArtifacts(t *testing.T) {
	lib := newTestSite(t).lib
	builds := piecedTeam()
	tr := teamResult{Chars: builds, Team: lib.Find(keys(builds))}
	scoreArtifacts(&tr)
	if len(tr.ArtifactScores) != 4 {
		t.Fatalf("%d scores", len(tr.ArtifactScores))
	}
	weights, _ := artifact.Lookup(tr.Team.ID)
	for i, s := range tr.ArtifactScores {
		subs := map[string]float64{}
		for _, p := range builds[i].Pieces {
			for _, sub := range p.Subs {
				subs[sub.Key] += sub.Value
			}
		}
		if want := weights.Characters[i].Score(subs); s == nil || s.Reload || math.Abs(s.Subs-want) > 1e-9 || s.Subs <= 0 {
			t.Errorf("%s: %+v, want substat score %v", builds[i].Key, s, want)
		}
	}
	if v := tr.ArtifactScores[3].Slots; v[0].Verdict != "right" || v[1].Verdict != "low" || v[1].Best != "cryo%" {
		t.Errorf("Citlali: %+v", v)
	}

	// Stripped pieces score the same from the totals.
	stripped := teamResult{Chars: slices.Clone(builds), Team: tr.Team}
	for i := range stripped.Chars {
		stripped.Chars[i] = stripped.Chars[i].Stripped()
	}
	scoreArtifacts(&stripped)
	for i, s := range stripped.ArtifactScores {
		if math.Abs(s.Subs-tr.ArtifactScores[i].Subs) > 1e-6 || s.Points() != tr.ArtifactScores[i].Points() {
			t.Errorf("%s stripped: %v, full %v", builds[i].Key, s.Subs, tr.ArtifactScores[i].Subs)
		}
	}

	// A wrong main stat costs its penalty.
	builds[0].Pieces[3].Main.Key = "em"
	if s := scoreCharacter(&weights.Characters[0], builds[0]); s.Slots[1].Verdict != "wrong" || s.Penalty != 49 {
		t.Errorf("EM goblet: %+v", s)
	}
	// Old links have no pieces; teams without weights no scores.
	if s := scoreCharacter(&weights.Characters[0], lohenTeam()[0]); !s.Reload {
		t.Errorf("no pieces: %+v", s)
	}
	none := teamResult{Chars: hutaoTeam(), Team: lib.Find(keys(hutaoTeam()))}
	scoreArtifacts(&none)
	if none.ArtifactScores != nil {
		t.Errorf("team without weights: %+v", none.ArtifactScores)
	}
}

// The artifact score is on the page, folded, for full and stripped
// pieces; an old link asks for the showcase again.
func TestScoreArtifactScore(t *testing.T) {
	ts := newTestServer(t)
	builds := piecedTeam()
	sel := selection{UID: "800000005", Chars: keys(builds), Builds: builds}
	tr := teamResult{Chars: builds, Team: newTestSite(t).lib.Find(keys(builds))}
	scoreArtifacts(&tr)
	for _, a := range []string{sel.String(), teamResult{Selection: sel}.Stripped()} {
		_, body := get(t, ts, "/score?a="+url.QueryEscape(a))
		mustContain(t, body, "<summary>Artifact score</summary>", tr.ArtifactScores[0].Points(), tr.ArtifactScores[3].Points(),
			"Sands ✓ ATK%", "Goblet ✓ Cryo DMG Bonus (best; every main stat here is within 2% DPS)")
	}
	sel.Builds = lohenTeam()
	_, body := get(t, ts, "/score?a="+url.QueryEscape(sel.String()))
	i := strings.Index(body, "<summary>Artifact score</summary>")
	if i < 0 || !strings.Contains(body[i:], "This link was made before") || strings.Contains(body, " · 0 · ") {
		t.Error("old link: no reload note in the artifact score")
	}
	// Teams the weights lack show none.
	_, body = get(t, ts, "/score?a="+url.QueryEscape("800000001:hutao,yelan,xingqiu,zhongli"))
	if !strings.Contains(body, "DPS") || strings.Contains(body, "Artifact score") {
		t.Error("a team without weights shows an artifact score")
	}
}

// The browser build has no weights table: the artifact scores and the
// duplicate check reach its page through the Job, as the server
// rendered them.
func TestRunJobArtifacts(t *testing.T) {
	srv, _ := newClientSite(t)
	ctx := context.Background()
	builds := shareCirclet(piecedTeam())
	sel := selection{UID: "800000005", Chars: keys(builds), Builds: builds}
	tr := srv.prepare(ctx, english, sel, nil)
	tr.ID = "a"
	raw, err := json.Marshal(srv.job(english, "score", tr))
	if err != nil {
		t.Fatal(err)
	}
	var job Job
	if err := json.Unmarshal(raw, &job); err != nil {
		t.Fatal(err)
	}
	body, _, err := RunJob(ctx, job, serverSim{srv})
	if err != nil {
		t.Fatal(err)
	}
	want := srv.evaluate(ctx, english, sel, nil)
	want.ID = "a"
	var b strings.Builder
	if err := srv.tmpl[english].ExecuteTemplate(&b, "score-body", scoreData{Team: want}); err != nil {
		t.Fatal(err)
	}
	if body != b.String() {
		t.Error("RunJob's body differs from the server's")
	}
	mustContain(t, body, "<summary>Artifact score</summary>", want.ArtifactScores[0].Points(), "wear the same artifact")
}
