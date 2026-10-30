package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/cenoda/gscouter/internal/enka"
	"github.com/cenoda/gscouter/internal/library"
	"github.com/cenoda/gscouter/teams"
)

// savedProfiles is fakeProfiles whose players saved builds on Enka.
type savedProfiles struct {
	fakeProfiles
	saved map[enka.Owner][]enka.SavedBuild
	err   error
	calls int
}

func (s *savedProfiles) SavedBuilds(_ context.Context, owner enka.Owner) ([]enka.SavedBuild, error) {
	s.calls++
	return s.saved[owner], s.err
}

// newSavedSite is a site where player 800000006, whose showcase lacks
// Zhongli, saved two Zhongli builds and a Hu Tao one on Enka.
func newSavedSite(t *testing.T) (*savedProfiles, *Server) {
	t.Helper()
	lib, err := library.LoadLibrary(teams.FS)
	if err != nil {
		t.Fatal(err)
	}
	owner := enka.Owner{Username: "player", Hash: "h4sh"}
	team := hutaoTeam()
	tank, old := team[3], team[3]
	old.Weapon.Key = "favoniuslance"
	c6 := team[0]
	c6.Cons = 6
	src := &savedProfiles{
		fakeProfiles: fakeProfiles{
			"800000006": {UID: "800000006", Nickname: "Saver", Characters: team[:3], Owner: &owner},
			"800000003": {UID: "800000003", Nickname: "Moved", Characters: team[:3]},
		},
		// In enka.ParseBuilds's order: by key, the player's order first.
		saved: map[enka.Owner][]enka.SavedBuild{owner: {
			{ID: 3, Name: "C6", Character: c6},
			{ID: 2, Name: "Tank", Character: tank},
			{ID: 1, Name: "", Character: old},
		}},
	}
	srv, err := New(lib, src, Settings{Iterations: 4, Warmup: 30, Duration: 90, MaxConcurrent: 1, MaxQueue: 4})
	if err != nil {
		t.Fatal(err)
	}
	return src, srv
}

func TestSavedBuilds(t *testing.T) {
	src, srv := newSavedSite(t)
	ts := newTestServerFor(t, srv)

	// The front page asks for the card again when the player has an
	// account; one without does not.
	_, body := get(t, ts, "/?uid=800000006")
	mustContain(t, body, `id="showcase" data-saved>`)
	if _, body := get(t, ts, "/?uid=800000003"); strings.Contains(body, `id="showcase" data-saved`) {
		t.Error("data-saved without an Enka account")
	}
	if src.calls != 0 {
		t.Errorf("the front page fetched saved builds %d times", src.calls)
	}

	form := url.Values{"uid": {"800000006"}, "kept": {"[]"}}
	code, body := post(t, ts, "/showcase", form)
	if code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	team := hutaoTeam()
	c6 := team[0]
	c6.Cons = 6
	old := team[3]
	old.Weapon.Key = "favoniuslance"
	mustContain(t, body,
		"Hu Tao / Xingqiu / Zhongli / Yelan",
		// Zhongli comes with his first saved build, the other to choose.
		`data-kept="`+buildToken(team[3])+`"`, "Saved on Enka",
		`<option value="" data-line="Lv.90 C0 · Black Tassel R1">Tank</option>`,
		`<option value="`+buildToken(old)+`" data-line="Lv.90 C0 · Favonius Lance R1">Unnamed build</option>`,
		// Hu Tao stays the showcase's, with her saved build to choose.
		`data-build="`+buildToken(team[0])+`"`,
		`<option value="" data-line="Lv.90 C0 · Staff of Homa R1">Showcase</option>`,
		`<option value="`+buildToken(c6)+`" data-line="Lv.90 C6 · Staff of Homa R1">C6</option>`,
		"This player saved builds on Enka.Network")
	if strings.Contains(body, "forget-kept") {
		t.Error("saved builds are offered to forget like kept ones")
	}

	// A kept build of a character comes before his saved ones.
	kept := team[3]
	kept.Cons = 2
	form.Set("kept", `[{"b":"`+buildToken(kept)+`","t":1767225600000}]`)
	_, body = post(t, ts, "/showcase", form)
	mustContain(t, body, `data-kept="`+buildToken(kept)+`"`,
		`<option value="" data-line="Lv.90 C2 · Black Tassel R1">As of 2026-01-01</option>`,
		`<option value="`+buildToken(team[3])+`" data-line="Lv.90 C0 · Black Tassel R1">Tank</option>`)

	// Picking Hu Tao's saved build sends it; it wins over the showcase's
	// and the result link carries it.
	pick := "/score?uid=800000006&c=hutao&c=yelan&c=xingqiu&c=zhongli&build=" + buildToken(team[3]) + "&build=" + buildToken(c6)
	resp := noRedirects(t, ts, ts.URL+pick)
	loc := resp.Header.Get("Location")
	q, _ := url.ParseQuery(loc[strings.Index(loc, "?")+1:])
	sel, err := parseSelection(q.Get("a"))
	if err != nil {
		t.Fatalf("location %q: %v", loc, err)
	}
	if sel.Builds[0].Key != "hutao" || sel.Builds[0].Cons != 6 || sel.Builds[3].Config() != team[3].Config() {
		t.Errorf("builds %+v", sel.Builds)
	}

	// Enka failing leaves the card as it was.
	src.err = errors.New("enka down")
	form.Set("kept", "[]")
	code, body = post(t, ts, "/showcase", form)
	if code != http.StatusOK || strings.Contains(body, "Saved on Enka") || strings.Contains(body, "<select") {
		t.Errorf("status %d with Enka failing:\n%s", code, body)
	}
	mustContain(t, body, "Saver", "No library team")
}

func TestSavedBuildsCached(t *testing.T) {
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		if strings.Contains(r.URL.Path, "/fail/") {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	e := NewEnkaSource(&enka.Client{HTTP: srv.Client(), BaseURL: srv.URL})
	for range 2 {
		if _, err := e.SavedBuilds(context.Background(), enka.Owner{Username: "ok", Hash: "h"}); err != nil {
			t.Fatal(err)
		}
		if _, err := e.SavedBuilds(context.Background(), enka.Owner{Username: "x", Hash: "fail"}); err == nil {
			t.Error("no error")
		}
	}
	if n != 2 {
		t.Errorf("%d requests to Enka, want 2", n)
	}
}
