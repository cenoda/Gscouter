package web

import (
	"context"
	"encoding/json"
	"html"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/cenoda/gscouter/internal/chart"
	"github.com/cenoda/gscouter/internal/config"
	"github.com/cenoda/gscouter/internal/library"
	"github.com/cenoda/gscouter/teams"
)

// The editor needs the browser build; without it, it says so.
func TestEditNeedsBrowser(t *testing.T) {
	ts := newTestServer(t)
	_, body := get(t, ts, "/edit?a=800000001:hutao,yelan,xingqiu,zhongli")
	mustContain(t, body, "runs in the browser")
}

func TestEditPage(t *testing.T) {
	_, ts := newClientSite(t)
	code, body := get(t, ts, "/edit?a=800000001:hutao,yelan,xingqiu,zhongli")
	if code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	// The page has what the browser records the first chart from: the
	// library rotation in the player's names, the members by slot.
	mustContain(t, body, "Rotation editor", "zhongli skill[hold=1]", `["hutao","xingqiu","yelan","zhongli"]`)
	// The party order is as picked, by slot, for the number keys and the
	// lanes, and the settings change it and the keys.
	mustContain(t, body, "ORDER_DEFAULT = [0,2,1,3]", `id="ed-settings"`)
}

func TestScoreWithChart(t *testing.T) {
	ts := newTestServer(t)
	c, err := chart.Parse("3E(hold=1) 2E 2Q? 1Q? 1E 0E [0N 0C 0D]6 0Q?")
	if err != nil {
		t.Fatal(err)
	}
	q := url.Values{"a": {"800000001:hutao,yelan,xingqiu,zhongli"}, "r": {c.Encode()}}
	code, body := get(t, ts, "/score?"+q.Encode())
	if code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	mustContain(t, body, "Scored with a custom rotation", "hutao charge;", "/edit?a=800000001~")
	// The editor link and the enemy form keep the chart.
	mustContain(t, body, "&amp;r="+c.Encode(), `name="r" value="`+c.Encode()+`"`)

	q.Set("r", "not-a-chart")
	_, body = get(t, ts, "/score?"+q.Encode())
	mustContain(t, body, "chart cannot be read")
}

// A chart scores even when the library's rotations for the team all
// fail: the comparison with them is only left out (docs/cycle-swap.md §7).
func TestChartScoresWithoutLibrary(t *testing.T) {
	srv := newTestSiteWith(t, func(lib *library.Library) {
		team := lib.Find([]string{"hutao", "yelan", "xingqiu", "zhongli"})
		team.Rotation = "active hutao;\nhutao audit_invalid_action;\n"
		team.Alternatives = nil
	})
	ts := newTestServerFor(t, srv)
	c, err := chart.Parse("0N")
	if err != nil {
		t.Fatal(err)
	}
	q := url.Values{"a": {"800000001:hutao,yelan,xingqiu,zhongli"}, "r": {c.Encode()}}
	code, body := get(t, ts, "/score?"+q.Encode())
	if code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	if strings.Contains(body, "The calculation failed.") {
		t.Fatal("the chart did not score")
	}
	mustContain(t, body, "Scored with your own rotation", "hutao attack;")

	// Without a chart, the failing rotation still fails the team.
	q.Del("r")
	_, body = get(t, ts, "/score?"+q.Encode())
	mustContain(t, body, "The calculation failed.")
}

// A link's chart of only notes played when ready, which would loop on
// one frame forever, is refused at once with why, on the result page
// and in the editor.
func TestIdleChartRefused(t *testing.T) {
	srv := newTestSite(t)
	idle := (&chart.Chart{Notes: []chart.Note{{Slot: 1, Action: "burst", Ready: true}}}).Encode()
	for _, page := range []string{"/score", "/edit"} {
		q := url.Values{"a": {"800000001:hutao,yelan,xingqiu,zhongli"}, "r": {idle}}
		rec := httptest.NewRecorder()
		done := make(chan struct{})
		go func() {
			srv.Handler().ServeHTTP(rec, httptest.NewRequest("GET", page+"?"+q.Encode(), nil))
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(20 * time.Second):
			t.Fatalf("%s: still running after 20 s", page)
		}
		mustContain(t, rec.Body.String(), "plays only when ready")
	}
}

// Comparing a result scored with a chart compares that chart: the
// compare form, the opponent's page and its links to the versus page
// carry it, as "vr" and then "ra".
func TestVersusKeepsChart(t *testing.T) {
	ts := newTestServer(t)
	c, err := chart.Parse("3E(hold=1) 2E 2Q? 1Q? 1E 0E [0N 0C 0D]6 0Q?")
	if err != nil {
		t.Fatal(err)
	}
	r := c.Encode()
	_, body := get(t, ts, "/score?"+url.Values{"a": {"800000001:hutao,yelan,xingqiu,zhongli"}, "r": {r}}.Encode())
	mustContain(t, body, `<input type="hidden" name="vs" value="800000001~`, `<input type="hidden" name="vr" value="`+r+`">`)

	sel := "800000001:hutao,yelan,xingqiu,zhongli"
	q := url.Values{"uid": {"800000002"}, "vs": {sel}, "vr": {r}}
	_, body = get(t, ts, "/?"+q.Encode())
	mustContain(t, body,
		// Loading another UID, and the picker.
		`<input type="hidden" name="vr" value="`+r+`">`, `<input type="hidden" name="ra" value="`+r+`">`,
		// The suggestions.
		`href="/versus?a=800000001%3Ahutao%2Cyelan%2Cxingqiu%2Czhongli&amp;b=800000002%3Ahutao%2Cxingqiu%2Czhongli%2Cyelan&amp;ra=`+r+`"`)
	// The showcase card rendered again with kept builds keeps it too.
	_, body = post(t, ts, "/showcase", url.Values{"uid": {"800000002"}, "vs": {sel}, "vr": {r}, "kept": {"[]"}})
	mustContain(t, body, `<input type="hidden" name="ra" value="`+r+`">`, "&amp;ra="+r+`"`)

	// The versus page scores the chart on that side.
	_, body = get(t, ts, "/versus?"+url.Values{"a": {sel}, "ra": {r}, "b": {"800000002:hutao,xingqiu,zhongli,yelan"}}.Encode())
	mustContain(t, body, "Scored with a custom rotation")

	q.Set("vr", "not-a-chart")
	if code, body := get(t, ts, "/?"+q.Encode()); code != http.StatusBadRequest || !strings.Contains(body, "chart cannot be read") {
		t.Errorf("bad chart: status %d", code)
	}
	if code, _ := post(t, ts, "/showcase", url.Values{"uid": {"800000002"}, "vs": {sel}, "vr": {"bad"}, "kept": {"[]"}}); code != http.StatusBadRequest {
		t.Errorf("bad chart in the showcase: status %d", code)
	}
}

func TestShortTargetKeepsChart(t *testing.T) {
	sel := "800000001~" + packBuilds("Strong", hutaoTeam())
	c, _ := chart.Parse("0E 0N")
	got, err := shortTarget("/score?" + url.Values{"a": {sel}, "r": {c.Encode()}, "x": {"1"}}.Encode())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "r="+c.Encode()) || strings.Contains(got, "x=") {
		t.Errorf("shortTarget = %q", got)
	}
	if _, err := shortTarget("/score?" + url.Values{"a": {sel}, "r": {"bad"}}.Encode()); err == nil {
		t.Error("a bad chart passed")
	}
}

type memSends struct{ got []string }

func (m *memSends) AddSend(_ context.Context, target string) error {
	m.got = append(m.got, target)
	return nil
}

func TestSendChart(t *testing.T) {
	srv := newTestSite(t)
	sends := &memSends{}
	srv.SetSends(sends)
	ts := newTestServerFor(t, srv)
	packed := "800000001~" + packBuilds("Strong", hutaoTeam())
	c, _ := chart.Parse("0E 0N")
	post := func(u string) int {
		resp, err := ts.Client().PostForm(ts.URL+"/send", url.Values{"u": {u}})
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	if code := post("/score?" + url.Values{"a": {packed}, "r": {c.Encode()}}.Encode()); code != http.StatusNoContent {
		t.Errorf("send: status %d", code)
	}
	// A result without a chart, or not a result, is not sent.
	if code := post("/score?" + url.Values{"a": {packed}}.Encode()); code != http.StatusBadRequest {
		t.Errorf("send without a chart: status %d", code)
	}
	if code := post("/edit?" + url.Values{"a": {packed}, "r": {c.Encode()}}.Encode()); code != http.StatusBadRequest {
		t.Errorf("send of the editor: status %d", code)
	}
	if len(sends.got) != 1 || !strings.Contains(sends.got[0], "r="+c.Encode()) {
		t.Errorf("kept %v", sends.got)
	}
}

// A team the library doesn't have: the editor starts from a library
// team sharing three members, and the chart scores it.
func TestEditUnsupportedTeam(t *testing.T) {
	_, ts := newClientSite(t)
	_, body := get(t, ts, "/edit?a=800000001:hutao,yelan,xingqiu,amber")
	mustContain(t, body, "Rotation editor", "ROTATION = \"\"", "TEMPLATE = {name:")

	ts2 := newTestServer(t)
	// Slots: amber, hutao, xingqiu, yelan.
	c, _ := chart.Parse("3E 3Q? 2Q? 2E 1E [1N 1C 1D]5 1Q?")
	q := url.Values{"a": {"800000001:hutao,yelan,xingqiu,amber"}, "r": {c.Encode()}}
	_, body = get(t, ts2, "/score?"+q.Encode())
	mustContain(t, body, "DPS", "the library has no rotation for this team")
}

func TestPickupExplained(t *testing.T) {
	_, ts := newClientSite(t)
	_, body := get(t, ts, "/edit?a=800000001:hutao,yelan,xingqiu,zhongli")
	mustContain(t, body, "Pick up crystallize shards automatically", "Archaic Petra", "does not pick up shards")

	ts2 := newTestServer(t)
	c, _ := chart.Parse("3E(hold=1) 2E 1E 0E [0N 0C 0D]6 ; pickup")
	_, body = get(t, ts2, "/score?"+url.Values{"a": {"800000001:hutao,yelan,xingqiu,zhongli"}, "r": {c.Encode()}}.Encode())
	mustContain(t, body, "picks up crystallize shards after every action")
}

// The link preview image is drawn with the result's chart, whether the
// server or the browser calculates the result.
func TestPreviewImageKeepsChart(t *testing.T) {
	ca, _ := chart.Parse("3E(hold=1) 2E 2Q? 1Q? 1E 0E [0N 0C 0D]6 0Q?")
	cb, _ := chart.Parse("3E(hold=1) 2E 1E 0E [0N 0C 0D]6 0Q?")
	const team = "800000001:hutao,yelan,xingqiu,zhongli"
	_, client := newClientSite(t)
	for name, ts := range map[string]*httptest.Server{"server": newTestServer(t), "browser": client} {
		for _, c := range []struct {
			page, card string
			want       url.Values
		}{
			{"/score?" + url.Values{"a": {team}, "r": {ca.Encode()}}.Encode(), "/card/score.png", url.Values{"r": {ca.Encode()}}},
			{"/versus?" + url.Values{"a": {team}, "b": {team}, "ra": {ca.Encode()}, "rb": {cb.Encode()}}.Encode(), "/card/versus.png", url.Values{"ra": {ca.Encode()}, "rb": {cb.Encode()}}},
		} {
			_, body := get(t, ts, c.page)
			i := strings.Index(body, `og:image" content="`)
			if i < 0 {
				t.Fatalf("%s %s: no og:image", name, c.page)
			}
			img := body[i+len(`og:image" content="`):]
			u, err := url.Parse(html.UnescapeString(img[:strings.Index(img, `"`)]))
			if err != nil || u.Path != c.card {
				t.Fatalf("%s %s: og:image %q", name, c.page, img[:strings.Index(img, `"`)])
			}
			for k := range c.want {
				if got := u.Query().Get(k); got != c.want.Get(k) {
					t.Errorf("%s %s: og:image %s = %q, want %q", name, c.page, k, got, c.want.Get(k))
				}
			}
		}
	}
}

// pageString is the JavaScript string the editor page assigns to name.
func pageString(t *testing.T, body, name string) string {
	t.Helper()
	m := regexp.MustCompile(`\b` + name + ` = ("(?:[^"\\]|\\.)*")`).FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("page has no %s", name)
	}
	var s string
	if err := json.Unmarshal([]byte(m[1]), &s); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return s
}

// The editor's first chart is the library rotation recorded with the
// library's character params, as the chart's preview and score play it:
// Skirk / Furina / Citlali / Escoffier start without energy, and their
// rotation decides some notes on energy.
func TestEditRecordsWithParams(t *testing.T) {
	_, ts := newClientSite(t)
	_, body := get(t, ts, "/edit?a=800000006:skirk,furina,citlali,escoffier")
	lib, err := library.LoadLibrary(teams.FS)
	if err != nil {
		t.Fatal(err)
	}
	team := lib.Find([]string{"skirk", "furina", "citlali", "escoffier"})
	if team == nil || len(team.Params) == 0 {
		t.Fatal("library team with params not found")
	}
	var names [chart.Slots]string
	var build strings.Builder
	for i, c := range slotted(skirkTeam()) {
		names[i] = c.Key
		build.WriteString(config.AddParams(c.Config(), c.Key, team.Params[c.Key]))
		build.WriteString("\n")
	}
	want, err := chart.Record(build.String(), team.Rotation, names)
	if err != nil {
		t.Fatal(err)
	}
	got, err := chart.Record(pageString(t, body, "RECORD_BUILD"), pageString(t, body, "ROTATION"), names)
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != want.String() {
		t.Errorf("editor records\n%s\nwant\n%s", got, want)
	}
}

// Editing a result keeps its enemy: the result's editor link, the
// editor's URL, the preview and the link back to the score all carry it.
func TestEditKeepsEnemy(t *testing.T) {
	ts := newTestServer(t)
	_, cts := newClientSite(t)
	noFollow := *cts.Client()
	noFollow.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	c, err := chart.Parse("3E(hold=1) 2E 2Q? 1Q? 1E 0E [0N 0C 0D]6 0Q?")
	if err != nil {
		t.Fatal(err)
	}
	r := c.Encode()
	packed := "800000001~" + packBuilds("Strong", hutaoTeam())
	editLink := regexp.MustCompile(`href="(/edit\?[^"]*)"`)
	scoreVar := regexp.MustCompile(`SCORE = ("[^"]*")`)
	enemyVar := regexp.MustCompile(`ENEMY = ([^;]*);`)
	for _, tc := range []struct {
		name  string
		enemy url.Values
		note  string
		want  config.Enemy
	}{
		{"level", url.Values{"lvl": {"90"}}, "Lv. 90", config.Enemy{Level: 90, Resist: config.StandardEnemy.Resist}},
		{"resistance", url.Values{"geo": {"70"}}, "Geo RES 70%", config.Enemy{Level: 100, Resist: [8]int{10, 10, 10, 10, 10, 10, 70, 10}}},
		{"boss", url.Values{"boss": {"lasignora"}, "lvl": {"95"}}, "La Signora · Lv. 95", config.Enemy{Level: 95, Resist: [8]int{70, 10, 10, 10, 10, 10, 10, 10}, Boss: "lasignora"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q := url.Values{"a": {packed}, "r": {r}}
			for k, v := range tc.enemy {
				q[k] = v
			}
			_, body := get(t, ts, "/score?"+q.Encode())
			m := editLink.FindStringSubmatch(body)
			if m == nil {
				t.Fatal("no editor link")
			}
			edit := html.UnescapeString(m[1])
			eq := mustQuery(t, edit)
			for k, v := range tc.enemy {
				if eq.Get(k) != v[0] {
					t.Errorf("editor link %q: %s = %q, want %q", edit, k, eq.Get(k), v[0])
				}
			}

			// The editor's URL keeps the enemy as it is.
			resp, err := noFollow.Get(cts.URL + edit)
			if err != nil {
				t.Fatal(err)
			}
			b := new(strings.Builder)
			_, _ = io.Copy(b, resp.Body)
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("editor: status %d, location %q", resp.StatusCode, resp.Header.Get("Location"))
			}
			page := b.String()

			// The preview plays against it.
			m = enemyVar.FindStringSubmatch(page)
			if m == nil {
				t.Fatal("no ENEMY in the editor")
			}
			var got config.Enemy
			if err := json.Unmarshal([]byte(m[1]), &got); err != nil {
				t.Fatalf("ENEMY = %s: %v", m[1], err)
			}
			if got != tc.want {
				t.Errorf("ENEMY = %+v, want %+v", got, tc.want)
			}

			// The score the editor sends back to is against it too.
			m = scoreVar.FindStringSubmatch(page)
			if m == nil {
				t.Fatal("no SCORE in the editor")
			}
			var score string
			if err := json.Unmarshal([]byte(m[1]), &score); err != nil {
				t.Fatal(err)
			}
			_, body = get(t, ts, score+"&r="+url.QueryEscape(r))
			mustContain(t, body, "Calculated against a custom enemy", tc.note, "Scored with a custom rotation")
		})
	}

	// The standard enemy has no ENEMY to play against.
	_, page := get(t, cts, "/edit?a="+url.QueryEscape(packed))
	if !regexp.MustCompile(`ENEMY = \s*null\s*;`).MatchString(page) {
		t.Error("the standard enemy is not ENEMY = null")
	}
	// A bad enemy is refused.
	if code, _ := get(t, cts, "/edit?a="+url.QueryEscape(packed)+"&lvl=500"); code != http.StatusBadRequest {
		t.Errorf("bad enemy: status %d", code)
	}
}

func mustQuery(t *testing.T, link string) url.Values {
	t.Helper()
	u, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	return u.Query()
}
