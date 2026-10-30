package enka

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// The testdata are Enka.Network's own example account (API docs),
// recorded 2026-10-30 and trimmed to a few characters.

func TestParseOwner(t *testing.T) {
	raw, err := os.ReadFile("testdata/owner.json")
	if err != nil {
		t.Fatal(err)
	}
	p, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if p.Owner == nil || *p.Owner != (Owner{Username: "Algoinde", Hash: "4Wjv2e"}) {
		t.Errorf("owner = %+v", p.Owner)
	}
	if len(p.Characters) != 1 || p.Characters[0].Key != "amber" {
		t.Errorf("characters = %+v", p.Characters)
	}

	raw, _ = os.ReadFile("testdata/skirk.json")
	if p, _ := Parse(raw); p.Owner != nil {
		t.Errorf("owner without an account: %+v", p.Owner)
	}
}

func TestParseBuilds(t *testing.T) {
	raw, err := os.ReadFile("testdata/builds.json")
	if err != nil {
		t.Fatal(err)
	}
	builds, err := ParseBuilds(raw)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, b := range builds {
		got = append(got, b.Character.Key+":"+b.Name)
	}
	// The player's order first, then the newest of equal order.
	want := []string{
		"amber:Full Weakspot AC",
		"amber:Snoipe 'em head off",
		"amber:Pyro Melt Snipe DPS",
		"amber:????",
		"ganyu:somewhat dps",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("builds\n%q\nwant\n%q", got, want)
	}
	// A saved build is a showcase entry: the converter reads it as one.
	amber := builds[0].Character
	if amber.Level != 90 || amber.Weapon.Key == "" || len(amber.Sets) == 0 || len(amber.Stats) == 0 {
		t.Errorf("amber = %+v", amber)
	}
	if builds[0].Order != -1 || builds[0].ID != 940318 {
		t.Errorf("first build = %d, order %v", builds[0].ID, builds[0].Order)
	}

	// Live builds are the showcase, and private ones are not shown.
	live := strings.Replace(string(raw), `"live": false`, `"live": true`, 1)
	if b, _ := ParseBuilds([]byte(live)); len(b) != len(builds)-1 {
		t.Errorf("with a live build: %d builds, want %d", len(b), len(builds)-1)
	}
	private := strings.Replace(string(raw), `"public": true`, `"public": false`, 1)
	if b, _ := ParseBuilds([]byte(private)); len(b) != len(builds)-1 {
		t.Errorf("with a private build: %d builds, want %d", len(b), len(builds)-1)
	}
	if b, err := ParseBuilds([]byte(`{}`)); err != nil || len(b) != 0 {
		t.Errorf("no builds: %v, %v", b, err)
	}
}

func TestFetchBuilds(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/profile/Algoinde/hoyos/4Wjv2e/builds/" {
			http.NotFound(w, r)
			return
		}
		if !strings.HasPrefix(r.Header.Get("User-Agent"), "Gscouter/") {
			t.Errorf("User-Agent = %q", r.Header.Get("User-Agent"))
		}
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	c := &Client{HTTP: srv.Client(), BaseURL: srv.URL}
	if b, err := c.FetchBuilds(context.Background(), Owner{"Algoinde", "4Wjv2e"}); err != nil || string(b) != "{}" {
		t.Errorf("got %q, %v", b, err)
	}
	if _, err := c.FetchBuilds(context.Background(), Owner{"Algoinde", "other"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown hash: %v", err)
	}
	if _, err := c.FetchBuilds(context.Background(), Owner{"../uid", "4Wjv2e"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("path in a username: %v", err)
	}
}
