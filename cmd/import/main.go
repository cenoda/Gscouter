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
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/cenoda/gscouter/internal/library"
	"github.com/cenoda/gscouter/internal/share"
)

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
	source := "https://gcsim.app/sh/" + res.Key
	sources := []string{source}
	if link != source {
		sources = append([]string{link}, sources...)
	}
	ref := &library.Reference{DPS: round(res.DPS), GcsimVersion: short(res.SimVersion)}
	team, err := sp.Team(sources, ref, "Imported from a gcsim share.")
	if err != nil {
		return err
	}
	dir, err := sp.Write(root, team, force)
	if err != nil {
		return err
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

func round(v float64) float64 { return float64(int64(v + 0.5)) }

func short(sha string) string { return sha[:min(7, len(sha))] }
