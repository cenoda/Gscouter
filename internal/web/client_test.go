package web

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// newClientSite is the test site with a stand-in browser build.
func newClientSite(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	dir := t.TempDir()
	for name, body := range map[string]string{"gscouter.wasm": "wasm", "wasm_exec.js": "// go"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	f, err := os.Create(filepath.Join(dir, "gscouter.wasm.gz"))
	if err != nil {
		t.Fatal(err)
	}
	zw := gzip.NewWriter(f)
	zw.Write([]byte("wasm"))
	zw.Close()
	f.Close()

	srv := newTestSite(t)
	if err := srv.ServeClient(dir); err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return srv, ts
}

// With the browser build, result pages leave the calculation to the
// browser, and fall back to the server on request.
func TestScoreInBrowser(t *testing.T) {
	_, ts := newClientSite(t)
	const path = "/score?a=800000001:hutao,yelan,xingqiu,zhongli"
	code, body := get(t, ts, path)
	if code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	mustContain(t, body, `class="pending"`, "Calculating in your browser", `"Rotation":`, "server=1", "/static/worker.js?v=", "/card/score.png?a=")
	if strings.Contains(body, `class="score"`) {
		t.Error("the server calculated the score")
	}

	_, body = get(t, ts, path+"&server=1")
	mustContain(t, body, `class="score"`)
	if strings.Contains(body, `class="pending"`) {
		t.Error("the server left the score to the browser")
	}

	// Nothing to calculate: no job.
	_, body = get(t, ts, "/score?a=800000001:amber,yelan,xingqiu,zhongli")
	mustContain(t, body, "not in the library yet", "/edit?a=800000001")
	if strings.Contains(body, "/static/worker.js") {
		t.Error("job for an unsupported team")
	}
}

// RunJob, as the browser runs it, renders what the server renders.
func TestRunJobMatchesServer(t *testing.T) {
	srv, _ := newClientSite(t)
	ctx := context.Background()
	sel := selection{UID: "800000001", Chars: []string{"hutao", "yelan", "xingqiu", "zhongli"}}
	tr := srv.prepare(ctx, english, sel, nil)
	tr.ID = "a"
	job := srv.job(english, "score", tr)
	if job == nil {
		t.Fatal("no job")
	}
	body, title, err := RunJob(ctx, *job, serverSim{srv})
	if err != nil {
		t.Fatal(err)
	}

	want := srv.evaluate(ctx, english, sel, nil)
	want.ID = "a"
	var b strings.Builder
	if err := srv.tmpl[english].ExecuteTemplate(&b, "score-body", scoreData{Team: want}); err != nil {
		t.Fatal(err)
	}
	if body != b.String() {
		t.Error("RunJob's body differs from the server's")
	}
	if want := scoreMeta(english, want, "").Title; title != want {
		t.Errorf("title %q, want %q", title, want)
	}
	if want.Result.Iterations != srv.settings.Iterations {
		t.Errorf("score from %d iterations, want %d", want.Result.Iterations, srv.settings.Iterations)
	}
}

func TestStaticBuild(t *testing.T) {
	srv, ts := newClientSite(t)
	req, _ := http.NewRequest("GET", ts.URL+"/static/gscouter.wasm?v="+srv.client.version, nil)
	req.Header.Set("Accept-Encoding", "gzip")
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	h := resp.Header
	if resp.StatusCode != http.StatusOK || h.Get("Content-Encoding") != "gzip" || h.Get("Content-Type") != "application/wasm" || !strings.Contains(h.Get("Cache-Control"), "immutable") {
		t.Errorf("gscouter.wasm: %d %v", resp.StatusCode, h)
	}
	for _, path := range []string{"/static/worker.js", "/static/wasm_exec.js"} {
		if code, _ := get(t, ts, path); code != http.StatusOK {
			t.Errorf("%s: status %d", path, code)
		}
	}
	if code, _ := get(t, ts, "/static/other"); code != http.StatusNotFound {
		t.Errorf("/static/other: status %d", code)
	}
}

// The front page warms the browser build up for the result page, at the
// URL the result page loads it from, once the visitor interacts with the
// UID input (not on focus: it is focused on load), or right away on a loaded
// showcase; without a browser build it does not.
func TestIndexPrefetchesClient(t *testing.T) {
	srv, ts := newClientSite(t)
	_, body := get(t, ts, "/")
	mustContain(t, body, "gscouterPrefetched", `var v = "`+srv.client.version+`"`, `"/static/gscouter.wasm?v=" + v`,
		`["pointerdown", "keydown", "input"]`, `getElementById("showcase")) return warm()`)
	if strings.Contains(body, `"focus"`) || strings.Contains(body, `id="showcase"`) {
		t.Error("the warm-up would start on page load")
	}
	_, body = get(t, ts, "/?uid=800000001")
	mustContain(t, body, `id="showcase"`, "gscouterPrefetched")

	plain := newTestServer(t)
	if _, body := get(t, plain, "/"); strings.Contains(body, "gscouterPrefetched") {
		t.Error("prefetching without a browser build")
	}
}

// A candidate the engine rejects fails its simulation alone: the page's
// script, with the real worker script, hands the error to the lead's
// search, which goes on to score the next candidate in the browser and
// keeps only the finished simulation (testdata/worker_protocol.js).
func TestWorkerProtocol(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not found")
	}
	_, ts := newClientSite(t)
	_, body := get(t, ts, "/score?a=800000001:hutao,yelan,xingqiu,zhongli")
	i := strings.Index(body, "var job = ")
	if i < 0 {
		t.Fatal("no client script")
	}
	i = strings.LastIndex(body[:i], "<script>") + len("<script>")
	end := strings.Index(body[i:], "</script>")
	if end < 0 {
		t.Fatal("client script not closed")
	}
	page := filepath.Join(t.TempDir(), "page.js")
	if err := os.WriteFile(page, []byte(body[i:i+end]), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := exec.Command(node, filepath.Join("testdata", "worker_protocol.js"), filepath.Join("static", "worker.js"), page).Output()
	if err != nil {
		t.Fatalf("node: %v", err)
	}
	type lead struct {
		Cfg                  string
		Iterations, Distinct int
		Error                string
	}
	var out struct {
		Fallback, Body *string
		Lead           []lead
		Saved, Errors  []string
	}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("%v: %s", err, b)
	}
	if out.Fallback != nil || len(out.Errors) > 0 {
		t.Errorf("fell back to the server (%q)", out.Errors)
	}
	want := []lead{{Cfg: "rejected", Error: "candidate rotation rejected"}, {Cfg: "accepted", Iterations: 10, Distinct: 10}}
	if !reflect.DeepEqual(out.Lead, want) {
		t.Errorf("the lead saw %+v, want %+v", out.Lead, want)
	}
	if out.Body == nil || *out.Body != "scored" {
		t.Error("the page was not rendered")
	}
	if !reflect.DeepEqual(out.Saved, []string{"accepted"}) {
		t.Errorf("kept %q, want only the finished simulation", out.Saved)
	}
}
