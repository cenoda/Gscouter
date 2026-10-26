// Package config assembles a gcsim config from the common simulation
// conditions, a user's build and a library rotation.
package config

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

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
}

// header renders the options, enemy and energy lines.
func (c Conditions) header() string {
	if c.Duration > 0 {
		return fmt.Sprintf(`options iteration=%d duration=%v swap_delay=12;
target lvl=100 resist=0.1 radius=2 pos=0,2.4;
energy every interval=480,720 amount=1;
`, c.Iterations, c.Duration)
	}
	return fmt.Sprintf(`options iteration=%d swap_delay=12;
target lvl=100 resist=0.1 radius=2 pos=0,2.4 hp=999999999;
energy every interval=480,720 amount=1;
`, c.Iterations)
}

// Assemble joins the conditions, build and rotation into one config.
// The build must only contain character setup lines (char, add weapon,
// add set, add stats); the rotation supplies the active character and
// the action list.
func Assemble(c Conditions, build, rotation string) string {
	var b strings.Builder
	b.WriteString(c.header())
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

// Loop turns a one-pass rotation into an endless one: the first statement
// (the "active <char>;" line) runs once and the rest repeats, so energy
// shortfalls show up as they would over a long fight.
func Loop(rotation string) (string, error) {
	head, body, ok := strings.Cut(strings.TrimSpace(rotation), ";")
	if !ok || !strings.HasPrefix(strings.TrimSpace(head), "active ") {
		return "", errors.New("rotation must start with an active statement")
	}
	return head + ";\nwhile 1 {\n" + strings.TrimSpace(body) + "\n}\n", nil
}
