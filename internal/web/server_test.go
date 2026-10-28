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
	return newTestServerFor(t, newTestSite(t))
}

func newTestServerFor(t *testing.T, srv *Server) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts
}

// newTestSite is the site with two players, scoring with few iterations.
func newTestSite(t *testing.T) *Server {
	t.Helper()
	return newTestSiteWith(t, nil)
}

// newTestSiteWith is newTestSite with edit applied to the library first.
func newTestSiteWith(t *testing.T, edit func(*library.Library)) *Server {
	t.Helper()
	lib, err := library.LoadLibrary(teams.FS)
	if err != nil {
		t.Fatal(err)
	}
	if edit != nil {
		edit(lib)
	}
	strong := hutaoTeam()
	weak := hutaoTeam()
	weak[0].Stats = map[string]float64{"hp": 4780, "atk": 311, "cr": 0.1, "cd": 0.3}
	// Xingqiu with a weapon only the gcsim fork has.
	hymn := hutaoTeam()
	hymn[2].Weapon.Key = "hymnofthemaelstrom"
	profiles := fakeProfiles{
		"800000001": {UID: "800000001", Nickname: "Strong", Characters: append(strong, char("amber", "skywardharp", "noblesseoblige", nil))},
		"800000002": {UID: "800000002", Nickname: "Weak", Characters: weak},
		// Zhongli has left this showcase.
		"800000003": {UID: "800000003", Nickname: "Moved", Characters: hutaoTeam()[:3]},
		"800000004": {UID: "800000004", Nickname: "Hymn", Characters: hymn},
	}
	srv, err := New(lib, profiles, Settings{Iterations: 4, Warmup: 30, Duration: 90, MaxConcurrent: 1, MaxQueue: 4})
	if err != nil {
		t.Fatal(err)
	}
	return srv
}

// get fetches path in English, the language of browsers the site does
// not speak.
func get(t *testing.T, ts *httptest.Server, path string) (int, string) {
	t.Helper()
	return getLang(t, ts, path, "")
}

// getKo fetches path from a Korean browser.
func getKo(t *testing.T, ts *httptest.Server, path string) (int, string) {
	t.Helper()
	return getLang(t, ts, path, "ko-KR,ko;q=0.9,en;q=0.8")
}

func getLang(t *testing.T, ts *httptest.Server, path, acceptLanguage string) (int, string) {
	t.Helper()
	req, err := http.NewRequest("GET", ts.URL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if acceptLanguage != "" {
		req.Header.Set("Accept-Language", acceptLanguage)
	}
	resp, err := ts.Client().Do(req)
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
	code, body := getKo(t, ts, "/?uid=800000001")
	if code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	mustContain(t, body, "Strong", "바로 계산할 수 있는 파티", "호두 / 행추 / 종려 / 야란", "엠버", "호마의 지팡이")
	// The picker only submits four that form a suggested team.
	mustContain(t, body, `data-buildable="[&#34;hutao,xingqiu,yelan,zhongli&#34;]"`, "라이브러리에 있는 파티가 아니어서")
}

func TestTeamsFilter(t *testing.T) {
	ts := newTestServer(t)
	code, body := get(t, ts, "/teams?q=hutao")
	if code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	mustContain(t, body, "Hu Tao / Xingqiu / Zhongli / Yelan")
	if strings.Contains(body, "Skirk") {
		t.Error("team without the character listed")
	}
	// In-game names find teams too, in the page's language and English.
	for _, q := range []string{"hu tao", "호두 야란", "胡桃"} {
		lang := map[string]string{"hu tao": "", "호두 야란": "ko", "胡桃": "ja"}[q]
		_, body := getLang(t, ts, "/teams?q="+url.QueryEscape(q), lang)
		if !strings.Contains(body, "Xingqiu") && !strings.Contains(body, "행추") && !strings.Contains(body, "行秋") {
			t.Errorf("%q (%s) finds no Hu Tao team", q, lang)
		}
	}
}

func TestIndexUnknownUID(t *testing.T) {
	ts := newTestServer(t)
	_, body := get(t, ts, "/?uid=800000009")
	mustContain(t, body, "No player has this UID")
	_, body = getKo(t, ts, "/?uid=800000009")
	mustContain(t, body, "플레이어를 찾을 수 없습니다")
}

func TestScoreFromPicker(t *testing.T) {
	ts := newTestServer(t)
	// The picker form redirects to a shareable URL, which the client follows.
	code, body := getKo(t, ts, "/score?uid=800000001&c=hutao&c=yelan&c=xingqiu&c=zhongli")
	if code != http.StatusOK {
		t.Fatalf("status %d: %s", code, body)
	}
	mustContain(t, body, "DPS", "호두", "호마의 지팡이", "https://enka.network/ui/UI_AvatarIcon_Hutao.png")
	// The library's Hu Tao team is supported: no draft warning.
	if strings.Contains(body, "검증 전") {
		t.Error("draft warning on a supported team")
	}
	// Link previews show the score and the members.
	mustContain(t, body, `<meta property="og:title" content="Strong의 파티 `, `<meta property="og:description" content="호두 C0 `,
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
	_, body := getKo(t, ts, "/score?a="+url.QueryEscape("800000001:hutao,yelan,xingqiu,amber"))
	mustContain(t, body, "아직 지원하지 않는 파티")
}

func TestScoreMissingCharacter(t *testing.T) {
	ts := newTestServer(t)
	_, body := getKo(t, ts, "/score?a="+url.QueryEscape("800000002:hutao,yelan,xingqiu,amber"))
	mustContain(t, body, "엠버 캐릭터가 쇼케이스에 없습니다")
}

func TestScoreRejectsWrongCount(t *testing.T) {
	ts := newTestServer(t)
	code, body := get(t, ts, "/score?uid=800000001&c=skirk&c=yelan")
	if code != http.StatusBadRequest {
		t.Errorf("status %d, want 400", code)
	}
	mustContain(t, body, "Pick exactly four characters (2 picked)")
	_, body = getKo(t, ts, "/score?uid=800000001&c=skirk&c=yelan")
	mustContain(t, body, "정확히 4명")
}

func TestVersus(t *testing.T) {
	ts := newTestServer(t)
	v := url.Values{
		"a": {"800000001:hutao,yelan,xingqiu,zhongli"},
		"b": {"800000002:zhongli,xingqiu,yelan,hutao"},
	}
	code, body := getKo(t, ts, "/versus?"+v.Encode())
	if code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	mustContain(t, body, "왼쪽 파티가", "높습니다", "Strong", "Weak", `href="#el-pyro"`,
		`<meta property="og:title" content="Strong vs Weak: Strong 파티가 `)

	v.Set("b", v.Get("a"))
	_, body = getKo(t, ts, "/versus?"+v.Encode())
	mustContain(t, body, "비슷합니다")
	_, body = get(t, ts, "/versus?"+v.Encode())
	mustContain(t, body, "About even", "Copy comparison link")
}

func TestWarningsDraft(t *testing.T) {
	got := warnings(localizer{Lang: langKorean}, &library.Team{Members: []string{"hutao"}, Status: library.StatusDraft}, nil, &score.Result{})
	if len(got) != 1 || !strings.Contains(got[0], "검증 전") {
		t.Errorf("draft team warnings %q", got)
	}
	if got := warnings(english, &library.Team{Members: []string{"hutao"}, Status: library.StatusSupported}, nil, &score.Result{}); len(got) != 0 {
		t.Errorf("supported team warnings %q", got)
	}
}

func TestWarningsKnownIssues(t *testing.T) {
	got := strings.Join(warnings(localizer{Lang: langKorean}, &library.Team{Members: []string{"arlecchino", "bennett"}}, nil, &score.Result{}), "\n")
	if !strings.Contains(got, "아를레키노") || !strings.Contains(got, "20%") {
		t.Errorf("no Arlecchino note in %q", got)
	}
	if got := warnings(english, &library.Team{Members: []string{"hutao"}}, nil, &score.Result{}); len(got) != 0 {
		t.Errorf("unexpected warnings %q", got)
	}
}

func TestExperimentalBadge(t *testing.T) {
	ts := httptest.NewServer(newTestSiteWith(t, func(lib *library.Library) {
		team := lib.Find([]string{"hutao", "yelan", "xingqiu", "zhongli"})
		team.Tier = library.TierExperimental
		team.Reference = nil // written from a guide: every member is upstream
	}).Handler())
	t.Cleanup(ts.Close)
	badge := `<span class="badge">Experimental</span>`
	_, body := get(t, ts, "/teams?q=hutao")
	mustContain(t, body, "Hu Tao / Xingqiu / Zhongli / Yelan</a> "+badge)
	if strings.Count(body, badge) != 1 {
		t.Errorf("%d badges on /teams?q=hutao, want only the experimental team's", strings.Count(body, badge))
	}
	_, body = get(t, ts, "/score?server=1&a="+url.QueryEscape("800000001:hutao,yelan,xingqiu,zhongli"))
	mustContain(t, body, badge, "was written from a community guide")
	if strings.Contains(body, "is not in gcsim yet") {
		t.Error("fork-only warning on a team of upstream characters")
	}

	_, body = get(t, newTestServer(t), "/score?server=1&a="+url.QueryEscape("800000001:hutao,yelan,xingqiu,zhongli"))
	if strings.Contains(body, badge) || strings.Contains(body, "is not in gcsim yet") || strings.Contains(body, "written from a community guide") {
		t.Error("verified team shown as experimental")
	}
}

func TestForkOnlyWeaponBadge(t *testing.T) {
	ts := newTestServer(t)
	badge := `<span class="badge">Experimental</span>`
	_, body := get(t, ts, "/score?server=1&a="+url.QueryEscape("800000004:hutao,yelan,xingqiu,zhongli"))
	mustContain(t, body, badge, "A weapon in this team is not in gcsim yet")
	if strings.Contains(body, "A member of this team is not in gcsim yet") {
		t.Error("fork-only character warning on a team of upstream characters")
	}

	// Without the weapon, the same team stays verified.
	_, body = get(t, ts, "/score?server=1&a="+url.QueryEscape("800000001:hutao,yelan,xingqiu,zhongli"))
	if strings.Contains(body, badge) || strings.Contains(body, "A weapon in this team") {
		t.Error("verified team without a fork-only weapon shown as experimental")
	}
}

func TestScoreTimeline(t *testing.T) {
	ts := newTestServer(t)
	_, body := getKo(t, ts, "/score?a="+url.QueryEscape("800000001:hutao,yelan,xingqiu,zhongli"))
	mustContain(t, body, "로테이션 타임라인", `id="tl-a-cycle"`, `id="tl-a-steady"`, "교체 순서와 구간별 피해",
		"능력별 피해", "로테이션 스크립트", `class="tl-bar" data-tip=`, ">E</text>", ">Q</text>")
	_, body = get(t, ts, "/score?a="+url.QueryEscape("800000001:hutao,yelan,xingqiu,zhongli"))
	mustContain(t, body, "Rotation timeline", "Swaps and damage per stint", "Damage by ability", "Rotation script",
		"One rotation (full energy):", `<meta property="og:title" content="Strong&#39;s team: `)
}

func TestTalentsOf(t *testing.T) {
	tr := teamResult{Chars: hutaoTeam()}
	if got, want := tr.TalentsOf(0), "9/9/9"; got != want {
		t.Errorf("without fight levels: %q, want %q", got, want)
	}
	tr.Talents = [][3]int{{9, 10, 12}, {9, 9, 9}, {9, 9, 9}, {9, 9, 9}}
	if got, want := tr.TalentsOf(0), "9/10 (9+1)/12 (9+3)"; got != want {
		t.Errorf("with bonuses: %q, want %q", got, want)
	}
	if got, want := tr.TalentsOf(1), "9/9/9"; got != want {
		t.Errorf("without bonuses: %q, want %q", got, want)
	}
}
