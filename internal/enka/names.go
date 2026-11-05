package enka

import (
	"encoding/json"
	"fmt"
	"sync"
)

// names[kind][gcsim key][language] is an in-game name; see gennames.
var (
	namesOnce sync.Once
	names     map[string]map[string]map[string]string
)

// CharacterName returns a character's in-game name in a game language
// ("en", "ko", "zh-cn", ...) by gcsim key, falling back to English, or ""
// if the key is unknown.
func CharacterName(key, lang string) string { return name("characters", key, lang) }

// WeaponName is CharacterName for weapons.
func WeaponName(key, lang string) string { return name("weapons", key, lang) }

// ArtifactName is CharacterName for artifact sets.
func ArtifactName(key, lang string) string { return name("artifacts", key, lang) }

// StatName is CharacterName for artifact stats by gcsim stat name ("cr",
// "atk%"). A percentage has its flat stat's name, as in the game.
func StatName(key, lang string) string { return name("stats", key, lang) }

// ArtifactPieceName returns the in-game name of an artifact piece (e.g.
// "Gladiator's Nostalgia") in the given language by set key and slot,
// falling back to English, or "" if unknown.
func ArtifactPieceName(set string, slot Slot, lang string) string {
	if set == "" || slot >= numSlots {
		return ""
	}
	return name("artifact_pieces", set+"."+slot.String(), lang)
}

// ArtifactBonus returns the 2-piece or 4-piece set bonus description in
// the given language, falling back to English, or "" if unknown.
func ArtifactBonus(set string, pieces int, lang string) string {
	if set == "" || (pieces != 2 && pieces != 4) {
		return ""
	}
	return name("artifact_effects", fmt.Sprintf("%s.%d", set, pieces), lang)
}

func name(kind, key, lang string) string {
	namesOnce.Do(func() {
		raw, err := dataFS.ReadFile("data/names.json")
		if err == nil {
			err = json.Unmarshal(raw, &names)
		}
		if err != nil {
			panic("enka: data/names.json: " + err.Error())
		}
	})
	byLang := names[kind][key]
	if n := byLang[lang]; n != "" {
		return n
	}
	return byLang["en"]
}
