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
	Duration   float64 // seconds
	Iterations int
}

// header renders the options, enemy and energy lines.
func (c Conditions) header() string {
	return fmt.Sprintf(`options iteration=%d duration=%v swap_delay=12;
target lvl=100 resist=0.1;
energy every interval=480,720 amount=1;
`, c.Iterations, c.Duration)
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
