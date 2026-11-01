package web

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/cenoda/gscouter/internal/enka"
)

func postGOOD(t *testing.T, base string, client *http.Client, raw []byte) (int, goodResponse) {
	t.Helper()
	return postGOODLang(t, base, client, raw, "")
}

// postGOODLang posts a GOOD file with an Accept-Language header.
func postGOODLang(t *testing.T, base string, client *http.Client, raw []byte, lang string) (int, goodResponse) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, base+"/good", bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if lang != "" {
		req.Header.Set("Accept-Language", lang)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var out goodResponse
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("%s: %v", body, err)
	}
	return resp.StatusCode, out
}

// A GOOD file's characters come back as builds the browser keeps, and
// flow as kept showcase builds do: their teams are suggested and their
// results carry them.
func TestGOODImport(t *testing.T) {
	ts := newTestServer(t)
	raw, err := os.ReadFile("../enka/testdata/good.json")
	if err != nil {
		t.Fatal(err)
	}
	code, res := postGOOD(t, ts.URL, ts.Client(), raw)
	if code != http.StatusOK || len(res.Builds) != 6 {
		t.Fatalf("status %d, %+v", code, res)
	}
	notes := noteText(res.Notes)
	mustContain(t, notes, "Imported 6 characters.", "Not imported: Manekina (Manekin and Manekina are not supported by gcsim)",
		"Not imported: Chongyun (weapon SomeSwordFromTheFuture is not supported by gcsim yet)", "Not imported: Bennett (no weapon equipped)")

	var kept []keptBuild
	tokens := map[string]string{}
	for _, b := range res.Builds {
		kept = append(kept, keptBuild{Build: b.Build, Seen: 1767225600000})
		tokens[b.Key] = b.Build
	}
	k, _ := json.Marshal(kept)
	// Zhongli has left this showcase; the file has him.
	code, body := post(t, ts, "/showcase", url.Values{"uid": {"800000003"}, "kept": {string(k)}})
	if code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	mustContain(t, body, "Hu Tao / Xingqiu / Zhongli / Yelan", `data-kept="`+tokens["zhongli"]+`"`, "Traveler (Anemo)")
	// The showcase's own builds win over the file's.
	if strings.Contains(body, `data-kept="`+tokens["hutao"]+`"`) {
		t.Error("the file's Hu Tao replaced the showcase's")
	}

	pick := "/score?uid=800000003&c=hutao&c=yelan&c=xingqiu&c=zhongli&build=" + tokens["zhongli"]
	resp := noRedirects(t, ts, ts.URL+pick)
	if loc := resp.Header.Get("Location"); !strings.Contains(loc, "a=800000003~") || strings.Contains(loc, "build=") {
		t.Errorf("location %q", loc)
	}

	// Anything else is not a GOOD file.
	code, res = postGOOD(t, ts.URL, ts.Client(), []byte(`{"format":"EnkaNetwork"}`))
	if code != http.StatusBadRequest || res.Error != "This is not a GOOD file." || res.Builds != nil {
		t.Errorf("status %d, %+v", code, res)
	}
}

// A Traveler from a GOOD file has no gender, and does not join the
// showcase's Traveler of the same element as a second one.
func TestGOODTravelerJoinsShowcase(t *testing.T) {
	srv := newTestSite(t)
	profiles := srv.profiles.(fakeProfiles)
	lumine := char("lumineanemo", "freedomsworn", "viridescentvenerer", nil)
	profiles["800000009"] = &enka.Profile{UID: "800000009", Nickname: "Lumine", Characters: []enka.Character{lumine}}
	ts := newTestServerFor(t, srv)
	aether := char("aetheranemo", "freedomsworn", "viridescentvenerer", nil)
	_, body := post(t, ts, "/showcase", url.Values{"uid": {"800000009"}, "kept": {`[{"b":"` + buildToken(aether) + `","t":0}]`}})
	if strings.Contains(body, "data-kept") {
		t.Error("the kept Aether joined the showcase's Lumine")
	}
}

func noteText(notes []goodNote) string {
	var lines []string
	for _, n := range notes {
		lines = append(lines, n.Text)
	}
	return strings.Join(lines, "\n")
}

// Why a character was not imported is said in the visitor's language,
// with the character's in-game name once.
func TestGOODSkippedLanguage(t *testing.T) {
	ts := newTestServer(t)
	raw := []byte(`{"format":"GOOD","characters":[
		{"key":"HuTao","level":90,"constellation":0,"ascension":6,"talent":{"auto":1,"skill":1,"burst":1}},
		{"key":"Xingqiu","level":90,"constellation":0,"ascension":6,"talent":{"auto":10,"skill":13,"burst":13}}],
		"weapons":[{"key":"SacrificialSword","level":90,"ascension":6,"refinement":1,"location":"Xingqiu"}]}`)
	for lang, want := range map[string][]string{
		"ko": {"가져오지 못함: 호두 (장착한 무기 없음)", "가져오지 못함: 행추 (특성 레벨 10/13/13이 기본 레벨"},
		"ja": {"読み込めなかったキャラクター: 胡桃 (武器を装備していません)", "読み込めなかったキャラクター: 行秋 (天賦レベル10/13/13が"},
	} {
		code, res := postGOODLang(t, ts.URL, ts.Client(), raw, lang)
		if code != http.StatusOK || len(res.Notes) != 3 {
			t.Fatalf("%s: status %d, %+v", lang, code, res)
		}
		notes := noteText(res.Notes)
		mustContain(t, notes, want...)
		if strings.Contains(notes, "HuTao") || strings.Contains(notes, "weapon") {
			t.Errorf("%s: notes in English or by GOOD key:\n%s", lang, notes)
		}
		for _, n := range res.Notes {
			if n.Lang != "" {
				t.Errorf("%s: %q marked %q", lang, n.Text, n.Lang)
			}
		}
	}
}

// Every reason has a message, and one the catalog does not know stays
// in English, marked so.
func TestGOODReasonMessages(t *testing.T) {
	for _, r := range enka.GOODReasons {
		if _, ok := catalog[langEnglish]["good."+r.Code()]; !ok {
			t.Errorf("no message for reason %q", r.Code())
		}
	}
	slot := enka.GOODSkipped{Key: "HuTao", Char: "hutao", Reason: enka.GOODSlotTwice, Args: []any{enka.Circlet}}
	if n := (localizer{Lang: "ko"}).goodSkipped(slot); n.Text != "가져오지 못함: 호두 (왕관 부위에 성유물이 두 개)" || n.Lang != "" {
		t.Errorf("slot: %+v", n)
	}
	odd := enka.GOODSkipped{Key: "HuTao", Char: "hutao", Reason: 0}
	if n := (localizer{Lang: "ko"}).goodSkipped(odd); n.Lang != "en" || n.Text != "Not imported: HuTao (skipped)" {
		t.Errorf("unknown reason: %+v", n)
	}
}
