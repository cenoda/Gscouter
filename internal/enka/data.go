package enka

import (
	"embed"
	"encoding/json"
	"fmt"
	"strconv"
)

// The data files come from gcsim's UI package at the version pinned in
// go.mod (MIT license), names.json from Enka.Network's store data (see
// gennames). Regenerate them after upgrading gcsim.
//
//go:generate ./gen.sh
//go:embed data/*.json
var dataFS embed.FS

type charData struct {
	ID           int    `json:"id"`
	SubID        int    `json:"sub_id"`
	Key          string `json:"key"`
	Element      string `json:"element"`
	IconName     string `json:"icon_name"`
	SkillDetails struct {
		Attack int `json:"attack"`
		Skill  int `json:"skill"`
		Burst  int `json:"burst"`
	} `json:"skill_details"`
}

type weaponData struct {
	ID  int    `json:"id"`
	Key string `json:"key"`
}

type artifactData struct {
	TextMapID int64  `json:"text_map_id"`
	Key       string `json:"key"`
}

// gameData is the lookup tables the converter needs.
type gameData struct {
	chars     map[string]charData // by avatar ID, or "avatarID-skillDepotID" for the Traveler
	weapons   map[int]string      // weapon item ID -> gcsim key
	artifacts map[string]string   // set name text map hash -> gcsim key
	// mainStats[rarity][GOOD stat key][level] is an artifact main stat value.
	mainStats map[string]map[string][]float64
}

func loadGameData() (*gameData, error) {
	var chars struct {
		Data map[string]charData `json:"data"`
	}
	var weapons struct {
		Data map[string]weaponData `json:"data"`
	}
	var artifacts struct {
		Data map[string]artifactData `json:"data"`
	}
	g := &gameData{
		chars:     map[string]charData{},
		weapons:   map[int]string{},
		artifacts: map[string]string{},
	}
	for file, v := range map[string]any{
		"data/character.dm.json":      &chars,
		"data/weapon.dm.json":         &weapons,
		"data/artifact.dm.json":       &artifacts,
		"data/artifact_main_gen.json": &g.mainStats,
	} {
		raw, err := dataFS.ReadFile(file)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, v); err != nil {
			return nil, fmt.Errorf("%s: %w", file, err)
		}
	}
	for _, c := range chars.Data {
		g.chars[charID(c.ID, c.SubID)] = c
	}
	for _, w := range weapons.Data {
		g.weapons[w.ID] = w.Key
	}
	for _, a := range artifacts.Data {
		g.artifacts[strconv.FormatInt(a.TextMapID, 10)] = a.Key
	}
	return g, nil
}

// charID returns the lookup key for a character. The Traveler shares one
// avatar ID per gender across elements, so the skill depot ID is added.
func charID(avatarID, skillDepotID int) string {
	if isTraveler(avatarID) {
		return fmt.Sprintf("%d-%d", avatarID, skillDepotID)
	}
	return strconv.Itoa(avatarID)
}

func isTraveler(avatarID int) bool {
	return avatarID == 10000005 || avatarID == 10000007
}
