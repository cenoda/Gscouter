package web

import (
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
