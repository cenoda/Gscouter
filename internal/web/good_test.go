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
	resp, err := client.Post(base+"/good", "application/json", bytes.NewReader(raw))
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
	notes := strings.Join(res.Notes, "\n")
	mustContain(t, notes, "Imported 6 characters.", "Not imported: Manekina (character Manekina is not supported by gcsim yet)", "Not imported: Bennett")

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
