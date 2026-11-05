// Command server runs the Gscouter website.
package main

import (
	"cmp"
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cenoda/gscouter/internal/engine"
	"github.com/cenoda/gscouter/internal/enka"
	"github.com/cenoda/gscouter/internal/library"
	"github.com/cenoda/gscouter/internal/links"
	"github.com/cenoda/gscouter/internal/web"
	"github.com/cenoda/gscouter/teams"
)

func main() {
	s := web.DefaultSettings
	addr := flag.String("addr", ":"+cmp.Or(os.Getenv("PORT"), "8080"), "listen address")
	flag.IntVar(&s.Iterations, "iter", s.Iterations, "iterations per score")
	flag.Float64Var(&s.Warmup, "warmup", s.Warmup, "seconds excluded from the score")
	flag.Float64Var(&s.Duration, "duration", s.Duration, "fight duration in seconds")
	flag.IntVar(&s.MaxConcurrent, "concurrent", s.MaxConcurrent, "simulations running at once")
	flag.IntVar(&s.MaxQueue, "queue", s.MaxQueue, "simulations waiting before requests are rejected")
	static := flag.String("static", os.Getenv("GSCOUTER_STATIC"), "directory of the browser build (cmd/wasm); browsers then calculate results")
	rotationLab := flag.String("rotation-lab", os.Getenv("GSCOUTER_ROTATION_LAB"), "directory with the optional browser rotation lab's gso.wasm, dictionary.json and manifest.json")
	linksDB := flag.String("links", os.Getenv("GSCOUTER_LINKS"), "SQLite file of short links; none turns them off")
	site := flag.String("site", os.Getenv("GSCOUTER_SITE"), "the site's public URL, e.g. https://gscouter.example; without it, links for search engines take each request's host")
	indexNow := flag.String("indexnow", os.Getenv("GSCOUTER_INDEXNOW"), "state file of IndexNow submissions (needs -site); none turns them off")
	usageFile := flag.String("usage", os.Getenv("GSCOUTER_USAGE"), "JSON file of daily usage counts; none turns them off")
	useEngine := flag.Bool("engine", false, "score the approved teams on gscouter-engine (internal/engine); false runs every team on gcsim")
	engineDir := flag.String("engine-dir", "", "directory with approved.txt, gse.wasm, manifest.json and teams.txt to use instead of the pinned release, to try one")
	flag.BoolVar(&s.EngineOwnSearch, "engine-own-search", s.EngineOwnSearch, "engine teams run their own filler search, ranked on 50 iterations, and use its pick (docs/engine.md §5); false keeps gcsim's pick")
	flag.Parse()

	lib, err := library.LoadLibrary(teams.FS)
	if err != nil {
		slog.Error("load team library", "err", err)
		os.Exit(1)
	}
	srv, err := web.New(lib, web.NewEnkaSource(enka.NewClient()), s)
	if err != nil {
		slog.Error("init server", "err", err)
		os.Exit(1)
	}

	if *static != "" {
		if err := srv.ServeClient(*static); err != nil {
			slog.Error("browser build", "dir", *static, "err", err)
			os.Exit(1)
		}
		slog.Info("browsers calculate results", "dir", *static)
	}
	if *rotationLab != "" {
		if err := srv.SetRotationLab(*rotationLab); err != nil {
			slog.Error("rotation lab", "dir", *rotationLab, "err", err)
			os.Exit(1)
		}
		slog.Info("browser rotation lab", "dir", *rotationLab, "path", "/lab")
	}

	if *useEngine {
		// A release that does not load leaves every team on gcsim.
		e, skipped, err := engine.Release()
		if *engineDir != "" {
			e, skipped, err = engine.ReleaseDir(*engineDir)
		}
		switch {
		case err != nil:
			slog.Error("engine off", "err", err)
		case e == nil:
			slog.Info("engine off: no release pinned")
		default:
			srv.SetEngine(e)
			slog.Info("engine", "commit", e.Commit(), "module", e.Hash()[:16], "teams", len(e.Teams()))
		}
		for _, s := range skipped {
			slog.Warn("approved team stays on gcsim", "why", s)
		}
	}

	if on(*linksDB) {
		// Without the store the site still works: copying a result
		// link copies the long link.
		if store, err := links.Open(*linksDB); err != nil {
			slog.Error("short links off", "db", *linksDB, "err", err)
		} else {
			defer store.Close()
			srv.SetLinks(store)
			srv.SetSends(store)
			srv.SetMessages(store)
			slog.Info("short links", "db", *linksDB)
		}
	}

	if *site != "" {
		srv.SetSite(*site)
	}
	if on(*indexNow) && *site != "" {
		// Search engines are told what changed; the site works the
		// same without it.
		if err := srv.SetIndexNow(*indexNow); err != nil {
			slog.Error("IndexNow off", "file", *indexNow, "err", err)
		} else {
			go submitChanges(srv)
		}
	}

	if on(*usageFile) {
		// Counts only: views and distinct UIDs per day (see web.SetUsage).
		if err := srv.SetUsage(*usageFile); err != nil {
			slog.Error("usage counts off", "file", *usageFile, "err", err)
		} else {
			slog.Info("usage counts", "file", *usageFile)
			srv.SetUsageToken(os.Getenv("GSCOUTER_USAGE_TOKEN"))
		}
	}

	httpSrv := &http.Server{
		Addr:              *addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	// ListenAndServe returns as soon as Shutdown starts; main waits for
	// Shutdown to finish the requests in flight before it flushes usage
	// and closes the link store (deferred above).
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := httpSrv.Shutdown(shutdown); err != nil {
			slog.Error("shutdown", "err", err)
		}
	}()

	slog.Info("listening", "addr", *addr, "teams", len(lib.Teams))
	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("serve", "err", err)
		os.Exit(1)
	}
	<-drained
	if err := srv.FlushUsage(); err != nil {
		slog.Error("usage", "err", err)
	}
}

// on reports whether a file flag is set: empty or "none" turns its
// feature off, so a flag can override its environment variable.
func on(file string) bool {
	return file != "" && file != "none"
}

// submitChanges tells search engines which pages changed, once the site
// is up to serve the key file they check submissions against.
func submitChanges(srv *web.Server) {
	time.Sleep(time.Minute)
	n, err := srv.SubmitChanges(context.Background())
	if err != nil {
		slog.Error("IndexNow", "submitted", n, "err", err)
		return
	}
	slog.Info("IndexNow", "submitted", n)
}
