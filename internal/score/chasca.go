package score

import (
	"maps"

	"github.com/genshinsim/gcsim/pkg/core"
	"github.com/genshinsim/gcsim/pkg/core/action"
	"github.com/genshinsim/gcsim/pkg/core/info"
	"github.com/genshinsim/gcsim/pkg/core/keys"
	"github.com/genshinsim/gcsim/pkg/core/player/character"
)

// chascaC6Key is the status gcsim's Chasca has while her C6 instant
// charge is ready (internal/characters/chasca/cons.go). A test fails if
// it is renamed.
const chascaC6Key = "chasca-c6"

// gcsimChasca is gcsim's own constructor for Chasca, which init wraps.
var gcsimChasca core.NewCharacterFunc

// While that status is up, gcsim rejects a Multitarget Fire aim with
// fewer than 6 bullets ("bullets must be 6 when c6 instant charge is
// active") and ends the whole run. Library rotations come from C0 configs
// and write aim[bullets=4], so 131 of the 150 Chasca teams failed at C6.
// In the game the instant charge loads all six shells at once, so such an
// aim fires six. The status is set by a shell loading during the aim
// before, after the rotation has already asked for the next action, so a
// rotation cannot test it in time; the aim's bullet count is raised here
// instead, as the action runs. Below C6 the status never appears and
// Chasca is not wrapped, so nothing changes (docs/fork.md §7).
func init() {
	var ok bool
	gcsimChasca, ok = core.NewCharFuncMap[keys.Chasca]
	if !ok {
		return
	}
	core.NewCharFuncMap[keys.Chasca] = func(c *core.Core, w *character.CharWrapper, p info.CharacterProfile) error {
		if err := gcsimChasca(c, w, p); err != nil {
			return err
		}
		if w.Base.Cons >= 6 {
			w.Character = &chascaC6{Character: w.Character, w: w}
		}
		return nil
	}
}

// chascaC6 is a C6 Chasca whose aims fire six shells while her instant
// charge is ready.
type chascaC6 struct {
	character.Character
	w *character.CharWrapper
}

func (c *chascaC6) Aimed(p map[string]int) (action.Info, error) {
	if n, ok := p["bullets"]; ok && n > 0 && n < 6 && c.w.StatusIsActive(chascaC6Key) {
		p = maps.Clone(p)
		p["bullets"] = 6
	}
	return c.Character.Aimed(p)
}
