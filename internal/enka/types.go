package enka

import (
	"encoding/json"
	"strconv"
)

// response is the part of the Enka /api/uid/<uid> response the converter
// reads.
type response struct {
	UID        string `json:"uid"`
	TTL        int    `json:"ttl"`
	PlayerInfo struct {
		Nickname string `json:"nickname"`
	} `json:"playerInfo"`
	AvatarInfoList []avatar `json:"avatarInfoList"`
}

type avatar struct {
	AvatarID      int                 `json:"avatarId"`
	SkillDepotID  int                 `json:"skillDepotId"`
	TalentIDList  []int               `json:"talentIdList"`
	PropMap       map[string]propItem `json:"propMap"`
	SkillLevelMap map[string]int      `json:"skillLevelMap"`
	EquipList     []equip             `json:"equipList"`
}

const (
	propAscension = "1002"
	propLevel     = "4001"
)

func (a avatar) prop(id string) int {
	v, _ := strconv.Atoi(a.PropMap[id].Val)
	return v
}

type propItem struct {
	Val string `json:"val"`
}

type equip struct {
	ItemID    int `json:"itemId"`
	Reliquary struct {
		Level int `json:"level"`
	} `json:"reliquary"`
	Weapon struct {
		Level        int            `json:"level"`
		PromoteLevel int            `json:"promoteLevel"`
		AffixMap     map[string]int `json:"affixMap"`
	} `json:"weapon"`
	Flat struct {
		ItemType           string      `json:"itemType"`
		RankLevel          int         `json:"rankLevel"`
		SetNameTextMapHash json.Number `json:"setNameTextMapHash"`
		ReliquaryMainstat  struct {
			MainPropID string  `json:"mainPropId"`
			StatValue  float64 `json:"statValue"`
		} `json:"reliquaryMainstat"`
		ReliquarySubstats []struct {
			AppendPropID string  `json:"appendPropId"`
			StatValue    float64 `json:"statValue"`
		} `json:"reliquarySubstats"`
	} `json:"flat"`
}

type stat struct {
	gcsim string  // gcsim stat name
	good  string  // GOOD stat key, used by the main stat table
	scale float64 // Enka shows percentages as 46.6, gcsim wants 0.466
}

var stats = map[string]stat{
	"FIGHT_PROP_HP":                {"hp", "hp", 1},
	"FIGHT_PROP_HP_PERCENT":        {"hp%", "hp_", 100},
	"FIGHT_PROP_ATTACK":            {"atk", "atk", 1},
	"FIGHT_PROP_ATTACK_PERCENT":    {"atk%", "atk_", 100},
	"FIGHT_PROP_DEFENSE":           {"def", "def", 1},
	"FIGHT_PROP_DEFENSE_PERCENT":   {"def%", "def_", 100},
	"FIGHT_PROP_CHARGE_EFFICIENCY": {"er", "enerRech_", 100},
	"FIGHT_PROP_ELEMENT_MASTERY":   {"em", "eleMas", 1},
	"FIGHT_PROP_CRITICAL":          {"cr", "critRate_", 100},
	"FIGHT_PROP_CRITICAL_HURT":     {"cd", "critDMG_", 100},
	"FIGHT_PROP_HEAL_ADD":          {"heal", "heal_", 100},
	"FIGHT_PROP_FIRE_ADD_HURT":     {"pyro%", "pyro_dmg_", 100},
	"FIGHT_PROP_ELEC_ADD_HURT":     {"electro%", "electro_dmg_", 100},
	"FIGHT_PROP_ICE_ADD_HURT":      {"cryo%", "cryo_dmg_", 100},
	"FIGHT_PROP_WATER_ADD_HURT":    {"hydro%", "hydro_dmg_", 100},
	"FIGHT_PROP_WIND_ADD_HURT":     {"anemo%", "anemo_dmg_", 100},
	"FIGHT_PROP_ROCK_ADD_HURT":     {"geo%", "geo_dmg_", 100},
	"FIGHT_PROP_GRASS_ADD_HURT":    {"dendro%", "dendro_dmg_", 100},
	"FIGHT_PROP_PHYSICAL_ADD_HURT": {"phys%", "physical_dmg_", 100},
}

// statOrder is the order stats are written in, matching gcsim's own.
var statOrder = []string{
	"def%", "def", "hp", "hp%", "atk", "atk%", "er", "em", "cr", "cd", "heal",
	"pyro%", "hydro%", "anemo%", "electro%", "dendro%", "cryo%", "geo%", "phys%",
}
