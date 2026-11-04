package engine

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
)

// scoring are the library files of a team that its scores depend on, as
// the server embeds them (teams/embed.go): the team's own and each
// alternative's (alt/<name>/).
var scoring = map[string]bool{"team.yaml": true, "rotation.gcsl": true, "build.txt": true}

// TeamHash is the SHA-256 of team id's library files in library, a tree
// laid out as teams/ is (id/team.yaml, id/alt/<name>/rotation.gcsl, ...):
// their names and contents, length-prefixed, in name order. A release
// pins each verified team's hash at the manifest's Gscouter commit
// (release/teams.txt, cmd/pinengine), and a team whose files differ
// from it runs on gcsim (docs/engine.md §3).
func TeamHash(library fs.FS, id string) (string, error) {
	var names []string
	err := fs.WalkDir(library, id, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(p, id+"/")
		dir := path.Dir(rel)
		if !d.IsDir() && scoring[d.Name()] && (dir == "." || path.Dir(dir) == "alt") {
			names = append(names, p)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(names) == 0 {
		return "", fmt.Errorf("teams/%s: no library files", id)
	}
	sort.Strings(names)
	h := sha256.New()
	for _, p := range names {
		b, err := fs.ReadFile(library, p)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(h, "%d:%s%d:", len(p), p, len(b))
		h.Write(b)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ParsePinned reads release/teams.txt: lines of a team id and its
// TeamHash; blank lines and lines starting with # are skipped.
func ParsePinned(text string) (map[string]string, error) {
	out := map[string]string{}
	sc := bufio.NewScanner(strings.NewReader(text))
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Fields(line)
		if len(f) != 2 || len(f[1]) != 64 {
			return nil, fmt.Errorf("teams.txt, line %d: want a team id and a SHA-256, got %q", n, line)
		}
		out[f[0]] = f[1]
	}
	return out, sc.Err()
}

// FormatPinned writes ParsePinned's file for the hashes of a release
// whose manifest names Gscouter at commit.
func FormatPinned(commit string, hashes map[string]string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Library files of each verified team at Gscouter %s, the manifest's\n# gscouter_commit (engine.TeamHash). Written by cmd/pinengine.\n", commit)
	ids := make([]string, 0, len(hashes))
	for id := range hashes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		fmt.Fprintf(&b, "%s %s\n", id, hashes[id])
	}
	return b.String()
}
