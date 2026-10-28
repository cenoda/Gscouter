package web

import (
	"bytes"
	"embed"
	"net/http"
	"time"
)

// The site icons are served from the root rather than /static/, which
// robots.txt keeps crawlers out of: search engines show the favicon next
// to results only when they may fetch it. icons/favicon.svg is the source;
// the PNG and ICO files are renders of it.

//go:embed icons/*
var iconFS embed.FS

var iconTypes = map[string]string{
	"favicon.ico":          "image/x-icon",
	"favicon.svg":          "image/svg+xml",
	"apple-touch-icon.png": "image/png",
	"icon-512.png":         "image/png",
}

func handleIcon(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Path[1:]
	b, err := iconFS.ReadFile("icons/" + name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", iconTypes[name])
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(b))
}
