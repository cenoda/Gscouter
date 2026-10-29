package web

import (
	"cmp"
	"encoding/xml"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/cenoda/gscouter/internal/library"
)

// Search engines index the library's pages (the front page, the team
// list, and a page per team and per character) in every site language.
// A page's language can be set by its URL so that each language has an
// address of its own: English is the page's path, the others add
// "?hl=<code>". Results are left out: they run simulations and look
// players up on Enka.Network.

// langParam is the URL parameter that sets a page's language.
const langParam = "hl"

// urlLang is the site language r's URL names, or "".
func urlLang(r *http.Request) string {
	if code := r.URL.Query().Get(langParam); catalog[code] != nil {
		return code
	}
	return ""
}

// langQuery keeps the language r's URL names in links to other indexed
// pages, e.g. "?hl=ko"; "" when it names none.
func langQuery(r *http.Request) string {
	if code := urlLang(r); code != "" {
		return "?" + langParam + "=" + code
	}
	return ""
}

// langURL is path's address in language code.
func langURL(path, code string) string {
	if code == langEnglish {
		return path
	}
	return path + "?" + langParam + "=" + url.QueryEscape(code)
}

// alternate is a page's address in one language, for hreflang links.
type alternate struct{ Lang, Href string }

// SetSite sets the site's public URL, e.g. "https://gscouter.example",
// for the absolute URLs of canonical links, sitemaps and IndexNow.
// Without it they take the host of each request.
func (s *Server) SetSite(u string) { s.site = strings.TrimSuffix(u, "/") }

// base is the site's URL for r.
func (s *Server) base(r *http.Request) string { return cmp.Or(s.site, baseURL(r)) }

// indexed adds to m the canonical and alternate links of the indexed
// page at path, shown in l's language.
func (s *Server) indexed(r *http.Request, l localizer, m pageMeta, path string) pageMeta {
	base := s.base(r)
	m.Canonical = base + langURL(path, l.Lang)
	for _, lang := range langs {
		m.Alternates = append(m.Alternates, alternate{lang.Code, base + langURL(path, lang.Code)})
	}
	m.Alternates = append(m.Alternates, alternate{"x-default", base + path})
	return m
}

// indexLibrary indexes the library's teams by ID and by member, each
// member's teams by score with the reference build, best first.
func indexLibrary(lib *library.Library) (map[string]*library.Team, map[string][]*library.Team) {
	byID := map[string]*library.Team{}
	byChar := map[string][]*library.Team{}
	for _, t := range lib.Teams {
		byID[t.ID] = t
		for _, m := range t.Members {
			c := library.Canonical(m)
			byChar[c] = append(byChar[c], t)
		}
	}
	for _, teams := range byChar {
		slices.SortStableFunc(teams, func(a, b *library.Team) int {
			return cmp.Or(cmp.Compare(referenceScore(b), referenceScore(a)), strings.Compare(a.ID, b.ID))
		})
	}
	return byID, byChar
}

// referenceScore is a team's score with the build of its source, as the
// site scores it, or 0 when it has not been measured.
func referenceScore(t *library.Team) int {
	if t.Validation == nil {
		return 0
	}
	return t.Validation.DPS
}

// pages lists the paths of the indexed pages.
func (s *Server) pages() []string {
	out := []string{"/", "/teams", "/about"}
	for _, t := range s.lib.Teams {
		out = append(out, teamPath(t))
	}
	for _, c := range slices.Sorted(maps.Keys(s.byChar)) {
		out = append(out, characterPath(c))
	}
	return out
}

func teamPath(t *library.Team) string { return "/teams/" + t.ID }

func characterPath(key string) string { return "/characters/" + key }

// robots keeps crawlers to the indexed pages. Link previews (the second
// group) may read everything, so that shared results show their cards.
const robots = `User-agent: *
Disallow: /score
Disallow: /versus
Disallow: /s/
Disallow: /card/
Disallow: /showcase
Disallow: /lang
Disallow: /static/
Disallow: /*?uid=
Disallow: /*&uid=
Disallow: /*?vs=
Disallow: /*&vs=
Disallow: /*?q=
Disallow: /*&q=

User-agent: Twitterbot
User-agent: facebookexternalhit
User-agent: Discordbot
User-agent: Slackbot
User-agent: TelegramBot
User-agent: kakaotalk-scrap
Allow: /

Sitemap: %s/sitemap.xml
`

func (s *Server) handleRobots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, robots, s.base(r))
}

// handleSitemapIndex lists a sitemap per language: all of them in one
// would pass the 50,000 URLs a sitemap may hold.
func (s *Server) handleSitemapIndex(w http.ResponseWriter, r *http.Request) {
	base := s.base(r)
	var b strings.Builder
	b.WriteString(xml.Header + `<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	for _, lang := range langs {
		writeLoc(&b, "sitemap", base+"/sitemaps/"+lang.Code+".xml")
	}
	b.WriteString("</sitemapindex>\n")
	writeXML(w, b.String())
}

// handleSitemap lists the indexed pages in one language, for
// /sitemaps/<code>.xml.
func (s *Server) handleSitemap(w http.ResponseWriter, r *http.Request) {
	code, ok := strings.CutSuffix(r.PathValue("file"), ".xml")
	if !ok || catalog[code] == nil {
		http.NotFound(w, r)
		return
	}
	base := s.base(r)
	var b strings.Builder
	b.WriteString(xml.Header + `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	for _, path := range s.pages() {
		writeLoc(&b, "url", base+langURL(path, code))
	}
	b.WriteString("</urlset>\n")
	writeXML(w, b.String())
}

func writeLoc(b *strings.Builder, elem, loc string) {
	b.WriteString("<" + elem + "><loc>")
	xml.EscapeText(b, []byte(loc))
	b.WriteString("</loc></" + elem + ">\n")
}

func writeXML(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Write([]byte(body))
}
