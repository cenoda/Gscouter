// Package enka fetches showcase characters from Enka.Network and converts
// them into gcsim character setup lines.
package enka

import (
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"
	"sync"
)

// Character is one showcase character in gcsim terms.
type Character struct {
	Key      string // gcsim character key, e.g. "skirk" or "aethercryo"
	Level    int
	MaxLevel int
	Cons     int
	// Talents are the base levels of normal attack, skill and burst,
	// without the constellation bonus (gcsim adds it).
	Talents [3]int
	Weapon  Weapon
	Sets    map[string]int     // artifact set key -> pieces
	Stats   map[string]float64 // gcsim stat name -> artifact total
	// UnknownSets counts artifacts whose set gcsim does not have. Their
	// stats are kept; only the set bonus is missing.
	UnknownSets int
	// Pieces are the artifacts piece by piece, in slot order, as far as
	// PieceDetail says. Sets and Stats stay the totals the gcsim config
	// is built from.
	Pieces      []Artifact
	PieceDetail PieceDetail
}

type Weapon struct {
	Key      string
	Refine   int
	Level    int
	MaxLevel int
}

// Skipped is a showcase character that could not be converted.
type Skipped struct {
	AvatarID int
	Reason   string
}

type Profile struct {
	UID        string
	Nickname   string
	Characters []Character
	Skipped    []Skipped
	TTL        int // seconds Enka asks clients to cache the response
}

var (
	dataOnce sync.Once
	data     *gameData
	dataErr  error
)

// elements maps Enka's element names to gcsim's.
var elements = map[string]string{
	"Fire": "pyro", "Water": "hydro", "Wind": "anemo", "Electric": "electro",
	"Grass": "dendro", "Ice": "cryo", "Rock": "geo",
}

// Element returns the element of a character by gcsim key ("pyro",
// "hydro", ...), or "" if the key is unknown.
func Element(key string) string {
	if c, ok := lookup(key); ok {
		return elements[c.Element]
	}
	return ""
}

// IconURL returns the URL of a character's face icon on Enka.Network by
// gcsim key, or "" if the key is unknown. The images are HoYoverse's;
// Enka serves them for tools built on its API.
func IconURL(key string) string {
	if c, ok := lookup(key); ok && c.IconName != "" {
		return "https://enka.network/ui/" + c.IconName + ".png"
	}
	return ""
}

func lookup(key string) (charData, bool) {
	dataOnce.Do(func() { data, dataErr = loadGameData() })
	if dataErr != nil {
		return charData{}, false
	}
	for _, c := range data.chars {
		if c.Key == key {
			return c, true
		}
	}
	return charData{}, false
}

// Parse converts an Enka /api/uid/<uid> response.
func Parse(raw []byte) (*Profile, error) {
	dataOnce.Do(func() { data, dataErr = loadGameData() })
	if dataErr != nil {
		return nil, dataErr
	}

	var resp response
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("decode enka response: %w", err)
	}
	p := &Profile{UID: resp.UID, Nickname: resp.PlayerInfo.Nickname, TTL: resp.TTL}
	for _, a := range resp.AvatarInfoList {
		c, err := data.convert(a)
		if err != nil {
			p.Skipped = append(p.Skipped, Skipped{AvatarID: a.AvatarID, Reason: err.Error()})
			continue
		}
		p.Characters = append(p.Characters, c)
	}
	return p, nil
}

func (g *gameData) convert(a avatar) (Character, error) {
	cd, ok := g.chars[charID(a.AvatarID, a.SkillDepotID)]
	if !ok {
		return Character{}, fmt.Errorf("character %d is not supported by gcsim yet", a.AvatarID)
	}
	level := a.prop(propLevel)
	c := Character{
		Key:      cd.Key,
		Level:    level,
		MaxLevel: max(level, maxLevel(a.prop(propAscension))),
		Cons:     len(a.TalentIDList),
		Talents: [3]int{
			a.SkillLevelMap[strconv.Itoa(cd.SkillDetails.Attack)],
			a.SkillLevelMap[strconv.Itoa(cd.SkillDetails.Skill)],
			a.SkillLevelMap[strconv.Itoa(cd.SkillDetails.Burst)],
		},
		Sets:  map[string]int{},
		Stats: map[string]float64{},
	}

	hasWeapon := false
	var pieces []Artifact
	piecesOK := true
	for _, e := range a.EquipList {
		switch e.Flat.ItemType {
		case "ITEM_WEAPON":
			key, ok := g.weapons[e.ItemID]
			if !ok {
				return Character{}, fmt.Errorf("weapon %d is not supported by gcsim yet", e.ItemID)
			}
			refine := 1
			for _, r := range e.Weapon.AffixMap {
				refine = r + 1
			}
			c.Weapon = Weapon{
				Key:      key,
				Refine:   refine,
				Level:    e.Weapon.Level,
				MaxLevel: max(e.Weapon.Level, maxLevel(e.Weapon.PromoteLevel)),
			}
			hasWeapon = true
		case "ITEM_RELIQUARY":
			set, ok := g.artifacts[e.Flat.SetNameTextMapHash.String()]
			if ok {
				c.Sets[set]++
			} else {
				c.UnknownSets++
			}
			main, err := g.mainStat(e)
			if err != nil {
				return Character{}, err
			}
			c.Stats[main.Key] += main.Value
			slot, ok := equipSlots[e.Flat.EquipType]
			piecesOK = piecesOK && ok
			p := Artifact{Slot: slot, Set: set, Rarity: e.Flat.RankLevel, Level: e.Reliquary.Level - 1, Main: main}
			for _, s := range e.Flat.ReliquarySubstats {
				if st, ok := stats[s.AppendPropID]; ok {
					c.Stats[st.gcsim] += s.StatValue / st.scale
					// Pieces keep substats at the precision they are
					// packed at; Stats keeps Enka's value as before.
					v := math.Round(s.StatValue*10) / 10
					p.Subs = append(p.Subs, StatValue{st.gcsim, v / st.scale})
				} else {
					piecesOK = false
				}
			}
			pieces = append(pieces, p)
		}
	}
	if !hasWeapon {
		return Character{}, fmt.Errorf("character %s has no weapon", cd.Key)
	}
	// Pieces the game could not have (or that a link could not carry) are
	// left out rather than failing the character: the totals still score.
	slices.SortFunc(pieces, func(a, b Artifact) int { return int(a.Slot) - int(b.Slot) })
	if piecesOK && g.checkPieces(pieces, PiecesFull) == nil {
		c.Pieces, c.PieceDetail = pieces, PiecesFull
	}
	return c, nil
}

// mainStat returns an artifact's main stat. The value is looked up from
// the main stat table, since Enka rounds the displayed value.
func (g *gameData) mainStat(e equip) (StatValue, error) {
	ms := e.Flat.ReliquaryMainstat
	st, ok := stats[ms.MainPropID]
	if !ok {
		return StatValue{}, fmt.Errorf("unknown artifact main stat %s", ms.MainPropID)
	}
	value := ms.StatValue / st.scale
	// Enka reports artifact level + 1.
	if v, ok := g.mainValue(st.gcsim, e.Flat.RankLevel, e.Reliquary.Level-1); ok {
		value = v
	}
	return StatValue{st.gcsim, value}, nil
}

// Config renders the character as gcsim setup lines.
func (c Character) Config() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s char lvl=%d/%d cons=%d talent=%d,%d,%d;\n",
		c.Key, c.Level, c.MaxLevel, c.Cons, c.Talents[0], c.Talents[1], c.Talents[2])
	fmt.Fprintf(&b, "%s add weapon=%q refine=%d lvl=%d/%d;\n",
		c.Key, c.Weapon.Key, c.Weapon.Refine, c.Weapon.Level, c.Weapon.MaxLevel)
	for _, set := range slices.Sorted(maps.Keys(c.Sets)) {
		fmt.Fprintf(&b, "%s add set=%q count=%d;\n", c.Key, set, c.Sets[set])
	}
	if len(c.Stats) > 0 {
		fmt.Fprintf(&b, "%s add stats", c.Key)
		for _, name := range statOrder {
			if v, ok := c.Stats[name]; ok && v != 0 {
				// Round off float noise from summing, e.g. 0.43100000000000005.
				v = math.Round(v*1e6) / 1e6
				fmt.Fprintf(&b, " %s=%s", name, strconv.FormatFloat(v, 'f', -1, 64))
			}
		}
		b.WriteString(";\n")
	}
	return b.String()
}

// maxLevel returns the level cap for an ascension phase.
func maxLevel(ascension int) int {
	caps := []int{20, 40, 50, 60, 70, 80, 90}
	return caps[min(max(ascension, 0), len(caps)-1)]
}
