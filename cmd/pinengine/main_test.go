package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cenoda/gscouter/internal/engine"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// The hashes are of the files at the manifest's commit, not of the
// working tree, and cover a team's alternatives.
func TestPinReadsTheCommit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	dir := t.TempDir()
	write := func(name, body string) {
		p := filepath.Join(dir, "teams", name)
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git(t, dir, "init", "-q")
	write("a/team.yaml", "id: a\n")
	write("a/rotation.gcsl", "x attack;\n")
	write("a/alt/b/rotation.gcsl", "y attack;\n")
	write("a/source.txt", "not scored\n")
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-q", "-m", "one")
	commit := git(t, dir, "rev-parse", "HEAD")
	at := os.DirFS(filepath.Join(dir, "teams"))
	want, err := engine.TeamHash(at, "a")
	if err != nil {
		t.Fatal(err)
	}
	write("a/alt/b/rotation.gcsl", "y skill;\n")
	if changed, _ := engine.TeamHash(at, "a"); changed == want {
		t.Fatal("an alternative's rotation does not change the hash")
	}
	write("a/source.txt", "changed\n")

	library, err := teamsAt(dir, commit)
	if err != nil {
		t.Fatal(err)
	}
	var m engine.Manifest
	m.Teams = map[string]struct {
		Verified bool `json:"verified"`
	}{"a": {true}, "gone": {true}, "unverified": {false}}
	hashes, missing := pin(m, library)
	if hashes["a"] != want || len(hashes) != 1 {
		t.Errorf("hashes %v, want a: %s", hashes, want)
	}
	if len(missing) != 1 || missing[0] != "gone" {
		t.Errorf("missing %v, want [gone]", missing)
	}
}
