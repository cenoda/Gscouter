package chart

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/cenoda/gscouter/internal/config"
	"github.com/cenoda/gscouter/internal/library"
	"github.com/cenoda/gscouter/internal/score"
	"github.com/cenoda/gscouter/internal/timeline"
	"github.com/genshinsim/gcsim/pkg/core"
	"github.com/genshinsim/gcsim/pkg/core/action"
	"github.com/genshinsim/gcsim/pkg/core/event"
	"github.com/genshinsim/gcsim/pkg/stats"
)

// PreviewPasses is how many times a preview plays the chart: the first
// from full energy, the rest show where the loop runs short.
const PreviewPasses = 3

// previewFight bounds a preview, in seconds.
const previewFight = 240

// energyStep is how often a preview samples energy and cooldowns, in
// frames.
const energyStep = 15

// Preview is one run of a chart looped PreviewPasses times, laid out for
// the editor: where each note landed, and each lane's time on field,
// waits, skill cooldowns and burst energy. Times are in seconds.
type Preview struct {
	Notes  []Placed    `json:"notes"`
	Passes []float64   `json:"passes"` // when each pass started
	End    float64     `json:"end"`
	Lanes  [Slots]Lane `json:"lanes"`
	DPS    float64     `json:"dps"` // from the second pass on, waits filled (filledDPS)
	// Error is gcsim's, when the fight stopped at a note.
	Error *NoteError `json:"error,omitempty"`
}

// Placed is a note as played: a repeated note is placed once per play.
// A note played only when ready that was not is not placed.
type Placed struct {
	Note  int     `json:"note"`
	Pass  int     `json:"pass"`
	Asked float64 `json:"asked"` // when the chart got to it
	Start float64 `json:"start"` // when it ran; -1 if it never did
}

type Lane struct {
	Field    []timeline.Span `json:"field"`
	Waits    []timeline.Wait `json:"waits"`
	Cooldown []timeline.Span `json:"cooldown"` // the skill's
	Energy   [][2]float64    `json:"energy"`   // [time, energy / burst cost]
}

type NoteError struct {
	Note    int    `json:"note"`
	Message string `json:"message"`
}

// WithParams adds to build each slot's character params: defaults (the
// library team's, as "k=v, k=v"), with the chart's own in their place.
func (c *Chart) WithParams(build string, names, defaults [Slots]string) string {
	for s, name := range names {
		var ps []Param
		for _, kv := range strings.Split(defaults[s], ",") {
			k, v, ok := strings.Cut(strings.TrimSpace(kv), "=")
			n, err := strconv.Atoi(strings.TrimSpace(v))
			if ok && err == nil && !slices.ContainsFunc(c.Params[s], func(p Param) bool { return p.Key == strings.TrimSpace(k) }) {
				ps = append(ps, Param{strings.TrimSpace(k), n})
			}
		}
		ps = append(ps, c.Params[s]...)
		var b strings.Builder
		writeParams(&b, ps, ", ")
		build = config.AddParams(build, name, b.String())
	}
	return build
}

// marked is the chart looped, with a marker starting each pass and one
// before each note, which tells Play which note the fight is at.
func (c *Chart) marked(names [Slots]string) string {
	src := c.actions(names)
	var b strings.Builder
	b.WriteString(c.setup())
	note := 0
	for _, line := range strings.SplitAfter(src, "\n") {
		t := strings.TrimSpace(line)
		switch {
		case t == "":
			continue
		case strings.HasPrefix(t, "active "):
			b.WriteString(line)
			fmt.Fprintf(&b, "while 1 {\ndelay(%d);\n", score.Marker)
			continue
		case strings.HasPrefix(t, "for ") || t == "}":
		case strings.HasPrefix(t, "wait(") && note == len(c.Notes):
			// The final wait has no note of its own: the last note's
			// time runs on into it.
		case strings.HasPrefix(t, "wait("):
			// The note's marker goes before its wait, so the wait counts
			// as the note's own time.
			fmt.Fprintf(&b, "delay(%d);\n", score.Marker+1+note)
			b.WriteString(line)
			continue
		default:
			if note == 0 || !strings.HasPrefix(lastLine(b.String()), "wait(") {
				fmt.Fprintf(&b, "delay(%d);\n", score.Marker+1+note)
			}
			note++
		}
		b.WriteString(line)
	}
	b.WriteString("}\n")
	return b.String()
}

func lastLine(s string) string {
	s = strings.TrimRight(s, "\n")
	return strings.TrimSpace(s[strings.LastIndexByte(s, '\n')+1:])
}

// Preview plays the chart with build (setup lines with the params
// already added, as WithParams gives) for the members named names, by
// slot, against the standard enemy unless enemy is set.
func (c *Chart) Preview(build string, names [Slots]string, enemy *config.Enemy) (*Preview, error) {
	cfg := config.Assemble(config.Conditions{Iterations: 1, Duration: previewFight, Enemy: enemy}, build, c.marked(names))
	ctx, stop := context.WithCancel(context.Background())
	defer stop()

	p := &Preview{}
	sec := func(f int) float64 { return float64(f) / 60 }
	slotOf := map[int]int{} // party index -> slot, once the party is set up
	pending := -1           // index in p.Notes of the note being played
	lane := func(core *core.Core, i int) *Lane {
		if len(slotOf) == 0 {
			for i, ch := range core.Player.Chars() {
				for s, n := range names {
					if library.Canonical(n) == library.Canonical(ch.Base.Key.String()) {
						slotOf[i] = s
					}
				}
			}
		}
		return &p.Lanes[slotOf[i]]
	}
	setup := func(core *core.Core) {
		core.Events.Subscribe(event.OnActionExec, func(args ...any) {
			a := args[1].(action.Action)
			if a == action.ActionSwap || pending < 0 || p.Notes[pending].Start >= 0 {
				return
			}
			p.Notes[pending].Start = sec(core.F)
		}, "gscouter-preview")
		core.Events.Subscribe(event.OnTick, func(...any) {
			if core.F%energyStep != 0 {
				return
			}
			for i, ch := range core.Player.Chars() {
				l := lane(core, i)
				if ch.EnergyMax > 0 {
					l.Energy = append(l.Energy, [2]float64{sec(core.F), ch.Energy / ch.EnergyMax})
				}
				// The skill's cooldown, as a span from when it started
				// to when it ends as of now.
				if cd := ch.Cooldown(action.ActionSkill); cd > 0 {
					end := sec(core.F + cd)
					if n := len(l.Cooldown); n > 0 && l.Cooldown[n-1].End >= sec(core.F-energyStep)-1e-9 {
						l.Cooldown[n-1].End = end
					} else {
						l.Cooldown = append(l.Cooldown, timeline.Span{Start: sec(core.F), End: end})
					}
				}
			}
		}, "gscouter-preview-energy")
	}
	res, err := score.Play(ctx, cfg, 1, setup, func(core *core.Core, e *action.Eval) {
		if !score.IsMarker(e) {
			return
		}
		if f := e.Param["f"] - score.Marker; f > 0 {
			p.Notes = append(p.Notes, Placed{Note: f - 1, Pass: len(p.Passes) - 1, Asked: sec(core.F), Start: -1})
			pending = len(p.Notes) - 1
			return
		}
		if len(p.Passes) == PreviewPasses {
			stop()
			return
		}
		p.Passes = append(p.Passes, sec(core.F))
	})
	if res == nil {
		return nil, err
	}
	if err != nil && !errors.Is(err, context.Canceled) {
		if pending < 0 {
			return nil, err
		}
		p.Error = &NoteError{Note: p.Notes[pending].Note, Message: err.Error()}
	}
	end := res.Duration
	p.End = sec(end)
	tl := timeline.Build(res, nil, 0, end)
	for i, ch := range tl.Characters {
		l := &p.Lanes[slotOf[i]]
		l.Field, l.Waits = ch.Field, ch.Waits
	}
	p.DPS = passDPS(res, p.Passes)
	if p.Error == nil {
		p.DPS = max(p.DPS, c.filledDPS(build, names, enemy, p.Lanes))
	}
	return p, nil
}

// passDPS is a preview run's DPS from the second pass on, as the score
// leaves out the opener; over the whole run when the first pass never
// ended.
func passDPS(res *stats.Result, passes []float64) float64 {
	end := float64(res.Duration) / 60
	from := 0.0
	if len(passes) > 1 && end > passes[1] {
		from = passes[1]
	}
	var dmg float64
	for _, ch := range res.Characters {
		for _, d := range ch.DamageEvents {
			if float64(d.Frame)/60 >= from {
				dmg += d.Damage
			}
		}
	}
	if end <= from {
		return 0
	}
	return dmg / (end - from)
}

// filledDPS is the best DPS of the chart played with the fillers the
// score tries first (steady.Score) for the members lanes show waiting:
// each way of handling a wait for burst energy, with skill cooldowns
// filled. The score fills such waits, so a chart that stands still
// waiting for a burst would otherwise preview far below its score. The
// lanes keep the chart as written, waits and all. 0 when no member
// waits.
func (c *Chart) filledDPS(build string, names [Slots]string, enemy *config.Enemy, lanes [Slots]Lane) float64 {
	var energy, skill []string
	for s, l := range lanes {
		for _, w := range l.Waits {
			switch w.Reason {
			case "insufficient_energy":
				if !slices.Contains(energy, names[s]) {
					energy = append(energy, names[s])
				}
			case "skill_cd":
				if !slices.Contains(skill, names[s]) {
					skill = append(skill, names[s])
				}
			}
		}
	}
	if len(energy) == 0 && len(skill) == 0 {
		return 0
	}
	marked := c.marked(names)
	ways := []config.Filler{config.FillSelf, config.FillParty, config.FillSkip}
	if len(energy) == 0 {
		ways = ways[:1]
	}
	best := 0.0
	for _, f := range ways {
		m := map[string]config.Filler{}
		for _, n := range energy {
			m[n] = f
		}
		rotation := config.Fill(marked, m, skill, names[:])
		cfg := config.Assemble(config.Conditions{Iterations: 1, Duration: previewFight, Enemy: enemy}, build, rotation)
		ctx, stop := context.WithCancel(context.Background())
		var passes []float64
		res, err := score.Play(ctx, cfg, 1, nil, func(core *core.Core, e *action.Eval) {
			if !score.IsMarker(e) || e.Param["f"] > score.Marker {
				return
			}
			if len(passes) == PreviewPasses {
				stop()
				return
			}
			passes = append(passes, float64(core.F)/60)
		})
		stop()
		// A filler can lead the chart into actions gcsim rejects; that
		// filler is then not an option, as in the score.
		if res == nil || err != nil && !errors.Is(err, context.Canceled) {
			continue
		}
		best = max(best, passDPS(res, passes))
	}
	return best
}
