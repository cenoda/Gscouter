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

// goodNote is one line of a GOOD import's notes. Lang is set on a line
// that is not in the page's language (an unexpected reason, in English),
// for the browser to mark it.
type goodNote struct {
	Text string `json:"text"`
	Lang string `json:"lang,omitempty"`
}

type goodResponse struct {
	Builds []goodBuild `json:"builds"`
	// Notes say, in the visitor's language, how many characters were
	// imported and which were not, and why.
	Notes []goodNote `json:"notes"`
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
		resp.Notes = append(resp.Notes, goodNote{Text: l.T("index.good_done", len(chars))})
		for _, sk := range skipped {
			resp.Notes = append(resp.Notes, l.goodSkipped(sk))
		}
	}
	json.NewEncoder(w).Encode(resp)
}

// goodSkipped says why a GOOD character was not imported, with its
// in-game name once: "Not imported: Hu Tao (no weapon equipped)". A
// reason the catalog does not know stays in English, marked so.
func (l localizer) goodSkipped(sk enka.GOODSkipped) goodNote {
	name := sk.Key
	if enka.CharacterName(enkaKey(sk.Char), l.Lang) != "" {
		name = l.charName(sk.Char)
	}
	key := "good." + sk.Reason.Code()
	if _, ok := catalog[defaultLang][key]; !ok {
		return goodNote{Text: english.T("index.good_skipped", sk.Key, sk.Message()), Lang: langEnglish}
	}
	args := make([]any, len(sk.Args))
	for i, a := range sk.Args {
		if slot, ok := a.(enka.Slot); ok {
			a = l.T("art.slot." + slot.String())
		}
		args[i] = a
	}
	return goodNote{Text: l.T("index.good_skipped", name, l.T(key, args...))}
}
