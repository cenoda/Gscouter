package weights

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/cenoda/gscouter/internal/enka"
)

// Stat is one key=value of an add stats line.
type Stat struct {
	Key   string
	Value float64
}

// Build is a team's build.txt split into what the weights change: per
// character, the main stats and the substat totals.
type Build struct {
	lines []string
	Chars []Char
}

// Char is one character of a Build.
type Char struct {
	Key string // gcsim key, as the build writes it
	// Main holds the sands, goblet and circlet main stats, in Slots
	// order.
	Main [3]Stat
	// Rarity is each of those pieces' rarity, 5 or 4, told from the
	// value; a variant puts its main stat at the same rarity's maximum.
	Rarity [3]int
	// Reordered is true when the main stat line does not write them in
	// the order flower, plume, sands, goblet, circlet; each was assigned
	// to the slot that can have it.
	Reordered bool
	Subs      []Stat // the substat totals

	mainLine, subLine int
	tokens            []string // the main stat line's key=value tokens
	slotToken         [3]int   // the token of each slot's main stat
}

// The slots with a main stat to choose, in order.
var Slots = []string{"sands", "goblet", "circlet"}

// Options are the main stats each slot can have in the game, by gcsim
// key (Slots order): sands 5, goblet 12, circlet 7.
var Options = [3][]string{
	{"hp%", "atk%", "def%", "em", "er"},
	{"hp%", "atk%", "def%", "em", "pyro%", "hydro%", "electro%", "cryo%", "anemo%", "geo%", "dendro%", "phys%"},
	{"hp%", "atk%", "def%", "em", "cr", "cd", "heal"},
}

// MainValue is the main stat of a fully levelled artifact of the given
// rarity: level 20 for a 5★, 16 for a 4★.
func MainValue(key string, rarity int) (float64, bool) {
	switch rarity {
	case 5:
		return enka.MainStat(5, 20, key)
	case 4:
		return enka.MainStat(4, 16, key)
	}
	return 0, false
}

// mainTolerance is how far, relatively, a written main stat may be from
// the game's value: configs write 187 for 186.5 EM.
const mainTolerance = 0.01

// rarityOf returns the rarity of the fully levelled artifact whose main
// stat s is, 0 for none.
func rarityOf(s Stat) int {
	for _, r := range []int{5, 4} {
		if want, ok := MainValue(s.Key, r); ok && math.Abs(s.Value-want) <= mainTolerance*want {
			return r
		}
	}
	return 0
}

var addStats = regexp.MustCompile(`^\s*\w+\s+add\s+stats\b`)

var statsLine = regexp.MustCompile(`^\s*(\w+)\s+add\s+stats\s+([^;#]*);\s*$`)

// ParseBuild splits a library build.txt. Each character must have two
// add stats lines: first its five main stats (flower, plume, sands,
// goblet, circlet; keys can repeat), each the value of a fully levelled
// 5★ or 4★ piece and one the slot can have, then its substat totals,
// only substat keys. The error says
// what does not hold.
func ParseBuild(text string) (*Build, error) {
	b := &Build{lines: strings.Split(text, "\n")}
	lines := map[string][]int{}
	var order []string
	for i, line := range b.lines {
		if !addStats.MatchString(line) {
			continue
		}
		m := statsLine.FindStringSubmatch(line)
		if m == nil {
			return nil, fmt.Errorf("line %d: an add stats line not in the form \"<char> add stats k=v ...;\"", i+1)
		}
		if _, ok := lines[m[1]]; !ok {
			order = append(order, m[1])
		}
		lines[m[1]] = append(lines[m[1]], i)
	}
	if len(order) == 0 {
		return nil, fmt.Errorf("no add stats lines")
	}
	for _, key := range order {
		ls := lines[key]
		if len(ls) != 2 {
			return nil, fmt.Errorf("%s: %d add stats lines, want 2 (main stats, then substats)", key, len(ls))
		}
		c := Char{Key: key, mainLine: ls[0], subLine: ls[1]}
		var err error
		if c.tokens, err = tokens(b.lines[ls[0]]); err != nil {
			return nil, fmt.Errorf("%s: main stats: %w", key, err)
		}
		if err := c.assign(); err != nil {
			return nil, fmt.Errorf("%s: main stats: %w", key, err)
		}
		subs, err := tokens(b.lines[ls[1]])
		if err != nil {
			return nil, fmt.Errorf("%s: substats: %w", key, err)
		}
		for _, t := range subs {
			s, _ := parseStat(t)
			if !slices.ContainsFunc(Substats, func(r Roll) bool { return r.Key == s.Key }) {
				return nil, fmt.Errorf("%s: substats: %s is not a substat", key, s.Key)
			}
			c.Subs = append(c.Subs, s)
		}
		b.Chars = append(b.Chars, c)
	}
	return b, nil
}

// statToken is one key=value of an add stats line. gcsim allows spaces
// around "=" and a constant expression as the value; configs write
// products such as def%=0.062*2.
var statToken = regexp.MustCompile(`([a-z]+%?)\s*=\s*([0-9.]+(?:\s*[*/]\s*[0-9.]+)*)`)

// tokens returns the stats of an add stats line, each as key=value
// without spaces.
func tokens(line string) ([]string, error) {
	m := statsLine.FindStringSubmatch(line)
	if rest := strings.TrimSpace(statToken.ReplaceAllString(m[2], "")); rest != "" {
		return nil, fmt.Errorf("%q is not key=value", strings.Fields(rest)[0])
	}
	var out []string
	for _, t := range statToken.FindAllStringSubmatch(m[2], -1) {
		tok := t[1] + "=" + strings.Join(strings.Fields(t[2]), "")
		if _, err := parseStat(tok); err != nil {
			return nil, err
		}
		out = append(out, tok)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no stats")
	}
	return out, nil
}

// parseStat parses a token from tokens: a number, or numbers
// multiplied and divided.
func parseStat(tok string) (Stat, error) {
	k, v, _ := strings.Cut(tok, "=")
	x, op := 1.0, byte('*')
	for {
		i := strings.IndexAny(v, "*/")
		num := v
		if i >= 0 {
			num = v[:i]
		}
		f, err := strconv.ParseFloat(num, 64)
		if err != nil {
			return Stat{}, fmt.Errorf("%q: %v", tok, err)
		}
		if op == '*' {
			x *= f
		} else {
			x /= f
		}
		if i < 0 {
			return Stat{k, x}, nil
		}
		op, v = v[i], v[i+1:]
	}
}

// assign finds the flower (flat HP), the plume (flat ATK) and a slot
// for each of the other three main stats. The written order is kept
// when the game allows it; otherwise the first assignment it allows is
// taken, so that "atk%, cr, pyro%" reads as ATK% sands, Pyro DMG%
// goblet and CRIT Rate circlet.
func (c *Char) assign() error {
	if len(c.tokens) != 5 {
		return fmt.Errorf("%d stats, want 5 (flower, plume, sands, goblet, circlet)", len(c.tokens))
	}
	var rest []int
	var flower, plume bool
	for i, t := range c.tokens {
		s, _ := parseStat(t)
		switch {
		case s.Key == "hp" && !flower:
			flower = true
		case s.Key == "atk" && !plume:
			plume = true
		default:
			rest = append(rest, i)
			continue
		}
		if rarityOf(s) == 0 {
			return fmt.Errorf("%s=%v is not the main stat of a fully levelled 5★ or 4★ artifact", s.Key, s.Value)
		}
	}
	if !flower || !plume || len(rest) != 3 {
		return fmt.Errorf("no flower (hp=4780) or no plume (atk=311)")
	}
	for _, i := range rest {
		if s, _ := parseStat(c.tokens[i]); rarityOf(s) == 0 {
			return fmt.Errorf("%s=%v is not the main stat of a fully levelled 5★ or 4★ artifact", s.Key, s.Value)
		}
	}
	for n, perm := range permutations3 {
		ok := true
		for slot, j := range perm {
			s, _ := parseStat(c.tokens[rest[j]])
			if !slices.Contains(Options[slot], s.Key) {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		for slot, j := range perm {
			c.slotToken[slot] = rest[j]
			c.Main[slot], _ = parseStat(c.tokens[rest[j]])
			c.Rarity[slot] = rarityOf(c.Main[slot])
		}
		c.Reordered = n > 0 || rest[0] != 2 || rest[1] != 3
		return nil
	}
	var keys []string
	for _, i := range rest {
		s, _ := parseStat(c.tokens[i])
		keys = append(keys, s.Key)
	}
	return fmt.Errorf("%s cannot be the main stats of a sands, a goblet and a circlet", strings.Join(keys, ", "))
}

// permutations3 lists the orders of three items, the identity first.
var permutations3 = [][3]int{{0, 1, 2}, {0, 2, 1}, {1, 0, 2}, {1, 2, 0}, {2, 0, 1}, {2, 1, 0}}

// String is the build as given.
func (b *Build) String() string { return strings.Join(b.lines, "\n") }

// WithSub returns the build with value added to character c's substat
// key. gcsim sums repeated keys, so the roll is appended to the line.
func (b *Build) WithSub(c int, key string, value float64) string {
	ch := b.Chars[c]
	line := strings.TrimRight(b.lines[ch.subLine], " \t")
	line = strings.TrimSuffix(line, ";") + " " + key + "=" + format(value) + ";"
	return b.with(ch.subLine, line)
}

// WithMain returns the build with character c's main stat in the given
// slot (an index into Slots) replaced by key, at the maximum value of
// the piece's rarity.
// Only that one stat changes: a set bonus or another piece with the
// same key stays.
func (b *Build) WithMain(c, slot int, key string) (string, error) {
	ch := b.Chars[c]
	value, ok := MainValue(key, ch.Rarity[slot])
	if !ok {
		return "", fmt.Errorf("%s is not a main stat", key)
	}
	toks := slices.Clone(ch.tokens)
	toks[ch.slotToken[slot]] = key + "=" + format(value)
	line := ch.Key + " add stats " + strings.Join(toks, " ") + ";"
	return b.with(ch.mainLine, line), nil
}

func (b *Build) with(i int, line string) string {
	lines := slices.Clone(b.lines)
	lines[i] = line
	return strings.Join(lines, "\n")
}

func format(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }
