package web

import (
	"net/http"
	"strings"
	"testing"
)

func TestAboutAndUpdates(t *testing.T) {
	ts := newTestServer(t)

	// Every page's footer links both, the updates by their newest date.
	_, body := get(t, ts, "/")
	mustContain(t, body, `<a href="/about">Our promises</a>`, `<a href="/updates">Updated `+updates[0].Date+`</a>`)

	code, body := get(t, ts, "/about")
	if code != http.StatusOK {
		t.Fatalf("about: status %d", code)
	}
	mustContain(t, body, "<h1>Our promises</h1>", `<link rel="canonical" href="`)
	_, body = getKo(t, ts, "/about")
	mustContain(t, body, "<h1>우리의 약속</h1>")

	// Entries are in English, or Korean for Korean.
	code, body = get(t, ts, "/updates")
	if code != http.StatusOK {
		t.Fatalf("updates: status %d", code)
	}
	mustContain(t, body, "<h2>"+updates[0].Date, updates[0].En[0])
	_, body = getKo(t, ts, "/updates")
	mustContain(t, body, "<h1>업데이트 내역</h1>", updates[0].Ko[0])
	_, body = getLang(t, ts, "/updates", "ja")
	mustContain(t, body, "<h1>更新履歴</h1>", updates[0].En[0])
}

func TestUpdatesWellFormed(t *testing.T) {
	for i, u := range updates {
		if len(u.Date) != len("2006-01-02") || len(u.En) == 0 || len(u.En) != len(u.Ko) {
			t.Errorf("update %s: %d English lines, %d Korean", u.Date, len(u.En), len(u.Ko))
		}
		if i > 0 && strings.Compare(u.Date, updates[i-1].Date) >= 0 {
			t.Errorf("update %s is not older than %s", u.Date, updates[i-1].Date)
		}
	}
}
