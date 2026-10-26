package web

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/cenoda/gscouter/internal/enka"
	"github.com/cenoda/gscouter/internal/library"
	"github.com/cenoda/gscouter/internal/score"
	"github.com/cenoda/gscouter/teams"
)

type fakeProfiles map[string]*enka.Profile

func (f fakeProfiles) Profile(_ context.Context, uid string) (*enka.Profile, error) {
	if p, ok := f[uid]; ok {
		return p, nil
	}
	return nil, enka.ErrNotFound
}

func char(key, weapon string, set string, stats map[string]float64) enka.Character {
	return enka.Character{
		Key: key, Level: 90, MaxLevel: 90, Talents: [3]int{9, 9, 9},
		Weapon: enka.Weapon{Key: weapon, Refine: 1, Level: 90, MaxLevel: 90},
		Sets:   map[string]int{set: 4},
		Stats:  stats,
	}
}

func hutaoTeam() []enka.Character {
	return []enka.Character{
		char("hutao", "staffofhoma", "crimsonwitchofflames", map[string]float64{"hp": 4780, "atk": 311, "hp%": 0.6, "pyro%": 0.466, "cr": 0.65, "cd": 0.8}),
		char("yelan", "favoniuswarbow", "emblemofseveredfate", map[string]float64{"hp": 4780, "atk": 311, "hp%": 0.6, "hydro%": 0.466, "er": 0.6, "cr": 0.6, "cd": 0.8}),
		char("xingqiu", "sacrificialsword", "emblemofseveredfate", map[string]float64{"hp": 4780, "atk": 311, "atk%": 0.6, "hydro%": 0.466, "er": 0.3, "cr": 0.6, "cd": 0.8}),
		char("zhongli", "blacktassel", "tenacityofthemillelith", map[string]float64{"hp": 4780, "atk": 311, "hp%": 1.4}),
	}
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	lib, err := library.LoadLibrary(teams.FS)
	if err != nil {
		t.Fatal(err)
	}
	strong := hutaoTeam()
	weak := hutaoTeam()
	weak[0].Stats = map[string]float64{"hp": 4780, "atk": 311, "cr": 0.1, "cd": 0.3}
	profiles := fakeProfiles{
		"800000001": {UID: "800000001", Nickname: "Strong", Characters: append(strong, char("amber", "skywardharp", "noblesseoblige", nil))},
		"800000002": {UID: "800000002", Nickname: "Weak", Characters: weak},
	}
	srv, err := New(lib, profiles, Settings{Iterations: 4, Warmup: 30, Duration: 90, MaxConcurrent: 1, MaxQueue: 4})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts
}

func get(t *testing.T, ts *httptest.Server, path string) (int, string) {
	t.Helper()
	resp, err := ts.Client().Get(ts.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

func mustContain(t *testing.T, body string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(body, w) {
			t.Errorf("response does not contain %q", w)
		}
	}
}

func TestIndexSuggestsTeams(t *testing.T) {
	ts := newTestServer(t)
	code, body := get(t, ts, "/?uid=800000001")
	if code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	mustContain(t, body, "Strong", "바로 계산할 수 있는 파티", "Hutao / Xingqiu / Zhongli / Yelan", "amber")
}

func TestTeamsFilter(t *testing.T) {
	ts := newTestServer(t)
	code, body := get(t, ts, "/teams?q=hutao")
	if code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	mustContain(t, body, "Hutao / Xingqiu / Zhongli / Yelan")
	if strings.Contains(body, "Skirk") {
		t.Error("team without the character listed")
	}
}

func TestIndexUnknownUID(t *testing.T) {
	ts := newTestServer(t)
	_, body := get(t, ts, "/?uid=800000009")
	mustContain(t, body, "플레이어를 찾을 수 없습니다")
}

func TestScoreFromPicker(t *testing.T) {
	ts := newTestServer(t)
	// The picker form redirects to a shareable URL, which the client follows.
	code, body := get(t, ts, "/score?uid=800000001&c=hutao&c=yelan&c=xingqiu&c=zhongli")
	if code != http.StatusOK {
		t.Fatalf("status %d: %s", code, body)
	}
	mustContain(t, body, "DPS", "hutao", "staffofhoma", "검증 전", "https://enka.network/ui/UI_AvatarIcon_Hutao.png")
	// Link previews show the score and the members.
	mustContain(t, body, `<meta property="og:title" content="Strong의 파티 `, `<meta property="og:description" content="hutao C0 `,
		`<meta property="og:image" content="http://127.0.0.1:`)
	i := strings.Index(body, `og:image" content="`)
	card := strings.ReplaceAll(body[i+len(`og:image" content="`):][:strings.Index(body[i+len(`og:image" content="`):], `"`)], "&amp;", "&")
	resp, err := ts.Client().Get(card)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "image/png" {
		t.Errorf("card %s: status %d, type %q", card, resp.StatusCode, resp.Header.Get("Content-Type"))
	}
}

func TestScoreUnsupportedTeam(t *testing.T) {
	ts := newTestServer(t)
	_, body := get(t, ts, "/score?a="+url.QueryEscape("800000001:hutao,yelan,xingqiu,amber"))
	mustContain(t, body, "아직 지원하지 않는 파티")
}

func TestScoreMissingCharacter(t *testing.T) {
	ts := newTestServer(t)
	_, body := get(t, ts, "/score?a="+url.QueryEscape("800000002:hutao,yelan,xingqiu,amber"))
	mustContain(t, body, "amber 캐릭터가 쇼케이스에 없습니다")
}

func TestScoreRejectsWrongCount(t *testing.T) {
	ts := newTestServer(t)
	code, body := get(t, ts, "/score?uid=800000001&c=skirk&c=yelan")
	if code != http.StatusBadRequest {
		t.Errorf("status %d, want 400", code)
	}
	mustContain(t, body, "정확히 4명")
}

func TestVersus(t *testing.T) {
	ts := newTestServer(t)
	v := url.Values{
		"a": {"800000001:hutao,yelan,xingqiu,zhongli"},
		"b": {"800000002:zhongli,xingqiu,yelan,hutao"},
	}
	code, body := get(t, ts, "/versus?"+v.Encode())
	if code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	mustContain(t, body, "왼쪽 파티가", "높습니다", "Strong", "Weak", `href="#el-pyro"`,
		`<meta property="og:title" content="Strong vs Weak: Strong 파티가 `)

	v.Set("b", v.Get("a"))
	_, body = get(t, ts, "/versus?"+v.Encode())
	mustContain(t, body, "비슷합니다")
}

func TestWarningsKnownIssues(t *testing.T) {
	got := strings.Join(warnings(&library.Team{Members: []string{"arlecchino", "bennett"}}, &score.Result{}), "\n")
	if !strings.Contains(got, "아를레키노") || !strings.Contains(got, "20%") {
		t.Errorf("no Arlecchino note in %q", got)
	}
	if got := warnings(&library.Team{Members: []string{"hutao"}}, &score.Result{}); len(got) != 0 {
		t.Errorf("unexpected warnings %q", got)
	}
}
