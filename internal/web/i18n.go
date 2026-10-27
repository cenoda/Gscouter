package web

import (
	"cmp"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/cenoda/gscouter/internal/library"
)

// Site languages are the game's text languages, coded as Enka codes
// them. A visitor gets the language they picked (kept in a cookie), else
// the best one their browser asks for, else English. A URL can name its
// language too (langParam), for search engines.
const (
	langEnglish = "en"
	langKorean  = "ko"
	defaultLang = langEnglish
	langCookie  = "lang"
)

// langs are the languages offered in the picker, in the game's order,
// each named in itself.
var langs = []struct{ Code, Name string }{
	{"zh-cn", "简体中文"},
	{"zh-tw", "繁體中文"},
	{langEnglish, "English"},
	{langKorean, "한국어"},
	{"ja", "日本語"},
	{"es", "Español"},
	{"fr", "Français"},
	{"ru", "Русский"},
	{"th", "ภาษาไทย"},
	{"vi", "Tiếng Việt"},
	{"de", "Deutsch"},
	{"id", "Bahasa Indonesia"},
	{"pt", "Português"},
	{"tr", "Türkçe"},
	{"it", "Italiano"},
}

// localizer writes the site's text in one language. Auto is set when the
// language came from the browser rather than the visitor's pick.
type localizer struct {
	Lang string
	Auto bool
}

// english is for output that is always in English, like card images.
var english = localizer{Lang: langEnglish}

// T returns the message for key, formatted with args when there are any.
// A message missing in the language falls back to English, then to key.
func (l localizer) T(key string, args ...any) string {
	msg, ok := catalog[l.Lang][key]
	if !ok {
		msg = cmp.Or(catalog[defaultLang][key], key)
	}
	if len(args) > 0 {
		msg = fmt.Sprintf(msg, args...)
	}
	if glossed[key] && ok && l.Lang != langEnglish && l.Lang != langKorean {
		msg += " (" + english.T(key, args...) + ")"
	}
	return msg
}

// glossed are the messages that no game data or wiki has in the site's
// other languages. Their translations are followed by the English in
// parentheses, except in Korean, the owner's language.
var glossed = map[string]bool{
	"enemy.heading": true, "enemy.standard": true, "enemy.lead": true,
	"enemy.apply": true, "enemy.reset": true, "enemy.custom_note": true,
	"enemy.same_note": true, "enemy.carried": true, "enemy.preset": true,
	"enemy.custom": true, "enemy.preset_note": true,
	"err.enemy": true, "err.boss": true,
}

// name returns the message for prefix+key, or key when there is none:
// gcsim's names for things the catalog does not know yet.
func (l localizer) name(prefix, key string) string {
	if msg, ok := catalog[l.Lang][prefix+key]; ok {
		return msg
	}
	if msg, ok := catalog[defaultLang][prefix+key]; ok {
		return msg
	}
	return key
}

// localizerFor picks the language of a request: the one its URL names
// (see langParam), else the visitor's.
func localizerFor(r *http.Request) localizer {
	if code := urlLang(r); code != "" {
		return localizer{Lang: code}
	}
	if c, err := r.Cookie(langCookie); err == nil {
		if _, ok := catalog[c.Value]; ok {
			return localizer{Lang: c.Value}
		}
	}
	return localizer{Lang: negotiate(r.Header.Get("Accept-Language")), Auto: true}
}

// negotiate returns the site language an Accept-Language header prefers
// most, matching on the primary subtag ("ko-KR" is Korean), or
// defaultLang. Chinese goes by script or region: Traditional for
// Taiwan, Hong Kong and Macau, else Simplified.
func negotiate(header string) string {
	best, bestQ := defaultLang, 0.0
	for _, part := range strings.Split(header, ",") {
		tag, params, _ := strings.Cut(part, ";")
		q := 1.0
		for _, p := range strings.Split(params, ";") {
			if v, ok := strings.CutPrefix(strings.TrimSpace(p), "q="); ok {
				f, err := strconv.ParseFloat(v, 64)
				if err != nil {
					f = 0
				}
				q = f
			}
		}
		code := siteLang(strings.ToLower(strings.TrimSpace(tag)))
		if _, ok := catalog[code]; ok && q > bestQ {
			best, bestQ = code, q
		}
	}
	return best
}

// siteLang maps a lowercase language tag to a site language code, which
// may not be one the site has.
func siteLang(tag string) string {
	primary, rest, _ := strings.Cut(tag, "-")
	if primary != "zh" {
		return primary
	}
	for _, sub := range strings.Split(rest, "-") {
		switch sub {
		case "hant", "tw", "hk", "mo":
			return "zh-tw"
		}
	}
	return "zh-cn"
}

// handleLang saves the language picked in the page header, or forgets it
// when the visitor picks "Auto", and goes back to the page they were on.
func (s *Server) handleLang(w http.ResponseWriter, r *http.Request) {
	c := &http.Cookie{Name: langCookie, Path: "/", SameSite: http.SameSiteLaxMode}
	if v := r.FormValue("lang"); catalog[v] != nil {
		c.Value, c.MaxAge = v, int((400 * 24 * time.Hour).Seconds())
	} else {
		c.MaxAge = -1
	}
	http.SetCookie(w, c)
	http.Redirect(w, r, backTo(r), http.StatusSeeOther)
}

// backTo is the page on this site that sent r, or the front page.
func backTo(r *http.Request) string {
	u, err := url.Parse(r.Referer())
	if err != nil || u.Host != r.Host {
		return "/"
	}
	// The page's own language would override the one just picked.
	q := u.Query()
	if q.Has(langParam) {
		q.Del(langParam)
		u.RawQuery = q.Encode()
	}
	uri := u.RequestURI()
	if !strings.HasPrefix(uri, "/") || strings.HasPrefix(uri, "//") {
		return "/"
	}
	return uri
}

// localeFuncs are the template functions that depend on the language.
func localeFuncs(l localizer) template.FuncMap {
	return template.FuncMap{
		"t": l.T,
		// th is a message with markup; the catalog is trusted.
		"th":       func(key string) template.HTML { return template.HTML(l.T(key)) },
		"lang":     func() string { return l.Lang },
		"langAuto": func() bool { return l.Auto },
		"langs":    func() any { return langs },
		"meta":     func(v any) pageMeta { return metaOf(l, v) },
		"name":     l.charName,
		"weapon":   l.weaponName,
		"teamName": l.teamName,
		// enemyText describes a result's enemy.
		"enemyText": l.enemyText,
		"presets":   l.presets,
		"term":      l.term,
		// members lists what finds a team's members, for search.
		"members": func(t *library.Team) string { return strings.Join(l.searchNames(t), " ") },
	}
}

// catalog holds the site's text by language and key; each language's
// file adds its messages. Format verbs take the arguments in Korean
// order; %[n] reorders them where a language needs.
var catalog = map[string]map[string]string{}
