// Command genenemy writes enemy.json: the game's own words for the
// enemy settings in each of the site's languages, so they need no
// translation. Boss names (config.Bosses) come from Yatta's public game
// data (gi.yatta.moe); the resistance and level labels from Enka.Network's
// public store data (MIT license). Run it from internal/web after
// changing the presets.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/cenoda/gscouter/internal/config"
)

// yattaLangs maps Yatta's language codes to the site's.
var yattaLangs = map[string]string{
	"chs": "zh-cn", "cht": "zh-tw", "en": "en", "kr": "ko", "jp": "ja",
	"es": "es", "fr": "fr", "ru": "ru", "th": "th", "vi": "vi",
	"de": "de", "id": "id", "pt": "pt", "tr": "tr", "it": "it",
}

// resistProps are the game's names for the resistance stats, by
// config.Elements.
var resistProps = map[string]string{
	"pyro": "FIGHT_PROP_FIRE_SUB_HURT", "hydro": "FIGHT_PROP_WATER_SUB_HURT",
	"anemo": "FIGHT_PROP_WIND_SUB_HURT", "electro": "FIGHT_PROP_ELEC_SUB_HURT",
	"dendro": "FIGHT_PROP_GRASS_SUB_HURT", "cryo": "FIGHT_PROP_ICE_SUB_HURT",
	"geo": "FIGHT_PROP_ROCK_SUB_HURT", "physical": "FIGHT_PROP_PHYSICAL_SUB_HURT",
}

type output struct {
	// Bosses[boss key][language] is a boss's name.
	Bosses map[string]map[string]string `json:"bosses"`
	// Terms[term][language] is "level" or an element's resistance.
	Terms map[string]map[string]string `json:"terms"`
}

func main() {
	out := output{Bosses: map[string]map[string]string{}, Terms: map[string]map[string]string{}}
	for yatta, lang := range yattaLangs {
		var list struct {
			Data struct {
				Items map[string]struct{ Name string }
			}
		}
		fetch("https://gi.yatta.moe/api/v2/"+yatta+"/monster", &list)
		for _, b := range config.Bosses {
			it, ok := list.Data.Items[fmt.Sprint(b.NameID)]
			if !ok || it.Name == "" {
				// The site falls back to English.
				if lang == "en" {
					log.Fatalf("%s (%d) is not in Yatta's data", b.Key, b.NameID)
				}
				log.Printf("%s: no name for %s (%d)", yatta, b.Key, b.NameID)
				continue
			}
			set(out.Bosses, b.Key, lang, it.Name)
		}
	}

	var locs map[string]map[string]string // language -> key -> text
	fetch("https://raw.githubusercontent.com/EnkaNetwork/API-docs/master/store/gi/locs.json", &locs)
	for _, lang := range yattaLangs {
		texts := locs[lang]
		if texts == nil {
			log.Fatalf("no %s in Enka's data", lang)
		}
		terms := map[string]string{"level": "level"}
		for _, el := range config.Elements {
			terms[el] = resistProps[el]
		}
		for term, key := range terms {
			if texts[key] == "" {
				log.Fatalf("%s: no %s in Enka's data", lang, key)
			}
			set(out.Terms, term, lang, texts[key])
		}
	}

	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile("enemy.json", append(b, '\n'), 0o644); err != nil {
		log.Fatal(err)
	}
}

func set(m map[string]map[string]string, key, lang, text string) {
	if m[key] == nil {
		m[key] = map[string]string{}
	}
	m[key][lang] = text
}

func fetch(url string, v any) {
	resp, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Fatalf("%s: %s", url, resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		log.Fatalf("%s: %v", url, err)
	}
}
