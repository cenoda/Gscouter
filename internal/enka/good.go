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

// GOODSkipped is a GOOD character that could not be converted: its GOOD
// key ("HuTao"), its gcsim key ("hutao") and why, as a reason code with
// its arguments, so that a page can say it in the visitor's language.
// Error is the reason in English, for logs.
type GOODSkipped struct {
	Key    string
	Char   string
	Reason GOODReason
	// Args are the reason's arguments, in the order of its English
	// message: strings, ints and Slots.
	Args []any
	// Err is the error Unpack gave, for GOODUnreadable.
	Err error
}

func (s GOODSkipped) Error() string { return "character " + s.Key + ": " + s.Message() }

// Message is the reason in English, without the character.
func (s GOODSkipped) Message() string {
	msg, ok := goodReasons[s.Reason]
	if !ok {
		msg = "skipped"
	}
	msg = fmt.Sprintf(msg, s.Args...)
	if s.Err != nil {
		msg += ": " + s.Err.Error()
	}
	return msg
}

// GOODReason is why ParseGOOD skipped a character.
type GOODReason int

const (
	// GOODUnsupported: gcsim does not have the character.
	GOODUnsupported GOODReason = iota + 1
	// GOODManekin: Manekin or Manekina, Miliastra Wonderland's
	// characters, which gcsim does not have.
	GOODManekin
	// GOODNoWeapon: the file equips no weapon on the character.
	GOODNoWeapon
	// GOODUnknownWeapon: gcsim does not have the weapon; Args: its
	// GOOD key.
	GOODUnknownWeapon
	// GOODLevel: a character level outside 1-100; Args: the level.
	GOODLevel
	// GOODConstellation: a constellation outside 0-6; Args: it.
	GOODConstellation
	// GOODTalent: a talent level outside 1-10, usually one with the
	// constellation bonus; Args: auto, skill, burst.
	GOODTalent
	// GOODWeaponLevel: a weapon level outside 1-100; Args: the level.
	GOODWeaponLevel
	// GOODRefinement: a refinement outside 1-5; Args: it.
	GOODRefinement
	// GOODArtifactSlot: an artifact slot GOOD does not have; Args:
	// the slot key.
	GOODArtifactSlot
	// GOODSlotTwice: two artifacts in one slot; Args: the Slot.
	GOODSlotTwice
	// GOODMainStat: an unknown artifact main stat; Args: its key.
	GOODMainStat
	// GOODMainStatValue: a main stat the game has no value for at the
	// piece's rarity and level; Args: the Slot, rarity, level.
	GOODMainStatValue
	// GOODSubstat: an unknown artifact substat; Args: its key.
	GOODSubstat
	// GOODUnreadable: the build does not read back through Unpack (a
	// number the game does not allow); Err says why.
	GOODUnreadable
)

// GOODReasons are the reasons, in order.
var GOODReasons = []GOODReason{
	GOODUnsupported, GOODManekin, GOODNoWeapon, GOODUnknownWeapon, GOODLevel,
	GOODConstellation, GOODTalent, GOODWeaponLevel, GOODRefinement,
	GOODArtifactSlot, GOODSlotTwice, GOODMainStat, GOODMainStatValue,
	GOODSubstat, GOODUnreadable,
}

// goodReasons are the reasons' English messages, after the character.
var goodReasons = map[GOODReason]string{
	GOODUnsupported:   "not supported by gcsim yet",
	GOODManekin:       "Manekin and Manekina are not supported by gcsim",
	GOODNoWeapon:      "no weapon",
	GOODUnknownWeapon: "weapon %s is not supported by gcsim yet",
	GOODLevel:         "level %d is not a character level",
	GOODConstellation: "constellation %d is not one of 0-6",
	GOODTalent:        "talent levels %d/%d/%d are not base levels of 1-10",
	GOODWeaponLevel:   "weapon level %d is not a weapon level",
	GOODRefinement:    "refinement %d is not one of 1-5",
	GOODArtifactSlot:  "unknown artifact slot %q",
	GOODSlotTwice:     "two artifacts in the %s slot",
	GOODMainStat:      "unknown artifact main stat %q",
	GOODMainStatValue: "%s: no main stat value at rarity %d level %d",
	GOODSubstat:       "unknown artifact substat %q",
	GOODUnreadable:    "the build does not read back",
}

// Code is the reason's name in snake case ("no_weapon"), for message
// catalogs.
func (r GOODReason) Code() string {
	return map[GOODReason]string{
		GOODUnsupported: "unsupported", GOODManekin: "manekin", GOODNoWeapon: "no_weapon",
		GOODUnknownWeapon: "unknown_weapon", GOODLevel: "level", GOODConstellation: "constellation",
		GOODTalent: "talent", GOODWeaponLevel: "weapon_level", GOODRefinement: "refinement",
		GOODArtifactSlot: "artifact_slot", GOODSlotTwice: "slot_twice", GOODMainStat: "main_stat",
		GOODMainStatValue: "main_stat_value", GOODSubstat: "substat", GOODUnreadable: "unreadable",
	}[r]
}

// goodError is a reason convertGOOD skips a character for.
type goodError struct {
	reason GOODReason
	args   []any
}

func (e goodError) Error() string { return fmt.Sprintf(goodReasons[e.reason], e.args...) }

func skip(reason GOODReason, args ...any) error { return goodError{reason, args} }

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
// weapon gcsim does not have, that has no weapon, or that has a number
// the game does not allow, is skipped with the reason (GOODReason); an artifact set gcsim does not have keeps its stats, as from
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
		if err != nil {
			e := err.(goodError)
			skipped = append(skipped, GOODSkipped{Key: gc.Key, Char: key, Reason: e.reason, Args: e.args})
			continue
		}
		// Unpack checks every key and number against the game.
		back, err := Unpack(Pack([]Character{c}))
		if err != nil {
			skipped = append(skipped, GOODSkipped{Key: gc.Key, Char: key, Reason: GOODUnreadable, Err: err})
			continue
		}
		out = append(out, back[0])
	}
	return out, skipped, nil
}

func (g *gameData) convertGOOD(gc goodCharacter, key string, w goodWeapon, arts []goodArtifact) (Character, error) {
	if _, ok := lookup(key); !ok {
		if key == "manekin" || key == "manekina" {
			return Character{}, skip(GOODManekin)
		}
		return Character{}, skip(GOODUnsupported)
	}
	if w.Key == "" {
		return Character{}, skip(GOODNoWeapon)
	}
	weapon := goodKey(w.Key)
	if !g.hasWeapon(weapon) {
		return Character{}, skip(GOODUnknownWeapon, w.Key)
	}
	// The numbers Unpack allows, checked first to say which is wrong.
	t := gc.Talent
	switch {
	case gc.Level < 1 || gc.Level > 100:
		return Character{}, skip(GOODLevel, gc.Level)
	case gc.Constellation < 0 || gc.Constellation > 6:
		return Character{}, skip(GOODConstellation, gc.Constellation)
	case slices.ContainsFunc([]int{t.Auto, t.Skill, t.Burst}, func(n int) bool { return n < 1 || n > 10 }):
		// Base levels: gcsim refuses any above 10.
		return Character{}, skip(GOODTalent, t.Auto, t.Skill, t.Burst)
	case w.Level < 1 || w.Level > 100:
		return Character{}, skip(GOODWeaponLevel, w.Level)
	case w.Refinement < 1 || w.Refinement > 5:
		return Character{}, skip(GOODRefinement, w.Refinement)
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
			return Character{}, skip(GOODArtifactSlot, a.SlotKey)
		}
		if slices.ContainsFunc(pieces, func(p Artifact) bool { return p.Slot == slot }) {
			return Character{}, skip(GOODSlotTwice, slot)
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
			return Character{}, skip(GOODMainStat, a.MainStatKey)
		}
		value, ok := g.mainValue(main.gcsim, a.Rarity, a.Level)
		if !ok {
			return Character{}, skip(GOODMainStatValue, slot, a.Rarity, a.Level)
		}
		c.Stats[main.gcsim] += value
		p := Artifact{Slot: slot, Set: set, Rarity: a.Rarity, Level: a.Level, Main: StatValue{main.gcsim, value}}
		for _, s := range a.Substats {
			if s.Key == "" {
				continue // an empty line
			}
			st, ok := statsByGOOD[s.Key]
			if !ok {
				return Character{}, skip(GOODSubstat, s.Key)
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
