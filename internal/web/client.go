package web

import (
	"bytes"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Result pages can be calculated in the visitor's browser instead of on
// the server, so the server's load does not grow with its visitors: the
// server prepares the teams (Enka lookup, library match), and a
// WebAssembly build of this package (cmd/wasm) runs the simulations in
// Web Workers and renders the page body. The workers run the same
// iterations with the same seeds, so the numbers are the server's.

//go:embed static/*
var staticFS embed.FS

// Job is a result page's calculation, handed to the browser: the teams as
// the server prepared them, and how to score them.
type Job struct {
	Page     string // "score" or "versus"
	Lang     string
	Auto     bool
	URL      bool
	Settings Settings
	Teams    []teamResult
}

// clientFiles are the browser build: gscouter.wasm (and its .gz) and the
// Go runtime's wasm_exec.js, built together with the server.
type clientFiles struct {
	dir     string
	version string // cache-busting hash of the WebAssembly build
}

// ServeClient lets browsers calculate results, with the browser build in
// dir. Without it, the server calculates every result.
func (s *Server) ServeClient(dir string) error {
	wasm, err := os.ReadFile(filepath.Join(dir, "gscouter.wasm"))
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(dir, "wasm_exec.js")); err != nil {
		return err
	}
	sum := sha256.Sum256(wasm)
	s.client = &clientFiles{dir: dir, version: hex.EncodeToString(sum[:6])}
	return nil
}

// inBrowser says whether r's results are calculated by the browser. The
// "server" parameter asks the server to do it, for browsers that cannot.
func (s *Server) inBrowser(r *http.Request) bool {
	return s.client != nil && r.URL.Query().Get("server") == ""
}

// job is the Job for a page whose teams were prepared but not computed,
// or nil when none of them needs computing.
func (s *Server) job(l localizer, page string, teams ...teamResult) *Job {
	for _, tr := range teams {
		if tr.Problem == "" {
			return &Job{Page: page, Lang: l.Lang, Auto: l.Auto, URL: l.URL, Settings: s.settings, Teams: teams}
		}
	}
	return nil
}

// clientData is what the page script needs to run a Job.
type clientData struct {
	Job     *Job
	Version string
	// Fallback is the page calculated on the server, for browsers
	// without WebAssembly or Web Workers.
	Fallback string
}

func (s *Server) clientData(r *http.Request, job *Job) *clientData {
	if job == nil {
		return nil
	}
	q := r.URL.Query()
	q.Set("server", "1")
	return &clientData{Job: job, Version: s.client.version, Fallback: r.URL.Path + "?" + q.Encode()}
}

// RunJob calculates job with sim and renders the page body it is waiting
// for. It returns the body and the page title.
func RunJob(ctx context.Context, job Job, sim Simulator) (body, title string, err error) {
	l := localizer{Lang: job.Lang, Auto: job.Auto, URL: job.URL}
	if catalog[l.Lang] == nil {
		l = english
	}
	tmpl, err := parseTemplates(l)
	if err != nil {
		return "", "", err
	}
	for i := range job.Teams {
		if tr := &job.Teams[i]; tr.Problem == "" && tr.Team != nil && tr.Result == nil {
			compute(ctx, l, sim, job.Settings, tr)
		}
	}
	var buf bytes.Buffer
	switch {
	case job.Page == "score" && len(job.Teams) == 1:
		title = scoreMeta(l, job.Teams[0], "").Title
		err = tmpl.ExecuteTemplate(&buf, "score-body", scoreData{Team: job.Teams[0]})
	case job.Page == "versus" && len(job.Teams) == 2:
		d := newVersus(job.Teams[0], job.Teams[1])
		title = versusMeta(l, d, "").Title
		err = tmpl.ExecuteTemplate(&buf, "versus-body", d)
	default:
		err = errors.New("unknown job")
	}
	return buf.String(), title, err
}

// parseTemplates parses the site's templates in l's language.
func parseTemplates(l localizer) (*template.Template, error) {
	return template.New("").Funcs(funcs).Funcs(localeFuncs(l)).Funcs(artifactFuncs(l)).ParseFS(templateFS, "templates/*.html")
}

// pendingScoreMeta describes a result the browser has yet to calculate.
// Link previews don't run scripts; the card is drawn on the server.
func pendingScoreMeta(l localizer, tr teamResult, base string) pageMeta {
	if tr.Problem != "" {
		return scoreMeta(l, tr, base)
	}
	return pageMeta{
		Title:       l.T("meta.result_title", tr.owner()),
		Description: l.teamName(tr.Team),
		Image:       base + string(link("/card/score.png", tr.Enemy, "a", tr.Selection.String())),
	}
}

func pendingVersusMeta(l localizer, d versusData, base string) pageMeta {
	m := pageMeta{Title: fmt.Sprintf("%s vs %s · Gscouter", d.A.owner(), d.B.owner())}
	if d.A.Problem == "" && d.B.Problem == "" {
		m.Description = l.teamName(d.A.Team) + " vs " + l.teamName(d.B.Team)
		m.Image = base + string(link("/card/versus.png", d.A.Enemy, "a", d.A.Selection.String(), "b", d.B.Selection.String()))
	}
	return m
}

// handleStatic serves the browser build. Its URLs carry the build's
// version, so browsers may keep the files for good.
func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("file")
	if s.client != nil && r.URL.Query().Get("v") == s.client.version {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	switch {
	case name == "worker.js":
		b, _ := staticFS.ReadFile("static/worker.js")
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(b))
	case s.client == nil:
		http.NotFound(w, r)
	case name == "wasm_exec.js":
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		http.ServeFile(w, r, filepath.Join(s.client.dir, name))
	case name == "gscouter.wasm":
		w.Header().Set("Content-Type", "application/wasm")
		w.Header().Add("Vary", "Accept-Encoding")
		path := filepath.Join(s.client.dir, name)
		// The build compresses to a sixth; serve it compressed when it
		// was, instead of compressing 40 MB on every download.
		if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			if f, err := os.Open(path + ".gz"); err == nil {
				defer f.Close()
				w.Header().Set("Content-Encoding", "gzip")
				serveFile(w, r, f)
				return
			}
		}
		f, err := os.Open(path)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		serveFile(w, r, f)
	default:
		http.NotFound(w, r)
	}
}

func serveFile(w http.ResponseWriter, r *http.Request, f *os.File) {
	st, err := f.Stat()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.ServeContent(w, r, "", st.ModTime(), io.ReadSeeker(f))
}
