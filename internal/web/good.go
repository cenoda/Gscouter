package web

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/cenoda/gscouter/internal/enka"
)

// maxGOOD bounds a GOOD file: a whole account's export, materials
// included, is one or two megabytes.
const maxGOOD = 16 << 20

// goodBuild is one character converted from a GOOD file: its key and its
// buildToken, which the browser keeps as it keeps showcase builds.
type goodBuild struct {
	Key   string `json:"key"`
	Build string `json:"b"`
}

type goodResponse struct {
	Builds []goodBuild `json:"builds"`
	// Notes say, in the visitor's language, how many characters were
	// imported and which were not, and why.
	Notes []string `json:"notes"`
	Error string   `json:"error,omitempty"`
}

// handleGOOD converts a GOOD file (the request body) into packed builds
// for the browser to keep with a UID's characters, so that they flow as
// kept showcase builds do (design §4.1, §4.8). The file is read and
// dropped: nothing of it is stored or logged.
func (s *Server) handleGOOD(w http.ResponseWriter, r *http.Request) {
	l := localizerFor(r)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxGOOD))
	var resp goodResponse
	var chars []enka.Character
	var skipped []enka.GOODSkipped
	if err == nil {
		chars, skipped, err = enka.ParseGOOD(raw)
	}
	var tooLarge *http.MaxBytesError
	switch {
	case errors.As(err, &tooLarge):
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		resp.Error = l.T("index.good_bad")
	case err != nil:
		w.WriteHeader(http.StatusBadRequest)
		resp.Error = l.T("index.good_bad")
	default:
		resp.Builds = []goodBuild{}
		for _, c := range chars {
			resp.Builds = append(resp.Builds, goodBuild{Key: c.Key, Build: buildToken(c)})
		}
		resp.Notes = append(resp.Notes, l.T("index.good_done", len(chars)))
		for _, sk := range skipped {
			resp.Notes = append(resp.Notes, l.T("index.good_skipped", sk.Key, sk.Reason))
		}
	}
	json.NewEncoder(w).Encode(resp)
}
