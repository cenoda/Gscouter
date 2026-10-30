// Package chart is a custom rotation written in the editor
// (docs/cycle-swap.md): notes in order, one per action, on the lanes of
// a team's four members. A chart compiles to gcsl, so it scores, loops
// and is filled like any library rotation, and it travels in result
// links as a short text.
//
// The text form is a list of tokens separated by spaces:
//
//	2E(hold=1) 3E 3Q? ~30 0N [0N 0C 0D]3 ; p0 start_hp=1
//
// A note is a slot (0-3) and an action code, optionally gcsim params in
// parentheses and "?" to play it only when ready. "~N" waits N frames
// before the next note. "[" ... "]N" plays the notes inside N times.
// After ";", "pS k=v ..." are the character params of slot S, and
// "pickup" picks up crystallize shards after every action.
package chart

import (
	"bytes"
	"compress/flate"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Slots are the members of a team in a fixed order: their canonical
// names sorted (library.Key), so a chart does not depend on the order a
// player picked them in.
const Slots = 4

// Limits keep a chart that anyone can write in a link small and its
// simulation bounded.
const (
	MaxNotes   = 400
	MaxWait    = 600 // frames, 10 s
	MaxTimes   = 20
	MaxParams  = 4
	MaxCharKey = 6 // character params per slot
	MaxText    = 8000
)

// Codes are the action codes of the text form, and the gcsim actions
// they stand for.
var Codes = map[byte]string{
	'N': "attack",
	'C': "charge",
	'E': "skill",
	'Q': "burst",
	'A': "aim",
	'H': "high_plunge",
	'L': "low_plunge",
	'D': "dash",
	'J': "jump",
	'W': "walk",
}

var codeOf = func() map[string]byte {
	m := map[string]byte{}
	for c, a := range Codes {
		m[a] = c
	}
	return m
}()

type Chart struct {
	Notes []Note
	// Repeats are blocks of notes played more than once, by index into
	// Notes, sorted and not overlapping.
	Repeats []Repeat
	// Params are each slot's gcsim character params (start_hp=1, ...).
	Params [Slots][]Param
	// Pickup picks up any crystallize shard on the ground after every
	// action, as library rotations do with pick_up_crystallize: the
	// shard's shield triggers Archaic Petra, Navia's Shrapnel and the
	// like (docs/cycle-swap.md §11.7).
	Pickup bool
}

type Note struct {
	Slot   int
	Action string // gcsim action name
	Params []Param
	// Ready plays a skill or burst only when it is ready, and skips it
	// otherwise (gcsl's if .x.burst.ready).
	Ready bool
	// Wait is the frames to wait before the note.
	Wait int
}

type Param struct {
	Key   string
	Value int
}

type Repeat struct {
	From, To int // first and last note, inclusive
	Times    int
}

var keyRe = regexp.MustCompile(`^[a-z][a-z0-9_%]{0,31}$`)

// ErrIdle is a chart that may never move time on: every note plays only
// when ready, and none waits. Looped, gcsim checks the notes over and
// over on one frame once none is ready, and the fight never ends.
var ErrIdle = errors.New("chart has no note that always plays and no wait")

// Validate checks the limits and that the notes are well formed.
func (c *Chart) Validate() error {
	if len(c.Notes) == 0 {
		return errors.New("chart has no notes")
	}
	if len(c.Notes) > MaxNotes {
		return fmt.Errorf("chart has more than %d notes", MaxNotes)
	}
	for i, n := range c.Notes {
		if n.Slot < 0 || n.Slot >= Slots {
			return fmt.Errorf("note %d: no slot %d", i+1, n.Slot)
		}
		if _, ok := codeOf[n.Action]; !ok {
			return fmt.Errorf("note %d: unknown action %q", i+1, n.Action)
		}
		if n.Ready && n.Action != "skill" && n.Action != "burst" {
			return fmt.Errorf("note %d: only a skill or burst waits until ready", i+1)
		}
		if n.Wait < 0 || n.Wait > MaxWait {
			return fmt.Errorf("note %d: wait out of range", i+1)
		}
		if err := checkParams(n.Params, MaxParams); err != nil {
			return fmt.Errorf("note %d: %w", i+1, err)
		}
	}
	// A wait, or an action gcsim is asked for whatever its state, passes
	// time on every pass of the loop: gcsim holds an action until it is
	// ready. An action that takes no time (walk[f=0]) is stopped by the
	// score instead (score.maxStillActions).
	if !slices.ContainsFunc(c.Notes, func(n Note) bool { return !n.Ready || n.Wait > 0 }) {
		return ErrIdle
	}
	end := -1
	for _, r := range c.Repeats {
		if r.From <= end || r.From > r.To || r.To >= len(c.Notes) {
			return errors.New("repeats overlap or fall outside the notes")
		}
		if r.Times < 2 || r.Times > MaxTimes {
			return fmt.Errorf("a repeat plays 2 to %d times", MaxTimes)
		}
		end = r.To
	}
	for _, ps := range c.Params {
		if err := checkParams(ps, MaxCharKey); err != nil {
			return err
		}
	}
	return nil
}

func checkParams(ps []Param, max int) error {
	if len(ps) > max {
		return errors.New("too many params")
	}
	for i, p := range ps {
		if !keyRe.MatchString(p.Key) {
			return fmt.Errorf("bad param name %q", p.Key)
		}
		if p.Value < -100000 || p.Value > 100000 {
			return fmt.Errorf("param %s out of range", p.Key)
		}
		for _, q := range ps[:i] {
			if q.Key == p.Key {
				return fmt.Errorf("param %s twice", p.Key)
			}
		}
	}
	return nil
}

// String is the chart's text form.
func (c *Chart) String() string {
	var toks []string
	for i, n := range c.Notes {
		open := slices.ContainsFunc(c.Repeats, func(r Repeat) bool { return r.From == i })
		if n.Wait > 0 {
			toks = append(toks, fmt.Sprintf("~%d", n.Wait))
			if open {
				toks[len(toks)-1] = "[" + toks[len(toks)-1]
				open = false
			}
		}
		var b strings.Builder
		if open {
			b.WriteByte('[')
		}
		b.WriteByte(byte('0' + n.Slot))
		b.WriteByte(codeOf[n.Action])
		if len(n.Params) > 0 {
			b.WriteByte('(')
			writeParams(&b, n.Params, ",")
			b.WriteByte(')')
		}
		if n.Ready {
			b.WriteByte('?')
		}
		for _, r := range c.Repeats {
			if r.To == i {
				fmt.Fprintf(&b, "]%d", r.Times)
			}
		}
		toks = append(toks, b.String())
	}
	for s, ps := range c.Params {
		if len(ps) > 0 {
			var b strings.Builder
			fmt.Fprintf(&b, "; p%d ", s)
			writeParams(&b, ps, " ")
			toks = append(toks, b.String())
		}
	}
	if c.Pickup {
		toks = append(toks, "; pickup")
	}
	return strings.Join(toks, " ")
}

func writeParams(b *strings.Builder, ps []Param, sep string) {
	for i, p := range ps {
		if i > 0 {
			b.WriteString(sep)
		}
		fmt.Fprintf(b, "%s=%d", p.Key, p.Value)
	}
}

// Parse reads the text form and validates the chart.
func Parse(s string) (*Chart, error) {
	if len(s) > MaxText {
		return nil, errors.New("chart too long")
	}
	notes, params, _ := strings.Cut(s, ";")
	c := &Chart{}
	open := -1
	wait := 0
	for _, tok := range strings.Fields(notes) {
		if strings.HasPrefix(tok, "[") {
			if open >= 0 {
				return nil, errors.New("repeats cannot nest")
			}
			open = len(c.Notes)
			tok = tok[1:]
			if tok == "" {
				continue
			}
		}
		if strings.HasPrefix(tok, "~") {
			v, err := strconv.Atoi(tok[1:])
			if err != nil {
				return nil, fmt.Errorf("bad wait %q", tok)
			}
			wait += v
			continue
		}
		closing, closed := "", false
		if i := strings.IndexByte(tok, ']'); i >= 0 {
			tok, closing, closed = tok[:i], tok[i+1:], true
		}
		if tok != "" {
			n, err := parseNote(tok)
			if err != nil {
				return nil, err
			}
			n.Wait, wait = wait, 0
			c.Notes = append(c.Notes, n)
		}
		if closed {
			if open < 0 || open >= len(c.Notes) {
				return nil, errors.New("unmatched ]")
			}
			times, err := strconv.Atoi(closing)
			if err != nil {
				return nil, fmt.Errorf("bad repeat count %q", closing)
			}
			c.Repeats = append(c.Repeats, Repeat{From: open, To: len(c.Notes) - 1, Times: times})
			open = -1
		}
	}
	if open >= 0 {
		return nil, errors.New("unclosed [")
	}
	if wait > 0 {
		return nil, errors.New("a wait must come before a note")
	}
	for _, part := range strings.Split(params, ";") {
		f := strings.Fields(part)
		if len(f) == 0 {
			continue
		}
		if len(f) == 1 && f[0] == "pickup" {
			if c.Pickup {
				return nil, errors.New("pickup twice")
			}
			c.Pickup = true
			continue
		}
		if len(f[0]) != 2 || f[0][0] != 'p' || f[0][1] < '0' || f[0][1] >= '0'+Slots {
			return nil, fmt.Errorf("bad params %q", f[0])
		}
		slot := int(f[0][1] - '0')
		if c.Params[slot] != nil {
			return nil, fmt.Errorf("params of slot %d twice", slot)
		}
		ps, err := parseParams(f[1:])
		if err != nil {
			return nil, err
		}
		c.Params[slot] = ps
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return c, nil
}

func parseNote(tok string) (Note, error) {
	var n Note
	if len(tok) < 2 || tok[0] < '0' || tok[0] >= '0'+Slots {
		return n, fmt.Errorf("bad note %q", tok)
	}
	n.Slot = int(tok[0] - '0')
	a, ok := Codes[tok[1]]
	if !ok {
		return n, fmt.Errorf("bad note %q", tok)
	}
	n.Action = a
	rest := tok[2:]
	if strings.HasPrefix(rest, "(") {
		end := strings.IndexByte(rest, ')')
		if end < 0 {
			return n, fmt.Errorf("bad note %q", tok)
		}
		ps, err := parseParams(strings.Split(rest[1:end], ","))
		if err != nil {
			return n, err
		}
		n.Params = ps
		rest = rest[end+1:]
	}
	if rest == "?" {
		n.Ready = true
		rest = ""
	}
	if rest != "" {
		return n, fmt.Errorf("bad note %q", tok)
	}
	return n, nil
}

func parseParams(f []string) ([]Param, error) {
	var ps []Param
	for _, kv := range f {
		k, v, ok := strings.Cut(strings.TrimSpace(kv), "=")
		if !ok {
			return nil, fmt.Errorf("bad param %q", kv)
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("bad param %q", kv)
		}
		ps = append(ps, Param{k, n})
	}
	return ps, nil
}

// Encode is the chart for a URL: its text form, deflated, in base64url.
func (c *Chart) Encode() string {
	var buf bytes.Buffer
	w, _ := flate.NewWriter(&buf, flate.BestCompression)
	w.Write([]byte(c.String()))
	w.Close()
	return base64.RawURLEncoding.EncodeToString(buf.Bytes())
}

// Decode reads a chart written by Encode.
func Decode(s string) (*Chart, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, errors.New("chart: bad encoding")
	}
	text, err := io.ReadAll(io.LimitReader(flate.NewReader(bytes.NewReader(raw)), MaxText+1))
	if err != nil {
		return nil, errors.New("chart: bad encoding")
	}
	return Parse(string(text))
}

// Gcsl compiles the chart into a rotation for the members named names,
// by slot. Its setup is the active member, so config.Loop repeats the
// rest.
func (c *Chart) Gcsl(names [Slots]string) string {
	return c.setup() + c.actions(names)
}

// pickupHook makes every action pick up a crystallize shard of any
// element once it ends, if one is on the ground (Chart.Pickup).
const pickupHook = `let gscouter_action = execute_action;
fn execute_action(c number, a number, p map) {
  let done = gscouter_action(c, a, p);
  pick_up_crystallize("any");
  return done;
}
`

// setup is what the chart sets up before its active member.
func (c *Chart) setup() string {
	if c.Pickup {
		return pickupHook
	}
	return ""
}

// actions is the chart's active member and notes, after its setup.
func (c *Chart) actions(names [Slots]string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "active %s;\n", names[c.Notes[0].Slot])
	indent := ""
	for i, n := range c.Notes {
		for k, r := range c.Repeats {
			if r.From == i {
				fmt.Fprintf(&b, "for let r%d = 0; r%d < %d; r%d = r%d + 1 {\n", k, k, r.Times, k, k)
				indent = "  "
			}
		}
		if n.Wait > 0 {
			fmt.Fprintf(&b, "%swait(%d);\n", indent, n.Wait)
		}
		name := names[n.Slot]
		stmt := name + " " + n.Action
		if len(n.Params) > 0 {
			var p strings.Builder
			writeParams(&p, n.Params, ", ")
			stmt += "[" + p.String() + "]"
		}
		switch {
		case n.Ready:
			fmt.Fprintf(&b, "%sif .%s.%s.ready { %s; }\n", indent, name, n.Action, stmt)
		default:
			fmt.Fprintf(&b, "%s%s;\n", indent, stmt)
		}
		if slices.ContainsFunc(c.Repeats, func(r Repeat) bool { return r.To == i }) {
			b.WriteString("}\n")
			indent = ""
		}
	}
	return b.String()
}

// CharParams are slot's character params in the form config.AddParams
// takes ("start_hp=1, start_energy=40").
func (c *Chart) CharParams(slot int) string {
	var b strings.Builder
	writeParams(&b, c.Params[slot], ", ")
	return b.String()
}

// Remap moves the chart to other slots: slots[s] is the slot that slot s
// becomes, or -1 to drop its notes, repeats and params. A repeat that
// loses all its notes goes.
func (c *Chart) Remap(slots [Slots]int) *Chart {
	out := &Chart{}
	index := make([]int, len(c.Notes)) // new index of each note, -1 if dropped
	for i, n := range c.Notes {
		index[i] = -1
		if to := slots[n.Slot]; to >= 0 {
			n.Slot = to
			index[i] = len(out.Notes)
			out.Notes = append(out.Notes, n)
		}
	}
	for _, r := range c.Repeats {
		from, to := -1, -1
		for i := r.From; i <= r.To; i++ {
			if index[i] >= 0 {
				if from < 0 {
					from = index[i]
				}
				to = index[i]
			}
		}
		if from >= 0 {
			out.Repeats = append(out.Repeats, Repeat{From: from, To: to, Times: r.Times})
		}
	}
	for s, ps := range c.Params {
		if to := slots[s]; to >= 0 {
			out.Params[to] = ps
		}
	}
	out.Pickup = c.Pickup
	return out
}
