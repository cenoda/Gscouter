package weights

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"slices"
)

// TableFormat is the version of the table's layout.
const TableFormat = 1

// Table is the weights of every library team, joined from the shards
// of cmd/weights runs (Join). It lives at teams/weights.json, beside the
// library it belongs to, to be embedded in the binary with it.
type Table struct {
	Format int               `json:"format"`
	Teams  map[string]*Entry `json:"teams"`
	// Skipped lists the teams without weights, with the reason.
	Skipped map[string]string `json:"skipped"`
}

// Entry is one team's weights.
type Entry struct {
	Gcsim      string      `json:"gcsim"`
	Iterations int         `json:"iterations"`
	DPS        float64     `json:"dps"` // the reference build's looped score
	Filler     string      `json:"filler,omitempty"`
	Characters []Character `json:"characters"`
}

// Totals sums the cost of the results Join read.
type Totals struct {
	Teams, Computed, Skipped, Failed int
	Simulations                      int
	Seconds                          float64
}

// ReadResults reads cmd/weights -json output: one Result per line.
func ReadResults(r io.Reader) ([]*Result, error) {
	var out []*Result
	sc := bufio.NewScanner(r)
	sc.Buffer(nil, 64<<20)
	for n := 1; sc.Scan(); n++ {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var res Result
		if err := json.Unmarshal(line, &res); err != nil {
			return nil, fmt.Errorf("line %d: %w", n, err)
		}
		out = append(out, &res)
	}
	return out, sc.Err()
}

// Join adds results to base (nil for an empty table) and returns the
// table. A result replaces the team's earlier entry or reason: a team
// calculated again is up to date, and one that now fails or is skipped
// has no weights.
func Join(base *Table, results []*Result) (*Table, Totals, error) {
	t := &Table{Format: TableFormat, Teams: map[string]*Entry{}, Skipped: map[string]string{}}
	if base != nil {
		if base.Format != TableFormat {
			return nil, Totals{}, fmt.Errorf("base table has format %d, want %d", base.Format, TableFormat)
		}
		maps.Copy(t.Teams, base.Teams)
		maps.Copy(t.Skipped, base.Skipped)
	}
	var tot Totals
	seen := map[string]bool{}
	for _, r := range results {
		if r.ID == "" {
			return nil, tot, fmt.Errorf("a result without a team id (%s)", r.Dir)
		}
		if seen[r.ID] {
			return nil, tot, fmt.Errorf("team %s appears twice", r.ID)
		}
		seen[r.ID] = true
		tot.Teams++
		tot.Simulations += r.Simulations
		tot.Seconds += r.Seconds
		delete(t.Teams, r.ID)
		delete(t.Skipped, r.ID)
		switch {
		case r.Error != "":
			tot.Failed++
			t.Skipped[r.ID] = "error: " + r.Error
		case r.Skipped != "":
			tot.Skipped++
			t.Skipped[r.ID] = r.Skipped
		default:
			tot.Computed++
			t.Teams[r.ID] = &Entry{Gcsim: r.Gcsim, Iterations: r.Iterations, DPS: r.DPS, Filler: r.Filler, Characters: r.Characters}
		}
	}
	return t, tot, nil
}

// ReadTable reads a table WriteTo wrote.
func ReadTable(r io.Reader) (*Table, error) {
	var t Table
	if err := json.NewDecoder(r).Decode(&t); err != nil {
		return nil, err
	}
	if t.Teams == nil {
		t.Teams = map[string]*Entry{}
	}
	if t.Skipped == nil {
		t.Skipped = map[string]string{}
	}
	return &t, nil
}

// WriteTo writes the table as JSON with one team per line, in id
// order, so that a rerun's changes show team by team in a diff.
func (t *Table) WriteTo(w io.Writer) (int64, error) {
	var b bytes.Buffer
	fmt.Fprintf(&b, "{\n\"format\": %d,\n\"teams\": {", t.Format)
	if err := writeMap(&b, t.Teams); err != nil {
		return 0, err
	}
	b.WriteString("},\n\"skipped\": {")
	if err := writeMap(&b, t.Skipped); err != nil {
		return 0, err
	}
	b.WriteString("}\n}\n")
	n, err := w.Write(b.Bytes())
	return int64(n), err
}

func writeMap[V any](b *bytes.Buffer, m map[string]V) error {
	for i, id := range slices.Sorted(maps.Keys(m)) {
		k, _ := json.Marshal(id)
		v, err := json.Marshal(m[id])
		if err != nil {
			return err
		}
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString("\n")
		b.Write(k)
		b.WriteString(": ")
		b.Write(v)
	}
	if len(m) > 0 {
		b.WriteString("\n")
	}
	return nil
}
