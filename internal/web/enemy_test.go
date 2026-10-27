package web

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/cenoda/gscouter/internal/config"
)

func TestEnemyQuery(t *testing.T) {
	for query, want := range map[string]string{
		"":                          "",
		"lvl=100&res=10":            "",
		"lvl=90":                    "lvl=90",
		"res=10&geo=70&physical=40": "geo=70&physical=40",
		"res=25":                    "res=25",
		"pyro=25&hydro=25&anemo=25&electro=25&dendro=25&cryo=25&geo=10&physical=10": "geo=10&physical=10&res=25",
		"lvl=95&pyro=70": "lvl=95&pyro=70",
	} {
		q, _ := url.ParseQuery(query)
		e, err := enemyFromQuery(q)
		if err != nil {
			t.Errorf("%q: %v", query, err)
			continue
		}
		if got := enemyQuery(e); got != want {
			t.Errorf("%q: got %q, want %q", query, got, want)
		}
	}
	for _, query := range []string{"lvl=0", "lvl=x", "res=2000", "pyro=-101", "geo=1.5"} {
		q, _ := url.ParseQuery(query)
		if _, err := enemyFromQuery(q); err == nil {
			t.Errorf("%q: no error", query)
		}
	}
}

func TestEnemyText(t *testing.T) {
	e := config.StandardEnemy
	e.Level = 90
	e.Resist[6], e.Resist[7] = 70, 40
	if got, want := english.enemyText(&e), "Lv. 90 · Geo 70%, Physical 40%, others 10%"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got := english.enemyText(nil); !strings.Contains(got, "standard") {
		t.Errorf("standard enemy: %q", got)
	}
	if got, want := (localizer{Lang: langKorean}).enemyText(&config.Enemy{Level: 100, Resist: [8]int{25, 25, 25, 25, 25, 25, 25, 25}}), "Lv.100 · 모든 내성 25%"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestScoreCustomEnemy(t *testing.T) {
	srv := newTestSite(t)
	ts := newTestServerFor(t, srv)
	sel := "800000001:hutao,yelan,xingqiu,zhongli"

	// The enemy form sends every field; the page redirects to the short URL.
	client := *ts.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	form := url.Values{"a": {sel}, "lvl": {"90"}, "pyro": {"70"}, "hydro": {"10"}, "anemo": {"10"}, "electro": {"10"}, "dendro": {"10"}, "cryo": {"10"}, "geo": {"10"}, "physical": {"10"}}
	resp, err := client.Get(ts.URL + "/score?" + form.Encode())
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	want := "/score?" + url.Values{"a": {sel}, "lvl": {"90"}, "pyro": {"70"}}.Encode()
	if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != want {
		t.Errorf("status %d, location %q, want %q", resp.StatusCode, resp.Header.Get("Location"), want)
	}

	_, body := get(t, ts, want)
	mustContain(t, body, "Calculated against a custom enemy (Lv. 90 · Pyro 70%, others 10%)",
		`name="lvl" min="1" max="200" step="1" value="90"`, `name="pyro" min="-100" max="1000" step="1" value="70"`,
		"Back to the standard enemy",
		// Comparing and picking again keep the enemy.
		`<input type="hidden" name="lvl" value="90">`, `href="/?lvl=90&amp;pyro=70&amp;uid=800000001"`,
		// Link previews too.
		`/card/score.png?a=800000001%3Ahutao%2Cyelan%2Cxingqiu%2Czhongli&amp;lvl=90&amp;pyro=70`, "custom enemy: Lv. 90")
	if code, _ := get(t, ts, "/score?a="+url.QueryEscape(sel)+"&lvl=500"); code != http.StatusBadRequest {
		t.Errorf("bad enemy: status %d", code)
	}

	// The opponent's picker carries the enemy to the comparison.
	_, body = get(t, ts, "/?uid=800000002&vs="+url.QueryEscape(sel)+"&lvl=90&pyro=70")
	mustContain(t, body, "Teams will be calculated against a custom enemy",
		`href="/versus?a=800000001%3Ahutao%2Cyelan%2Cxingqiu%2Czhongli&amp;b=800000002%3Ahutao%2Cxingqiu%2Czhongli%2Cyelan&amp;lvl=90&amp;pyro=70"`)
	_, body = get(t, ts, "/versus?"+url.Values{"a": {sel}, "b": {"800000002:hutao,yelan,xingqiu,zhongli"}, "lvl": {"90"}, "pyro": {"70"}}.Encode())
	mustContain(t, body, "Calculated against a custom enemy (Lv. 90 · Pyro 70%, others 10%)", `value="70"`)

	// Resistance to Hu Tao's element lowers the team's damage.
	s, _ := parseSelection(sel)
	ctx := context.Background()
	std := srv.evaluate(ctx, english, s, nil)
	tough := config.StandardEnemy
	tough.Resist[0] = 70
	res := srv.evaluate(ctx, english, s, &tough)
	if std.Result == nil || res.Result == nil {
		t.Fatalf("no score: %q, %q", std.Problem, res.Problem)
	}
	if res.Result.DPS >= std.Result.DPS*0.9 {
		t.Errorf("70%% Pyro resistance: %.0f DPS, standard %.0f", res.Result.DPS, std.Result.DPS)
	}
}
