package web

import (
	"net/http"

	"github.com/cenoda/gscouter/internal/library"
	"github.com/cenoda/gscouter/internal/score"
)

// libraryTeamData is a library team's page: its rotation, sources and
// score with the reference build, for visitors who search for a team.
type libraryTeamData struct {
	Meta     pageMeta
	Team     *library.Team
	Chars    []string // canonical member keys
	Warnings []string
	Lang     string // langQuery, for links to other indexed pages
}

func (s *Server) handleLibraryTeam(w http.ResponseWriter, r *http.Request) {
	l := localizerFor(r)
	t := s.byID[r.PathValue("id")]
	if t == nil {
		s.notFound(w, l)
		return
	}
	data := libraryTeamData{
		Team:     t,
		Warnings: warnings(l, t, nil, &score.Result{}),
		Lang:     langQuery(r),
	}
	for _, m := range t.Members {
		data.Chars = append(data.Chars, library.Canonical(m))
	}
	name := l.teamName(t)
	m := pageMeta{Title: l.T("lib.title", name), Description: l.T("lib.desc", name)}
	if dps := referenceScore(t); dps > 0 {
		m.Description += " " + l.T("lib.score_desc", thousands(float64(dps)))
	}
	data.Meta = s.indexed(r, l, m, teamPath(t))
	s.render(w, l, "library_team.html", data)
}

// characterData is a character's page: the library teams with them, by
// score with the reference build.
type characterData struct {
	Meta  pageMeta
	Key   string // canonical
	Teams []*library.Team
	Lang  string // langQuery
}

func (s *Server) handleCharacter(w http.ResponseWriter, r *http.Request) {
	l := localizerFor(r)
	key := r.PathValue("key")
	teams := s.byChar[key]
	if teams == nil {
		s.notFound(w, l)
		return
	}
	name := l.charName(key)
	m := pageMeta{Title: l.T("char.title", name), Description: l.T("char.desc", name, len(teams))}
	s.render(w, l, "character.html", characterData{
		Meta:  s.indexed(r, l, m, characterPath(key)),
		Key:   key,
		Teams: teams,
		Lang:  langQuery(r),
	})
}

func (s *Server) notFound(w http.ResponseWriter, l localizer) {
	s.writeHeader(w, l)
	w.WriteHeader(http.StatusNotFound)
	s.render(w, l, "problem.html", l.T("err.not_found"))
}
