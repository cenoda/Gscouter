package enka

import (
	"encoding/json"
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
