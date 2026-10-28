package web

import (
	"html/template"
	"maps"
	"math"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/cenoda/gscouter/internal/enka"
)

// substatOrder is the order pieced picks substats in, rotated per piece.
var substatOrder = []string{"cr", "cd", "atk%", "er", "em", "hp%", "atk", "hp", "def", "def%"}

// maxRoll is the largest 5★ substat roll, at gcsim scale.
var maxRoll = map[string]float64{
	"hp": 298.75, "atk": 19.45, "def": 23.15, "hp%": 0.0583, "atk%": 0.0583,
	"def%": 0.0729, "em": 23.31, "er": 0.0648, "cr": 0.0389, "cd": 0.0777,
}

// pieced is c wearing five 5★ +20 pieces: four of its set and a
// Gladiator's Finale, with the given sands, goblet and circlet main
// stats and four substats each, rolled as the game could from seed.
// Its totals are the pieces' sum, as a showcase's are.
func pieced(c enka.Character, sands, goblet, circlet string, seed int) enka.Character {
	var set string
	for s := range c.Sets {
		set = s
	}
	c.Sets = map[string]int{set: 4, "gladiatorsfinale": 1}
	c.UnknownSets = 0
	c.Stats = map[string]float64{}
	c.Pieces, c.PieceDetail = nil, enka.PiecesFull
	extras := [][4]int{{2, 1, 1, 1}, {1, 2, 1, 1}, {3, 1, 1, 0}, {0, 2, 2, 1}, {1, 1, 0, 3}}
	for i, main := range []string{"hp", "atk", sands, goblet, circlet} {
		v, ok := enka.MainStat(5, 20, main)
		if !ok {
			panic("no main stat " + main)
		}
		p := enka.Artifact{Slot: enka.Slot(i), Set: set, Rarity: 5, Level: 20, Main: enka.StatValue{Key: main, Value: v}}
		if i == 4 {
			p.Set = "gladiatorsfinale"
		}
		c.Stats[main] += v
		order := slices.DeleteFunc(slices.Clone(substatOrder), func(k string) bool { return k == main })
		for j := range 4 {
			key := order[(seed+i+j*2)%len(order)]
			for slices.ContainsFunc(p.Subs, func(s enka.StatValue) bool { return s.Key == key }) {
				key = order[(slices.Index(order, key)+1)%len(order)]
			}
			sum := 0.0
			for k := range 1 + extras[(seed+i)%len(extras)][j] {
				sum += maxRoll[key] * (0.7 + 0.1*float64((seed*7+j*3+k)%4))
			}
			scale := 100.0
			if flatStats[key] {
				scale = 1
			}
			sum = math.Round(sum*scale*10) / 10 / scale
			p.Subs = append(p.Subs, enka.StatValue{Key: key, Value: sum})
			c.Stats[key] += sum
		}
		c.Pieces = append(c.Pieces, p)
	}
	return c
}

// piecedTeam is lohenTeam wearing full pieces.
func piecedTeam() []enka.Character {
	chars := lohenTeam()
	mains := [][3]string{{"atk%", "cryo%", "cd"}, {"er", "pyro%", "cd"}, {"atk%", "atk%", "atk%"}, {"er", "cryo%", "em"}}
	for i := range chars {
		chars[i] = pieced(chars[i], mains[i][0], mains[i][1], mains[i][2], i)
	}
	return chars
}

func TestPiecedBuildsUnpack(t *testing.T) {
	builds := piecedTeam()
	sel := selection{UID: "800000005", Chars: keys(builds), Builds: builds, Nickname: "Lohen"}
	got, err := parseSelection(sel.String())
	if err != nil {
		t.Fatal(err)
	}
	for i, c := range got.Builds {
		if c.PieceDetail != enka.PiecesFull || len(c.Pieces) != 5 || c.Config() != builds[i].Config() {
			t.Errorf("%s: %v, %d pieces", c.Key, c.PieceDetail, len(c.Pieces))
		}
	}
}

// TestPackedSizes measures links with full pieces and checks that they
// fit every limit on the way: a link's packed text, a short link's
// target, and the kept builds a pick sends.
func TestPackedSizes(t *testing.T) {
	team := piecedTeam()
	other := piecedTeam()
	for i := range other {
		other[i] = pieced(other[i], team[i].Pieces[2].Main.Key, team[i].Pieces[3].Main.Key, team[i].Pieces[4].Main.Key, i+5)
	}
	a := selection{UID: "800000005", Chars: keys(team), Builds: team, Nickname: "Lohen"}
	b := selection{UID: "800000006", Chars: keys(other), Builds: other, Nickname: "Someone"}
	full := len(url.QueryEscape(a.String()))
	stripped := len(url.QueryEscape(teamResult{Selection: a}.Stripped()))
	none := a
	none.Builds = lohenTeam()
	totals := len(url.QueryEscape(none.String()))
	versus := "/versus?" + url.Values{"a": {a.String()}, "b": {b.String()}}.Encode()
	t.Logf("4 characters: full pieces %d URL characters, stripped %d, totals only %d; versus (8) link %d", full, stripped, totals, len(versus))

	// The largest a 4-character link gets: a 64-byte nickname and
	// every substat at its largest roll count.
	big := piecedTeam()
	for i := range big {
		for j := range big[i].Pieces {
			for k := range big[i].Pieces[j].Subs {
				s := &big[i].Pieces[j].Subs[k]
				big[i].Stats[s.Key] -= s.Value
				s.Value = math.Floor(maxRoll[s.Key]*(5.4+0.06*float64((i*31+j*7+k*3)%10))*1000) / 1000
				big[i].Stats[s.Key] += s.Value
			}
		}
	}
	largest := selection{UID: "800000005", Chars: keys(big), Builds: big, Nickname: strings.Repeat("가", 21)}
	packed := largest.String()[len("800000005~"):]
	t.Logf("largest 4-character packed text: %d characters (limit %d)", len(packed), maxPacked)
	if len(packed) > maxPacked/2 {
		t.Errorf("packed text %d characters, over half of maxPacked", len(packed))
	}
	if _, err := shortTarget("/versus?" + url.Values{"a": {largest.String()}, "b": {largest.String()}}.Encode()); err != nil {
		t.Errorf("short link to the largest versus: %v", err)
	}

	// A pick can send every showcase character as a kept build.
	var tokens []string
	for range 3 {
		for _, c := range team {
			tokens = append(tokens, buildToken(c))
		}
	}
	q := url.Values{"build": tokens}.Encode()
	t.Logf("12 kept builds: %d URL characters", len(q))
	if got := keptBuilds(httptest.NewRequest("GET", "/score?"+q, nil)); len(got) != 12 {
		t.Errorf("%d kept builds read, want 12", len(got))
	}
}

func TestStrippedKeepsScore(t *testing.T) {
	tr := teamResult{Selection: selection{UID: "800000005", Chars: keys(piecedTeam()), Builds: piecedTeam()}}
	got, err := parseSelection(tr.Stripped())
	if err != nil {
		t.Fatal(err)
	}
	for i, c := range got.Builds {
		if c.PieceDetail != enka.PiecesStripped || c.Pieces[0].Subs != nil || c.Config() != tr.Selection.Builds[i].Config() {
			t.Errorf("%s: %v, config %q", c.Key, c.PieceDetail, c.Config())
		}
	}
}

// A result page shows the artifact totals, keeps the pieces folded, and
// offers a link without them; a stripped link shows no substats, and a
// link from before pieces were kept asks for the showcase again.
func TestScorePieces(t *testing.T) {
	ts := newTestServer(t)
	builds := piecedTeam()
	sel := selection{UID: "800000005", Chars: keys(builds), Builds: builds, Nickname: "Lohen"}
	sub := english.subsText(builds[0].Pieces[0])
	_, body := get(t, ts, "/score?a="+url.QueryEscape(sel.String()))
	mustContain(t, body, template.HTMLEscapeString(english.totals(builds[0])), `<details class="rotation">
<summary>Artifact pieces</summary>`, "Circlet", "Finale", sub,
		"Leave artifact pieces out", `data-strip="/score?a=`+url.QueryEscape(teamResult{Selection: sel}.Stripped())+`"`)

	_, body = get(t, ts, "/score?a="+url.QueryEscape(teamResult{Selection: sel}.Stripped()))
	mustContain(t, body, "DPS", template.HTMLEscapeString(english.totals(builds[0])), "Substats left out", english.statText(builds[0].Pieces[4].Main))
	for _, no := range []string{sub, "Leave artifact pieces out"} {
		if strings.Contains(body, no) {
			t.Errorf("stripped link shows %q", no)
		}
	}

	sel.Builds = lohenTeam()
	_, body = get(t, ts, "/score?a="+url.QueryEscape(sel.String()))
	mustContain(t, body, "DPS", "This link was made before artifact pieces were kept")
}

// shareCirclet is builds with the first character's circlet also on the
// second, its totals changed to match.
func shareCirclet(builds []enka.Character) []enka.Character {
	builds = slices.Clone(builds)
	c := builds[1]
	c.Stats = maps.Clone(c.Stats)
	c.Pieces = slices.Clone(c.Pieces)
	old, p := c.Pieces[4], builds[0].Pieces[4]
	for _, s := range append([]enka.StatValue{old.Main}, old.Subs...) {
		c.Stats[s.Key] -= s.Value
	}
	for _, s := range append([]enka.StatValue{p.Main}, p.Subs...) {
		c.Stats[s.Key] += s.Value
	}
	c.Pieces[4] = p
	builds[1] = c
	return builds
}

func TestDuplicates(t *testing.T) {
	builds := piecedTeam()
	if got := duplicates(english, builds); got != nil {
		t.Errorf("distinct pieces: %q", got)
	}
	shared := shareCirclet(builds)
	got := duplicates(english, shared)
	if len(got) != 1 || !strings.Contains(got[0], "Lohen and Durin wear the same artifact (Circlet · Gladiator's Finale · CRIT DMG 62.2%)") {
		t.Errorf("shared circlet: %q", got)
	}
	// Stripped pieces cannot be checked.
	shared[2] = shared[2].Stripped()
	if got := duplicates(english, shared); got != nil {
		t.Errorf("with a stripped build: %q", got)
	}
}

// The shared artifact is named on the page, and the score is marked as
// not comparable.
func TestScoreDuplicate(t *testing.T) {
	ts := newTestServer(t)
	builds := shareCirclet(piecedTeam())
	sel := selection{UID: "800000005", Chars: keys(builds), Builds: builds}
	_, body := get(t, ts, "/score?a="+url.QueryEscape(sel.String()))
	mustContain(t, body, "DPS", `<div class="warn">Lohen and Durin wear the same artifact (Circlet`, "does not compare with standard scores")
}

func TestArtifactScoreText(t *testing.T) {
	s := &artifactScore{Subs: 184.6, Penalty: 69.6}
	if got := s.Points(); got != "185 · −70 · 115" {
		t.Errorf("points %q", got)
	}
	if got := (&artifactScore{Subs: 12.4}).Points(); got != "12 · 0 · 12" {
		t.Errorf("points %q", got)
	}
	for v, want := range map[slotVerdict]string{
		{Slot: "sands", Main: "atk%", Verdict: "right"}:               "Sands ✓ ATK%",
		{Slot: "goblet", Main: "em", Verdict: "wrong", Penalty: 30.1}: "Goblet ✗ Elemental Mastery −30",
		{Slot: "circlet", Main: "cr", Verdict: "low", Best: "cd"}:     "Circlet CRIT Rate little effect (best: CRIT DMG)",
		{Slot: "sands", Main: "er", Verdict: "unplayable"}:            "Sands Energy Recharge rotation cannot be played",
		{Slot: "goblet", Verdict: "unknown"}:                          "Goblet not judged",
		{Slot: "goblet", Main: "pyro%", Verdict: "wrong", Penalty: 8}: "Goblet ✗ Pyro DMG Bonus −8",
	} {
		if got := english.verdict(v); got != want {
			t.Errorf("%+v: %q, want %q", v, got, want)
		}
	}
	ko := localizer{Lang: langKorean}
	if got := ko.verdict(slotVerdict{Slot: "goblet", Main: "hydro%", Verdict: "low", Best: "hp%"}); got != "성배 물 원소 피해 보너스 영향 작음(최고: HP%)" {
		t.Errorf("Korean: %q", got)
	}
}
