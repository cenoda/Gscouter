// Command genbosses writes bosses.json: the in-game names of the boss
// presets (config.Bosses) in each of the site's languages, from Yatta's
// public game data (gi.yatta.moe). Run it from internal/web after
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

// langs maps Yatta's language codes to the site's.
var langs = map[string]string{
	"chs": "zh-cn", "cht": "zh-tw", "en": "en", "kr": "ko", "jp": "ja",
	"es": "es", "fr": "fr", "ru": "ru", "th": "th", "vi": "vi",
	"de": "de", "id": "id", "pt": "pt", "tr": "tr", "it": "it",
}

func main() {
	names := map[string]map[string]string{} // boss key -> language -> name
	for yatta, lang := range langs {
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
			if names[b.Key] == nil {
				names[b.Key] = map[string]string{}
			}
			names[b.Key][lang] = it.Name
		}
	}
	out, err := json.MarshalIndent(names, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile("bosses.json", append(out, '\n'), 0o644); err != nil {
		log.Fatal(err)
	}
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
