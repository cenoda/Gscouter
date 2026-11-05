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
// Pinning carries forward each group B approval of the pinned module (the
// owner, 2026-11-05): the team's library build runs on the pinned and the
// new module at its verdict's iterations, seed 1 and a warmup of 150 s,
// and when team DPS and every character's DPS are identical to the last
// digit, its line gets release=<new module> and
// carried=<old module>:identical:<today>, keeping the owner's date. The
// pinned module (-to) must be there to carry from; without it pinning
// refuses, unless -carry=false drops them.
//
// Pinning then takes off the approved list (internal/engine/approved.txt)
// every approval the new release drops (engine.Dropped: the team is not
// verified, has left group A, or was approved from group B for another
// release and did not carry), under a comment saying why: such a team
// comes back to the owner before it runs on the engine again
// (docs/engine.md §2).
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
	"context"
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
	carryB := flag.Bool("carry", true, "carry group B approvals of the pinned module forward when the new module scores their teams identically; false drops them")
	approveA := flag.String("approve-a", "", "the date (YYYY-MM-DD) of the owner's yes for the pinned release's group A: add its teams to the approved list, and pin nothing")
	flag.Parse()
	var err error
	if *approveA != "" {
		err = approveGroupA(*to, *approved, *approveA)
	} else {
		err = run(*from, *to, *repo, *approved, *carryB)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "pinengine:", err)
		os.Exit(1)
	}
}

func run(from, to, repo, approved string, carryB bool) error {
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
	today := time.Now().Format(time.DateOnly)
	rawList, err := os.ReadFile(approved)
	if err != nil {
		return err
	}
	list, err := engine.ParseApproved(string(rawList))
	if err != nil {
		return err
	}
	var carried map[string]engine.Approval
	var why map[string]string
	if carryB {
		prev, err := pinnedRelease(to, repo)
		if err != nil {
			return err
		}
		next := &release{module: module, manifest: m, library: library, runner: engine.ModuleRunner(module)}
		if carried, why, err = carry(context.Background(), list, prev, next, today); err != nil {
			return err
		}
		for _, a := range carried {
			fmt.Printf("approval carried, scores identical: %s\n", a)
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
	return dropApprovals(approved, m, today, carried, why)
}

// pinnedRelease is the release pinned in dir, to carry approvals from,
// with its library at its Gscouter commit; nil without a module there.
func pinnedRelease(dir, repo string) (*release, error) {
	module, err := os.ReadFile(filepath.Join(dir, "gse.wasm"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return nil, err
	}
	var m engine.Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("pinned manifest: %w", err)
	}
	library, err := teamsAt(repo, m.Gscouter)
	if err != nil {
		return nil, err
	}
	return &release{module: module, manifest: m, library: library, runner: engine.ModuleRunner(module)}, nil
}

// dropApprovals rewrites the approved list at path for the release of
// m: the lines of carried approvals get their new release and carry, and
// the approvals the release drops are taken off, with why (why's reason
// for a group B approval that did not carry, else engine.Dropped's) in a
// comment at its end, dated today.
func dropApprovals(path string, m engine.Manifest, today string, carried map[string]engine.Approval, carryWhy map[string]string) error {
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
		if _, ok := carried[a.Team]; ok {
			continue
		}
		w := engine.Dropped(m, a)
		if cw := carryWhy[a.Team]; cw != "" && w != "" {
			w = "approved from group B for module " + a.Release + ", not carried: " + cw
		}
		if w != "" {
			why[a.Team] = fmt.Sprintf("%s (approved %s): %s", a.Team, a.Date, w)
		}
	}
	if len(why) == 0 && len(carried) == 0 {
		return nil
	}
	var keep []string
	var dropped []string
	for _, line := range strings.SplitAfter(string(raw), "\n") {
		f := strings.Fields(line)
		if len(f) > 0 && !strings.HasPrefix(f[0], "#") {
			if why[f[0]] != "" {
				dropped = append(dropped, why[f[0]])
				continue
			}
			if c, ok := carried[f[0]]; ok {
				line = c.String() + "\n"
			}
		}
		keep = append(keep, line)
	}
	out := strings.Join(keep, "")
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	if len(dropped) > 0 {
		out += fmt.Sprintf("\n# %s, pinning engine %s (module %s): dropped, each back to the owner\n# before it runs on the engine again:\n", today, m.Engine.Commit, short(m.Engine.WasmSHA256))
		for _, d := range dropped {
			out += "#   " + d + "\n"
			fmt.Println("approval dropped:", d)
		}
	}
	if _, err := engine.ParseApproved(out); err != nil {
		return err
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
