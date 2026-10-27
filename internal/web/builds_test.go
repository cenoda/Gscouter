package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func TestSelectionWithBuilds(t *testing.T) {
	builds := hutaoTeam()
	sel := selection{UID: "800000001", Chars: keys(builds), Builds: builds, Nickname: "강한 사람"}
	s := sel.String()
	if !strings.HasPrefix(s, "800000001~") || len(url.QueryEscape(s)) > 400 {
		t.Errorf("packed selection %q (%d characters)", s, len(url.QueryEscape(s)))
	}
	got, err := parseSelection(s)
	if err != nil {
		t.Fatal(err)
	}
	if got.Nickname != sel.Nickname || !reflect.DeepEqual(got.Chars, sel.Chars) {
		t.Errorf("got %+v", got)
	}
	for i := range builds {
		if got.Builds[i].Config() != builds[i].Config() {
			t.Errorf("%s: config %q, want %q", builds[i].Key, got.Builds[i].Config(), builds[i].Config())
		}
	}

	for _, bad := range []string{
		"800000001~" + s[len("800000001~"):len(s)-4], // cut short
		"800000001~!!!",
		"abc~" + s[len("800000001~"):],
		// Three characters are not a team.
		selection{UID: "800000001", Builds: builds[:3]}.String(),
	} {
		if _, err := parseSelection(bad); err == nil {
			t.Errorf("%q: no error", bad)
		}
	}
}

func post(t *testing.T, ts *httptest.Server, path string, form url.Values) (int, string) {
	t.Helper()
	resp, err := ts.Client().PostForm(ts.URL+path, form)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

// noRedirects fetches path without following redirects.
func noRedirects(t *testing.T, ts *httptest.Server, url string) *http.Response {
	t.Helper()
	client := *ts.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp
}

// Results redirect to a link carrying the builds, which keeps working
// without the showcase.
func TestScoreLinkCarriesBuilds(t *testing.T) {
	ts := newTestServer(t)
	resp := noRedirects(t, ts, ts.URL+"/score?a="+url.QueryEscape("800000001:hutao,yelan,xingqiu,zhongli"))
	loc := resp.Header.Get("Location")
	if resp.StatusCode != http.StatusSeeOther || !strings.Contains(loc, "a=800000001~") {
		t.Fatalf("status %d, location %q", resp.StatusCode, loc)
	}
	packed, _ := url.ParseQuery(loc[strings.Index(loc, "?")+1:])
	sel, err := parseSelection(packed.Get("a"))
	if err != nil {
		t.Fatal(err)
	}

	// The same builds under a UID the source does not know: no lookup.
	sel.UID = "800000009"
	code, body := get(t, ts, "/score?a="+url.QueryEscape(sel.String()))
	if code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	mustContain(t, body, "DPS", "Strong", "Staff of Homa", "/card/score.png?a=800000009~")
	if resp := noRedirects(t, ts, ts.URL+"/score?a="+url.QueryEscape(sel.String())); resp.StatusCode != http.StatusOK {
		t.Errorf("packed link redirects: %d", resp.StatusCode)
	}
	resp = noRedirects(t, ts, ts.URL+"/card/score.png?a="+url.QueryEscape(sel.String()))
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "image/png" {
		t.Errorf("card: status %d, type %q", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	_, body = get(t, ts, "/score?a="+url.QueryEscape("800000001~bad"))
	mustContain(t, body, "800000001~bad")
}

// A character that has left the showcase can be picked with the build
// the browser kept.
func TestKeptBuilds(t *testing.T) {
	ts := newTestServer(t)
	zhongli := buildToken(hutaoTeam()[3])

	form := url.Values{"uid": {"800000003"}, "kept": {`[{"b":"` + zhongli + `","t":1767225600000},{"b":"junk","t":0}]`}}
	code, body := post(t, ts, "/showcase", form)
	if code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	mustContain(t, body, "Moved", "Hu Tao / Xingqiu / Zhongli / Yelan", `data-kept="`+zhongli+`"`, "As of 2026-01-01", `id="forget-kept"`)
	if strings.Contains(body, "<html") {
		t.Error("the showcase card came with the whole page")
	}
	// Without the kept build, there is no team.
	_, body = get(t, ts, "/?uid=800000003")
	mustContain(t, body, "No library team")

	// Picking sends the kept build along; the result link carries it.
	pick := "/score?uid=800000003&c=hutao&c=yelan&c=xingqiu&c=zhongli&build=" + zhongli
	resp := noRedirects(t, ts, ts.URL+pick)
	if loc := resp.Header.Get("Location"); !strings.Contains(loc, "a=800000003~") || strings.Contains(loc, "build=") {
		t.Errorf("location %q", loc)
	}
	_, body = get(t, ts, pick)
	mustContain(t, body, "DPS", "Moved", "Black Tassel")
	_, body = get(t, ts, "/score?uid=800000003&c=hutao&c=yelan&c=xingqiu&c=zhongli")
	mustContain(t, body, "not in the showcase")

	// The showcase's own build wins over a kept one.
	old := hutaoTeam()[0]
	old.Cons = 6
	form.Set("kept", `[{"b":"`+buildToken(old)+`","t":0}]`)
	_, body = post(t, ts, "/showcase", form)
	if strings.Contains(body, "C6") || strings.Contains(body, "data-kept") {
		t.Error("kept build replaced the showcase's")
	}
}
