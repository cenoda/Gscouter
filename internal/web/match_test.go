package web

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cenoda/gscouter/internal/enka"
	"github.com/cenoda/gscouter/internal/library"
	"github.com/cenoda/gscouter/teams"
)

func TestMatch(t *testing.T) {
	team := func(members ...string) *library.Team {
		return &library.Team{ID: strings.Join(members, "-"), Members: members}
	}
	teams := []*library.Team{
		team("hutao", "xingqiu", "yelan", "zhongli"),
		team("hutao", "yelan", "zhongli", "xingqiu"), // a variant of the same four
		team("xingqiu", "yelan", "zhongli", "lumineanemo"),
		// Aliases and the Traveler fold into canonical keys.
		team("hutao", "xingqiu", "yelan", "aetheranemo"),
		team("hutao", "xingqiu", "yelan", "kazuha"),
		team("hutao", "yelan", "kaedeharakazuha", "aetheranemo"),
		team("hutao", "xingqiu", "yelan", "travelercryo"),
		team("hutao", "xingqiu", "yelan", "furina"),
		team("xingqiu", "yelan", "lumineanemo", "bennett"),
		team("hutao", "xingqiu", "lumineanemo", "xiangling"),
		team("hutao", "xingqiu", "zhongli", "furina"), // two short
		team("hutao", "yelan", "xingqiu"),             // not four
	}
	p := &enka.Profile{UID: "800000001", Characters: []enka.Character{
		{Key: "hutao"}, {Key: "xingqiu"}, {Key: "yelan"}, {Key: "lumineanemo"},
	}}
	got, unlocks := match(teams, canonicalMembers(teams), p)
	var ids []string
	for _, s := range got {
		ids = append(ids, s.Team.ID)
	}
	if want := []string{"hutao-xingqiu-yelan-aetheranemo"}; !slices.Equal(ids, want) {
		t.Fatalf("suggested %q, want %q", ids, want)
	}
	if c := got[0].Selection.Chars; c[3] != "lumineanemo" {
		t.Errorf("selection %v, want the showcase's Traveler", c)
	}
	// By teams completed, then by teams left one short (Furina's two-short
	// team), then by key; five at most (Xiangling is left out).
	want := []unlock{{"zhongli", 3, 1}, {"kaedeharakazuha", 2, 0}, {"furina", 1, 1}, {"bennett", 1, 0}, {"travelercryo", 1, 0}}
	if !slices.Equal(unlocks, want) {
		t.Errorf("unlocks %v, want %v", unlocks, want)
	}
}

// TestMatchLibrary runs match over the whole library: every unlock
// completes the teams it says, and the ranking is by teams completed.
func TestMatchLibrary(t *testing.T) {
	srv := newTestSite(t)
	p := &enka.Profile{UID: "800000001", Characters: hutaoTeam()[:3]}
	_, unlocks := match(srv.lib.Teams, srv.members, p)
	if len(unlocks) != maxUnlocks {
		t.Fatalf("%d unlocks, want %d", len(unlocks), maxUnlocks)
	}
	for i, u := range unlocks {
		with := &enka.Profile{UID: p.UID, Characters: append(hutaoTeam()[:3], enka.Character{Key: u.Key})}
		got, _ := match(srv.lib.Teams, srv.members, with)
		if len(got) != u.Teams {
			t.Errorf("adding %s builds %d teams, unlock says %d", u.Key, len(got), u.Teams)
		}
		if i > 0 && (u.Teams > unlocks[i-1].Teams || u.Teams == unlocks[i-1].Teams && u.Near > unlocks[i-1].Near) {
			t.Errorf("unlocks not ranked: %v", unlocks)
		}
	}
}

func BenchmarkMatch(b *testing.B) {
	lib, err := library.LoadLibrary(teams.FS)
	if err != nil {
		b.Fatal(err)
	}
	p := &enka.Profile{UID: "800000001", Characters: append(hutaoTeam(), lohenTeam()...)}
	members := canonicalMembers(lib.Teams)
	b.ResetTimer()
	for range b.N {
		match(lib.Teams, members, p)
	}
}

func TestIndexUnlocksAndRefresh(t *testing.T) {
	srv := newTestSite(t)
	srv.profiles.(fakeProfiles)["800000009"] = &enka.Profile{
		UID: "800000009", Nickname: "Fresh", Characters: hutaoTeam()[:3],
		Expires: time.Now().Add(47 * time.Second),
	}
	_, unlocks := match(srv.lib.Teams, srv.members, srv.profiles.(fakeProfiles)["800000009"])
	if len(unlocks) == 0 {
		t.Fatal("no unlocks")
	}
	first := unlocks[0]
	ts := newTestServerFor(t, srv)
	_, body := get(t, ts, "/?uid=800000009")
	mustContain(t, body, "Add to your showcase next",
		fmt.Sprintf(`<a href="/characters/%s">`, first.Key), fmt.Sprintf("%s</span><b>+%d</b>", english.charName(first.Key), first.Teams),
		"Changed your showcase?", "Reload showcase")
	var wait int
	if i := strings.Index(body, `data-wait="`); i < 0 {
		t.Error("no countdown")
	} else if _, err := fmt.Sscanf(body[i+len(`data-wait="`):], "%d", &wait); err != nil || wait < 45 || wait > 47 {
		t.Errorf("countdown from %d s (%v), want about 47", wait, err)
	}
	_, body = getKo(t, ts, "/?uid=800000009")
	mustContain(t, body, "다음에 쇼케이스에 넣을 캐릭터", localizer{Lang: langKorean}.charName(first.Key)+"</span><b>+", "쇼케이스를 바꾸셨나요?")

	// Without an expiry there is no countdown; when picking an opponent,
	// whose showcase is someone else's, no hints.
	_, body = get(t, ts, "/?uid=800000003&vs=800000001:hutao,xingqiu,yelan,zhongli")
	if strings.Contains(body, `id="refresh"`) || strings.Contains(body, "Add to your showcase next") {
		t.Error("countdown without an expiry, or hints for an opponent")
	}
	if code, _ := get(t, ts, "/?uid=800000003"); code != http.StatusOK {
		t.Errorf("status %d", code)
	}
}
