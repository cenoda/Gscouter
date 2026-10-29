package config

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// Filler says how a character spends the time a rotation would otherwise
// stand still waiting for its burst's energy.
type Filler int

const (
	// FillSelf: the character uses its own skill when ready, otherwise
	// attacks.
	FillSelf Filler = iota
	// FillParty: as FillSelf, but when its skill is on cooldown another
	// member whose skill is ready swaps in and uses it; the character's
	// next action swaps back, in time to catch the particles.
	FillParty
	// FillSkip: the character passes its burst when it lacks the energy
	// and the rotation moves on, as Hu Tao teams play. It is also the
	// only way out for a rotation that waits for a burst it cannot
	// charge, such as Skirk's opening burst, written for her full
	// starting Serpent's Subtlety, when the rotation loops.
	FillSkip
	// FillBrief: as FillParty, but the character attacks at most
	// maxFillAttacks times and then passes a burst still not ready, as
	// FillSkip. A player swaps out rather than stand on a support until
	// its burst charges: Ineffa is played with two skills to a burst.
	FillBrief
)

// maxFillAttacks is how many attacks FillBrief makes before passing the
// burst.
const maxFillAttacks = 2

func (f Filler) String() string {
	switch f {
	case FillParty:
		return "party skills"
	case FillSkip:
		return "skipped bursts"
	case FillBrief:
		return "party skills, then skipped bursts"
	}
	return "own skill"
}

// Fill rewrites a rotation so that characters stop standing still:
//
//   - before each burst of a character in energy, it fills until the
//     burst is ready, or skips the burst when it is not ready, as
//     energy maps the character (see Filler);
//   - before each skill of a character in skill, it attacks until the
//     skill is ready.
//
// Either filler gives up after a fixed number of actions, and the
// rotation then waits as before. party lists every member, and energy
// and skill name members as party does. A rotation may write a member
// under one of gcsim's aliases ("chev" for "chevreuse", "amy" for
// "fischl"): its statements are matched by gcsim key, and a filled
// statement is written under the party's name.
func Fill(rotation string, energy map[string]Filler, skill, party []string) string {
	forms := skillForms(rotation, party)
	var b strings.Builder
	n := 0
	start := true // at the start of a statement
	for i := 0; i < len(rotation); {
		if start {
			if m := actionRe.FindStringSubmatchIndex(rotation[i:]); m != nil {
				lead := rotation[i+m[2] : i+m[3]]
				char := member(rotation[i+m[4]:i+m[5]], party)
				items := rotation[i+m[6] : i+m[7]]
				how, inEnergy := energy[char]
				if inEnergy || slices.Contains(skill, char) {
					b.WriteString(lead)
					b.WriteString(fillStatement(char, items, inEnergy, slices.Contains(skill, char), party, forms, how, &n))
					i += m[1]
					continue
				}
			}
		}
		c := rotation[i]
		b.WriteByte(c)
		i++
		switch c {
		case ';', '{', '}':
			start = true
		case ' ', '\t', '\n', '\r':
		default:
			start = false
		}
	}
	return b.String()
}

// actionRe matches an action statement at the start of the text:
// "<char> <action>, <action>...;".
var actionRe = regexp.MustCompile(`^(\s*)(\w+)\s+([^;{}()=\s][^;{}()]*);`)

func fillStatement(char, items string, energy, skill bool, party []string, forms map[string]string, how Filler, n *int) string {
	var out, run []string
	flush := func() {
		if len(run) > 0 {
			out = append(out, char+" "+strings.Join(run, ", ")+";")
			run = nil
		}
	}
	for _, item := range strings.Split(items, ",") {
		item = strings.TrimSpace(item)
		action := strings.FieldsFunc(item, func(r rune) bool { return r == '[' || r == ':' })
		if len(action) == 0 {
			continue
		}
		switch {
		case energy && action[0] == "burst" && (how == FillSkip || how == FillBrief):
			flush()
			if how == FillBrief {
				*n++
				out = append(out, energyFiller(char, party, forms, how, fmt.Sprintf("gs_fill%d", *n)))
			}
			// The frame waited keeps time passing in a loop whose only
			// actions are skipped bursts; gcsim cannot stop a loop that
			// takes no time.
			out = append(out, fmt.Sprintf("if .%s.burst.ready { %s %s; } else { wait(1); }", char, char, item))
			continue
		case energy && action[0] == "burst":
			flush()
			*n++
			out = append(out, energyFiller(char, party, forms, how, fmt.Sprintf("gs_fill%d", *n)))
		case skill && action[0] == "skill":
			flush()
			*n++
			v := fmt.Sprintf("gs_fill%d", *n)
			out = append(out, fmt.Sprintf("let %s = 0; while !.%s.skill.ready && %s < 30 { %s attack; %s = %s + 1; }", v, char, v, char, v, v))
		}
		run = append(run, item)
	}
	flush()
	return strings.Join(out, " ")
}

func energyFiller(char string, party []string, forms map[string]string, how Filler, v string) string {
	var body strings.Builder
	fmt.Fprintf(&body, "if .%s.skill.ready { %s %s; }", char, char, forms[char])
	if how == FillParty || how == FillBrief {
		for _, m := range party {
			if m != char {
				fmt.Fprintf(&body, " else if .%s.skill.ready { %s %s; }", m, m, forms[m])
			}
		}
	}
	if how == FillBrief {
		fmt.Fprintf(&body, " else { %s attack; %sa = %sa + 1; }", char, v, v)
		return fmt.Sprintf("let %s = 0; let %sa = 0; while !.%s.burst.ready && %s < 20 && %sa < %d { %s %s = %s + 1; }",
			v, v, char, v, v, maxFillAttacks, body.String(), v, v)
	}
	fmt.Fprintf(&body, " else { %s attack; }", char)
	return fmt.Sprintf("let %s = 0; while !.%s.burst.ready && %s < 20 { %s %s = %s + 1; }", v, char, v, body.String(), v, v)
}

// waitLoopRe matches a loop in which a character attacks until its burst
// is ready: "while !.<char>.burst.ready { <char> attack; }".
var waitLoopRe = regexp.MustCompile(`while\s*!\s*\.(\w+)\.burst\.ready\s*\{\s*(\w+)\s+attack\s*;\s*\}`)

// WaitLoops rewrites the loops in which a member attacks until its burst
// is ready, for Fill with energy to fill after. gcsim does not count such
// a loop as a wait for energy, so a build short of Energy Recharge
// attacks on a support for as long as its burst takes to charge, and no
// filler sees it (Alhaitham / Xingqiu / Yelan / Raiden without its
// Favonius weapons kept 23% of its score). A member energy does not map
// attacks at most maxFillAttacks times, and the rest of its wait is
// gcsim's, which the filler search sees. For FillSelf, which also attacks
// until the burst is ready, the loop stays as written; for the other
// fillers it goes, and their filler before the burst takes its place.
func WaitLoops(rotation string, energy map[string]Filler, party []string) string {
	n := 0
	return replaceSubmatch(waitLoopRe, rotation, func(m []string) string {
		char := member(m[1], party)
		if member(m[2], party) != char || !slices.Contains(party, char) {
			return m[0]
		}
		how, ok := energy[char]
		switch {
		case ok && how == FillSelf:
			return m[0]
		case ok:
			return ""
		}
		n++
		v := fmt.Sprintf("gs_wait%d", n)
		return fmt.Sprintf("let %s = 0; while !.%s.burst.ready && %s < %d { %s attack; %s = %s + 1; }", v, m[1], v, maxFillAttacks, m[2], v, v)
	})
}

// replaceSubmatch is re.ReplaceAllStringFunc with the submatches.
func replaceSubmatch(re *regexp.Regexp, s string, f func([]string) string) string {
	var b strings.Builder
	last := 0
	for _, m := range re.FindAllStringSubmatchIndex(s, -1) {
		b.WriteString(s[last:m[0]])
		sub := make([]string, len(m)/2)
		for i := range sub {
			if m[2*i] >= 0 {
				sub[i] = s[m[2*i]:m[2*i+1]]
			}
		}
		b.WriteString(f(sub))
		last = m[1]
	}
	b.WriteString(s[last:])
	return b.String()
}

var (
	anyActionRe = regexp.MustCompile(`(?m)(?:^|[;{}])\s*(\w+)\s+([^;{}()=\s][^;{}()]*);`)
	plungeRe    = regexp.MustCompile(`^(high|low)_plunge\b`)
)

// skillForms returns, per member, its skill as the rotation first uses
// it: with its parameters (a hold, for example) and the plunges that
// follow it, as Kazuha's and Xianyun's skills are written. A member
// whose skill the rotation never uses gets a plain "skill".
func skillForms(rotation string, party []string) map[string]string {
	forms := map[string]string{}
	for _, m := range anyActionRe.FindAllStringSubmatch(rotation, -1) {
		char := member(m[1], party)
		if !slices.Contains(party, char) || forms[char] != "" {
			continue
		}
		items := strings.Split(m[2], ",")
		for i, item := range items {
			item = strings.TrimSpace(item)
			if item != "skill" && !strings.HasPrefix(item, "skill[") {
				continue
			}
			form := []string{item}
			for _, next := range items[i+1:] {
				next = strings.TrimSpace(next)
				if !plungeRe.MatchString(next) {
					break
				}
				form = append(form, next)
			}
			forms[char] = strings.Join(form, ", ")
			break
		}
	}
	for _, m := range party {
		if forms[m] == "" {
			forms[m] = "skill"
		}
	}
	return forms
}

// member returns the party member a rotation's name stands for, matched
// by gcsim key, or the name itself when it is no member's. Before
// matching by key, a member written under an alias was never filled:
// 765 library teams write one that way (docs/fillers.md §2).
func member(name string, party []string) string {
	if slices.Contains(party, name) {
		return name
	}
	key := charKey(name)
	for _, m := range party {
		if charKey(m) == key {
			return m
		}
	}
	return name
}
