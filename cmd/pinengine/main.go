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
	"strings"
	"testing/fstest"

	"github.com/cenoda/gscouter/internal/engine"
	"github.com/cenoda/gscouter/teams"
)

func main() {
	from := flag.String("from", "", "directory of the engine-release artifact (gse_wasm.wasm or gse.wasm, manifest.json)")
	to := flag.String("to", "internal/engine/release", "directory to write the pinned release to")
	repo := flag.String("repo", ".", "the Gscouter clone whose history holds the manifest's commit")
	flag.Parse()
	if err := run(*from, *to, *repo); err != nil {
		fmt.Fprintln(os.Stderr, "pinengine:", err)
		os.Exit(1)
	}
}

func run(from, to, repo string) error {
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
	return nil
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
