package web

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func labAssets(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string][]byte{"gso.wasm": []byte("test wasm bytes"), "dictionary.json": []byte(`{"blocks":{}}`)}
	hashes := map[string]string{}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
		h := sha256.Sum256(data)
		hashes[name] = hex.EncodeToString(h[:])
	}
	manifest := map[string]any{"format": 1, "engine": map[string]string{"commit": strings.Repeat("a", 40)}, "wasm_sha256": hashes["gso.wasm"], "dictionary_sha256": hashes["dictionary.json"]}
	b, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestRotationLabOptInAndAssets(t *testing.T) {
	srv := newTestSite(t)
	for _, path := range []string{"/lab", "/lab/profile?uid=800000001", "/lab/assets/lab.js"} {
		w := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 404 {
			t.Fatalf("disabled %s: %d", path, w.Code)
		}
	}
	dir := labAssets(t)
	if err := srv.SetRotationLab(dir); err != nil {
		t.Fatal(err)
	}
	h := srv.Handler()
	for _, path := range []string{"/lab", "/lab/assets/lab.js", "/lab/assets/lab-worker.js", "/lab/assets/gso.wasm", "/lab/assets/manifest.json"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 {
			t.Fatalf("enabled %s: %d %s", path, w.Code, w.Body.String())
		}
		if path == "/lab" && (w.Header().Get("X-Robots-Tag") != "noindex, nofollow" || !strings.Contains(w.Body.String(), "lab-start")) {
			t.Fatal("lab lacks noindex or UI")
		}
	}
	for _, path := range []string{"/lab/assets/secret.txt", "/lab/assets/server.go"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 404 {
			t.Fatalf("unexpected asset: %s %d", path, w.Code)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "gso.wasm"), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := srv.SetRotationLab(dir); err == nil {
		t.Fatal("accepted inconsistent WASM asset")
	}
	if err := srv.SetRotationLab(""); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/lab", nil))
	if w.Code != 404 {
		t.Fatal("lab did not disable")
	}
}

func TestRotationLabRealBuildInput(t *testing.T) {
	srv := newTestSite(t)
	if err := srv.SetRotationLab(labAssets(t)); err != nil {
		t.Fatal(err)
	}
	h := srv.Handler()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/lab/profile?uid=800000001", nil))
	var profile struct {
		Characters []struct {
			Build string `json:"build"`
		} `json:"characters"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &profile) != nil || len(profile.Characters) != 5 {
		t.Fatalf("profile: %d %s", w.Code, w.Body.String())
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("player data cacheable")
	}
	tokens := []string{}
	for _, c := range profile.Characters[:4] {
		tokens = append(tokens, c.Build)
	}
	b, _ := json.Marshal(map[string]any{"builds": tokens})
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/lab/input", strings.NewReader(string(b))))
	var result struct {
		Build, Rotation, Engine string
		TeamID                  string `json:"team_id"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &result) != nil {
		t.Fatalf("input: %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(result.Build, "hutao char lvl=90/90 cons=0 talent=9,9,9;") && !strings.Contains(result.Build, "hutao char lvl=90/90 cons=0 talent=9,9,9 +params=") {
		t.Fatalf("actual C0 build was changed: %s", result.Build)
	}
	if !strings.Contains(result.Build, "cr=0.65") || result.Rotation == "" || result.TeamID != "hutao-xingqiu-zhongli-yelan" || result.Engine != strings.Repeat("a", 40) {
		t.Fatalf("wrong input: %+v", result)
	}
	// Metadata and build preparation must not schedule a server simulation.
	if len(srv.sem) != 0 || srv.waiting.Load() != 0 {
		t.Fatal("lab submitted server computation")
	}
	// Library assumptions are retained without replacing the player's equipment.
	skirkTokens := []string{}
	for _, c := range skirkTeam() {
		skirkTokens = append(skirkTokens, buildToken(c))
	}
	skirkBody, _ := json.Marshal(map[string]any{"builds": skirkTokens})
	skirkResponse := httptest.NewRecorder()
	h.ServeHTTP(skirkResponse, httptest.NewRequest("POST", "/lab/input", strings.NewReader(string(skirkBody))))
	var skirkInput struct{ Build string }
	if skirkResponse.Code != 200 || json.Unmarshal(skirkResponse.Body.Bytes(), &skirkInput) != nil || !strings.Contains(skirkInput.Build, "start_serpents_subtlety=0") || !strings.Contains(skirkInput.Build, "start_energy=0") || !strings.Contains(skirkInput.Build, `weapon="azurelight"`) {
		t.Fatalf("lost library params or player weapon: %d %s", skirkResponse.Code, skirkResponse.Body.String())
	}
	for _, body := range []string{`{"builds":[]}`, `{"builds":["bad","bad","bad","bad"]}`, string(b) + `{}`, `{"builds":[],"command":"run"}`} {
		w = httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("POST", "/lab/input", strings.NewReader(body)))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("accepted invalid input: %d %s", w.Code, w.Body.String())
		}
	}
	// A repeated character and a party absent from the library are explicit errors.
	for _, tc := range []struct {
		tokens []string
		status int
	}{
		{[]string{tokens[0], tokens[0], tokens[2], tokens[3]}, 400},
		{[]string{tokens[0], tokens[1], tokens[2], profile.Characters[4].Build}, 422},
	} {
		b, _ = json.Marshal(map[string]any{"builds": tc.tokens})
		w = httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("POST", "/lab/input", strings.NewReader(string(b))))
		if w.Code != tc.status {
			t.Fatalf("party status %d want %d: %s", w.Code, tc.status, w.Body.String())
		}
	}
}
