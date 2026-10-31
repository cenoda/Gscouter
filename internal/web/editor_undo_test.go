package web

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/cenoda/gscouter/internal/enka"
)

// The editor's undo: a reset can be taken back, even after its
// recording finishes, fails or is overtaken by another reset; undoing a
// blank chart's first note gives the blank chart back; the pickup
// switch shows the chart undo gives back; an edit made while a reset
// records outlives the reset's late answer, recorded or failed; and the
// saved-chart list's placeholder loads nothing; and Skirk's params sheet
// has one starting-resource control, start_energy, with the legacy
// start_serpents_subtlety folded into it (testdata/editor_undo.js).
func TestEditorUndo(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not found")
	}
	_, ts := newClientSite(t)
	page := editorScript(t, ts, "/edit?a=800000001:hutao,yelan,xingqiu,zhongli")
	skirk := editorScript(t, ts, "/edit?a=800000006:skirk,furina,citlali,escoffier")
	slots := slotted(skirkTeam())
	slot := slices.IndexFunc(slots[:], func(c enka.Character) bool { return c.Key == "skirk" })
	b, err := exec.Command(node, filepath.Join("testdata", "editor_undo.js"), page, skirk, strconv.Itoa(slot)).Output()
	if err != nil {
		t.Fatalf("node: %v", err)
	}
	var out struct{ Errors []string }
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("%v: %s", err, b)
	}
	for _, e := range out.Errors {
		t.Error(e)
	}
}

// editorScript is the editor page's script at path, written to a file.
func editorScript(t *testing.T, ts *httptest.Server, path string) string {
	t.Helper()
	_, body := get(t, ts, path)
	i := strings.Index(body, "var ORDER_DEFAULT = ")
	if i < 0 {
		t.Fatalf("%s: no editor script", path)
	}
	i = strings.LastIndex(body[:i], "<script>") + len("<script>")
	end := strings.Index(body[i:], "</script>")
	if end < 0 {
		t.Fatalf("%s: editor script not closed", path)
	}
	page := filepath.Join(t.TempDir(), "page.js")
	if err := os.WriteFile(page, []byte(body[i:i+end]), 0o644); err != nil {
		t.Fatal(err)
	}
	return page
}
