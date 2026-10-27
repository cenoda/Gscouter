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
// after the format changes.
const packVersion = 1

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
	if r.byte() != packVersion {
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
		for range r.uint(5) {
			set, count := r.string(), r.int(1, 5)
			if !data.hasArtifact(set) {
				r.fail("unknown artifact set %q", set)
			}
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
