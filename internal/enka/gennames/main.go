// Command gennames writes data/names.json: the in-game names of every
// character and weapon in data/*.dm.json, in each of the game's text
// languages, from Enka.Network's public store data (MIT license). Run it
// from internal/enka after updating the gcsim data.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

const store = "https://raw.githubusercontent.com/EnkaNetwork/API-docs/master/store/gi/"

type entry struct {
	ID    int    `json:"id"`
	SubID int    `json:"sub_id"`
	Key   string `json:"key"`
}

type item struct {
	NameTextMapHash int64
}

func main() {
	var avatars, weapons map[string]item
	var locs map[string]map[string]string // language -> text map hash -> text
	fetch("avatars.json", &avatars)
	fetch("weapons.json", &weapons)
	fetch("locs.json", &locs)

	names := map[string]map[string]map[string]string{
		"characters": {},
		"weapons":    {},
	}
	for _, kind := range []struct {
		file, out string
		items     map[string]item
	}{
		{"data/character.dm.json", "characters", avatars},
		{"data/weapon.dm.json", "weapons", weapons},
	} {
		var dm struct{ Data map[string]entry }
		read(kind.file, &dm)
		for _, e := range dm.Data {
			id := fmt.Sprint(e.ID)
			if e.ID == 10000005 || e.ID == 10000007 { // the Traveler, by element
				id = fmt.Sprintf("%d-%d", e.ID, e.SubID)
			}
			it, ok := kind.items[id]
			if !ok {
				log.Fatalf("%s: %s (%s) is not in Enka's data", kind.file, e.Key, id)
			}
			byLang := map[string]string{}
			for lang, texts := range locs {
				if name := texts[fmt.Sprint(it.NameTextMapHash)]; name != "" {
					byLang[lang] = name
				}
			}
			if byLang["en"] == "" {
				log.Fatalf("%s: no English name for %s", kind.file, e.Key)
			}
			names[kind.out][e.Key] = byLang
		}
	}
	out, err := json.MarshalIndent(names, "", "\t")
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile("data/names.json", append(out, '\n'), 0o644); err != nil {
		log.Fatal(err)
	}
}

func fetch(file string, v any) {
	resp, err := http.Get(store + file)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Fatalf("%s: %s", file, resp.Status)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	if err := json.Unmarshal(raw, v); err != nil {
		log.Fatalf("%s: %v", file, err)
	}
}

func read(file string, v any) {
	raw, err := os.ReadFile(file)
	if err != nil {
		log.Fatal(err)
	}
	if err := json.Unmarshal(raw, v); err != nil {
		log.Fatalf("%s: %v", file, err)
	}
}
