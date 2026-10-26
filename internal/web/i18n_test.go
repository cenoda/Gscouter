package web

import (
	"io"
	"maps"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"
)

func TestNegotiate(t *testing.T) {
	for header, want := range map[string]string{
		"":                               "en",
		"ko":                             "ko",
		"ko-KR,ko;q=0.9,en-US;q=0.8":     "ko",
		"en-US,en;q=0.9,ko;q=0.8":        "en",
		"ja-JP,ja;q=0.9":                 "ja",
		"ja;q=0.4, KO-kr;q=0.5":          "ko",
		"ar-SA,ar;q=0.9":                 "en",
		"zh":                             "zh-cn",
		"zh-Hans-CN":                     "zh-cn",
		"zh-TW":                          "zh-tw",
		"zh-Hant-HK, en;q=0.5":           "zh-tw",
		"pt-BR,pt;q=0.9":                 "pt",
		"ar, ko;q=0.4, en;q=0.6":         "en",
		"ko;q=0":                         "en",
		"zh-CN;q=0.9;foo=bar, ko;q=0.95": "ko",
	} {
		if got := negotiate(header); got != want {
			t.Errorf("negotiate(%q) = %q, want %q", header, got, want)
		}
	}
}

// verbs are the format verbs of a message, in order.
var verbs = regexp.MustCompile(`%(\[\d\])?[-+#0]*[\d.]*[a-zA-Z%]`)

func TestCatalogComplete(t *testing.T) {
	for _, l := range langs {
		if catalog[l.Code] == nil {
			t.Errorf("no catalog for %s", l.Code)
		}
	}
	en := catalog[langEnglish]
	for code, msgs := range catalog {
		for _, key := range slices.Sorted(maps.Keys(en)) {
			msg, ok := msgs[key]
			if !ok {
				t.Errorf("%s: missing %q", code, key)
				continue
			}
			// The same arguments, maybe in another order.
			a, b := verbs.FindAllString(en[key], -1), verbs.FindAllString(msg, -1)
			if len(a) != len(b) {
				t.Errorf("%s %q: verbs %q, English has %q", code, key, b, a)
			}
		}
		for key := range msgs {
			if _, ok := en[key]; !ok {
				t.Errorf("%s: %q is not in English", code, key)
			}
		}
	}
}

func TestLanguage(t *testing.T) {
	ts := newTestServer(t)
	_, body := get(t, ts, "/")
	mustContain(t, body, `<html lang="en">`, "UID lookup", `<option value="" selected>Auto</option>`)
	_, body = getKo(t, ts, "/")
	mustContain(t, body, `<html lang="ko">`, "UID 조회", `<option value="" selected>자동</option>`)

	// Picking a language saves it and goes back to the page.
	client := *ts.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	pick := func(lang, referer string) *http.Response {
		t.Helper()
		req, _ := http.NewRequest("POST", ts.URL+"/lang", strings.NewReader(url.Values{"lang": {lang}}.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Referer", referer)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp
	}
	resp := pick("ko", ts.URL+"/teams?q=hutao")
	if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != "/teams?q=hutao" {
		t.Errorf("status %d, location %q", resp.StatusCode, resp.Header.Get("Location"))
	}
	cookies := resp.Cookies()
	if len(cookies) != 1 || cookies[0].Name != langCookie || cookies[0].Value != "ko" || cookies[0].MaxAge <= 0 {
		t.Fatalf("cookies %v", cookies)
	}

	// The pick wins over the browser.
	req, _ := http.NewRequest("GET", ts.URL+"/", nil)
	req.Header.Set("Accept-Language", "en")
	req.AddCookie(cookies[0])
	r, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body2, _ := io.ReadAll(r.Body)
	r.Body.Close()
	mustContain(t, string(body2), `<html lang="ko">`, `<option value="ko" lang="ko" selected>한국어</option>`)
	if got := r.Header.Get("Content-Language"); got != "ko" {
		t.Errorf("Content-Language %q", got)
	}

	// "Auto" forgets the pick; another site's page is not gone back to.
	resp = pick("", "https://example.com/")
	if loc := resp.Header.Get("Location"); loc != "/" {
		t.Errorf("location %q", loc)
	}
	if c := resp.Cookies(); len(c) != 1 || c[0].MaxAge >= 0 {
		t.Errorf("cookies %v", c)
	}
	if loc := pick("en", ts.URL+"//example.com/").Header.Get("Location"); loc != "/" {
		t.Errorf("location %q", loc)
	}
}
