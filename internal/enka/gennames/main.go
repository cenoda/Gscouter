// Command gennames writes data/names.json: the in-game names of every
// character, weapon and artifact set in data/*.dm.json and of the
// artifact stats, in each of the game's text languages, from
// Enka.Network's public store data (MIT license), and individual artifact
// piece names and 2pc/4pc set bonuses from genshin-db (MIT license).
// Run it from internal/enka after updating the gcsim data.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
)

const store = "https://raw.githubusercontent.com/EnkaNetwork/API-docs/master/store/gi/"
const genshinDBStore = "https://raw.githubusercontent.com/theBowja/genshin-db/main/src/data/"

type entry struct {
	ID    int    `json:"id"`
	SubID int    `json:"sub_id"`
	Key   string `json:"key"`
}

type item struct {
	NameTextMapHash int64
}

type genshinDBArtifact struct {
	Name      string `json:"name"`
	Effect2Pc string `json:"effect2Pc"`
	Effect4Pc string `json:"effect4Pc"`
	Flower    struct{ Name string } `json:"flower"`
	Plume     struct{ Name string } `json:"plume"`
	Sands     struct{ Name string } `json:"sands"`
	Goblet    struct{ Name string } `json:"goblet"`
	Circlet   struct{ Name string } `json:"circlet"`
}

var genshinDBLangs = map[string]string{
	"zh-cn": "ChineseSimplified",
	"zh-tw": "ChineseTraditional",
	"en":    "English",
	"ko":    "Korean",
	"ja":    "Japanese",
	"es":    "Spanish",
	"fr":    "French",
	"ru":    "Russian",
	"th":    "Thai",
	"vi":    "Vietnamese",
	"de":    "German",
	"id":    "Indonesian",
	"pt":    "Portuguese",
	"tr":    "Turkish",
	"it":    "Italian",
}

// statProps are the game's names of the artifact stats, by gcsim key.
// The percentage stats share their flat stat's name ("ATK"); the value
// says which it is.
var statProps = map[string]string{
	"hp": "FIGHT_PROP_HP", "hp%": "FIGHT_PROP_HP_PERCENT",
	"atk": "FIGHT_PROP_ATTACK", "atk%": "FIGHT_PROP_ATTACK_PERCENT",
	"def": "FIGHT_PROP_DEFENSE", "def%": "FIGHT_PROP_DEFENSE_PERCENT",
	"er": "FIGHT_PROP_CHARGE_EFFICIENCY", "em": "FIGHT_PROP_ELEMENT_MASTERY",
	"cr": "FIGHT_PROP_CRITICAL", "cd": "FIGHT_PROP_CRITICAL_HURT",
	"heal": "FIGHT_PROP_HEAL_ADD", "pyro%": "FIGHT_PROP_FIRE_ADD_HURT",
	"hydro%": "FIGHT_PROP_WATER_ADD_HURT", "anemo%": "FIGHT_PROP_WIND_ADD_HURT",
	"electro%": "FIGHT_PROP_ELEC_ADD_HURT", "dendro%": "FIGHT_PROP_GRASS_ADD_HURT",
	"cryo%": "FIGHT_PROP_ICE_ADD_HURT", "geo%": "FIGHT_PROP_ROCK_ADD_HURT",
	"phys%": "FIGHT_PROP_PHYSICAL_ADD_HURT",
}

func main() {
	var avatars, weapons map[string]item
	var relics struct {
		Sets map[string]struct{ Name string }
	}
	var locs map[string]map[string]string // language -> text map hash -> text
	fetch("avatars.json", &avatars)
	fetch("weapons.json", &weapons)
	fetch("relics.json", &relics)
	fetch("locs.json", &locs)
	sets := map[string]item{}
	for id, s := range relics.Sets {
		var hash int64
		fmt.Sscan(s.Name, &hash)
		sets[id] = item{hash}
	}

	names := map[string]map[string]map[string]string{
		"characters":       {},
		"weapons":          {},
		"artifacts":        {},
		"stats":            {},
		"artifact_pieces":  {},
		"artifact_effects": {},
	}
	for _, kind := range []struct {
		file, out string
		items     map[string]item
	}{
		{"data/character.dm.json", "characters", avatars},
		{"data/weapon.dm.json", "weapons", weapons},
		{"data/artifact.dm.json", "artifacts", sets},
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
	for key, prop := range statProps {
		byLang := map[string]string{}
		for lang, texts := range locs {
			if name := texts[prop]; name != "" {
				byLang[lang] = name
			}
		}
		if byLang["en"] == "" {
			log.Fatalf("no English name for %s", prop)
		}
		names["stats"][key] = byLang
	}

	// Artifact piece names and set effects from genshin-db.
	var artifactDM struct{ Data map[string]entry }
	read("data/artifact.dm.json", &artifactDM)
	type task struct {
		langCode, langFolder, setKey string
	}
	var tasks []task
	for setKey := range artifactDM.Data {
		for code, folder := range genshinDBLangs {
			tasks = append(tasks, task{code, folder, setKey})
		}
	}

	type result struct {
		t   task
		art *genshinDBArtifact
		err error
	}
	taskCh := make(chan task, len(tasks))
	resCh := make(chan result, len(tasks))
	for _, t := range tasks {
		taskCh <- t
	}
	close(taskCh)

	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for t := range taskCh {
				u := fmt.Sprintf("%s%s/artifacts/%s.json", genshinDBStore, t.langFolder, t.setKey)
				resp, err := http.Get(u)
				if err != nil {
					resCh <- result{t: t, err: err}
					continue
				}
				if resp.StatusCode != http.StatusOK {
					resp.Body.Close()
					resCh <- result{t: t, err: fmt.Errorf("%s: %s", u, resp.Status)}
					continue
				}
				raw, err := io.ReadAll(resp.Body)
				resp.Body.Close()
				if err != nil {
					resCh <- result{t: t, err: err}
					continue
				}
				var a genshinDBArtifact
				if err := json.Unmarshal(raw, &a); err != nil {
					resCh <- result{t: t, err: err}
					continue
				}
				resCh <- result{t: t, art: &a}
			}
		}()
	}
	wg.Wait()
	close(resCh)

	for res := range resCh {
		if res.err != nil {
			log.Printf("warning: artifact %s in %s: %v", res.t.setKey, res.t.langCode, res.err)
			continue
		}
		a := res.art
		slotMap := map[string]string{
			"flower":  a.Flower.Name,
			"plume":   a.Plume.Name,
			"sands":   a.Sands.Name,
			"goblet":  a.Goblet.Name,
			"circlet": a.Circlet.Name,
		}
		for slot, pname := range slotMap {
			if pname != "" {
				pkey := res.t.setKey + "." + slot
				if names["artifact_pieces"][pkey] == nil {
					names["artifact_pieces"][pkey] = map[string]string{}
				}
				names["artifact_pieces"][pkey][res.t.langCode] = pname
			}
		}
		if a.Effect2Pc != "" {
			ekey := res.t.setKey + ".2"
			if names["artifact_effects"][ekey] == nil {
				names["artifact_effects"][ekey] = map[string]string{}
			}
			names["artifact_effects"][ekey][res.t.langCode] = a.Effect2Pc
		}
		if a.Effect4Pc != "" {
			ekey := res.t.setKey + ".4"
			if names["artifact_effects"][ekey] == nil {
				names["artifact_effects"][ekey] = map[string]string{}
			}
			names["artifact_effects"][ekey][res.t.langCode] = a.Effect4Pc
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


