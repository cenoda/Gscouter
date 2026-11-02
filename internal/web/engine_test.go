package web

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/cenoda/gscouter/internal/engine"
	"github.com/cenoda/gscouter/teams"
)

// testEngine is an engine on the stand-in module (internal/engine's
// testdata/fake.wasm, whose iteration i deals 1 + i DPS at the site's
// seed) with verified teams, of which approved run on it.
func testEngine(t *testing.T, verified map[string]bool, approved ...string) *engine.Engine {
	t.Helper()
	mod, err := os.ReadFile("../engine/testdata/fake.wasm")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(mod)
	var entries []string
	for id, v := range verified {
		entries = append(entries, fmt.Sprintf("%q: {\"verified\": %v}", id, v))
	}
	manifest := fmt.Sprintf(`{"engine": {"commit": "test", "wasm_sha256": %q}, "teams": {%s}}`, hex.EncodeToString(sum[:]), strings.Join(entries, ","))
	var list []engine.Approval
	pinned := map[string]string{}
	for _, id := range approved {
		list = append(list, engine.Approval{Team: id, Date: "2026-11-02"})
		// The library as it is; "other" is not in it.
		if h, err := engine.TeamHash(teams.FS, id); err == nil {
			pinned[id] = h
		}
	}
	e, _, err := engine.New(mod, []byte(manifest), pinned, teams.FS, list)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

var hutaoSel = selection{UID: "800000001", Chars: []string{"hutao", "yelan", "xingqiu", "zhongli"}}

// A team runs on the engine when it is verified and approved, and is
// scored with the library's rotations; otherwise on gcsim.
func TestEngineRouting(t *testing.T) {
	srv := newTestSite(t)
	ctx := context.Background()
	id := srv.prepare(ctx, english, hutaoSel, nil).Team.ID
	if tr := srv.prepare(ctx, english, hutaoSel, nil); tr.Engine {
		t.Error("no engine: the team runs on it")
	}
	for name, c := range map[string]struct {
		e    *engine.Engine
		want bool
	}{
		"verified and approved":    {testEngine(t, map[string]bool{id: true}, id), true},
		"verified, not approved":   {testEngine(t, map[string]bool{id: true}), false},
		"approved, not verified":   {testEngine(t, map[string]bool{id: false}, id), false},
		"approved, not in release": {testEngine(t, nil, id), false},
		"another team approved":    {testEngine(t, map[string]bool{id: true, "other": true}, "other"), false},
	} {
		srv.SetEngine(c.e)
		if got := srv.prepare(ctx, english, hutaoSel, nil).Engine; got != c.want {
			t.Errorf("%s: Engine = %v, want %v", name, got, c.want)
		}
	}
	// A team whose library files changed since the release verified it
	// stays on gcsim.
	mod, _ := os.ReadFile("../engine/testdata/fake.wasm")
	sum := sha256.Sum256(mod)
	manifest := fmt.Sprintf(`{"engine": {"wasm_sha256": %q}, "gscouter_commit": "old", "teams": {%q: {"verified": true}}}`, hex.EncodeToString(sum[:]), id)
	e, skipped, err := engine.New(mod, []byte(manifest), map[string]string{id: strings.Repeat("0", 64)}, teams.FS, []engine.Approval{{Team: id, Date: "2026-11-02"}})
	if err != nil {
		t.Fatal(err)
	}
	srv.SetEngine(e)
	if srv.prepare(ctx, english, hutaoSel, nil).Engine || len(skipped) != 1 || !strings.Contains(skipped[0], "changed since Gscouter old") {
		t.Errorf("a changed team runs on the engine (skipped %v)", skipped)
	}

	// The player's own rotation stays on gcsim, with the library's
	// score beside it.
	srv.SetEngine(testEngine(t, map[string]bool{id: true}, id))
	chart := hutaoSel
	chart.Chart = "x"
	if srv.prepare(ctx, english, chart, nil).Engine {
		t.Error("a chart runs on the engine")
	}
}

// The server scores an approved team on the engine (link-preview cards
// and server=1 pages both come through evaluate), keyed apart from
// gcsim's results; timelines and details stay on gcsim.
func TestEngineServerPath(t *testing.T) {
	srv := newTestSite(t)
	ctx := context.Background()
	gcsim := srv.evaluate(ctx, english, hutaoSel, nil)
	if gcsim.Problem != "" {
		t.Fatal(gcsim.Problem)
	}
	srv.SetEngine(testEngine(t, map[string]bool{gcsim.Team.ID: true}, gcsim.Team.ID))
	tr := srv.evaluate(ctx, english, hutaoSel, nil)
	if tr.Problem != "" {
		t.Fatal(tr.Problem)
	}
	n := float64(srv.settings.Iterations)
	if want := (n + 1) / 2; tr.Result.DPS != want || tr.OneRotation.DPS != want {
		t.Errorf("DPS %v and %v, want the stand-in's %v", tr.Result.DPS, tr.OneRotation.DPS, want)
	}
	if len(tr.Timelines) == 0 || len(tr.Stats) != 4 {
		t.Error("no gcsim timelines or details beside the engine's score")
	}
	// The same configs on gcsim are not read from the engine's results.
	srv.SetEngine(nil)
	if again := srv.evaluate(ctx, english, hutaoSel, nil); again.Result.DPS != gcsim.Result.DPS {
		t.Errorf("gcsim again: DPS %v, want %v", again.Result.DPS, gcsim.Result.DPS)
	}

	// The page the server calculates shows the engine's score.
	srv.SetEngine(testEngine(t, map[string]bool{gcsim.Team.ID: true}, gcsim.Team.ID))
	_, body := get(t, newTestServerFor(t, srv), "/score?a=800000001:hutao,yelan,xingqiu,zhongli&server=1")
	mustContain(t, body, `class="score"`)

	// The browser's calculation (RunJob) agrees with the server.
	sel := tr.Selection
	job := srv.job(english, "score", srv.prepare(ctx, english, sel, nil))
	if job == nil || !job.Teams[0].Engine {
		t.Fatal("the job does not run the team on the engine")
	}
	got, _, err := RunJob(ctx, *job, serverSim{srv})
	if err != nil {
		t.Fatal(err)
	}
	var want strings.Builder
	if err := srv.tmpl[english].ExecuteTemplate(&want, "score-body", scoreData{Team: tr}); err != nil {
		t.Fatal(err)
	}
	if got != want.String() {
		t.Error("RunJob's body differs from the server's")
	}
}

// Without an engine team, a page's job and script are as before: no
// Engine field, no engine module.
func TestEngineClientData(t *testing.T) {
	srv, ts := newClientSite(t)
	const path = "/score?a=800000001:hutao,yelan,xingqiu,zhongli"
	_, body := get(t, ts, path)
	if strings.Contains(body, `"Engine"`) {
		t.Error("a page without engine teams runs one on the engine")
	}
	mustContain(t, body, `engine = ""`)
	if code, _ := get(t, ts, "/static/gse.wasm"); code != http.StatusNotFound {
		t.Errorf("gse.wasm without an engine: status %d", code)
	}

	id := srv.prepare(context.Background(), english, hutaoSel, nil).Team.ID
	e := testEngine(t, map[string]bool{id: true}, id)
	srv.SetEngine(e)
	_, body = get(t, ts, path)
	v := e.Hash()[:16]
	mustContain(t, body, `"Engine":true`, `engine = "`+v+`"`)

	// Another approved team, so the job has no team on the engine.
	srv.SetEngine(testEngine(t, map[string]bool{"other": true}, "other"))
	_, body = get(t, ts, path)
	mustContain(t, body, `engine = ""`)

	srv.SetEngine(e)
	req, _ := http.NewRequest("GET", ts.URL+"/static/gse.wasm?v="+v, nil)
	req.Header.Set("Accept-Encoding", "gzip")
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	h := resp.Header
	if resp.StatusCode != http.StatusOK || h.Get("Content-Encoding") != "gzip" || h.Get("Content-Type") != "application/wasm" || !strings.Contains(h.Get("Cache-Control"), "immutable") {
		t.Errorf("gse.wasm: %d %v", resp.StatusCode, h)
	}
}

// A job's teams carry Engine only when set, so an empty list leaves the
// job's JSON as it was.
func TestJobEngineField(t *testing.T) {
	b, _ := json.Marshal(teamResult{})
	if strings.Contains(string(b), "Engine") {
		t.Errorf("job JSON without the engine: %s", b)
	}
	b, _ = json.Marshal(teamResult{Engine: true})
	if !strings.Contains(string(b), `"Engine":true`) {
		t.Errorf("job JSON with the engine: %s", b)
	}
}
