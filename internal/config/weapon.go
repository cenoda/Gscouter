package config

import (
	"regexp"
	"slices"
	"strings"

	"github.com/genshinsim/gcsim/pkg/core/keys"
	"github.com/genshinsim/gcsim/pkg/shortcut"
)

// modWeapon is a weapon whose passive puts a status mod on its holder.
type modWeapon struct {
	mod string
	// plain is a weapon of the same type without such a mod, for the
	// validation check that a rotation still plays without the weapon.
	plain keys.Weapon
}

// modWeapons are the weapons whose mods rotations read to time actions:
// `while !.zhongli.mods.favonius-cd { zhongli attack; }` attacks until
// Favonius Lance's passive procs, which only happens with that weapon
// (docs/cycle.md §5).
var modWeapons = map[keys.Weapon]modWeapon{
	keys.FavoniusSword:         {"favonius-cd", keys.TheAlleyFlash},
	keys.FavoniusGreatsword:    {"favonius-cd", keys.Rainslasher},
	keys.FavoniusWarbow:        {"favonius-cd", keys.TheStringless},
	keys.FavoniusCodex:         {"favonius-cd", keys.TheWidsith},
	keys.FavoniusLance:         {"favonius-cd", keys.DragonsBane},
	keys.SacrificialSword:      {"sacrificial-cd", keys.TheAlleyFlash},
	keys.SacrificialGreatsword: {"sacrificial-cd", keys.Rainslasher},
	keys.SacrificialBow:        {"sacrificial-cd", keys.TheStringless},
	keys.SacrificialFragments:  {"sacrificial-cd", keys.TheWidsith},
}

// neverCrits are the characters whose Favonius passive, which procs on
// a CRIT Hit, never procs: Kokomi's passive takes 100% off her CRIT Rate,
// more than artifacts and team buffs give back.
var neverCrits = map[string]bool{keys.SangonomiyaKokomi.String(): true}

// weaponModRead matches a rotation's read of a weapon mod, such as
// ".zhongli.mods.favonius-cd".
var weaponModRead = regexp.MustCompile(`\.([A-Za-z0-9]+)\.mods\.(favonius-cd|sacrificial-cd)\b`)

// buildWeapon matches a build's weapon line.
var buildWeapon = regexp.MustCompile(`(?m)^(\s*([A-Za-z0-9]+)\s+add\s+weapon\s*=\s*"?)([^"\s;]+)`)

// GuardWeaponMods rewrites a rotation's reads of weapon mods for the
// build: a character whose weapon does not give the mod reads it as
// always on, a cooldown that never ends, since its passive never procs;
// so does one whose Favonius weapon never procs (neverCrits).
// A rotation written for Favonius Lance on Zhongli otherwise waits for
// the proc forever with any other weapon, and the team's score collapses
// to a few percent (docs/cycle.md §5). Reads of characters the build does
// not set up are left as they are, and so is a build with the weapon,
// whose config stays the same.
func GuardWeaponMods(build, rotation string) string {
	weapons := buildWeapons(build)
	return weaponModRead.ReplaceAllStringFunc(rotation, func(read string) string {
		m := weaponModRead.FindStringSubmatch(read)
		key := charKey(m[1])
		w, ok := weapons[key]
		if !ok || modWeapons[w].mod == m[2] && !(m[2] == "favonius-cd" && neverCrits[key]) {
			return read
		}
		return "1"
	})
}

// SwapModWeapons gives every character whose weapon mod the rotation
// reads, and whose weapon gives it, a weapon of the same type without
// one. It returns the build and the characters it changed, in the
// build's order; none when the rotation reads no weapon mod.
func SwapModWeapons(build, rotation string) (string, []string) {
	read := map[string]bool{}
	for _, m := range weaponModRead.FindAllStringSubmatch(rotation, -1) {
		read[charKey(m[1])+" "+m[2]] = true
	}
	var changed []string
	build = buildWeapon.ReplaceAllStringFunc(build, func(line string) string {
		m := buildWeapon.FindStringSubmatch(line)
		w, ok := shortcut.WeaponNameToKey[strings.ToLower(m[3])]
		mw, gives := modWeapons[w]
		if !ok || !gives || !read[charKey(m[2])+" "+mw.mod] {
			return line
		}
		changed = append(changed, m[2])
		return m[1] + mw.plain.String()
	})
	return build, slices.Clip(changed)
}

// buildWeapons maps each character of a build, by gcsim key, to its
// weapon's key. A name gcsim does not know is left out.
func buildWeapons(build string) map[string]keys.Weapon {
	weapons := map[string]keys.Weapon{}
	for _, m := range buildWeapon.FindAllStringSubmatch(build, -1) {
		if w, ok := shortcut.WeaponNameToKey[strings.ToLower(m[3])]; ok {
			weapons[charKey(m[2])] = w
		}
	}
	return weapons
}

// charKey is gcsim's key for a character name or one of its aliases
// ("xilo" -> "xilonen"), or the lowercased name when gcsim does not know
// it.
func charKey(name string) string {
	name = strings.ToLower(name)
	if key, ok := shortcut.CharNameToKey[name]; ok {
		return key.String()
	}
	return name
}
