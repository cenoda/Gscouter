package web

import (
	"encoding/json"
	"errors"
	"html"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/cenoda/gscouter/internal/enka"
)

// goodBuilds are the committed GOOD fixture's characters by key.
func goodBuilds(t *testing.T) map[string]enka.Character {
	t.Helper()
	raw, err := os.ReadFile("../enka/testdata/good.json")
	if err != nil {
		t.Fatal(err)
	}
	chars, _, err := enka.ParseGOOD(raw)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]enka.Character{}
	for _, c := range chars {
		out[c.Key] = c
	}
	return out
}

func isTravelerError(err error) bool {
	var e inputError
	return errors.As(err, &e) && e.key == "err.traveler"
}

// A party holds one Traveler: two of its elements, or both genders, are
// refused whether the link names the characters or carries their builds;
// one Traveler is a member like any other.
func TestOneTraveler(t *testing.T) {
	builds := goodBuilds(t)
	pickBuilds := func(keys ...string) []enka.Character {
		var out []enka.Character
		for _, k := range keys {
			c, ok := builds[k]
			if !ok {
				t.Fatalf("the GOOD fixture has no %s", k)
			}
			out = append(out, c)
		}
		return out
	}

	twins := pickBuilds("hutao", "xingqiu", "aetheranemo", "aethergeo")
	if _, err := newSelection("800000001", keys(twins)); !isTravelerError(err) {
		t.Errorf("newSelection of two GOOD Travelers: %v", err)
	}
	packed := selection{UID: "800000001", Nickname: "GOOD", Builds: twins}.String()
	if _, err := parseSelection(packed); !isTravelerError(err) {
		t.Errorf("packed link with two Travelers: %v", err)
	}
	for _, s := range []string{
		"800000001:hutao,xingqiu,aetheranemo,aethergeo",
		"800000001:hutao,xingqiu,aetheranemo,lumineanemo",
		"800000001:hutao,xingqiu,luminecryo,aethergeo",
		"800000001:hutao,xingqiu,travelercryo,aethercryo",
	} {
		if _, err := parseSelection(s); !isTravelerError(err) {
			t.Errorf("%s: %v", s, err)
		}
	}
	// Exact duplicates stay the generic error.
	if _, err := parseSelection("800000001:hutao,hutao,xingqiu,yelan"); err == nil || isTravelerError(err) {
		t.Errorf("duplicate Hu Tao: %v", err)
	}

	// One Traveler keeps its element.
	one := pickBuilds("hutao", "xingqiu", "yelan", "aethergeo")
	sel, err := parseSelection(selection{UID: "800000001", Nickname: "GOOD", Builds: one}.String())
	if err != nil {
		t.Fatalf("one GOOD Traveler: %v", err)
	}
	if want := []string{"hutao", "xingqiu", "yelan", "aethergeo"}; !slices.Equal(sel.Chars, want) {
		t.Errorf("chars %v, want %v", sel.Chars, want)
	}
	for _, s := range []string{"800000001:hutao,xingqiu,yelan,lumineanemo", "800000001:xingqiu,yelan,zhongli,aetheranemo"} {
		if _, err := parseSelection(s); err != nil {
			t.Errorf("%s: %v", s, err)
		}
	}

	// The pages say why.
	ts := newTestServer(t)
	for _, path := range []string{
		"/edit?a=" + url.QueryEscape(packed),
		"/score?a=" + url.QueryEscape(packed) + "&r=" + url.QueryEscape("0N 1E 2E 3E"),
		"/score?uid=800000001&c=hutao&c=xingqiu&c=aetheranemo&c=lumineanemo",
	} {
		_, body := get(t, ts, path)
		mustContain(t, html.UnescapeString(body), english.T("err.traveler"))
	}
}

// The picker refuses a second Traveler as the server does, and says why
// (testdata/picker_traveler.js).
func TestPickerTraveler(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not found")
	}
	srv := newTestSite(t)
	profiles := srv.profiles.(fakeProfiles)
	profiles["800000010"] = &enka.Profile{UID: "800000010", Nickname: "Twins", Characters: append(hutaoTeam()[:3],
		char("aetheranemo", "freedomsworn", "viridescentvenerer", nil),
		char("aethergeo", "freedomsworn", "viridescentvenerer", nil),
		char("luminecryo", "freedomsworn", "viridescentvenerer", nil))}
	ts := newTestServerFor(t, srv)
	_, body := get(t, ts, "/?uid=800000010")
	mustContain(t, body, `id="pick-traveler"`)

	i := strings.Index(body, `"gscouter.recentUIDs"`)
	if i < 0 {
		t.Fatal("no showcase script")
	}
	i = strings.LastIndex(body[:i], "<script>") + len("<script>")
	end := strings.Index(body[i:], "</script>")
	if end < 0 {
		t.Fatal("showcase script not closed")
	}
	page := filepath.Join(t.TempDir(), "page.js")
	if err := os.WriteFile(page, []byte(body[i:i+end]), 0o644); err != nil {
		t.Fatal(err)
	}
	var in struct {
		Keys      []string `json:"keys"`
		Buildable []string `json:"buildable"`
	}
	for _, m := range regexp.MustCompile(`name="c" value="([^"]+)"`).FindAllStringSubmatch(body, -1) {
		in.Keys = append(in.Keys, m[1])
	}
	m := regexp.MustCompile(`data-buildable="([^"]*)"`).FindStringSubmatch(body)
	if m == nil {
		t.Fatal("no data-buildable")
	}
	if err := json.Unmarshal([]byte(html.UnescapeString(m[1])), &in.Buildable); err != nil {
		t.Fatal(err)
	}
	arg, _ := json.Marshal(in)
	b, err := exec.Command(node, filepath.Join("testdata", "picker_traveler.js"), page, string(arg)).Output()
	if err != nil {
		t.Fatalf("node: %v", err)
	}
	var out struct{ Errors []string }
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("%v: %s", err, b)
	}
	for _, e := range out.Errors {
		t.Error(e)
	}
}
