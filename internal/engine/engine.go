// Package engine decides which teams Gscouter scores on gscouter-engine,
// a Rust port of gcsim (https://github.com/cenoda/gscouter-engine), and
// runs its WebAssembly module on the server (wazero) as the browser's
// helper workers do (docs/engine.md).
//
// A library team runs on the engine when it is both verified in the
// engine release's manifest (release/manifest.json) and on the owner's
// approved list (approved.txt). Everything else runs on gcsim, as does a
// config the engine refuses.
package engine

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/cenoda/gscouter/internal/score"
)

// Manifest is the part of an engine release's manifest.json that
// Gscouter reads (gscouter-engine's reference/manifest).
type Manifest struct {
	Engine struct {
		Commit     string `json:"commit"`
		WasmSHA256 string `json:"wasm_sha256"`
	} `json:"engine"`
	Gscouter string `json:"gscouter_commit"`
	Teams    map[string]struct {
		Verified bool `json:"verified"`
	} `json:"teams"`
}

// Approval is a line of the approved list.
type Approval struct {
	Team string
	Date string // of the owner's answer, YYYY-MM-DD
}

// ParseApproved reads the approved list: one team per line, its id and
// the date of the owner's answer, separated by spaces. Blank lines and
// lines starting with # are skipped.
func ParseApproved(text string) ([]Approval, error) {
	var out []Approval
	seen := map[string]bool{}
	sc := bufio.NewScanner(strings.NewReader(text))
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Fields(line)
		if len(f) != 2 {
			return nil, fmt.Errorf("approved list, line %d: want a team id and a date, got %q", n, line)
		}
		if _, err := time.Parse(time.DateOnly, f[1]); err != nil {
			return nil, fmt.Errorf("approved list, line %d: date %q is not YYYY-MM-DD", n, f[1])
		}
		if seen[f[0]] {
			return nil, fmt.Errorf("approved list, line %d: %s is listed twice", n, f[0])
		}
		seen[f[0]] = true
		out = append(out, Approval{Team: f[0], Date: f[1]})
	}
	return out, sc.Err()
}

// Engine is a release of gscouter-engine and the teams that run on it.
// A nil *Engine runs nothing: every team is on gcsim.
type Engine struct {
	hash   string // the module's SHA-256, hex
	commit string
	module []byte
	teams  map[string]bool // approved and verified
	runner score.Engine
}

// New checks module against manifest and returns the engine with the
// teams that are both verified in manifest and approved. skipped lists
// the approved teams it leaves on gcsim, and why.
func New(module, manifest []byte, approved []Approval) (e *Engine, skipped []string, err error) {
	var m Manifest
	if err := json.Unmarshal(manifest, &m); err != nil {
		return nil, nil, fmt.Errorf("manifest: %w", err)
	}
	sum := sha256.Sum256(module)
	hash := hex.EncodeToString(sum[:])
	if m.Engine.WasmSHA256 != hash {
		return nil, nil, fmt.Errorf("the module's SHA-256 is %s, the manifest's %q: they are not of one release", hash, m.Engine.WasmSHA256)
	}
	e = &Engine{hash: hash, commit: m.Engine.Commit, module: module, teams: map[string]bool{}}
	for _, a := range approved {
		switch t, ok := m.Teams[a.Team]; {
		case !ok:
			skipped = append(skipped, a.Team+": not in the manifest")
		case !t.Verified:
			skipped = append(skipped, a.Team+": not verified")
		default:
			e.teams[a.Team] = true
		}
	}
	e.runner = newRunner(e)
	return e, skipped, nil
}

// Use reports whether team, a library team's id, runs on the engine.
func (e *Engine) Use(team string) bool {
	return e != nil && e.teams[team]
}

// Teams lists the teams that run on the engine, sorted.
func (e *Engine) Teams() []string {
	if e == nil {
		return nil
	}
	out := make([]string, 0, len(e.teams))
	for t := range e.teams {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

// Hash is the module's SHA-256 in hex, which names the engine in cache
// keys and in the module's URL.
func (e *Engine) Hash() string { return e.hash }

// Commit is the engine's commit the release was built at.
func (e *Engine) Commit() string { return e.commit }

// Module is the WebAssembly module, for the browser's helper workers.
func (e *Engine) Module() []byte { return e.module }

// Runner runs configs on the module, for score.Options.Engine; nil
// where the module cannot run (the browser build of Gscouter, whose
// helper workers run it from JavaScript).
func (e *Engine) Runner() score.Engine {
	if e == nil {
		return nil
	}
	return e.runner
}
