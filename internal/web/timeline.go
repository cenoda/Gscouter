package web

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/cenoda/gscouter/internal/score"
	"github.com/cenoda/gscouter/internal/timeline"
)

// Timeline windows (design §4.5): the one pass is shown up to maxCycle
// seconds, and the looped fight from the warmup for one pass's length,
// within [minSteady, maxSteady].
const (
	maxCycle  = 120.0
	minSteady = 20.0
	maxSteady = 60.0
)

// timeline runs cfg once and reduces seconds [from, to) of it, reusing the
// cached result for the same config and window. It waits for the same
// simulation slot as score.
func (s *Server) timeline(ctx context.Context, cfg string, from, to float64) (*timeline.Timeline, error) {
	sum := sha256.Sum256(fmt.Appendf(nil, "timeline %v %v\n%s", from, to, cfg))
	key := hex.EncodeToString(sum[:])
	if t, ok := s.timelines.get(key); ok {
		return t, nil
	}
	if int(s.waiting.Add(1)) > s.settings.MaxQueue+cap(s.sem) {
		s.waiting.Add(-1)
		return nil, errBusy
	}
	defer s.waiting.Add(-1)
	select {
	case s.sem <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-s.sem }()

	res, err := score.Once(cfg, 1)
	if err != nil {
		return nil, err
	}
	t := timeline.Build(res, int(from*60), int(to*60))
	s.timelines.put(key, t, 24*time.Hour)
	return t, nil
}

// Layout of the timeline drawing, in pixels.
const (
	pxPerSec   = 24
	axisH      = 22
	chartH     = 76
	laneH      = 40
	laneGap    = 6
	chartInset = 6 // space above the tallest bar
)

// timelineView is a timeline laid out for the template.
type timelineView struct {
	ID       string // unique on the page, for the tab controls
	Label    string
	Note     string
	Width    float64
	Height   float64
	ChartTop float64
	ChartBot float64
	LaneTop  float64
	Ticks    []tick
	Bars     []bar
	Lanes    []lane
	Stints   []stintView
	Damage   float64
	DPS      float64
	Seconds  float64
	Sources  []sourceGroup
}

type tick struct {
	X     float64
	Label string
}

// bar is one damage bucket: a hit target over the column with stacked
// segments, one per character with damage.
type bar struct {
	X, W float64
	Tip  string
	Segs []seg
}

type seg struct {
	Slot int
	Y, H float64
}

type lane struct {
	Slot    int
	Name    string
	Y       float64 // top
	Mid     float64
	Field   []rect
	Waits   []rect
	Actions []mark
}

type rect struct {
	X, W float64
	Tip  string
}

type mark struct {
	X     float64
	Kind  string // "skill", "burst" or "minor"
	Label string // letter drawn in the mark
	Tip   string
}

type stintView struct {
	Slot     int
	Name     string
	Start    float64
	Duration float64
	Actions  []string
	Total    float64
	Share    float64 // of the window's damage
	Scale    float64 // percent of the largest stint's damage, for the bar
	Parts    []part
}

// part is one character's slice of a stacked share bar.
type part struct {
	Slot  int
	Name  string
	Width float64 // percent
	Tip   string
}

type sourceGroup struct {
	Slot    int
	Name    string
	Damage  float64
	Share   float64 // of the window's damage
	Sources []sourceView
}

type sourceView struct {
	Name     string
	Hits     int
	CritRate float64
	Damage   float64
	Share    float64 // of the character's damage
}

// actionNames are short Korean names of gcsim actions.
var actionNames = map[string]string{
	"attack":      "평타",
	"charge":      "강공격",
	"skill":       "E",
	"burst":       "Q",
	"dash":        "대시",
	"jump":        "점프",
	"walk":        "걷기",
	"aim":         "조준",
	"high_plunge": "낙하",
	"low_plunge":  "낙하",
}

// actionLong names an action in a tooltip.
var actionLong = map[string]string{
	"attack":      "일반 공격",
	"charge":      "강공격",
	"skill":       "원소전투 스킬",
	"burst":       "원소폭발",
	"dash":        "대시",
	"jump":        "점프",
	"walk":        "걷기",
	"aim":         "조준 사격",
	"high_plunge": "낙하 공격",
	"low_plunge":  "낙하 공격",
}

var waitNames = map[string]string{
	"insufficient_energy":  "원소 에너지 부족",
	"skill_cd":             "원소전투 스킬 쿨타임",
	"burst_cd":             "원소폭발 쿨타임",
	"insufficient_stamina": "스태미나 부족",
}

func or(m map[string]string, k string) string {
	if v, ok := m[k]; ok {
		return v
	}
	return k
}

func seconds(v float64) string { return fmt.Sprintf("%.1f초", v) }

// layoutTimeline lays t out. Characters are colored by party slot.
func layoutTimeline(id, label, note string, t *timeline.Timeline) *timelineView {
	span := t.End - t.Start
	v := &timelineView{
		ID: id, Label: label, Note: note,
		Width:    math.Ceil(span*pxPerSec) + 8,
		ChartTop: axisH,
		ChartBot: axisH + chartH,
		LaneTop:  axisH + chartH + laneGap,
		Damage:   t.Damage,
		Seconds:  span,
	}
	v.Height = v.LaneTop + float64(len(t.Characters)*laneH)
	if span > 0 {
		v.DPS = t.Damage / span
	}
	x := func(sec float64) float64 { return (sec - t.Start) * pxPerSec }

	step := 5.0
	if span > 90 {
		step = 10
	}
	for s := math.Ceil(t.Start/step) * step; s <= t.End; s += step {
		tk := tick{X: x(s)}
		// A label that would run off the right edge is left out.
		if tk.X+28 <= v.Width {
			tk.Label = fmt.Sprintf("%.0fs", s)
		}
		v.Ticks = append(v.Ticks, tk)
	}

	// Damage per bucket, stacked by slot, scaled to the largest bucket.
	var peak float64
	n := 0
	for _, c := range t.Characters {
		n = max(n, len(c.Buckets))
	}
	totals := make([]float64, n)
	for i := range totals {
		for _, c := range t.Characters {
			if i < len(c.Buckets) {
				totals[i] += c.Buckets[i]
			}
		}
		peak = math.Max(peak, totals[i])
	}
	const barGap = 4.0
	bw := timeline.BucketSeconds * pxPerSec
	for i, total := range totals {
		if total <= 0 {
			continue
		}
		start := t.Start + float64(i)*timeline.BucketSeconds
		b := bar{X: x(start), W: bw}
		var tip strings.Builder
		fmt.Fprintf(&tip, "%.0f–%.0f초 · 피해 %s", start, start+timeline.BucketSeconds, thousands(total))
		base := v.ChartBot
		for slot, c := range t.Characters {
			if i >= len(c.Buckets) || c.Buckets[i] <= 0 {
				continue
			}
			h := c.Buckets[i] / peak * (chartH - chartInset)
			if c.Buckets[i] >= 0.5 {
				fmt.Fprintf(&tip, "\n%s %s", c.Name, thousands(c.Buckets[i]))
			}
			// A 1px surface gap between stacked segments.
			drawn := math.Max(h-1, 0.5)
			b.Segs = append(b.Segs, seg{Slot: slot, Y: base - h, H: drawn})
			base -= h
		}
		b.Tip = tip.String()
		// The visible bar is narrower than its hit target.
		b.X += barGap / 2
		b.W -= barGap
		v.Bars = append(v.Bars, b)
	}

	for slot, c := range t.Characters {
		l := lane{Slot: slot, Name: c.Name, Y: v.LaneTop + float64(slot*laneH)}
		l.Mid = l.Y + laneH/2
		for _, f := range c.Field {
			l.Field = append(l.Field, rect{X: x(f.Start), W: (f.End - f.Start) * pxPerSec,
				Tip: fmt.Sprintf("%s 필드 %.1f–%.1f초 (%s)", c.Name, f.Start, f.End, seconds(f.End-f.Start))})
		}
		for _, w := range c.Waits {
			l.Waits = append(l.Waits, rect{X: x(w.Start), W: math.Max((w.End-w.Start)*pxPerSec, 2),
				Tip: fmt.Sprintf("%s 대기 %s · %s", c.Name, seconds(w.End-w.Start), or(waitNames, w.Reason))})
		}
		for _, a := range c.Actions {
			m := mark{X: x(a.Time), Kind: "minor", Tip: fmt.Sprintf("%.1f초 · %s %s", a.Time, c.Name, or(actionLong, a.Kind))}
			switch a.Kind {
			case "skill":
				m.Kind, m.Label = "skill", "E"
			case "burst":
				m.Kind, m.Label = "burst", "Q"
			case "charge", "aim", "high_plunge", "low_plunge":
				m.Kind = "heavy"
			}
			l.Actions = append(l.Actions, m)
		}
		v.Lanes = append(v.Lanes, l)
	}

	var top float64
	for _, s := range t.Stints {
		top = math.Max(top, s.Total)
	}
	for _, s := range t.Stints {
		sv := stintView{Slot: s.Char, Name: t.Characters[s.Char].Name, Start: s.Start, Duration: s.End - s.Start, Total: s.Total}
		if t.Damage > 0 {
			sv.Share = s.Total / t.Damage
		}
		if top > 0 {
			sv.Scale = s.Total / top * 100
		}
		sv.Actions = actionChips(s.Actions)
		for slot, d := range s.Damage {
			if d > 0 && s.Total > 0 {
				sv.Parts = append(sv.Parts, part{Slot: slot, Name: t.Characters[slot].Name, Width: d / s.Total * 100,
					Tip: fmt.Sprintf("%s %s (%.0f%%)", t.Characters[slot].Name, thousands(d), d/s.Total*100)})
			}
		}
		v.Stints = append(v.Stints, sv)
	}

	for slot, c := range t.Characters {
		g := sourceGroup{Slot: slot, Name: c.Name, Damage: c.Damage}
		if t.Damage > 0 {
			g.Share = c.Damage / t.Damage
		}
		for _, s := range c.Sources {
			sv := sourceView{Name: s.Name, Hits: s.Hits, Damage: s.Damage}
			if s.Hits > 0 {
				sv.CritRate = float64(s.Crits) / float64(s.Hits)
			}
			if c.Damage > 0 {
				sv.Share = s.Damage / c.Damage
			}
			g.Sources = append(g.Sources, sv)
		}
		v.Sources = append(v.Sources, g)
	}
	return v
}

// actionChips names a stint's actions, folding a repeated pattern into
// one chip: "평타×2 · 강공격 · 대시" seven times becomes
// "(평타×2 · 강공격 · 대시) ×7".
func actionChips(runs []timeline.Run) []string {
	name := func(r timeline.Run) string {
		n := or(actionNames, r.Kind)
		if r.Count > 1 {
			n += fmt.Sprintf("×%d", r.Count)
		}
		return n
	}
	var out []string
	for i := 0; i < len(runs); {
		// The pattern covering the most runs from i, shortest first.
		bestK, bestR := 1, 1
		for k := 2; k <= 6 && i+2*k <= len(runs); k++ {
			r := 1
			for i+(r+1)*k <= len(runs) && slices.Equal(runs[i:i+k], runs[i+r*k:i+(r+1)*k]) {
				r++
			}
			if r > 1 && k*r > bestK*bestR {
				bestK, bestR = k, r
			}
		}
		if bestR == 1 {
			out = append(out, name(runs[i]))
			i++
			continue
		}
		names := make([]string, bestK)
		for j := range names {
			names[j] = name(runs[i+j])
		}
		out = append(out, fmt.Sprintf("(%s) ×%d", strings.Join(names, " · "), bestR))
		i += bestK * bestR
	}
	return out
}
