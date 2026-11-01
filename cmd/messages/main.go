// Command messages lists the messages visitors wrote to the developer
// from the form at the bottom of every page, newest first: each with
// when it was sent and the page it was written on.
//
//	go run ./cmd/messages -links /data/links.db -site https://gscouter.example
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/cenoda/gscouter/internal/links"
)

func main() {
	path := flag.String("links", "", "SQLite file of short links, sends and messages (the server's -links)")
	site := flag.String("site", "", "the site's public URL, put before each page")
	flag.Parse()
	// links.Open creates a missing database, which would read as "no
	// messages yet" for a wrong path.
	if *path == "" {
		log.Fatal("-links is required")
	}
	if _, err := os.Stat(*path); err != nil {
		log.Fatal(err)
	}
	store, err := links.Open(*path)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()
	msgs, err := store.Messages(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	for _, m := range msgs {
		fmt.Printf("── %s", m.Created.Format("2006-01-02 15:04"))
		if m.Page != "" {
			fmt.Printf("  %s%s", strings.TrimSuffix(*site, "/"), m.Page)
		}
		fmt.Printf("\n%s\n\n", m.Text)
	}
	if len(msgs) == 0 {
		fmt.Println("no messages yet")
	}
}
