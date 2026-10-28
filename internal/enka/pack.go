package enka

import (
	"encoding/binary"
	"errors"
	"fmt"
	"maps"
	"math"
	"slices"
)

// packVersion starts every packed build, so links made today still read
// after the format changes. Version 1 has the totals only; version 2
// adds the artifact pieces (see appendPieces).
const packVersion = 2

// statScale is the precision stats are packed at. Config rounds to it
// too, so a packed build writes the same gcsim config as the original.
const statScale = 1e6

// Pack writes characters compactly, for builds carried in URLs or kept
// in a browser. Keys are written as gcsim keys, not as indexes into the
// game data, so packed builds survive data updates.
func Pack(chars []Character) []byte {
	b := []byte{packVersion}
	b = binary.AppendUvarint(b, uint64(len(chars)))
	for _, c := range chars {
		b = appendString(b, c.Key)
		for _, v := range []int{c.Level, c.MaxLevel, c.Cons, c.Talents[0], c.Talents[1], c.Talents[2]} {
			b = binary.AppendUvarint(b, uint64(v))
		}
		b = appendString(b, c.Weapon.Key)
		for _, v := range []int{c.Weapon.Refine, c.Weapon.Level, c.Weapon.MaxLevel} {
			b = binary.AppendUvarint(b, uint64(v))
		}
		sets := slices.Sorted(maps.Keys(c.Sets))
		b = binary.AppendUvarint(b, uint64(len(sets)))
		for _, set := range sets {
			b = appendString(b, set)
			b = binary.AppendUvarint(b, uint64(c.Sets[set]))
		}
		b = binary.AppendUvarint(b, uint64(c.UnknownSets))
		var mask uint64
		for i, name := range statOrder {
			if c.Stats[name] != 0 {
				mask |= 1 << i
			}
		}
		b = binary.AppendUvarint(b, mask)
		for i, name := range statOrder {
			if mask&(1<<i) != 0 {
				b = binary.AppendVarint(b, int64(math.Round(c.Stats[name]*statScale)))
			}
		}
		b = appendPieces(b, c, sets)
	}
	return b
}

// appendPieces writes a character's pieces, after its totals:
//
//	detail                 PieceDetail: 0 none, 1 full, 2 stripped
//	count                  pieces, unless detail is none
//	per piece, in slot order:
//	  slot | rarity<<3     one byte
//	  level
//	  main stat            index into statOrder; the value comes from the
//	                       main stat table
//	  full pieces only:
//	    set                index into the character's sorted Sets keys,
//	                       or len(Sets) for a set gcsim does not have
//	    substats           count, then per substat its index into
//	                       statOrder and its value in tenths of Enka's
//	                       display unit (39 for CRIT Rate 3.9%)
//
// Every number is a uvarint.
func appendPieces(b []byte, c Character, sets []string) []byte {
	detail := c.PieceDetail
	if detail >= numPieceDetails {
		detail = PiecesNone
	}
	b = binary.AppendUvarint(b, uint64(detail))
	if detail == PiecesNone {
		return b
	}
	b = binary.AppendUvarint(b, uint64(len(c.Pieces)))
	for _, p := range c.Pieces {
		b = binary.AppendUvarint(b, uint64(p.Slot)|uint64(p.Rarity)<<3)
		b = binary.AppendUvarint(b, uint64(p.Level))
		b = binary.AppendUvarint(b, uint64(slices.Index(statOrder, p.Main.Key)))
		if detail != PiecesFull {
			continue
		}
		set := slices.Index(sets, p.Set)
		if p.Set == "" || set < 0 {
			set = len(sets)
		}
		b = binary.AppendUvarint(b, uint64(set))
		b = binary.AppendUvarint(b, uint64(len(p.Subs)))
		for _, s := range p.Subs {
			b = binary.AppendUvarint(b, uint64(slices.Index(statOrder, s.Key)))
			b = binary.AppendUvarint(b, tenths(s))
		}
	}
	return b
}

// MaxPacked is the most characters Unpack reads.
const MaxPacked = 256

var errPacked = errors.New("malformed packed build")

// Unpack reads characters written by Pack. Since anyone can write a
// packed build, every key must be one the game data knows and every
// number must be one the game allows.
func Unpack(b []byte) ([]Character, error) {
	dataOnce.Do(func() { data, dataErr = loadGameData() })
	if dataErr != nil {
		return nil, dataErr
	}
	r := &reader{b: b}
	version := r.byte()
	if version < 1 || version > packVersion {
		return nil, fmt.Errorf("%w: unknown version", errPacked)
	}
	n := r.uint(MaxPacked)
	var out []Character
	for range n {
		if r.err != nil {
			break
		}
		c := Character{
			Key:      r.string(),
			Level:    r.int(1, 100),
			MaxLevel: r.int(1, 100),
			Cons:     r.int(0, 6),
			Talents:  [3]int{r.int(1, 15), r.int(1, 15), r.int(1, 15)},
			Weapon: Weapon{
				Key:      r.string(),
				Refine:   r.int(1, 5),
				Level:    r.int(1, 100),
				MaxLevel: r.int(1, 100),
			},
			Sets:  map[string]int{},
			Stats: map[string]float64{},
		}
		pieces := 0
		var sets []string
		for range r.uint(5) {
			set, count := r.string(), r.int(1, 5)
			if !data.hasArtifact(set) {
				r.fail("unknown artifact set %q", set)
			}
			if len(sets) > 0 && set <= sets[len(sets)-1] {
				r.fail("artifact sets out of order")
			}
			sets = append(sets, set)
			c.Sets[set] += count
			pieces += count
		}
		c.UnknownSets = r.int(0, 5)
		if pieces+c.UnknownSets > 5 {
			r.fail("more than five artifacts")
		}
		mask := r.uint(1<<len(statOrder) - 1)
		for i, name := range statOrder {
			if mask&(1<<i) != 0 {
				v := float64(r.varint()) / statScale
				if math.Abs(v) > 1e5 {
					r.fail("stat %s out of range", name)
				}
				c.Stats[name] = v
			}
		}
		if version >= 2 {
			r.pieces(&c, sets)
		}
		if _, ok := lookup(c.Key); !ok {
			r.fail("unknown character %q", c.Key)
		}
		if !data.hasWeapon(c.Weapon.Key) {
			r.fail("unknown weapon %q", c.Weapon.Key)
		}
		if c.MaxLevel < c.Level || c.Weapon.MaxLevel < c.Weapon.Level {
			r.fail("level above its cap")
		}
		out = append(out, c)
	}
	if r.err == nil && len(r.b) > 0 {
		r.fail("trailing bytes")
	}
	if r.err != nil {
		return nil, r.err
	}
	return out, nil
}

// pieces reads what appendPieces wrote and checks it against the game's
// limits, the character's set counts and its stat totals.
func (r *reader) pieces(c *Character, sets []string) {
	c.PieceDetail = PieceDetail(r.uint(uint64(numPieceDetails) - 1))
	if c.PieceDetail == PiecesNone {
		return
	}
	n := r.uint(uint64(numSlots))
	if r.err != nil {
		return
	}
	c.Pieces = make([]Artifact, 0, n)
	counts := map[string]int{}
	for range n {
		head := r.uint(5<<3 | uint64(numSlots) - 1)
		p := Artifact{Slot: Slot(head & 7), Rarity: int(head >> 3), Level: r.int(0, 20)}
		p.Main.Key = r.stat()
		p.Main.Value, _ = data.mainValue(p.Main.Key, p.Rarity, p.Level)
		if c.PieceDetail == PiecesFull {
			if i := r.uint(uint64(len(sets))); int(i) < len(sets) {
				p.Set = sets[i]
			}
			counts[p.Set]++
			for range r.uint(4) {
				key := r.stat()
				p.Subs = append(p.Subs, StatValue{key, fromTenths(key, r.uint(1e5))})
			}
		}
		c.Pieces = append(c.Pieces, p)
	}
	if r.err != nil {
		return
	}
	if err := data.checkPieces(c.Pieces, c.PieceDetail); err != nil {
		r.fail("%v", err)
		return
	}
	// Full pieces must add up to the set counts the config is built from;
	// stripped pieces carry no set, but there are no more of them than
	// artifacts.
	if c.PieceDetail == PiecesFull {
		counts[""] -= c.UnknownSets
		for set, n := range c.Sets {
			counts[set] -= n
		}
		for _, n := range counts {
			if n != 0 {
				r.fail("pieces do not match the set counts")
				return
			}
		}
	} else if len(c.Pieces) != pieceCount(c) {
		r.fail("pieces do not match the set counts")
		return
	}
	if err := checkTotals(c); err != nil {
		r.fail("%v", err)
	}
}

func pieceCount(c *Character) int {
	n := c.UnknownSets
	for _, v := range c.Sets {
		n += v
	}
	return n
}

// stat reads an index into statOrder.
func (r *reader) stat() string {
	return statOrder[r.uint(uint64(len(statOrder))-1)]
}

func (g *gameData) hasWeapon(key string) bool {
	for _, k := range g.weapons {
		if k == key {
			return true
		}
	}
	return false
}

func (g *gameData) hasArtifact(key string) bool {
	for _, k := range g.artifacts {
		if k == key {
			return true
		}
	}
	return false
}

func appendString(b []byte, s string) []byte {
	b = binary.AppendUvarint(b, uint64(len(s)))
	return append(b, s...)
}

// reader reads a packed build, keeping the first error.
type reader struct {
	b   []byte
	err error
}

func (r *reader) fail(format string, args ...any) {
	if r.err == nil {
		r.err = fmt.Errorf("%w: %s", errPacked, fmt.Sprintf(format, args...))
	}
}

func (r *reader) byte() byte {
	if len(r.b) == 0 {
		r.fail("too short")
		return 0
	}
	v := r.b[0]
	r.b = r.b[1:]
	return v
}

// uint reads an unsigned number no larger than max.
func (r *reader) uint(max uint64) uint64 {
	v, n := binary.Uvarint(r.b)
	if n <= 0 {
		r.fail("too short")
		return 0
	}
	r.b = r.b[n:]
	if v > max {
		r.fail("number %d above %d", v, max)
		return 0
	}
	return v
}

// int reads a number in [lo, hi].
func (r *reader) int(lo, hi int) int {
	v := int(r.uint(uint64(hi)))
	if r.err == nil && v < lo {
		r.fail("number %d below %d", v, lo)
	}
	return v
}

func (r *reader) varint() int64 {
	v, n := binary.Varint(r.b)
	if n <= 0 {
		r.fail("too short")
		return 0
	}
	r.b = r.b[n:]
	return v
}

// string reads a gcsim key: short, lower-case letters and digits.
func (r *reader) string() string {
	n := int(r.uint(64))
	if r.err != nil || len(r.b) < n {
		r.fail("too short")
		return ""
	}
	s := string(r.b[:n])
	r.b = r.b[n:]
	for _, c := range []byte(s) {
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') {
			r.fail("bad key %q", s)
			break
		}
	}
	return s
}
