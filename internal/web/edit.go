package web

import (
	"cmp"
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/cenoda/gscouter/internal/chart"
	"github.com/cenoda/gscouter/internal/config"
	"github.com/cenoda/gscouter/internal/enka"
	"github.com/cenoda/gscouter/internal/library"
)

// editData is the chart editor (docs/cycle-swap.md): the player's team,
// what the browser needs to record and preview charts, and the chart.
type editData struct {
	Team teamResult
	// Members are the player's characters by chart slot.
	Members [chart.Slots]enka.Character
	// Names are their gcsim keys, Defaults the library team's character
	// params for each, by slot.
	Names, Defaults [chart.Slots]string
	// Build is the players' character lines, without params.
	Build string
	// Rotation is the library rotation as the player's names write it,
	// which the browser records into the first chart.
	Rotation string
	// Chart is the chart's text form from the link, "" to record one.
	Chart string
	// Score is the result page's link without its chart.
	Score   string
	Version string
}

// slotted orders chars by chart slot: their canonical names sorted, as
// library.Key sorts them.
func slotted(chars []enka.Character) [chart.Slots]enka.Character {
	s := slices.Clone(chars)
	slices.SortStableFunc(s, func(a, b enka.Character) int {
		return strings.Compare(library.Canonical(a.Key), library.Canonical(b.Key))
	})
	var out [chart.Slots]enka.Character
	copy(out[:], s)
	return out
}

// buildLines are the players' character lines for a gcsim config.
func buildLines(chars []enka.Character) string {
	var b strings.Builder
	for _, c := range chars {
		b.WriteString(c.Config())
		b.WriteString("\n")
	}
	return b.String()
}

// renames maps the team's names for its members to the player's keys.
func renames(t *library.Team, chars []enka.Character) map[string]string {
	out := map[string]string{}
	for _, c := range chars {
		for _, m := range t.Members {
			if library.Canonical(m) == library.Canonical(c.Key) {
				out[m] = c.Key
			}
		}
	}
	return out
}

func (s *Server) handleEdit(w http.ResponseWriter, r *http.Request) {
	l := localizerFor(r)
	sel, err := selectionFromQuery(r, "a")
	if err != nil {
		s.renderProblem(w, l, l.message(err))
		return
	}
	text := ""
	if c := r.URL.Query().Get("r"); c != "" {
		ch, err := chart.Decode(c)
		if err != nil {
			s.renderProblem(w, l, l.T("problem.chart"))
			return
		}
		text = ch.String()
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	sel = s.withBuilds(ctx, sel, keptBuilds(r))
	kv := []string{"a", sel.String()}
	if c := r.URL.Query().Get("r"); c != "" {
		kv = append(kv, "r", c)
	}
	if canonical(w, r, "/edit", nil, kv...) {
		return
	}
	if s.client == nil {
		s.renderProblem(w, l, l.T("problem.editor"))
		return
	}
	tr := s.prepare(ctx, l, sel, nil)
	if tr.Problem != "" {
		s.renderProblem(w, l, tr.Problem)
		return
	}
	d := editData{
		Team:    tr,
		Members: slotted(tr.Chars),
		Build:   buildLines(tr.Chars),
		Chart:   text,
		Score:   string(link("/score", nil, "a", sel.String())),
		Version: s.client.version,
	}
	d.Rotation = config.RenameCharacters(tr.Team.Rotation, renames(tr.Team, tr.Chars))
	d.Names, d.Defaults = chartSlots(tr.Team, tr.Chars)
	s.render(w, l, "edit.html", d)
}

// chartSlots are the gcsim keys of a player's characters by chart slot,
// and the team's character params for each.
func chartSlots(t *library.Team, chars []enka.Character) (names, defaults [chart.Slots]string) {
	for i, c := range slotted(chars) {
		names[i] = c.Key
		if t == nil {
			continue
		}
		for _, m := range t.Members {
			if library.Canonical(m) == library.Canonical(c.Key) {
				defaults[i] = cmp.Or(t.Params[m], t.Params[c.Key])
			}
		}
	}
	return names, defaults
}

// signedPercent is a relative change, e.g. "+12.3%".
func signedPercent(v float64) string {
	return fmt.Sprintf("%+.1f%%", v*100)
}
