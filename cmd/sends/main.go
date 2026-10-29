// Command sends lists the rotation charts visitors sent to the developer
// from result pages (docs/cycle-swap.md §7), newest first: each as the
// result link, which calculates the chart against the library's
// rotations again when opened.
//
//	go run ./cmd/sends -links /data/links.db -site https://gscouter.example
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/cenoda/gscouter/internal/links"
)

func main() {
	path := flag.String("links", "links.db", "SQLite file of short links and sends")
	site := flag.String("site", "", "the site's public URL, put before each link")
	flag.Parse()
	store, err := links.Open(*path)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()
	sends, err := store.Sends(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	for _, s := range sends {
		fmt.Printf("%s  %s%s\n", s.Created.Format("2006-01-02 15:04"), strings.TrimSuffix(*site, "/"), s.Target)
	}
	if len(sends) == 0 {
		fmt.Println("no rotations sent yet")
	}
}
