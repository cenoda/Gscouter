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
	// Order is the party as the player picked it: the chart slot at each
	// position, which the number keys and the lanes follow until the
	// player drags it into another order.
	Order [chart.Slots]int
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
	// LibraryPickup says whether the library rotation picks up
	// crystallize shards, which its recorded chart then does too.
	LibraryPickup bool
	// Score is the result page's link without its chart.
	Score   string
	Version string
	// For a team the library doesn't have, a library team sharing three
	// members is the first chart's template: its build, rotation and
	// members by slot, and Moves, the player's slot for each of them (-1
	// for the one the player doesn't have). Template is nil otherwise.
	Template      *library.Team
	TemplateBuild string
	TemplateNames [chart.Slots]string
	Moves         [chart.Slots]int
}

// customTeam stands in for a team the library doesn't have, scored with
// the player's own rotation: it has members and no rotation.
func customTeam(chars []string) *library.Team {
	return &library.Team{Members: chars, Status: library.StatusSupported}
}

// templateFor is a library team sharing three of chars' members, the
// most popular by its database entries' order (lowest ID first among
// equals), or nil.
func (s *Server) templateFor(chars []string) *library.Team {
	want := map[string]bool{}
	for _, c := range chars {
		want[library.Canonical(c)] = true
	}
	var best *library.Team
	for _, t := range s.lib.Teams {
		if t.Status != library.StatusSupported || t.Rotation == "" {
			continue
		}
		n := 0
		for _, m := range t.Members {
			if want[library.Canonical(m)] {
				n++
			}
		}
		if n == 3 && (best == nil || t.ID < best.ID) {
			best = t
		}
	}
	return best
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
			s.renderProblem(w, l, l.message(chartError(err)))
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
	if tr.Writable {
		tr.Problem, tr.Team = "", customTeam(sel.Chars)
	}
	if tr.Problem != "" {
		s.renderProblem(w, l, tr.Problem)
		return
	}
	d := editData{
		Team:    tr,
		Members: slotted(tr.Chars),
		Build:   buildLines(tr.Chars),
		Chart:   text,
		Score:   string(l.link("/score", nil, "a", sel.String())),
		Version: s.client.version,
	}
	d.Rotation = config.RenameCharacters(tr.Team.Rotation, renames(tr.Team, tr.Chars))
	d.LibraryPickup = strings.Contains(d.Rotation, "pick_up_crystallize")
	d.Names, d.Defaults = chartSlots(tr.Team, tr.Chars)
	d.Order = partyOrder(tr.Chars)
	if tr.Team.Rotation == "" {
		if t := s.templateFor(sel.Chars); t != nil {
			d.Template = t
			d.TemplateBuild = t.Build
			for m, p := range t.Params {
				d.TemplateBuild = config.AddParams(d.TemplateBuild, m, p)
			}
			members := slices.Clone(t.Members)
			slices.SortFunc(members, func(a, b string) int { return strings.Compare(library.Canonical(a), library.Canonical(b)) })
			for i, m := range members {
				d.TemplateNames[i] = m
				d.Moves[i] = slices.IndexFunc(d.Names[:], func(n string) bool { return library.Canonical(n) == library.Canonical(m) })
			}
		}
	}
	s.render(w, l, "edit.html", d)
}

// partyOrder is the chart slot of each of chars, in their order.
func partyOrder(chars []enka.Character) [chart.Slots]int {
	var out [chart.Slots]int
	slots := slotted(chars)
	for i := range out {
		out[i] = i
	}
	for i, c := range chars {
		if i < chart.Slots {
			out[i] = slices.IndexFunc(slots[:], func(s enka.Character) bool { return s.Key == c.Key })
		}
	}
	return out
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
