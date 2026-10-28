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
	linksDB := flag.String("links", os.Getenv("GSCOUTER_LINKS"), "SQLite file of short links; none turns them off")
	site := flag.String("site", os.Getenv("GSCOUTER_SITE"), "the site's public URL, e.g. https://gscouter.example; without it, links for search engines take each request's host")
	indexNow := flag.String("indexnow", os.Getenv("GSCOUTER_INDEXNOW"), "state file of IndexNow submissions (needs -site); none turns them off")
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

	if *linksDB != "" {
		// Without the store the site still works: copying a result
		// link copies the long link.
		if store, err := links.Open(*linksDB); err != nil {
			slog.Error("short links off", "db", *linksDB, "err", err)
		} else {
			defer store.Close()
			srv.SetLinks(store)
			slog.Info("short links", "db", *linksDB)
		}
	}

	if *site != "" {
		srv.SetSite(*site)
	}
	if *indexNow != "" && *site != "" {
		// Search engines are told what changed; the site works the
		// same without it.
		if err := srv.SetIndexNow(*indexNow); err != nil {
			slog.Error("IndexNow off", "file", *indexNow, "err", err)
		} else {
			go submitChanges(srv)
		}
	}

	httpSrv := &http.Server{
		Addr:              *addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		httpSrv.Shutdown(shutdown)
	}()

	slog.Info("listening", "addr", *addr, "teams", len(lib.Teams))
	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("serve", "err", err)
		os.Exit(1)
	}
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
