// Package config assembles a gcsim config from the common simulation
// conditions, a user's build and a library rotation.
package config

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/genshinsim/gcsim/pkg/gcs/ast"
	"github.com/genshinsim/gcsim/pkg/gcs/parser"
)

// Conditions are the simulation conditions shared by every team
// (design §3.2).
type Conditions struct {
	Iterations int
	// Duration, if set, runs the fight for a fixed time. Otherwise the
	// enemy has (practically infinite) HP, which puts gcsim in damage
	// mode: every character starts with full energy and the run ends when
	// the rotation's action list does, as in community gcsim configs.
	Duration float64 // seconds
	// Enemy is the target, StandardEnemy when nil.
	Enemy *Enemy
}

// header renders the options, enemy and energy lines.
func (c Conditions) header() string {
	enemy := StandardEnemy
	if c.Enemy != nil {
		enemy = *c.Enemy
	}
	if c.Duration > 0 {
		return fmt.Sprintf(`options iteration=%d duration=%v swap_delay=12;
target %s radius=2 pos=0,2.4;
energy every interval=480,720 amount=1;
`, c.Iterations, c.Duration, enemy.target())
	}
	return fmt.Sprintf(`options iteration=%d swap_delay=12;
target %s radius=2 pos=0,2.4 hp=999999999;
energy every interval=480,720 amount=1;
`, c.Iterations, enemy.target())
}

// Assemble joins the conditions, build and rotation into one config.
// The build must only contain character setup lines (char, add weapon,
// add set, add stats); the rotation supplies the active character and
// the action list. The rotation's reads of weapon mods follow the build
// (GuardWeaponMods).
func Assemble(c Conditions, build, rotation string) string {
	rotation = GuardWeaponMods(build, rotation)
	var b strings.Builder
	b.WriteString(c.header())
	b.WriteString("\n")
	b.WriteString(strings.TrimSpace(build))
	b.WriteString("\n\n")
	b.WriteString(strings.TrimSpace(rotation))
	b.WriteString("\n")
	return b.String()
}

var iterationRe = regexp.MustCompile(`\s*\biteration=\d+`)

// AssembleSource joins a source config's own conditions with the build
// and rotation, to reproduce the DPS the source reported. conditions are
// the source's options, target, energy and hurt lines, one statement per
// line, as cmd/import stores them in source.txt; lines starting with #
// are comments. The options line's iteration count is replaced, and the
// rotation's reads of weapon mods follow the build (GuardWeaponMods).
func AssembleSource(conditions string, iterations int, build, rotation string) string {
	rotation = GuardWeaponMods(build, rotation)
	var b strings.Builder
	hasOptions := false
	for _, line := range strings.Split(conditions, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "options") {
			hasOptions = true
			line = iterationRe.ReplaceAllString(strings.TrimSuffix(line, ";"), "") + fmt.Sprintf(" iteration=%d;", iterations)
		}
		b.WriteString(line + "\n")
	}
	if !hasOptions {
		b.WriteString(fmt.Sprintf("options iteration=%d;\n", iterations))
	}
	b.WriteString("\n")
	b.WriteString(strings.TrimSpace(build))
	b.WriteString("\n\n")
	b.WriteString(strings.TrimSpace(rotation))
	b.WriteString("\n")
	return b.String()
}

// Characters returns the gcsim keys of the characters set up in an
// assembled config, in the order they appear.
func Characters(cfg string) ([]string, error) {
	simcfg, _, err := parser.New(ast.NewFile(), cfg).Parse()
	if err != nil {
		return nil, err
	}
	if len(simcfg.Errors) > 0 {
		return nil, errors.Join(simcfg.Errors...)
	}
	chars := make([]string, len(simcfg.Characters))
	for i, p := range simcfg.Characters {
		chars[i] = p.Base.Key.String()
	}
	return chars, nil
}

// RenameCharacters replaces whole-word character names in a rotation, for
// example a library rotation written for "travelercryo" run by a player
// whose Traveler is "aethercryo".
func RenameCharacters(rotation string, names map[string]string) string {
	for from, to := range names {
		if from == to {
			continue
		}
		re := regexp.MustCompile(`\b` + regexp.QuoteMeta(from) + `\b`)
		rotation = re.ReplaceAllLiteralString(rotation, to)
	}
	return rotation
}

// AddParams adds gcsim character params (e.g. "start_hp=1") to the
// "<key> char ...;" line of a build. A line that already has params is
// left alone.
func AddParams(build, key, params string) string {
	if params == "" {
		return build
	}
	re := regexp.MustCompile(`(?m)^(` + regexp.QuoteMeta(key) + ` char [^;+]*?)\s*;`)
	return re.ReplaceAllString(build, "${1} +params=["+params+"];")
}

// Loop turns a one-pass rotation into an endless one. The setup at its
// start (the active character, variables, functions and set_* calls) runs
// once; the rest repeats, so energy shortfalls show up as they would over
// a long fight. The setup must choose the active character.
func Loop(rotation string) (string, error) {
	stmts := statements(rotation)
	setup, active := 0, false
	for ; setup < len(stmts); setup++ {
		word := firstWord(stmts[setup])
		if word == "active" {
			active = true
		} else if word != "let" && word != "fn" && !strings.HasPrefix(word, "set_") {
			break
		}
	}
	if !active {
		return "", errors.New("rotation must start with an active statement (after any let, fn or set_ statements)")
	}
	if setup == len(stmts) {
		return "", errors.New("rotation has no actions after its setup")
	}
	return strings.Join(stmts[:setup], "\n") + "\nwhile 1 {\n" + strings.Join(stmts[setup:], "\n") + "\n}\n", nil
}

// statements splits gcsl source into its top-level statements: each ends
// at a semicolon or at the brace closing its block, outside strings and
// comments, which are dropped.
func statements(src string) []string {
	var out []string
	var cur strings.Builder
	depth := 0
	flush := func() {
		if s := strings.TrimSpace(cur.String()); s != "" && s != ";" {
			out = append(out, s)
		}
		cur.Reset()
	}
	for i := 0; i < len(src); i++ {
		c := src[i]
		switch {
		case c == '#' || c == '/' && i+1 < len(src) && src[i+1] == '/':
			for i < len(src) && src[i] != '\n' {
				i++
			}
			cur.WriteByte('\n')
			continue
		case c == '"':
			end := len(src) // an unterminated string runs to the end
			if j := strings.IndexByte(src[i+1:], '"'); j >= 0 {
				end = i + j + 2
			}
			cur.WriteString(src[i:end])
			i = end - 1
			continue
		}
		cur.WriteByte(c)
		switch c {
		case '{':
			depth++
		case '}':
			depth--
			// A block ends the statement unless "else" follows.
			if depth == 0 && !strings.HasPrefix(strings.TrimSpace(src[i+1:]), "else") {
				flush()
			}
		case ';':
			if depth == 0 {
				flush()
			}
		}
	}
	flush()
	return out
}

func firstWord(stmt string) string {
	end := strings.IndexFunc(stmt, func(r rune) bool { return !(r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)) })
	if end < 0 {
		return stmt
	}
	return stmt[:end]
}
