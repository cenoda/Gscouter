// Command pinengine pins a gscouter-engine release (docs/engine.md §3):
// from the files of an engine-release artifact, it writes
// internal/engine/release/gse.wasm and manifest.json, and teams.txt, the
// hash of each verified team's library files at the Gscouter commit the
// release verified (the manifest's gscouter_commit, read from git), so
// that a team changed since runs on gcsim.
//
// Usage, from the repository's root:
//
//	pinengine -from <artifact dir>
//
// The commit must be in the clone's history (git fetch it if not).
//
// Pinning also takes off the approved list (internal/engine/approved.txt)
// every approval the new release drops (engine.Dropped: the team is not
// verified, has left group A, or was approved from group B for another
// release), under a comment saying why: such a team comes back to the
// owner before it runs on the engine again (docs/engine.md §2).
//
// After the owner's yes for the pinned release's group A, record it:
//
//	pinengine -approve-a <date of the answer>
//
// adds every group A team not yet listed, one line each with that date,
// under a comment naming the release. Run it only after the answer.
package main

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing/fstest"
	"time"

	"github.com/cenoda/gscouter/internal/engine"
	"github.com/cenoda/gscouter/teams"
)

func main() {
	from := flag.String("from", "", "directory of the engine-release artifact (gse_wasm.wasm or gse.wasm, manifest.json)")
	to := flag.String("to", "internal/engine/release", "directory to write the pinned release to")
	repo := flag.String("repo", ".", "the Gscouter clone whose history holds the manifest's commit")
	approved := flag.String("approved", "internal/engine/approved.txt", "the approved list, from which pinning takes the approvals the release drops")
	approveA := flag.String("approve-a", "", "the date (YYYY-MM-DD) of the owner's yes for the pinned release's group A: add its teams to the approved list, and pin nothing")
	flag.Parse()
	var err error
	if *approveA != "" {
		err = approveGroupA(*to, *approved, *approveA)
	} else {
		err = run(*from, *to, *repo, *approved)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "pinengine:", err)
		os.Exit(1)
	}
}

func run(from, to, repo, approved string) error {
	if from == "" {
		return errors.New("want -from")
	}
	module, err := os.ReadFile(filepath.Join(from, "gse_wasm.wasm"))
	if errors.Is(err, fs.ErrNotExist) {
		module, err = os.ReadFile(filepath.Join(from, "gse.wasm"))
	}
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(filepath.Join(from, "manifest.json"))
	if err != nil {
		return err
	}
	var m engine.Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return fmt.Errorf("manifest: %w", err)
	}
	if sum := sha256.Sum256(module); hex.EncodeToString(sum[:]) != m.Engine.WasmSHA256 {
		return errors.New("the module is not the manifest's (SHA-256 differs)")
	}
	if m.Gscouter == "" {
		return errors.New("the manifest names no gscouter_commit")
	}
	library, err := teamsAt(repo, m.Gscouter)
	if err != nil {
		return err
	}
	hashes, missing := pin(m, library)
	for _, id := range missing {
		fmt.Fprintf(os.Stderr, "pinengine: %s is verified but has no library files at %s; it stays on gcsim\n", id, m.Gscouter)
	}
	// Teams that changed since the commit, as the server will see them.
	for id, h := range hashes {
		if now, err := engine.TeamHash(teams.FS, id); err != nil || now != h {
			fmt.Printf("changed since %s, stays on gcsim: %s\n", m.Gscouter, id)
		}
	}
	if err := os.MkdirAll(to, 0o755); err != nil {
		return err
	}
	for name, b := range map[string][]byte{"gse.wasm": module, "manifest.json": raw, "teams.txt": []byte(engine.FormatPinned(m.Gscouter, hashes))} {
		if err := os.WriteFile(filepath.Join(to, name), b, 0o644); err != nil {
			return err
		}
	}
	fmt.Printf("pinned engine %s: %d verified teams at Gscouter %s\n", m.Engine.Commit, len(hashes), m.Gscouter)
	return dropApprovals(approved, m, time.Now().Format(time.DateOnly))
}

// dropApprovals takes off the approved list at path the approvals the
// release of m drops, and writes why in a comment at its end, dated
// today.
func dropApprovals(path string, m engine.Manifest, today string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	list, err := engine.ParseApproved(string(raw))
	if err != nil {
		return err
	}
	why := map[string]string{}
	for _, a := range list {
		if w := engine.Dropped(m, a); w != "" {
			why[a.Team] = fmt.Sprintf("%s (approved %s): %s", a.Team, a.Date, w)
		}
	}
	if len(why) == 0 {
		return nil
	}
	var keep []string
	var dropped []string
	for _, line := range strings.SplitAfter(string(raw), "\n") {
		f := strings.Fields(line)
		if len(f) > 0 && !strings.HasPrefix(f[0], "#") && why[f[0]] != "" {
			dropped = append(dropped, why[f[0]])
			continue
		}
		keep = append(keep, line)
	}
	out := strings.Join(keep, "")
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	out += fmt.Sprintf("\n# %s, pinning engine %s (module %s): dropped, each back to the owner\n# before it runs on the engine again:\n", today, m.Engine.Commit, short(m.Engine.WasmSHA256))
	for _, d := range dropped {
		out += "#   " + d + "\n"
		fmt.Println("approval dropped:", d)
	}
	return os.WriteFile(path, []byte(out), 0o644)
}

// approveGroupA adds the pinned release's group A teams not yet on the
// approved list at path, dated date, the day of the owner's yes.
func approveGroupA(release, path, date string) error {
	if _, err := time.Parse(time.DateOnly, date); err != nil {
		return fmt.Errorf("-approve-a: want the date of the owner's answer, YYYY-MM-DD, got %q", date)
	}
	rawM, err := os.ReadFile(filepath.Join(release, "manifest.json"))
	if err != nil {
		return err
	}
	var m engine.Manifest
	if err := json.Unmarshal(rawM, &m); err != nil {
		return fmt.Errorf("manifest: %w", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	list, err := engine.ParseApproved(string(raw))
	if err != nil {
		return err
	}
	listed := map[string]bool{}
	for _, a := range list {
		listed[a.Team] = true
	}
	var ids []string
	for id, t := range m.Teams {
		if t.Verified && t.Group == "A" && !listed[id] {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return errors.New("-approve-a: every group A team of the pinned release is listed already")
	}
	sort.Strings(ids)
	out := string(raw)
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	out += fmt.Sprintf("\n# Group A of engine %s (module %s), Gscouter %s: the owner's yes of %s.\n", m.Engine.Commit, short(m.Engine.WasmSHA256), short(m.Gscouter), date)
	for _, id := range ids {
		out += id + " " + date + "\n"
	}
	if _, err := engine.ParseApproved(out); err != nil {
		return err
	}
	fmt.Printf("approved %d group A teams, %s\n", len(ids), date)
	return os.WriteFile(path, []byte(out), 0o644)
}

func short(s string) string {
	if len(s) > 16 {
		return s[:16]
	}
	return s
}

// pin hashes each verified team's files in library; missing lists the
// verified teams library does not have.
func pin(m engine.Manifest, library fs.FS) (hashes map[string]string, missing []string) {
	hashes = map[string]string{}
	for id, t := range m.Teams {
		if !t.Verified {
			continue
		}
		h, err := engine.TeamHash(library, id)
		if err != nil {
			missing = append(missing, id)
			continue
		}
		hashes[id] = h
	}
	return hashes, missing
}

// teamsAt is teams/ at commit, from git, laid out as teams.FS is.
func teamsAt(repo, commit string) (fs.FS, error) {
	cmd := exec.Command("git", "-C", repo, "archive", "--format=tar", commit, "teams/")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git archive %s (fetch the commit if the clone lacks it): %v: %s", commit, err, strings.TrimSpace(stderr.String()))
	}
	files := fstest.MapFS{}
	tr := tar.NewReader(bytes.NewReader(out))
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return files, nil
		}
		if err != nil {
			return nil, err
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		b, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}
		files[strings.TrimPrefix(h.Name, "teams/")] = &fstest.MapFile{Data: b}
	}
}
