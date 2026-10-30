package enka

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
)

// GOOD is the Genshin Open Object Description, the inventory format
// Genshin Optimizer exports and scanners such as Irminsul and
// Inventory_Kamera write (https://frzyc.github.io/genshin-optimizer/#/doc).
// ParseGOOD reads the parts of it a build needs (design §4.1).

// ErrNotGOOD is returned for a file that is not GOOD JSON.
var ErrNotGOOD = errors.New("not a GOOD file")

// GOODSkipped is a GOOD character that could not be converted, by its
// GOOD key ("HuTao").
type GOODSkipped struct {
	Key    string
	Reason string
}

type goodFile struct {
	Format     string          `json:"format"`
	Characters []goodCharacter `json:"characters"`
	Weapons    []goodWeapon    `json:"weapons"`
	Artifacts  []goodArtifact  `json:"artifacts"`
}

type goodCharacter struct {
	Key           string `json:"key"`
	Level         int    `json:"level"`
	Constellation int    `json:"constellation"`
	Ascension     int    `json:"ascension"`
	// Talent levels in GOOD are base levels, without the constellation
	// bonus, as gcsim wants them (design §4.2).
	Talent struct {
		Auto  int `json:"auto"`
		Skill int `json:"skill"`
		Burst int `json:"burst"`
	} `json:"talent"`
}

type goodWeapon struct {
	Key        string `json:"key"`
	Level      int    `json:"level"`
	Ascension  int    `json:"ascension"`
	Refinement int    `json:"refinement"`
	Location   string `json:"location"`
}

type goodArtifact struct {
	SetKey      string `json:"setKey"`
	SlotKey     string `json:"slotKey"`
	Level       int    `json:"level"`
	Rarity      int    `json:"rarity"`
	MainStatKey string `json:"mainStatKey"`
	Location    string `json:"location"`
	// Substats are the active ones; GOOD lists a 4-line artifact's
	// fourth, not yet active line under unactivatedSubstats, which is
	// not read.
	Substats []struct {
		Key   string  `json:"key"`
		Value float64 `json:"value"`
	} `json:"substats"`
}

// goodKey turns a GOOD key into the gcsim key it names: GOOD keys are
// the English names in PascalCase without punctuation ("KaedeharaKazuha",
// "StaffOfHoma", "GladiatorsFinale"), and gcsim's are the same names in
// lower case, so lower-casing and dropping anything else gives the key.
func goodKey(key string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(key) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// goodCharKey returns the gcsim character key of a GOOD character key and
// its location key, the key its weapon and artifacts name. GOOD has one
// Traveler per element ("TravelerAnemo"), sharing equipment under
// "Traveler", and no gender; gcsim has both. Aether is taken unless the
// key ends in the gender, as some older files write ("TravelerAnemoF"):
// the score folds both into one Traveler (design §4.4).
func goodCharKey(key string) (gcsim, location string) {
	rest, ok := strings.CutPrefix(key, "Traveler")
	if !ok || rest == "" {
		return goodKey(key), key
	}
	prefix := "aether"
	if elem, ok := strings.CutSuffix(rest, "F"); ok {
		prefix, rest = "lumine", elem
	} else if elem, ok := strings.CutSuffix(rest, "M"); ok {
		rest = elem
	}
	return prefix + goodKey(rest), "Traveler"
}

// goodSlots maps GOOD slot keys to slots.
var goodSlots = map[string]Slot{"flower": Flower, "plume": Plume, "sands": Sands, "goblet": Goblet, "circlet": Circlet}

// statsByGOOD is stats keyed by GOOD stat key ("critRate_").
var statsByGOOD = func() map[string]stat {
	m := map[string]stat{}
	for _, s := range stats {
		m[s.good] = s
	}
	return m
}()

// ParseGOOD converts the characters of a GOOD file, each with the weapon
// and artifacts the file equips it with. A character whose character or
// weapon gcsim does not have, or that has no weapon, is skipped with the
// reason; an artifact set gcsim does not have keeps its stats, as from
// Enka. Every converted character reads back from Pack, so it is a
// build a link can carry.
func ParseGOOD(raw []byte) ([]Character, []GOODSkipped, error) {
	if !loadData() {
		return nil, nil, dataErr
	}
	var f goodFile
	if err := json.Unmarshal(raw, &f); err != nil || f.Format != "GOOD" {
		return nil, nil, ErrNotGOOD
	}
	weapons := map[string]goodWeapon{}
	for _, w := range f.Weapons {
		if _, ok := weapons[w.Location]; w.Location != "" && !ok {
			weapons[w.Location] = w
		}
	}
	artifacts := map[string][]goodArtifact{}
	for _, a := range f.Artifacts {
		if a.Location != "" {
			artifacts[a.Location] = append(artifacts[a.Location], a)
		}
	}
	var out []Character
	var skipped []GOODSkipped
	seen := map[string]bool{}
	for _, gc := range f.Characters {
		key, loc := goodCharKey(gc.Key)
		if seen[key] {
			continue
		}
		seen[key] = true
		c, err := data.convertGOOD(gc, key, weapons[loc], artifacts[loc])
		if err == nil {
			// Unpack checks every key and number against the game.
			var back []Character
			if back, err = Unpack(Pack([]Character{c})); err == nil {
				c = back[0]
			}
		}
		if err != nil {
			skipped = append(skipped, GOODSkipped{Key: gc.Key, Reason: err.Error()})
			continue
		}
		out = append(out, c)
	}
	return out, skipped, nil
}

func (g *gameData) convertGOOD(gc goodCharacter, key string, w goodWeapon, arts []goodArtifact) (Character, error) {
	if _, ok := lookup(key); !ok {
		return Character{}, fmt.Errorf("character %s is not supported by gcsim yet", gc.Key)
	}
	if w.Key == "" {
		return Character{}, fmt.Errorf("character %s has no weapon", gc.Key)
	}
	weapon := goodKey(w.Key)
	if !g.hasWeapon(weapon) {
		return Character{}, fmt.Errorf("weapon %s is not supported by gcsim yet", w.Key)
	}
	c := Character{
		Key:      key,
		Level:    gc.Level,
		MaxLevel: max(gc.Level, maxLevel(gc.Ascension)),
		Cons:     gc.Constellation,
		Talents:  [3]int{gc.Talent.Auto, gc.Talent.Skill, gc.Talent.Burst},
		Weapon: Weapon{
			Key:      weapon,
			Refine:   w.Refinement,
			Level:    w.Level,
			MaxLevel: max(w.Level, maxLevel(w.Ascension)),
		},
		Sets:  map[string]int{},
		Stats: map[string]float64{},
	}
	var pieces []Artifact
	for _, a := range arts {
		slot, ok := goodSlots[a.SlotKey]
		if !ok {
			return Character{}, fmt.Errorf("unknown artifact slot %q", a.SlotKey)
		}
		if slices.ContainsFunc(pieces, func(p Artifact) bool { return p.Slot == slot }) {
			return Character{}, fmt.Errorf("two artifacts in the %s slot", slot)
		}
		set := goodKey(a.SetKey)
		if g.hasArtifact(set) {
			c.Sets[set]++
		} else {
			set = ""
			c.UnknownSets++
		}
		main, ok := statsByGOOD[a.MainStatKey]
		if !ok {
			return Character{}, fmt.Errorf("unknown artifact main stat %q", a.MainStatKey)
		}
		value, ok := g.mainValue(main.gcsim, a.Rarity, a.Level)
		if !ok {
			return Character{}, fmt.Errorf("%s: rarity %d level %d", slot, a.Rarity, a.Level)
		}
		c.Stats[main.gcsim] += value
		p := Artifact{Slot: slot, Set: set, Rarity: a.Rarity, Level: a.Level, Main: StatValue{main.gcsim, value}}
		for _, s := range a.Substats {
			if s.Key == "" {
				continue // an empty line
			}
			st, ok := statsByGOOD[s.Key]
			if !ok {
				return Character{}, fmt.Errorf("unknown artifact substat %q", s.Key)
			}
			// GOOD has the values the game shows (3.9 for CRIT Rate 3.9%).
			c.Stats[st.gcsim] += s.Value / st.scale
			p.Subs = append(p.Subs, StatValue{st.gcsim, math.Round(s.Value*10) / 10 / st.scale})
		}
		pieces = append(pieces, p)
	}
	// As from Enka, pieces the game could not have are left out rather
	// than failing the character: the totals still score.
	slices.SortFunc(pieces, func(a, b Artifact) int { return int(a.Slot) - int(b.Slot) })
	if g.checkPieces(pieces, PiecesFull) == nil {
		c.Pieces, c.PieceDetail = pieces, PiecesFull
	}
	return c, nil
}
