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
)

func (f Filler) String() string {
	if f == FillParty {
		return "party skills"
	}
	return "own skill"
}

// Fill rewrites a rotation so that characters stop standing still:
//
//   - before each burst of a character in energy, it fills until the
//     burst is ready (see Filler);
//   - before each skill of a character in skill, it attacks until the
//     skill is ready.
//
// Either filler gives up after a fixed number of actions, and the
// rotation then waits as before. Names are as the rotation writes them;
// party lists every member.
func Fill(rotation string, energy, skill, party []string, how Filler) string {
	var b strings.Builder
	n := 0
	start := true // at the start of a statement
	for i := 0; i < len(rotation); {
		if start {
			if m := actionRe.FindStringSubmatchIndex(rotation[i:]); m != nil {
				lead := rotation[i+m[2] : i+m[3]]
				char := rotation[i+m[4] : i+m[5]]
				items := rotation[i+m[6] : i+m[7]]
				if slices.Contains(energy, char) || slices.Contains(skill, char) {
					b.WriteString(lead)
					b.WriteString(fillStatement(char, items, slices.Contains(energy, char), slices.Contains(skill, char), party, how, &n))
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

func fillStatement(char, items string, energy, skill bool, party []string, how Filler, n *int) string {
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
		case energy && action[0] == "burst":
			flush()
			*n++
			out = append(out, energyFiller(char, party, how, fmt.Sprintf("gs_fill%d", *n)))
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

func energyFiller(char string, party []string, how Filler, v string) string {
	var body strings.Builder
	fmt.Fprintf(&body, "if .%s.skill.ready { %s skill; }", char, char)
	if how == FillParty {
		for _, m := range party {
			if m != char {
				fmt.Fprintf(&body, " else if .%s.skill.ready { %s skill; }", m, m)
			}
		}
	}
	fmt.Fprintf(&body, " else { %s attack; }", char)
	return fmt.Sprintf("let %s = 0; while !.%s.burst.ready && %s < 20 { %s %s = %s + 1; }", v, char, v, body.String(), v, v)
}
