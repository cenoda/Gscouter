// Package gse calls gscouter-engine's WebAssembly module from Go
// (wazero), and holds its parse API (gse_parse, gscouter-engine
// docs/parse-api.md), which reads setup lines for Gscouter's input path
// in place of gcsim's parser. It has no part in scoring (internal/engine)
// and does not depend on gcsim.
package gse

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ParseVersion is the parse_version of gse_parse's JSON this package
// reads (gscouter-engine docs/parse-api.md). A module that answers with
// another is refused.
const ParseVersion = 1

// Parsed is what a config's setup lines say, from the engine's parse API
// (gse_parse, parse_version 1). Keys are the engine's key catalog's,
// aliases resolved; characters are in the order of their first line.
type Parsed struct {
	Version    int          `json:"parse_version"`
	OK         bool         `json:"ok"`
	Characters []ParsedChar `json:"characters"`
	// Errors are the checks of a whole config that gcsim's parser
	// reports without failing (no_active_character, no_targets, ...):
	// a build.txt alone has two.
	Errors []ParseIssue `json:"errors"`
	Error  *ParseError  `json:"error"`
}

// ParsedChar is a character's setup lines.
type ParsedChar struct {
	Key       string             `json:"key"`
	Level     int                `json:"level"`
	MaxLevel  int                `json:"max_level"`
	Ascension *int               `json:"ascension"`
	Cons      int                `json:"cons"`
	Talents   [3]int             `json:"talents"` // attack, skill, burst, as written
	Params    map[string]int     `json:"params"`
	Weapon    *ParsedWeapon      `json:"weapon"` // nil without an add weapon line
	Sets      []ParsedSet        `json:"sets"`   // sorted by key
	Stats     map[string]float64 `json:"stats"`  // by gcsim's stat names, zeros left out
}

// ParsedWeapon is a character's weapon line.
type ParsedWeapon struct {
	Key      string         `json:"key"`
	Level    int            `json:"level"`
	MaxLevel int            `json:"max_level"`
	Refine   int            `json:"refine"`
	Params   map[string]int `json:"params"`
}

// ParsedSet is one of a character's set lines.
type ParsedSet struct {
	Key    string         `json:"key"`
	Count  int            `json:"count"`
	Params map[string]int `json:"params"`
}

// ParseIssue is a non-fatal check of a whole config.
type ParseIssue struct {
	Class   string `json:"class"`
	Message string `json:"message"`
}

func (p ParseIssue) Error() string { return p.Message }

// ParseError is why a config does not parse: its class (unknown_character,
// unknown_weapon, unknown_set, unsupported, syntax), the 1-based line
// when known, and a message.
type ParseError struct {
	Class   string `json:"class"`
	Line    *int   `json:"line"`
	Message string `json:"message"`
}

func (e *ParseError) Error() string {
	where := ""
	if e.Line != nil {
		where = fmt.Sprintf(" (line %d)", *e.Line)
	}
	if e.Class == "unsupported" {
		// add stats random: gcsim reads it, the engine's parser does not
		return fmt.Sprintf("not supported by Gscouter's parser%s: %s", where, e.Message)
	}
	return fmt.Sprintf("%s%s: %s", strings.ReplaceAll(e.Class, "_", " "), where, e.Message)
}

// ErrNoParse is the error of Parse when the pinned module predates the
// parse API.
var ErrNoParse = errors.New("engine: the pinned module has no gse_parse (it predates the parse API); pin a release with it")

// decodeParse reads gse_parse's answer: status 0 with the result, 1 with
// the error as JSON (or a message for a config that is not UTF-8).
func decodeParse(status uint32, body []byte) (*Parsed, error) {
	var p Parsed
	if err := json.Unmarshal(body, &p); err != nil {
		if status != 0 {
			return nil, errors.New("engine: " + string(body))
		}
		return nil, fmt.Errorf("engine: gse_parse: %w", err)
	}
	if p.Version != ParseVersion {
		return nil, fmt.Errorf("engine: gse_parse answers parse_version %d, want %d", p.Version, ParseVersion)
	}
	if status != 0 || !p.OK {
		if p.Error == nil {
			return nil, fmt.Errorf("engine: gse_parse failed without an error (status %d)", status)
		}
		return &p, p.Error
	}
	return &p, nil
}
