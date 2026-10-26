// Command import adds a library team from a community gcsim share link.
//
// Usage:
//
//	import https://gcsim.app/sh/<key>
//	import https://gcsim.app/db/<id>
//
// It writes teams/<id>/ with team.yaml, rotation.gcsl and the source's
// build as build.txt, then prints what to check. Run cmd/validate next.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cenoda/gscouter/internal/library"
	"github.com/cenoda/gscouter/internal/share"
	"gopkg.in/yaml.v3"
)

// gcsimCommit is the gcsim version in go.mod, recorded on each team.
const gcsimCommit = "2350be9"

func main() {
	root := flag.String("root", "teams", "team library directory")
	force := flag.Bool("force", false, "overwrite an existing team")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: import [-root teams] [-force] <gcsim share link>")
		os.Exit(2)
	}
	if err := run(flag.Arg(0), *root, *force); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(link, root string, force bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	res, err := share.Fetch(ctx, link)
	if err != nil {
		return err
	}
	sp := share.SplitConfig(res.Config)
	if len(sp.Members) != 4 {
		return fmt.Errorf("the config has %d characters, not 4: %v", len(sp.Members), sp.Members)
	}

	var ids, names []string
	for _, m := range sp.Members {
		key := library.Canonical(m)
		ids = append(ids, key)
		names = append(names, displayName(key))
	}
	id := strings.Join(ids, "-")
	dir := filepath.Join(root, id)
	if _, err := os.Stat(dir); err == nil && !force {
		return fmt.Errorf("%s already exists; use -force to replace it", dir)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	source := "https://gcsim.app/sh/" + res.Key
	team := library.Team{
		ID:              id,
		Name:            strings.Join(names, " / "),
		Members:         sp.Members,
		Status:          library.StatusDraft,
		RotationVersion: 1,
		GcsimCommit:     gcsimCommit,
		Sources:         []string{source},
		Notes:           "Imported from a gcsim share.",
		Reference:       &library.Reference{DPS: round(res.DPS), GcsimVersion: short(res.SimVersion)},
	}
	if len(sp.Params) > 0 {
		team.Params = sp.Params
	}
	if link != source {
		team.Sources = append([]string{link}, team.Sources...)
	}
	meta, err := yaml.Marshal(team)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	files := map[string]string{
		"team.yaml":     string(meta),
		"rotation.gcsl": sp.Rotation,
		"build.txt":     "# Build from the source config, used to validate the rotation.\n\n" + sp.Build,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			return err
		}
	}

	fmt.Printf("Wrote %s (%s)\n", dir, team.Name)
	fmt.Printf("Source DPS: %.0f on gcsim %s\n", res.DPS, team.Reference.GcsimVersion)
	if len(sp.Params) > 0 {
		fmt.Printf("Character params: %v\n", sp.Params)
	}
	fmt.Println("Replaced by the common conditions:")
	for _, d := range sp.Dropped {
		fmt.Println("  " + d)
	}
	fmt.Println("Next: set archetype in team.yaml, then run: go run ./cmd/validate -v " + dir)
	return nil
}

// displayName turns a gcsim key into a readable name ("hutao" -> "Hutao").
func displayName(key string) string {
	if key == "" {
		return key
	}
	return strings.ToUpper(key[:1]) + key[1:]
}

func round(v float64) float64 { return float64(int64(v + 0.5)) }

func short(sha string) string { return sha[:min(7, len(sha))] }
