package web

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

// A result page's share sheet carries the result for its image as JSON,
// in the page's language, with each character's build details for the
// image to show or leave out.
func TestScoreShareData(t *testing.T) {
	ts := newTestServer(t)
	builds := piecedTeam()
	sel := selection{UID: "800000005", Chars: keys(builds), Builds: builds, Nickname: "Lohen <&>"}
	_, body := getKo(t, ts, "/score?a="+url.QueryEscape(sel.String()))
	mustContain(t, body, `onclick="openShare(this)"`, "공유하기", "세부 스펙 표시", "이미지 저장", "링크 복사")
	const open = `<script type="application/json" class="share-data">`
	i := strings.Index(body, open)
	if i < 0 {
		t.Fatal("no share data")
	}
	raw := body[i+len(open):]
	raw = raw[:strings.Index(raw, "</script>")]
	var data struct {
		Nickname, UID, Team, DPS, Stderr string
		Notes                            []string
		Chars                            []struct {
			Name, Face, Element, DPS, Percent, Build, Totals string
			Share                                            float64
		}
	}
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatalf("share data: %v\n%s", err, raw)
	}
	if data.Nickname != sel.Nickname || data.UID != sel.UID || data.Team == "" || data.DPS == "" ||
		!strings.Contains(data.Stderr, "표준오차") || len(data.Notes) != 1 || len(data.Chars) != 4 {
		t.Errorf("share data: %+v", data)
	}
	ko := localizer{Lang: langKorean}
	for i, c := range data.Chars {
		b := builds[i]
		if c.Name != ko.charName(b.Key) || c.Face != face(b.Key) || c.Share <= 0 ||
			!strings.HasPrefix(c.Build, "Lv.") || !strings.Contains(c.Build, ko.weaponName(b.Weapon.Key)) || c.Totals == "" {
			t.Errorf("char %d: %+v", i, c)
		}
	}
}
