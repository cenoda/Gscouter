package web

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The editor's undo: a reset can be taken back, even after its
// recording finishes, fails or is overtaken by another reset; undoing a
// blank chart's first note gives the blank chart back; and the pickup
// switch shows the chart undo gives back (testdata/editor_undo.js).
func TestEditorUndo(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not found")
	}
	_, ts := newClientSite(t)
	_, body := get(t, ts, "/edit?a=800000001:hutao,yelan,xingqiu,zhongli")
	i := strings.Index(body, "var ORDER_DEFAULT = ")
	if i < 0 {
		t.Fatal("no editor script")
	}
	i = strings.LastIndex(body[:i], "<script>") + len("<script>")
	end := strings.Index(body[i:], "</script>")
	if end < 0 {
		t.Fatal("editor script not closed")
	}
	page := filepath.Join(t.TempDir(), "page.js")
	if err := os.WriteFile(page, []byte(body[i:i+end]), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := exec.Command(node, filepath.Join("testdata", "editor_undo.js"), page).Output()
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
