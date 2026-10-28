package web

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cenoda/gscouter/internal/library"
)

const hutaoTeamID = "hutao-xingqiu-zhongli-yelan"

func TestLibraryTeamPage(t *testing.T) {
	ts := newTestServer(t)
	code, body := get(t, ts, "/teams/"+hutaoTeamID+"?hl=ko")
	if code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	// The URL's language wins over the browser's, and links to the other
	// indexed pages keep it.
	mustContain(t, body, `<html lang="ko">`, "<title>호두 / 행추 / 종려 / 야란 파티 DPS · Gscouter</title>",
		`class="score"`, `href="/characters/hutao?hl=ko"`, `href="https://db.kqm.gg/db/`,
		`<link rel="canonical" href="`+ts.URL+`/teams/`+hutaoTeamID+`?hl=ko">`,
		`<link rel="alternate" hreflang="en" href="`+ts.URL+`/teams/`+hutaoTeamID+`">`,
		`<link rel="alternate" hreflang="ja" href="`+ts.URL+`/teams/`+hutaoTeamID+`?hl=ja">`,
		`<link rel="alternate" hreflang="x-default" href="`+ts.URL+`/teams/`+hutaoTeamID+`">`)

	// Without hl, the page is in the browser's language and its links
	// don't name one.
	_, body = getKo(t, ts, "/teams/"+hutaoTeamID)
	mustContain(t, body, `<html lang="ko">`, `href="/characters/hutao"`,
		`<link rel="canonical" href="`+ts.URL+`/teams/`+hutaoTeamID+`?hl=ko">`)

	if code, _ := get(t, ts, "/teams/no-such-team"); code != http.StatusNotFound {
		t.Errorf("unknown team: status %d", code)
	}
}

func TestCharacterPage(t *testing.T) {
	ts := newTestServer(t)
	code, body := get(t, ts, "/characters/hutao")
	if code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	mustContain(t, body, "<title>Hu Tao teams ranked by DPS · Gscouter</title>", `href="/teams/`+hutaoTeamID+`"`,
		`<link rel="canonical" href="`+ts.URL+`/characters/hutao">`)
	if code, _ := get(t, ts, "/characters/nobody"); code != http.StatusNotFound {
		t.Errorf("unknown character: status %d", code)
	}

	// Best score first.
	srv := newTestSite(t)
	teams := srv.byChar["hutao"]
	if !slices.IsSortedFunc(teams, func(a, b *library.Team) int { return referenceScore(b) - referenceScore(a) }) {
		t.Error("Hu Tao's teams are not by score")
	}
}

func TestIndexedPagesLinked(t *testing.T) {
	ts := newTestServer(t)
	_, body := get(t, ts, "/teams")
	mustContain(t, body, `href="/teams/`+hutaoTeamID+`"`, `href="/characters/hutao"`,
		`<link rel="canonical" href="`+ts.URL+`/teams">`)
	_, body = get(t, ts, "/")
	mustContain(t, body, `<link rel="canonical" href="`+ts.URL+`/">`, `hreflang="ko" href="`+ts.URL+`/?hl=ko"`)

	// Searches and lookups are not indexed.
	for _, path := range []string{"/teams?q=hutao", "/?uid=800000001"} {
		if _, body := get(t, ts, path); strings.Contains(body, `rel="canonical"`) {
			t.Errorf("%s has a canonical link", path)
		}
	}
}

func TestLanguagePickDropsURLLanguage(t *testing.T) {
	ts := newTestServer(t)
	client := *ts.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	req, _ := http.NewRequest("POST", ts.URL+"/lang", strings.NewReader(url.Values{"lang": {"ja"}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", ts.URL+"/teams/"+hutaoTeamID+"?hl=ko")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if got := resp.Header.Get("Location"); got != "/teams/"+hutaoTeamID {
		t.Errorf("location %q", got)
	}
}

func TestSitemaps(t *testing.T) {
	srv := newTestSite(t)
	srv.SetSite("https://gscouter.example/")
	ts := newTestServerFor(t, srv)

	_, body := get(t, ts, "/robots.txt")
	mustContain(t, body, "Disallow: /score\n", "Sitemap: https://gscouter.example/sitemap.xml\n")

	_, body = get(t, ts, "/sitemap.xml")
	if n := strings.Count(body, "<sitemap>"); n != len(langs) {
		t.Errorf("%d sitemaps, want %d", n, len(langs))
	}
	mustContain(t, body, "<loc>https://gscouter.example/sitemaps/ko.xml</loc>")

	_, body = get(t, ts, "/sitemaps/ko.xml")
	mustContain(t, body, "<loc>https://gscouter.example/?hl=ko</loc>",
		"<loc>https://gscouter.example/teams/"+hutaoTeamID+"?hl=ko</loc>",
		"<loc>https://gscouter.example/characters/hutao?hl=ko</loc>")
	if n := strings.Count(body, "<url>"); n != len(srv.pages()) || n > 50000 {
		t.Errorf("%d URLs, want %d (at most 50,000)", n, len(srv.pages()))
	}
	_, body = get(t, ts, "/sitemaps/en.xml")
	mustContain(t, body, "<loc>https://gscouter.example/teams/"+hutaoTeamID+"</loc>")

	if code, _ := get(t, ts, "/sitemaps/xx.xml"); code != http.StatusNotFound {
		t.Errorf("unknown language: status %d", code)
	}
}

func TestIndexNow(t *testing.T) {
	type submission struct {
		Host, Key, KeyLocation string
		URLList                []string
	}
	var got []submission
	engine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var s submission
		if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
			t.Error(err)
		}
		got = append(got, s)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer engine.Close()

	state := filepath.Join(t.TempDir(), "indexnow.json")
	start := func(edit func(*library.Library)) (*Server, *httptest.Server) {
		t.Helper()
		srv := newTestSiteWith(t, edit)
		srv.SetSite("https://gscouter.example")
		if err := srv.SetIndexNow(state); err != nil {
			t.Fatal(err)
		}
		srv.indexNow.endpoint = engine.URL
		return srv, newTestServerFor(t, srv)
	}
	submit := func(srv *Server) int {
		t.Helper()
		got = nil
		n, err := srv.SubmitChanges(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		return n
	}

	// The first time, every page in every language, in requests the
	// protocol allows.
	srv, ts := start(nil)
	key := srv.indexNow.key
	if n := submit(srv); n != len(srv.pages())*len(langs) {
		t.Errorf("first submission: %d URLs, want %d", n, len(srv.pages())*len(langs))
	}
	for _, s := range got {
		if s.Host != "gscouter.example" || s.Key != key || s.KeyLocation != "https://gscouter.example/"+key+".txt" || len(s.URLList) > indexNowBatch {
			t.Errorf("submission %s %s %s with %d URLs", s.Host, s.Key, s.KeyLocation, len(s.URLList))
		}
	}
	resp, err := ts.Client().Get(ts.URL + "/" + key + ".txt")
	if err != nil {
		t.Fatal(err)
	}
	served, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(served) != key {
		t.Errorf("key file %q, want %q", served, key)
	}

	// Nothing changed: nothing to submit.
	if n := submit(srv); n != 0 {
		t.Errorf("unchanged: %d URLs", n)
	}

	// A restart keeps the key; a changed score resubmits the team and
	// its members' pages.
	srv, _ = start(func(lib *library.Library) {
		for _, t := range lib.Teams {
			if t.ID == hutaoTeamID {
				t.Validation.DPS++
			}
		}
	})
	if srv.indexNow.key != key {
		t.Error("the key changed")
	}
	if n := submit(srv); n != 5*len(langs) {
		t.Errorf("one team changed: %d URLs, want %d", n, 5*len(langs))
	}
	if len(got) != 1 || !slices.Contains(got[0].URLList, "https://gscouter.example/teams/"+hutaoTeamID+"?hl=ko") ||
		!slices.Contains(got[0].URLList, "https://gscouter.example/characters/yelan") {
		t.Errorf("submitted %v", got)
	}
}
