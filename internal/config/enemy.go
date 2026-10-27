package config

import (
	"fmt"
	"strings"
)

// Elements are the damage types an enemy resists, in the order of
// Enemy.Resist.
var Elements = [...]string{"pyro", "hydro", "anemo", "electro", "dendro", "cryo", "geo", "physical"}

// Enemy is the target a team is scored on: the standard enemy (design
// §3.2) or one a visitor sets up. Its DEF follows from its level, as for
// nearly every enemy in the game.
type Enemy struct {
	Level  int
	Resist [len(Elements)]int // in percent, by Elements
}

// StandardEnemy is the enemy every score is compared on: Lv. 100 with
// 10% resistance to everything.
var StandardEnemy = Enemy{Level: 100, Resist: [len(Elements)]int{10, 10, 10, 10, 10, 10, 10, 10}}

// Limits of a custom enemy.
const (
	MinLevel  = 1
	MaxLevel  = 200
	MinResist = -100
	MaxResist = 1000
)

// Validate reports whether e is within the limits.
func (e Enemy) Validate() error {
	if e.Level < MinLevel || e.Level > MaxLevel {
		return fmt.Errorf("enemy level %d is not within %d-%d", e.Level, MinLevel, MaxLevel)
	}
	for i, r := range e.Resist {
		if r < MinResist || r > MaxResist {
			return fmt.Errorf("enemy %s resistance %d%% is not within %d%%-%d%%", Elements[i], r, MinResist, MaxResist)
		}
	}
	return nil
}

// Uniform returns the resistance e has to everything, and whether it has
// the same one to everything.
func (e Enemy) Uniform() (int, bool) {
	for _, r := range e.Resist[1:] {
		if r != e.Resist[0] {
			return 0, false
		}
	}
	return e.Resist[0], true
}

// target renders e as gcsim target options, e.g. "lvl=100 resist=0.1" or
// "lvl=90 pyro=0.1 ... geo=0.7 physical=0.4".
func (e Enemy) target() string {
	if r, ok := e.Uniform(); ok {
		return fmt.Sprintf("lvl=%d resist=%v", e.Level, float64(r)/100)
	}
	// gcsim adds an element's value to resist=, so set each one alone.
	parts := []string{fmt.Sprintf("lvl=%d", e.Level)}
	for i, r := range e.Resist {
		parts = append(parts, fmt.Sprintf("%s=%v", Elements[i], float64(r)/100))
	}
	return strings.Join(parts, " ")
}
