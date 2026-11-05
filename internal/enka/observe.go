package enka

import (
	"fmt"
	"log/slog"
	"sync"
)

// NewID is a character or weapon a showcase carries whose ID the legacy
// mapping (data/README.md) lacks, with the values its key is matched by:
// the level, ascension and base stats the showcase shows. It holds no
// player data: no UID, nickname, talents, constellations, artifacts or
// other characters (gscouter-engine docs/independence.md §4, Enka data).
type NewID struct {
	Kind string // "character" or "weapon"
	// ID is the avatarId of a character or the itemId of a weapon.
	ID int
	// SkillDepotID tells a character's elements apart (the Traveler's).
	SkillDepotID int
	Level        int
	Ascension    int
	// A character's BaseHP, BaseATK and BaseDEF are its fightPropMap 1, 4
	// and 7; BaseATK includes its weapon's base ATK, which WeaponBaseATK
	// is, as the showcase shows it (rounded). A weapon's BaseATK is its
	// shown base ATK and SubProp, SubValue its substat.
	BaseHP, BaseATK, BaseDEF float64
	WeaponBaseATK            float64
	SubProp                  string
	SubValue                 float64
}

// Fight prop IDs of the base stats in an avatar's fightPropMap.
const (
	fightBaseHP  = "1"
	fightBaseATK = "4"
	fightBaseDEF = "7"
)

const propBaseATK = "FIGHT_PROP_BASE_ATTACK"

// observeNew reports the character and weapons of a showcase avatar that
// the mapping lacks. It changes nothing else: convert skips them as before.
func (g *gameData) observeNew(a avatar) {
	_, known := g.chars[charID(a.AvatarID, a.SkillDepotID)]
	var weaponATK float64
	for _, e := range a.EquipList {
		if e.Flat.ItemType != "ITEM_WEAPON" {
			continue
		}
		w := NewID{Kind: "weapon", ID: e.ItemID, Level: e.Weapon.Level, Ascension: e.Weapon.PromoteLevel}
		for _, s := range e.Flat.WeaponStats {
			if s.AppendPropID == propBaseATK {
				w.BaseATK = s.StatValue
			} else if w.SubProp == "" {
				w.SubProp, w.SubValue = s.AppendPropID, s.StatValue
			}
		}
		weaponATK = w.BaseATK
		if _, ok := g.weapons[e.ItemID]; !ok {
			observe(w)
		}
	}
	if !known {
		observe(NewID{
			Kind:          "character",
			ID:            a.AvatarID,
			SkillDepotID:  a.SkillDepotID,
			Level:         a.prop(propLevel),
			Ascension:     a.prop(propAscension),
			BaseHP:        a.FightPropMap[fightBaseHP],
			BaseATK:       a.FightPropMap[fightBaseATK],
			BaseDEF:       a.FightPropMap[fightBaseDEF],
			WeaponBaseATK: weaponATK,
		})
	}
}

// maxObserved bounds the observations remembered, so that a stream of
// showcases cannot grow the set without end; past it, each is logged.
const maxObserved = 10000

var (
	observedMu sync.Mutex
	observed   = map[NewID]bool{}
)

// observe logs an observation once per process. A var so tests can
// capture it.
var observe = func(n NewID) {
	observedMu.Lock()
	seen := observed[n]
	if !seen && len(observed) < maxObserved {
		observed[n] = true
	}
	observedMu.Unlock()
	if !seen {
		logNewID(n)
	}
}

// logNewID writes the line an operator reads: the message "enka new id"
// and the observation's fields, nothing else.
func logNewID(n NewID) {
	attrs := []any{"kind", n.Kind}
	if n.Kind == "character" {
		attrs = append(attrs,
			"avatar_id", n.ID, "skill_depot_id", n.SkillDepotID,
			"level", n.Level, "ascension", n.Ascension,
			"base_hp", fmt.Sprint(n.BaseHP), "base_atk", fmt.Sprint(n.BaseATK), "base_def", fmt.Sprint(n.BaseDEF),
			"weapon_base_atk", fmt.Sprint(n.WeaponBaseATK))
	} else {
		attrs = append(attrs,
			"item_id", n.ID, "level", n.Level, "ascension", n.Ascension,
			"base_atk", fmt.Sprint(n.BaseATK), "sub_prop", n.SubProp, "sub_value", fmt.Sprint(n.SubValue))
	}
	slog.Info("enka new id", attrs...)
}
