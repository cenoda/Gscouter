package main

import (
	"encoding/json"
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
	m.Teams = map[string]engine.ManifestTeam{"a": {Verified: true}, "gone": {Verified: true}, "unverified": {Verified: false}}
	hashes, missing := pin(m, library)
	if hashes["a"] != want || len(hashes) != 1 {
		t.Errorf("hashes %v, want a: %s", hashes, want)
	}
	if len(missing) != 1 || missing[0] != "gone" {
		t.Errorf("missing %v, want [gone]", missing)
	}
}

// Pinning takes off the list the approvals the release drops, and
// -approve-a adds group A's teams with the date of the answer.
func TestApprovals(t *testing.T) {
	dir := t.TempDir()
	list := filepath.Join(dir, "approved.txt")
	os.WriteFile(list, []byte("# header\nkept 2026-11-04\nleft-a 2026-11-04\nold-b 2026-11-04 release=0123456789abcdef\nunverified 2026-11-04\n"), 0o644)
	var m engine.Manifest
	m.Engine.Commit, m.Engine.WasmSHA256, m.Gscouter = "e2", "fedcba9876543210"+strings.Repeat("0", 48), "g2"
	m.Teams = map[string]engine.ManifestTeam{
		"kept":       {Verified: true, Group: "A"},
		"left-a":     {Verified: true, Group: "B", GroupWhy: []string{"2 settled"}},
		"old-b":      {Verified: true, Group: "B"},
		"unverified": {Verified: false},
		"new-a":      {Verified: true, Group: "A"},
		"new-b":      {Verified: true, Group: "B"},
	}
	if err := dropApprovals(list, m, "2026-12-01", nil, nil); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(list)
	got, err := engine.ParseApproved(string(b))
	if err != nil || len(got) != 1 || got[0].Team != "kept" {
		t.Fatalf("after pinning: %v %v\n%s", got, err, b)
	}
	for _, want := range []string{"# header\n", "2026-12-01, pinning engine e2 (module fedcba9876543210)", "#   left-a (approved 2026-11-04): in group B (2 settled)", "#   old-b (approved 2026-11-04): approved from group B for module 0123456789abcdef only", "#   unverified (approved 2026-11-04): not verified"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("list lacks %q:\n%s", want, b)
		}
	}

	rel := filepath.Join(dir, "release")
	os.MkdirAll(rel, 0o755)
	raw, _ := json.Marshal(m)
	os.WriteFile(filepath.Join(rel, "manifest.json"), raw, 0o644)
	if err := approveGroupA(rel, list, "2026-12-02"); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(list)
	got, _ = engine.ParseApproved(string(b))
	if len(got) != 2 || got[1].String() != "new-a 2026-12-02" || !strings.Contains(string(b), "# Group A of engine e2 (module fedcba9876543210), Gscouter g2: the owner's yes of 2026-12-02.") {
		t.Errorf("after -approve-a: %v\n%s", got, b)
	}
	if approveGroupA(rel, list, "2026-12-02") == nil {
		t.Error("-approve-a twice")
	}
	if approveGroupA(rel, list, "tomorrow") == nil {
		t.Error("-approve-a without a date")
	}
}
