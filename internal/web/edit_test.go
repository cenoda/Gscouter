package web

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/cenoda/gscouter/internal/chart"
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
