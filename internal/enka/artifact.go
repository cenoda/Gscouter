package enka

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
)

// Slot is an artifact's slot, in the game's order.
type Slot uint8

const (
	Flower Slot = iota
	Plume
	Sands
	Goblet
	Circlet
	numSlots
)

var slotNames = [numSlots]string{"flower", "plume", "sands", "goblet", "circlet"}

// String returns the slot's GOOD key, e.g. "sands".
func (s Slot) String() string {
	if s < numSlots {
		return slotNames[s]
	}
	return "slot" + strconv.Itoa(int(s))
}

// equipSlots maps Enka's equipType to a slot.
var equipSlots = map[string]Slot{
	"EQUIP_BRACER":   Flower,
	"EQUIP_NECKLACE": Plume,
	"EQUIP_SHOES":    Sands,
	"EQUIP_RING":     Goblet,
	"EQUIP_DRESS":    Circlet,
}

// mainStatsBySlot lists the main stats the game rolls on each slot.
var mainStatsBySlot = [numSlots][]string{
	Flower:  {"hp"},
	Plume:   {"atk"},
	Sands:   {"hp%", "atk%", "def%", "er", "em"},
	Goblet:  {"hp%", "atk%", "def%", "em", "pyro%", "hydro%", "anemo%", "electro%", "dendro%", "cryo%", "geo%", "phys%"},
	Circlet: {"hp%", "atk%", "def%", "em", "cr", "cd", "heal"},
}

// maxRolls is the largest 5-star substat roll, in Enka's display units
// (percent for percentages). Lower rarities roll less.
var maxRolls = map[string]float64{
	"hp": 298.75, "atk": 19.45, "def": 23.15, "hp%": 5.83, "atk%": 5.83,
	"def%": 7.29, "em": 23.31, "er": 6.48, "cr": 3.89, "cd": 7.77,
}

// PieceDetail says how much of each artifact piece a build carries.
type PieceDetail uint8

const (
	// PiecesNone: only the totals in Sets and Stats, as in links packed
	// before pieces were kept (format version 1).
	PiecesNone PieceDetail = iota
	// PiecesFull: every piece with its set and substats.
	PiecesFull
	// PiecesStripped: per piece only slot, rarity, level and main stat,
	// enough to separate main stats from substats in Stats. The sharer
	// chose to leave set and substats out (docs/artifacts.md §2).
	PiecesStripped
	numPieceDetails
)

// StatValue is one stat on an artifact, in gcsim terms: Key is a gcsim
// stat name ("cr", "atk%"), Value is at gcsim scale (0.039 for 3.9%).
type StatValue struct {
	Key   string
	Value float64
}

// Artifact is one artifact piece.
type Artifact struct {
	Slot Slot
	// Set is the gcsim set key, or "" if gcsim does not have the set (it
	// is then counted in UnknownSets) or the piece is stripped.
	Set    string
	Rarity int // 1 to 5 stars
	Level  int // 0 to 4 × Rarity
	// Main is the main stat, valued from the main stat table as in Stats.
	Main StatValue
	// Subs are the substats, at most four, as Enka shows them (one
	// decimal for percentages, whole numbers for flat stats). Nil when
	// the piece is stripped.
	Subs []StatValue
}

// Stripped returns c with its pieces stripped to slot, rarity, level and
// main stat. Sets and Stats are unchanged, so the score is the same.
func (c Character) Stripped() Character {
	if c.PieceDetail != PiecesFull {
		return c
	}
	pieces := make([]Artifact, len(c.Pieces))
	for i, p := range c.Pieces {
		pieces[i] = Artifact{Slot: p.Slot, Rarity: p.Rarity, Level: p.Level, Main: p.Main}
	}
	c.Pieces, c.PieceDetail = pieces, PiecesStripped
	return c
}

// Fingerprint identifies a full piece for the duplicate check
// (docs/artifacts.md §3): Enka gives artifacts no instance ID, so two
// pieces with the same set, slot, rarity, level, main stat and substats
// are taken to be the same artifact. It is only meaningful for pieces of
// a build with PiecesFull.
func (a Artifact) Fingerprint() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s/%s/%d/%d/%s", a.Set, a.Slot, a.Rarity, a.Level, a.Main.Key)
	for _, s := range a.Subs {
		fmt.Fprintf(&b, "/%s=%d", s.Key, tenths(s))
	}
	return b.String()
}

// tenths returns a substat's value in tenths of Enka's display unit, the
// precision substats are kept and packed at.
func tenths(s StatValue) uint64 {
	return uint64(math.Round(s.Value * statsByName[s.Key].scale * 10))
}

// fromTenths is the inverse of tenths.
func fromTenths(key string, n uint64) float64 {
	return float64(n) / 10 / statsByName[key].scale
}

// statsByName is stats keyed by gcsim stat name.
var statsByName = func() map[string]stat {
	m := map[string]stat{}
	for _, s := range stats {
		m[s.gcsim] = s
	}
	return m
}()

// mainValue looks a main stat's value up in the main stat table.
func (g *gameData) mainValue(key string, rarity, level int) (float64, bool) {
	table := g.mainStats[strconv.Itoa(rarity)][statsByName[key].good]
	if level < 0 || level >= len(table) {
		return 0, false
	}
	return table[level], true
}

// checkPieces checks pieces against the game's limits: one piece per
// slot in slot order, a main stat the slot rolls, a level the rarity
// reaches, and at most four distinct substats, none the main stat, each
// no larger than its rolls so far could make it.
func (g *gameData) checkPieces(pieces []Artifact, detail PieceDetail) error {
	if len(pieces) > int(numSlots) {
		return fmt.Errorf("more than five artifacts")
	}
	for i, p := range pieces {
		if p.Slot >= numSlots || (i > 0 && p.Slot <= pieces[i-1].Slot) {
			return fmt.Errorf("artifact slots out of order")
		}
		if p.Rarity < 1 || p.Rarity > 5 || p.Level < 0 || p.Level > 4*p.Rarity {
			return fmt.Errorf("%s: rarity %d level %d", p.Slot, p.Rarity, p.Level)
		}
		if !slices.Contains(mainStatsBySlot[p.Slot], p.Main.Key) {
			return fmt.Errorf("%s: main stat %q", p.Slot, p.Main.Key)
		}
		if v, ok := g.mainValue(p.Main.Key, p.Rarity, p.Level); !ok || v != p.Main.Value {
			return fmt.Errorf("%s: main stat value", p.Slot)
		}
		if detail == PiecesStripped {
			if p.Set != "" || p.Subs != nil {
				return fmt.Errorf("%s: stripped piece with set or substats", p.Slot)
			}
			continue
		}
		if p.Set != "" && !g.hasArtifact(p.Set) {
			return fmt.Errorf("unknown artifact set %q", p.Set)
		}
		if len(p.Subs) > 4 {
			return fmt.Errorf("%s: more than four substats", p.Slot)
		}
		// A substat rolls once when it appears and at most once more
		// every four levels.
		rolls := 1 + p.Level/4
		for j, s := range p.Subs {
			max, ok := maxRolls[s.Key]
			if !ok || s.Key == p.Main.Key {
				return fmt.Errorf("%s: substat %q", p.Slot, s.Key)
			}
			for _, t := range p.Subs[:j] {
				if t.Key == s.Key {
					return fmt.Errorf("%s: substat %q twice", p.Slot, s.Key)
				}
			}
			if n := tenths(s); n == 0 || float64(n) > math.Ceil(max*10*float64(rolls)) {
				return fmt.Errorf("%s: substat %s out of range", p.Slot, s.Key)
			}
		}
	}
	return nil
}

// totalsSlack is how far a total may be from its pieces' sum beyond the
// substats' rounding: the precision stats are packed at, summed.
const totalsSlack = 1e-5

// checkTotals checks a build's pieces against Stats, the totals its
// score is calculated from, so that a link cannot show pieces other than
// the ones it was scored with. Full pieces add up to Stats, give or take
// half a tenth of Enka's display unit per substat (pieces keep one
// decimal, Stats Enka's value). Stripped pieces have only main stats:
// Stats holds at least those, and exactly those of the stats no
// substat has (Healing Bonus, elemental DMG Bonus).
func checkTotals(c *Character) error {
	sum, round := map[string]float64{}, map[string]float64{}
	for _, p := range c.Pieces {
		sum[p.Main.Key] += p.Main.Value
		for _, s := range p.Subs {
			sum[s.Key] += s.Value
			round[s.Key] += 0.05 / statsByName[s.Key].scale
		}
	}
	for _, key := range statOrder {
		d := c.Stats[key] - sum[key]
		_, sub := maxRolls[key]
		switch {
		case c.PieceDetail == PiecesFull && math.Abs(d) > round[key]+totalsSlack,
			c.PieceDetail == PiecesStripped && (d < -totalsSlack || !sub && d > totalsSlack):
			return fmt.Errorf("pieces do not add up to the %s total", key)
		}
	}
	return nil
}
