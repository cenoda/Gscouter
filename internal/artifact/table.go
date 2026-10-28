package artifact

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"slices"
	"sync"
)

// TableFormat is the version of the small table's layout.
const TableFormat = 1

// Table is the small weights table: what the result page needs of the
// full one (teams/weights.json, internal/weights.Table), per library
// team with weights. cmd/weights -slim derives it; it is embedded from
// table.json.
type Table struct {
	Format int `json:"format"`
	// Gcsim is the gcsim module the weights were calculated with,
	// path@version: the one most teams were, for a team without its own.
	Gcsim string           `json:"gcsim"`
	Teams map[string]*Team `json:"teams"`
}

// Team is one team's characters, in the team's order.
type Team struct {
	// Gcsim is set when the team was calculated with another gcsim than
	// the table's; Lookup fills it in from the table otherwise.
	Gcsim      string      `json:"gcsim,omitempty"`
	Characters []Character `json:"characters"`
}

// Character is one character's weights and main stat judgements.
type Character struct {
	Key string `json:"key"` // gcsim key
	// Weights are the substats' weights, above 0 and at most 1, rounded
	// to 3 decimals; a substat without one has weight 0.
	Weights map[string]float64 `json:"weights,omitempty"`
	// Unplayable lists the substats whose roll makes the rotation
	// impossible to play; they have weight 0.
	Unplayable []string `json:"unplayable,omitempty"`
	Sands      *Slot    `json:"sands,omitempty"`
	Goblet     *Slot    `json:"goblet,omitempty"`
	Circlet    *Slot    `json:"circlet,omitempty"`
}

// Character returns the team's character with the gcsim key, nil if
// none.
func (t *Team) Character(key string) *Character {
	for i := range t.Characters {
		if t.Characters[i].Key == key {
			return &t.Characters[i]
		}
	}
	return nil
}

// Slot is how one slot's main stats are judged. A main stat of the
// slot's options is right, wrong (costing a penalty) or unplayable; on
// a LowImpact slot every playable one is right.
type Slot struct {
	// Right are the main stats within 2% of the best, in option order;
	// empty on a LowImpact slot, where it would list every playable
	// option.
	Right []string `json:"right,omitempty"`
	// Penalty and Others are each wrong main stat's cost in score
	// points, rounded to 1 decimal: Penalty[key], or Others for a wrong
	// main stat Penalty does not list. Others is the penalty most wrong
	// main stats share (the ones that do nothing for the team, such as
	// off-element goblets), so the table need not repeat it. Both are 0
	// when the character's substats gain the team nothing, so a loss
	// cannot be converted.
	Penalty map[string]float64 `json:"penalty,omitempty"`
	Others  float64            `json:"others,omitempty"`
	// LowImpact is true when every main stat is right.
	LowImpact bool `json:"lowImpact,omitempty"`
	// Best is the main stat that scored best, on a LowImpact slot only.
	Best string `json:"best,omitempty"`
	// Unplayable lists the main stats the rotation cannot be played
	// with, in option order.
	Unplayable []string `json:"unplayable,omitempty"`
}

// ReadTable reads a whole table WriteTo wrote.
func ReadTable(r io.Reader) (*Table, error) {
	var t Table
	if err := json.NewDecoder(r).Decode(&t); err != nil {
		return nil, err
	}
	if t.Format != TableFormat {
		return nil, fmt.Errorf("table has format %d, want %d", t.Format, TableFormat)
	}
	if t.Teams == nil {
		t.Teams = map[string]*Team{}
	}
	return &t, nil
}

// WriteTo writes the table as compact JSON with one team per line, in
// id order, so that a rerun's changes show team by team in a diff.
func (t *Table) WriteTo(w io.Writer) (int64, error) {
	var b bytes.Buffer
	g, _ := json.Marshal(t.Gcsim)
	fmt.Fprintf(&b, "{\n\"format\": %d,\n\"gcsim\": %s,\n\"teams\": {", t.Format, g)
	for i, id := range slices.Sorted(maps.Keys(t.Teams)) {
		k, _ := json.Marshal(id)
		v, err := json.Marshal(t.Teams[id])
		if err != nil {
			return 0, err
		}
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString("\n")
		b.Write(k)
		b.WriteString(": ")
		b.Write(v)
	}
	if len(t.Teams) > 0 {
		b.WriteString("\n")
	}
	b.WriteString("}\n}\n")
	n, err := w.Write(b.Bytes())
	return int64(n), err
}

//go:embed table.json
var embedded []byte

// index locates each team in a table's bytes, so that only the team
// looked up is parsed: the whole table parsed would keep tens of
// thousands of maps on the heap for the server's lifetime.
type index struct {
	data   []byte
	format int
	gcsim  string
	teams  map[string][2]int // id → the team's JSON in data
}

func newIndex(data []byte) (*index, error) {
	ix := &index{data: data, teams: map[string][2]int{}}
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := expect(dec, json.Delim('{')); err != nil {
		return nil, err
	}
	for dec.More() {
		key, err := stringToken(dec)
		if err != nil {
			return nil, err
		}
		switch key {
		case "format":
			err = dec.Decode(&ix.format)
		case "gcsim":
			err = dec.Decode(&ix.gcsim)
		case "teams":
			err = ix.readTeams(dec)
		default:
			var skip json.RawMessage
			err = dec.Decode(&skip)
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}
	}
	if ix.format != TableFormat {
		return nil, fmt.Errorf("table has format %d, want %d", ix.format, TableFormat)
	}
	return ix, nil
}

func (ix *index) readTeams(dec *json.Decoder) error {
	if err := expect(dec, json.Delim('{')); err != nil {
		return err
	}
	for dec.More() {
		id, err := stringToken(dec)
		if err != nil {
			return err
		}
		start := int(dec.InputOffset())
		var skip struct{}
		if err := dec.Decode(&skip); err != nil {
			return fmt.Errorf("%s: %w", id, err)
		}
		// InputOffset after the key is before the colon.
		for start < len(ix.data) && (ix.data[start] == ':' || ix.data[start] == ' ' || ix.data[start] == '\n' || ix.data[start] == '\t' || ix.data[start] == '\r') {
			start++
		}
		ix.teams[id] = [2]int{start, int(dec.InputOffset())}
	}
	return expect(dec, json.Delim('}'))
}

func expect(dec *json.Decoder, want json.Delim) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	if tok != want {
		return fmt.Errorf("got %v, want %v", tok, want)
	}
	return nil
}

func stringToken(dec *json.Decoder) (string, error) {
	tok, err := dec.Token()
	if err != nil {
		return "", err
	}
	s, ok := tok.(string)
	if !ok {
		return "", fmt.Errorf("got %v, want a key", tok)
	}
	return s, nil
}

func (ix *index) lookup(id string) (*Team, error) {
	at, ok := ix.teams[id]
	if !ok {
		return nil, nil
	}
	var t Team
	if err := json.Unmarshal(ix.data[at[0]:at[1]], &t); err != nil {
		return nil, fmt.Errorf("%s: %w", id, err)
	}
	if t.Gcsim == "" {
		t.Gcsim = ix.gcsim
	}
	return &t, nil
}

var loadEmbedded = sync.OnceValue(func() *index {
	ix, err := newIndex(embedded)
	if err != nil {
		// table.json is checked by the tests; a broken one is a build
		// mistake, not a runtime condition.
		panic("artifact: table.json: " + err.Error())
	}
	return ix
})

// Lookup returns the embedded table's entry for a library team id, and
// false when the team has no weights (not in the library, or skipped
// when the weights were calculated). Each call parses the team afresh,
// so the caller may change what it gets.
func Lookup(id string) (*Team, bool) {
	t, err := loadEmbedded().lookup(id)
	if err != nil {
		panic("artifact: table.json: " + err.Error())
	}
	return t, t != nil
}

// Version returns the embedded table's format and gcsim module.
func Version() (format int, gcsim string) {
	ix := loadEmbedded()
	return ix.format, ix.gcsim
}

// Len returns the number of teams in the embedded table.
func Len() int { return len(loadEmbedded().teams) }
