package web

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"maps"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cenoda/gscouter/internal/config"
	"github.com/cenoda/gscouter/internal/enka"
	"github.com/cenoda/gscouter/internal/library"
)

var funcs = template.FuncMap{
	"thousands":  thousands,
	"lastUpdate": lastUpdate,
	"float":      func(v int) float64 { return float64(v) },
	"inc":        func(v int) int { return v + 1 },
	"percent":    func(v float64) string { return fmt.Sprintf("%.1f%%", v*100) },
	"oneDecimal": func(v float64) string {
		return fmt.Sprintf("%.1f", v)
	},
	// width is a fraction as a CSS percentage, for bars.
	"width": func(v float64) string { return fmt.Sprintf("%.1f", math.Max(0, math.Min(v, 1))*100) },
	// ratio is a over the larger of a and b.
	"ratio": func(a, b float64) float64 {
		if m := math.Max(a, b); m > 0 {
			return a / m
		}
		return 0
	},
	"neg": func(v float64) float64 { return -v },
	// px is a+b as an SVG coordinate.
	"px":         func(a, b float64) string { return strconv.FormatFloat(a+b, 'f', 1, 64) },
	"element":    element,
	"face":       face,
	"link":       link,
	"elements":   func() []string { return config.Elements[:] },
	"enemyQuery": enemyQuery,
	// enemyValues are an enemy's URL parameters, for hidden fields.
	"enemyValues": func(e *config.Enemy) url.Values {
		v := url.Values{}
		addEnemy(v, e)
		return v
	},
	"keys":  keys,
	"build": buildToken,
	"json": func(v any) (string, error) {
		b, err := json.Marshal(v)
		return string(b), err
	},
}

// keys lists the characters' keys.
func keys(chars []enka.Character) []string {
	out := make([]string, len(chars))
	for i, c := range chars {
		out[i] = c.Key
	}
	return out
}

// thousands formats a number rounded to an integer with thousands
// separators, e.g. "85,512".
func thousands(v float64) string {
	s := fmt.Sprintf("%.0f", v)
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	return b.String()
}

var elementCache sync.Map // canonical name -> element

// element returns a character's element ("pyro", ...), or "".
func element(name string) string {
	key := library.Canonical(name)
	if v, ok := elementCache.Load(key); ok {
		return v.(string)
	}
	e := ""
	if elem, ok := strings.CutPrefix(key, "traveler"); ok {
		e = elem
	} else {
		e = enka.Element(key)
	}
	elementCache.Store(key, e)
	return e
}

var enkaKeys sync.Map // name -> Enka data key

// enkaKey returns the gcsim key Enka's data knows a character by: a
// showcase or result name as is, a library member name by its canonical
// key. The Traveler of a library team ("travelercryo") is Aether.
func enkaKey(name string) string {
	if v, ok := enkaKeys.Load(name); ok {
		return v.(string)
	}
	key := strings.ToLower(name)
	if enka.IconURL(key) == "" {
		key = library.Canonical(name)
		if elem, ok := strings.CutPrefix(key, "traveler"); ok {
			key = "aether" + elem
		}
	}
	enkaKeys.Store(name, key)
	return key
}

// face returns the URL of a character's face icon, or "".
func face(name string) string { return enka.IconURL(enkaKey(name)) }

// charName returns a character's in-game name in l's language, or name
// itself when the game data doesn't know it. The game names the Traveler
// the same in every element, so the element follows: "Traveler (Cryo)".
func (l localizer) charName(name string) string {
	key := enkaKey(name)
	n := cmp.Or(enka.CharacterName(key, l.Lang), name)
	for _, prefix := range []string{"aether", "lumine"} {
		if elem, ok := strings.CutPrefix(key, prefix); ok {
			if e := l.elementName(elem); e != "" {
				return n + " (" + e + ")"
			}
		}
	}
	return n
}

// elementNames are the game's names for the elements, by language.
var elementNames = map[string][7]string{
	// pyro, hydro, anemo, electro, dendro, cryo, geo
	"en":    {"Pyro", "Hydro", "Anemo", "Electro", "Dendro", "Cryo", "Geo"},
	"ko":    {"불", "물", "바람", "번개", "풀", "얼음", "바위"},
	"ja":    {"炎", "水", "風", "雷", "草", "氷", "岩"},
	"zh-cn": {"火", "水", "风", "雷", "草", "冰", "岩"},
	"zh-tw": {"火", "水", "風", "雷", "草", "冰", "岩"},
	"de":    {"Pyro", "Hydro", "Anemo", "Elektro", "Dendro", "Kryo", "Geo"},
	"fr":    {"Pyro", "Hydro", "Anémo", "Électro", "Dendro", "Cryo", "Géo"},
	"ru":    {"Пиро", "Гидро", "Анемо", "Электро", "Дендро", "Крио", "Гео"},
	"th":    {"ไฟ", "น้ำ", "ลม", "ไฟฟ้า", "ไม้", "น้ำแข็ง", "หิน"},
	"tr":    {"Ateş", "Su", "Rüzgar", "Elektrik", "Doğa", "Buz", "Toprak"},
	"vi":    {"Hỏa", "Thủy", "Phong", "Lôi", "Thảo", "Băng", "Nham"},
}

// elementName is the game's name for an element ("pyro", ...) in l's
// language, else in English, or "" for an unknown element.
func (l localizer) elementName(elem string) string {
	i := slices.Index([]string{"pyro", "hydro", "anemo", "electro", "dendro", "cryo", "geo"}, elem)
	if i < 0 {
		return ""
	}
	names, ok := elementNames[l.Lang]
	if !ok {
		names = elementNames[langEnglish]
	}
	return names[i]
}

// charNames is charName of each name, joined by ", ".
func (l localizer) charNames(names []string) string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = l.charName(n)
	}
	return strings.Join(out, ", ")
}

// weaponName is charName for a weapon's gcsim key.
func (l localizer) weaponName(key string) string {
	return cmp.Or(enka.WeaponName(key, l.Lang), key)
}

// teamName names a library team by its members' in-game names.
func (l localizer) teamName(t *library.Team) string {
	out := make([]string, len(t.Members))
	for i, m := range t.Members {
		out[i] = l.charName(m)
	}
	return strings.Join(out, " / ")
}

// searchNames lists what finds a team's members: gcsim names, canonical
// keys, and in-game names in l's language and in English.
func (l localizer) searchNames(t *library.Team) []string {
	var out []string
	for _, m := range t.Members {
		for _, n := range []string{m, library.Canonical(m), l.charName(m), english.charName(m)} {
			if n = strings.ToLower(n); !slices.Contains(out, n) {
				out = append(out, n)
			}
		}
	}
	return out
}

type suggestion struct {
	Team      *library.Team
	Selection selection
	Href      template.URL // the team's result page
}

type indexData struct {
	UID       string
	VS        string // packed first selection when picking the opponent
	VR        string // the first selection's chart, with VS; "" for the library's rotations
	Profile   *enka.Profile
	Problem   string
	Suggested []suggestion
	// Buildable lists the suggestions' characters, sorted and joined by
	// commas, so the picker can tell when four picks are not a team.
	Buildable []string
	// Unlocks are the characters that would complete the most library
	// teams with the showcase's, best first; not when picking an
	// opponent, whose showcase is someone else's.
	Unlocks []unlock
	// Refresh is the seconds until loading the UID again gets its
	// showcase anew from Enka, or -1 when unknown.
	Refresh int
	// Kept dates the characters the browser kept from earlier lookups
	// that are no longer in the showcase, by key.
	Kept map[string]string
	// Saved names the builds shown of characters added from the builds
	// the player saved on Enka.Network, by key.
	Saved map[string]string
	// Choices are the builds a character can be picked with, by key,
	// when there are several: the one the card shows, then saved builds.
	Choices   map[string][]buildChoice
	TeamCount int
	// Enemy is the custom enemy carried over from a result page, nil for
	// the standard one.
	Enemy *config.Enemy
	Meta  pageMeta
	// Prefetch is the browser build's version, which the page warms up
	// for the result page; "" when the server calculates results.
	Prefetch string
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	l := localizerFor(r)
	q := r.URL.Query()
	data := indexData{
		UID:       strings.TrimSpace(q.Get("uid")),
		VS:        q.Get("vs"),
		TeamCount: len(s.lib.Teams),
	}
	if s.client != nil {
		data.Prefetch = s.client.version
	}
	enemy, err := enemyFromQuery(q)
	if err != nil {
		s.renderProblem(w, l, l.message(err))
		return
	}
	data.Enemy = enemy
	if data.VR, err = opponentChart(q, data.VS); err != nil {
		s.renderProblem(w, l, l.message(err))
		return
	}
	s.usage.page(r)
	data.Meta = pageMeta{Title: l.T("site.title"), Description: l.T("site.desc")}
	if data.UID == "" && data.VS == "" && enemy == nil {
		data.Meta = s.indexed(r, l, data.Meta, "/")
		data.Meta.Home = s.base(r) + "/"
	}
	if data.UID != "" {
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		p, err := s.profiles.Profile(ctx, data.UID)
		if err != nil {
			data.Problem = enkaMessage(l, err)
		} else {
			s.usage.uid(r, data.UID)
			s.showcase(l, &data, p)
		}
	}
	s.render(w, l, "index.html", data)
}

// opponentChart reads the chart of the first selection vs, when picking
// an opponent, from the parameter "vr", checking it as withChart does.
// The comparison scores it as "ra". Without vs it is "".
func opponentChart(q url.Values, vs string) (string, error) {
	if vs == "" {
		return "", nil
	}
	sel, err := withChart(selection{}, q, "vr")
	return sel.Chart, err
}

// showcase fills in the front page's showcase card for p, in l's language.
func (s *Server) showcase(l localizer, data *indexData, p *enka.Profile) {
	data.Profile = p
	var unlocks []unlock
	data.Suggested, unlocks = match(s.lib.Teams, s.members, p)
	if data.VS == "" {
		data.Unlocks = unlocks
	}
	data.Refresh = -1
	if !p.Expires.IsZero() {
		data.Refresh = max(0, int(math.Ceil(time.Until(p.Expires).Seconds())))
	}
	for i, sg := range data.Suggested {
		if data.VS != "" {
			data.Suggested[i].Href = l.link("/versus", data.Enemy, "a", data.VS, "ra", data.VR, "b", sg.Selection.String())
		} else {
			data.Suggested[i].Href = l.link("/score", data.Enemy, "a", sg.Selection.String())
		}
		data.Buildable = append(data.Buildable, strings.Join(sorted(sg.Selection.Chars), ","))
	}
}

// keptBuild is a build the browser kept of a showcase character: its
// buildToken and when it was last in the showcase, in Unix milliseconds.
type keptBuild struct {
	Build string `json:"b"`
	Seen  int64  `json:"t"`
}

// buildChoice is one of the builds a character can be picked with. Token
// is its buildToken, "" for the build the card shows.
type buildChoice struct {
	Label string
	Token string
	Char  enka.Character
}

// handleShowcase renders the front page's showcase card again with the
// characters the browser kept from earlier lookups, or imported from a
// GOOD file (handleGOOD), added, and the builds the player saved on
// Enka.Network (addSaved), so that teams with characters no longer in the
// showcase can be picked too. The builds stay in the browser; picks send
// them along (keptBuilds).
func (s *Server) handleShowcase(w http.ResponseWriter, r *http.Request) {
	l := localizerFor(r)
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var kept []keptBuild
	if err := json.Unmarshal([]byte(r.PostFormValue("kept")), &kept); err != nil || len(kept) > enka.MaxPacked {
		http.Error(w, "bad kept builds", http.StatusBadRequest)
		return
	}
	data := indexData{UID: strings.TrimSpace(r.PostFormValue("uid")), VS: r.PostFormValue("vs"), Kept: map[string]string{}}
	enemy, err := enemyFromQuery(r.PostForm)
	if err != nil {
		http.Error(w, l.message(err), http.StatusBadRequest)
		return
	}
	data.Enemy = enemy
	if data.VR, err = opponentChart(r.PostForm, data.VS); err != nil {
		http.Error(w, l.message(err), http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	p, err := s.profiles.Profile(ctx, data.UID)
	if err != nil {
		http.Error(w, enkaMessage(l, err), http.StatusBadGateway)
		return
	}
	s.usage.uid(r, data.UID)
	merged := *p
	merged.Characters = slices.Clone(p.Characters)
	for _, k := range kept {
		c, err := parseBuildToken(k.Build)
		// By canonical name, so that a Traveler from a GOOD file, which
		// has no gender, does not join the showcase's other one.
		if err != nil || slices.ContainsFunc(merged.Characters, func(m enka.Character) bool { return library.Canonical(m.Key) == library.Canonical(c.Key) }) {
			continue
		}
		merged.Characters = append(merged.Characters, c)
		data.Kept[c.Key] = time.UnixMilli(k.Seen).UTC().Format(time.DateOnly)
	}
	s.addSaved(ctx, l, &data, &merged)
	s.showcase(l, &data, &merged)
	s.render(w, l, "showcase", data)
}

// addSaved adds the builds the owner of p saved on Enka.Network (design
// §4.8). A character not in p comes with its first saved build in the
// player's order on Enka (enka.ParseBuilds); the showcase, and builds
// kept from it, stay first. A character with several builds can be
// picked with any of them. Without an owner, or when Enka fails, p stays
// as it is: saved builds never stop the showcase.
func (s *Server) addSaved(ctx context.Context, l localizer, data *indexData, p *enka.Profile) {
	src, ok := s.profiles.(SavedBuildSource)
	if !ok || p.Owner == nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	saved, err := src.SavedBuilds(ctx, *p.Owner)
	if err != nil {
		slog.Warn("enka saved builds", "uid", p.UID, "err", err)
		return
	}
	data.Saved, data.Choices = map[string]string{}, map[string][]buildChoice{}
	for _, b := range saved {
		key := b.Character.Key
		label := cmp.Or(b.Name, l.T("index.saved_unnamed"))
		i := slices.IndexFunc(p.Characters, func(c enka.Character) bool { return c.Key == key })
		switch {
		case i < 0:
			p.Characters = append(p.Characters, b.Character)
			data.Saved[key] = label
			data.Choices[key] = []buildChoice{{Label: label, Char: b.Character}}
			continue
		case data.Choices[key] == nil:
			first := l.T("index.build_live")
			if date, ok := data.Kept[key]; ok {
				first = l.T("index.kept_on", date)
			}
			data.Choices[key] = []buildChoice{{Label: first, Char: p.Characters[i]}}
		}
		data.Choices[key] = append(data.Choices[key], buildChoice{Label: label, Token: buildToken(b.Character), Char: b.Character})
	}
	for key, c := range data.Choices {
		if len(c) < 2 {
			delete(data.Choices, key)
		}
	}
}

// unlock is a character missing from a showcase, by canonical key, with
// the number of library teams it would complete, and of those it would
// leave one member short.
type unlock struct {
	Key         string
	Teams, Near int
}

// maxUnlocks is how many unlocks the front page shows.
const maxUnlocks = 5

// match lists the library teams that can be built from a showcase, and
// the characters that would complete the most teams one member short of
// it: up to maxUnlocks, by teams completed, then by teams they would
// bring one member short (what adding another would complete), then by
// key. One pass over the library does both; members are the teams'
// canonical members (canonicalMembers), worked out once.
func match(teams []*library.Team, members [][]string, p *enka.Profile) ([]suggestion, []unlock) {
	have := map[string]string{} // canonical name -> showcase key
	for _, c := range p.Characters {
		have[library.Canonical(c.Key)] = c.Key
	}
	var out []suggestion
	counts := map[string]*unlock{}
	count := func(key string) *unlock {
		u := counts[key]
		if u == nil {
			u = &unlock{Key: key}
			counts[key] = u
		}
		return u
	}
	for i, t := range teams {
		if len(members[i]) != 4 {
			continue
		}
		// On the stack: most teams are two or more short.
		chars, missing := make([]string, 0, 4), make([]string, 0, 3)
		for _, c := range members[i] {
			if key, ok := have[c]; ok {
				chars = append(chars, key)
			} else if missing = append(missing, c); len(missing) > 2 {
				break
			}
		}
		switch len(missing) {
		case 0:
			out = append(out, suggestion{Team: t, Selection: selection{UID: p.UID, Chars: slices.Clone(chars)}})
		case 1:
			count(missing[0]).Teams++
		case 2:
			count(missing[0]).Near++
			count(missing[1]).Near++
		}
	}
	var unlocks []unlock
	for _, u := range counts {
		if u.Teams > 0 {
			unlocks = append(unlocks, *u)
		}
	}
	slices.SortFunc(unlocks, func(a, b unlock) int {
		return cmp.Or(cmp.Compare(b.Teams, a.Teams), cmp.Compare(b.Near, a.Near), strings.Compare(a.Key, b.Key))
	})
	if len(unlocks) > maxUnlocks {
		unlocks = unlocks[:maxUnlocks]
	}
	return out, unlocks
}

// canonicalMembers lists each team's members by canonical name.
func canonicalMembers(teams []*library.Team) [][]string {
	out := make([][]string, len(teams))
	for i, t := range teams {
		for _, m := range t.Members {
			out[i] = append(out[i], library.Canonical(m))
		}
	}
	return out
}

type teamsData struct {
	Meta  pageMeta
	Query string
	Teams []*library.Team // matching the query, by name
	Total int
	Chars []string // canonical keys of the library's characters, by name
	Lang  string   // langQuery, for links to other indexed pages
}

// handleTeams lists the library, or the teams that have, for every word
// of the query, a character whose name contains it.
func (s *Server) handleTeams(w http.ResponseWriter, r *http.Request) {
	l := localizerFor(r)
	data := teamsData{Query: strings.TrimSpace(r.URL.Query().Get("q")), Total: len(s.lib.Teams)}
	words := strings.Fields(strings.ToLower(data.Query))
	for _, t := range s.lib.Teams {
		names := l.searchNames(t)
		if !slices.ContainsFunc(words, func(w string) bool {
			return !slices.ContainsFunc(names, func(n string) bool { return strings.Contains(n, w) })
		}) {
			data.Teams = append(data.Teams, t)
		}
	}
	names := map[*library.Team]string{}
	for _, t := range data.Teams {
		names[t] = l.teamName(t)
	}
	slices.SortFunc(data.Teams, func(a, b *library.Team) int { return strings.Compare(names[a], names[b]) })
	data.Chars = slices.Collect(maps.Keys(s.byChar))
	slices.SortFunc(data.Chars, func(a, b string) int { return strings.Compare(l.charName(a), l.charName(b)) })
	data.Lang = langQuery(r)
	data.Meta = pageMeta{Title: l.T("teams.title"), Description: l.T("site.desc")}
	if data.Query == "" {
		data.Meta = s.indexed(r, l, data.Meta, "/teams")
	}
	s.render(w, l, "teams.html", data)
}

type scoreData struct {
	Team   teamResult
	Meta   pageMeta
	Client *clientData // set when the browser calculates Team
	Enemy  enemyForm
}

func (s *Server) handleScore(w http.ResponseWriter, r *http.Request) {
	l := localizerFor(r)
	sel, err := selectionFromQuery(r, "a")
	if err != nil {
		s.renderProblem(w, l, l.message(err))
		return
	}
	enemy, err := enemyFromQuery(r.URL.Query())
	if err != nil {
		s.renderProblem(w, l, l.message(err))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	// Normalize the URL to one that carries the builds, so the result
	// can be shared and stays the same after the showcase changes.
	sel = s.withBuilds(ctx, sel, keptBuilds(r))
	if sel, err = withChart(sel, r.URL.Query(), "r"); err != nil {
		s.renderProblem(w, l, l.message(err))
		return
	}
	if canonical(w, r, "/score", enemy, "a", sel.String(), "r", sel.Chart) {
		return
	}
	s.usage.result(r)
	form := newEnemyForm(l, "/score", enemy, "a", sel.String(), "r", sel.Chart)
	if s.inBrowser(r) {
		tr := s.prepare(ctx, l, sel, enemy)
		tr.ID = "a"
		job := s.job(l, "score", tr)
		s.render(w, l, "score.html", scoreData{Team: tr, Meta: pendingScoreMeta(l, tr, baseURL(r)), Client: s.clientData(r, job), Enemy: form})
		return
	}
	tr := s.evaluate(ctx, l, sel, enemy)
	tr.ID = "a"
	s.render(w, l, "score.html", scoreData{Team: tr, Meta: scoreMeta(l, tr, baseURL(r)), Enemy: form})
}

// canonical redirects a result page to its shortest URL: the teams as
// the key-value pairs kv, then the enemy's parameters, the server
// fallback's and the page's language. It reports whether it redirected.
func canonical(w http.ResponseWriter, r *http.Request, path string, enemy *config.Enemy, kv ...string) bool {
	q := r.URL.Query()
	want := url.Values{}
	for i := 0; i+1 < len(kv); i += 2 {
		if kv[i+1] != "" {
			want.Set(kv[i], kv[i+1])
		}
	}
	addEnemy(want, enemy)
	if v := q.Get("server"); v != "" {
		want.Set("server", v)
	}
	if code := urlLang(r); code != "" {
		want.Set(langParam, code)
	}
	if q.Encode() == want.Encode() {
		return false
	}
	http.Redirect(w, r, path+"?"+want.Encode(), http.StatusSeeOther)
	return true
}

type versusData struct {
	A, B teamResult
	// Diff is the higher team's lead over the lower one, relative to the
	// lower: (A - B) / B when A leads, -(B - A) / A when B leads. Only set
	// when both teams have a score.
	Diff   float64
	Scored bool
	Tie    bool
	Meta   pageMeta
	Client *clientData // set when the browser calculates the teams
	Enemy  enemyForm
}

// tieThreshold is the relative difference below which two scores are shown
// as a tie. Rotation error is larger than simulation noise (design §7).
const tieThreshold = 0.02

func (s *Server) handleVersus(w http.ResponseWriter, r *http.Request) {
	l := localizerFor(r)
	a, err := parseSelection(r.URL.Query().Get("a"))
	if err != nil {
		s.renderProblem(w, l, l.message(err))
		return
	}
	b, err := selectionFromQuery(r, "b")
	if err != nil {
		s.renderProblem(w, l, l.message(err))
		return
	}
	enemy, err := enemyFromQuery(r.URL.Query())
	if err != nil {
		s.renderProblem(w, l, l.message(err))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()
	kept := keptBuilds(r)
	a, b = s.withBuilds(ctx, a, kept), s.withBuilds(ctx, b, kept)
	if a, err = withChart(a, r.URL.Query(), "ra"); err == nil {
		b, err = withChart(b, r.URL.Query(), "rb")
	}
	if err != nil {
		s.renderProblem(w, l, l.message(err))
		return
	}
	if canonical(w, r, "/versus", enemy, "a", a.String(), "b", b.String(), "ra", a.Chart, "rb", b.Chart) {
		return
	}
	s.usage.result(r)
	form := newEnemyForm(l, "/versus", enemy, "a", a.String(), "b", b.String(), "ra", a.Chart, "rb", b.Chart)
	if s.inBrowser(r) {
		data := newVersus(s.prepare(ctx, l, a, enemy), s.prepare(ctx, l, b, enemy))
		data.Meta = pendingVersusMeta(l, data, baseURL(r))
		data.Client = s.clientData(r, s.job(l, "versus", data.A, data.B))
		data.Enemy = form
		s.render(w, l, "versus.html", data)
		return
	}
	data := s.versus(ctx, l, a, b, enemy)
	data.Meta = versusMeta(l, data, baseURL(r))
	data.Enemy = form
	s.render(w, l, "versus.html", data)
}

// withBuilds is sel carrying its characters' builds: from kept, the
// builds the browser sends of characters that have left the showcase or
// that the player picked with a build saved on Enka (see handleShowcase),
// else from the showcase. It is sel as it is when it already carries them
// or they can't all be found.
func (s *Server) withBuilds(ctx context.Context, sel selection, kept []enka.Character) selection {
	if sel.Builds != nil {
		return sel
	}
	p, err := s.profiles.Profile(ctx, sel.UID)
	if err != nil {
		return sel
	}
	var builds []enka.Character
	for _, want := range sel.Chars {
		byKey := func(c enka.Character) bool { return c.Key == want }
		if i := slices.IndexFunc(kept, byKey); i >= 0 {
			builds = append(builds, kept[i])
		} else if i := slices.IndexFunc(p.Characters, byKey); i >= 0 {
			builds = append(builds, p.Characters[i])
		} else {
			return sel
		}
	}
	sel.Builds, sel.Nickname = builds, p.Nickname
	return sel
}

// maxKept is the most kept builds a pick sends: as many characters as
// the showcase holds.
const maxKept = 12

// keptBuilds reads the builds the browser sends with a pick of characters
// that have left the showcase: buildToken values in "build" parameters.
func keptBuilds(r *http.Request) []enka.Character {
	var out []enka.Character
	for _, v := range r.URL.Query()["build"] {
		if c, err := parseBuildToken(v); err == nil && len(out) < maxKept {
			out = append(out, c)
		}
	}
	return out
}

func (s *Server) versus(ctx context.Context, l localizer, a, b selection, enemy *config.Enemy) versusData {
	return newVersus(s.evaluate(ctx, l, a, enemy), s.evaluate(ctx, l, b, enemy))
}

// newVersus compares two teams, scored or not.
func newVersus(a, b teamResult) versusData {
	data := versusData{A: a, B: b}
	data.A.ID, data.B.ID = "a", "b"
	if data.A.Result != nil && data.B.Result != nil && data.A.Result.DPS > 0 && data.B.Result.DPS > 0 {
		data.Scored = true
		a, b := data.A.Result.DPS, data.B.Result.DPS
		if a >= b {
			data.Diff = (a - b) / b
		} else {
			data.Diff = -(b - a) / a
		}
		data.Tie = math.Abs(data.Diff) < tieThreshold
	}
	return data
}

func (s *Server) renderProblem(w http.ResponseWriter, l localizer, msg string) {
	s.writeHeader(w, l)
	w.WriteHeader(http.StatusBadRequest)
	s.render(w, l, "problem.html", msg)
}

// writeHeader sets the headers of a page in l's language.
func (s *Server) writeHeader(w http.ResponseWriter, l localizer) {
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Content-Language", l.Lang)
	h.Set("Vary", "Accept-Language, Cookie")
}

func (s *Server) render(w http.ResponseWriter, l localizer, name string, data any) {
	s.writeHeader(w, l)
	if err := s.tmpl[l].ExecuteTemplate(w, name, data); err != nil {
		slog.Error("render", "template", name, "err", err)
	}
}

// EnkaSource fetches profiles from Enka.Network and caches them for the
// time Enka asks for, and the builds their owners saved (SavedBuilds).
type EnkaSource struct {
	client *enka.Client
	cache  *cache[string, *enka.Profile]
	saved  *cache[enka.Owner, savedResult]
}

func NewEnkaSource(client *enka.Client) *EnkaSource {
	return &EnkaSource{
		client: client,
		cache:  newCache[string, *enka.Profile](500),
		saved:  newCache[enka.Owner, savedResult](500),
	}
}

func (e *EnkaSource) Profile(ctx context.Context, uid string) (*enka.Profile, error) {
	if p, ok := e.cache.get(uid); ok {
		return p, nil
	}
	raw, err := e.client.Fetch(ctx, uid)
	if err != nil {
		return nil, err
	}
	p, err := enka.Parse(raw)
	if err != nil {
		return nil, err
	}
	if p.UID == "" {
		p.UID = uid
	}
	// The front page counts down to Expires, after which loading the UID
	// again fetches it anew (design §4.1). Enka's ttl is the time until it
	// refreshes the showcase itself, so the cache never serves a showcase
	// older than Enka's; it keeps one at least a minute.
	ttl := max(time.Duration(p.TTL)*time.Second, time.Minute)
	p.Expires = time.Now().Add(ttl)
	e.cache.put(uid, p, ttl)
	return p, nil
}

type savedResult struct {
	builds []enka.SavedBuild
	err    error
}

// Saved builds change when their player saves one, rarely, and Enka gives
// them no TTL: they are kept for savedTTL, and a failed fetch is not
// tried again for savedRetry, so that a showcase loaded again does not
// ask Enka again.
const (
	savedTTL   = 10 * time.Minute
	savedRetry = time.Minute
)

// SavedBuilds returns the builds owner saved on Enka.Network.
func (e *EnkaSource) SavedBuilds(ctx context.Context, owner enka.Owner) ([]enka.SavedBuild, error) {
	if r, ok := e.saved.get(owner); ok {
		return r.builds, r.err
	}
	raw, err := e.client.FetchBuilds(ctx, owner)
	var builds []enka.SavedBuild
	if err == nil {
		builds, err = enka.ParseBuilds(raw)
	}
	if err != nil {
		e.saved.put(owner, savedResult{err: err}, savedRetry)
		return nil, err
	}
	e.saved.put(owner, savedResult{builds: builds}, savedTTL)
	return builds, nil
}
