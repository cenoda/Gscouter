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
	Profile   *enka.Profile
	Problem   string
	Suggested []suggestion
	// Buildable lists the suggestions' characters, sorted and joined by
	// commas, so the picker can tell when four picks are not a team.
	Buildable []string
	// Kept dates the characters the browser kept from earlier lookups
	// that are no longer in the showcase, by key.
	Kept      map[string]string
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
			s.showcase(&data, p)
		}
	}
	s.render(w, l, "index.html", data)
}

// showcase fills in the front page's showcase card for p.
func (s *Server) showcase(data *indexData, p *enka.Profile) {
	data.Profile = p
	data.Suggested = s.suggest(p)
	for i, sg := range data.Suggested {
		if data.VS != "" {
			data.Suggested[i].Href = link("/versus", data.Enemy, "a", data.VS, "b", sg.Selection.String())
		} else {
			data.Suggested[i].Href = link("/score", data.Enemy, "a", sg.Selection.String())
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

// handleShowcase renders the front page's showcase card again with the
// characters the browser kept from earlier lookups, or imported from a
// GOOD file (handleGOOD), added, so that teams with characters no longer
// in the showcase can be picked too. The
// builds stay in the browser; picks send them along (keptBuilds).
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
	s.showcase(&data, &merged)
	s.render(w, l, "showcase", data)
}

// suggest lists the library teams that can be built from a showcase.
func (s *Server) suggest(p *enka.Profile) []suggestion {
	have := map[string]string{} // canonical name -> showcase key
	for _, c := range p.Characters {
		have[library.Canonical(c.Key)] = c.Key
	}
	var out []suggestion
	for _, t := range s.lib.Teams {
		var chars []string
		for _, m := range t.Members {
			key, ok := have[library.Canonical(m)]
			if !ok {
				break
			}
			chars = append(chars, key)
		}
		if len(chars) == 4 {
			out = append(out, suggestion{Team: t, Selection: selection{UID: p.UID, Chars: chars}})
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
	form := newEnemyForm("/score", enemy, "a", sel.String(), "r", sel.Chart)
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
// the key-value pairs kv, then the enemy's parameters, and the server
// fallback's. It reports whether it redirected.
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
	form := newEnemyForm("/versus", enemy, "a", a.String(), "b", b.String(), "ra", a.Chart, "rb", b.Chart)
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

// withBuilds is sel carrying its characters' builds: from the player's
// showcase, or from kept, builds the browser kept of characters that have
// left it (see handleShowcase). It is sel as it is when it already
// carries them or they can't all be found.
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
		if i := slices.IndexFunc(p.Characters, byKey); i >= 0 {
			builds = append(builds, p.Characters[i])
		} else if i := slices.IndexFunc(kept, byKey); i >= 0 {
			builds = append(builds, kept[i])
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
// time Enka asks for.
type EnkaSource struct {
	client *enka.Client
	cache  *cache[string, *enka.Profile]
}

func NewEnkaSource(client *enka.Client) *EnkaSource {
	return &EnkaSource{client: client, cache: newCache[string, *enka.Profile](500)}
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
	ttl := max(time.Duration(p.TTL)*time.Second, time.Minute)
	e.cache.put(uid, p, ttl)
	return p, nil
}
