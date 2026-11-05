// Package engine decides which teams Gscouter scores on gscouter-engine,
// a Rust port of gcsim (https://github.com/cenoda/gscouter-engine), and
// runs its WebAssembly module on the server (wazero) as the browser's
// helper workers do (docs/engine.md).
//
// A library team runs on the engine when it is verified in the engine
// release's manifest (release/manifest.json), its library files are those
// the release verified (release/teams.txt), and it is on the owner's
// approved list (approved.txt). Everything else runs on gcsim, as does a
// config the engine refuses.
package engine

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
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
	Gscouter string                  `json:"gscouter_commit"`
	Teams    map[string]ManifestTeam `json:"teams"`
}

// ManifestTeam is a team's entry in the manifest. Group is a verified
// team's group for the owner's answer: "A", approved at once and
// carried forward, or "B", one by one (gscouter-engine's
// docs/phase4-approval.md); GroupWhy says why a team is in B.
type ManifestTeam struct {
	Verified bool `json:"verified"`
	// VerifiedKinds are the kinds of build the team passed: library (and
	// the probes), the whole verification (docs/engine.md §2), and any
	// other kind its release judged.
	VerifiedKinds []string `json:"verified_kinds"`
	Group         string   `json:"group"`
	GroupWhy      []string `json:"group_why"`
	// Scores are the verdicts' runs, one per build judged.
	Scores []ManifestScore `json:"scores"`
}

// ManifestScore is the part of a verdict's run that cmd/pinengine reads:
// the build ("library") and the iterations it was decided at.
type ManifestScore struct {
	Build      string `json:"build"`
	Iterations int    `json:"iterations"`
}

// Approval is a line of the approved list.
type Approval struct {
	Team string
	Date string // of the owner's answer, YYYY-MM-DD
	// Release is set for a team approved from group B: the first 16 hex
	// digits of the module's SHA-256 the answer was for, the only
	// release the approval holds for. Without it the approval was of a
	// group A team and holds while the team stays verified in group A.
	Release string
	// Carried lists, oldest first, the releases a group B approval was
	// carried forward from at pin time because the new module scored the
	// team identically (cmd/pinengine, docs/engine.md §2). Date stays the
	// owner's answer.
	Carried []Carry
}

// Carry is one carry of a group B approval to a new module: from the
// module it held for, how (only "identical": team DPS and every
// character's DPS identical to the last digit on both modules), and when.
type Carry struct {
	From string // first 16 hex digits of the old module's SHA-256
	How  string
	Date string
}

// String is the carry as the approved list writes it.
func (c Carry) String() string { return "carried=" + c.From + ":" + c.How + ":" + c.Date }

// String is the approval as a line of the approved list.
func (a Approval) String() string {
	s := a.Team + " " + a.Date
	if a.Release != "" {
		s += " release=" + a.Release
	}
	for _, c := range a.Carried {
		s += " " + c.String()
	}
	return s
}

func isModulePrefix(s string) bool {
	_, err := hex.DecodeString(s)
	return len(s) == 16 && err == nil
}

func parseCarry(field string) (Carry, error) {
	v, ok := strings.CutPrefix(field, "carried=")
	p := strings.Split(v, ":")
	if !ok || len(p) != 3 || !isModulePrefix(p[0]) || p[1] != "identical" {
		return Carry{}, fmt.Errorf("want carried=<16 hex digits of the old module's SHA-256>:identical:<date>, got %q", field)
	}
	if _, err := time.Parse(time.DateOnly, p[2]); err != nil {
		return Carry{}, fmt.Errorf("carry date %q is not YYYY-MM-DD", p[2])
	}
	return Carry{From: p[0], How: p[1], Date: p[2]}, nil
}

// ParseApproved reads the approved list: one team per line, its id, the
// date of the owner's answer and, for a team approved from group B,
// release=<first 16 hex digits of the module's SHA-256>, then any
// carried=<old module>:identical:<date> fields of the carries that brought
// it to that release, separated by spaces. Blank lines and lines starting
// with # are skipped.
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
		if len(f) < 2 {
			return nil, fmt.Errorf("approved list, line %d: want a team id, a date and for group B release=<module>, got %q", n, line)
		}
		a := Approval{Team: f[0], Date: f[1]}
		if len(f) >= 3 {
			r, ok := strings.CutPrefix(f[2], "release=")
			if !ok || !isModulePrefix(r) {
				return nil, fmt.Errorf("approved list, line %d: want release=<16 hex digits of the module's SHA-256>, got %q", n, f[2])
			}
			a.Release = r
		}
		for _, field := range f[min(3, len(f)):] {
			c, err := parseCarry(field)
			if err != nil {
				return nil, fmt.Errorf("approved list, line %d: %v", n, err)
			}
			a.Carried = append(a.Carried, c)
		}
		if _, err := time.Parse(time.DateOnly, f[1]); err != nil {
			return nil, fmt.Errorf("approved list, line %d: date %q is not YYYY-MM-DD", n, f[1])
		}
		if seen[f[0]] {
			return nil, fmt.Errorf("approved list, line %d: %s is listed twice", n, f[0])
		}
		seen[f[0]] = true
		out = append(out, a)
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
// approved teams that manifest verifies and whose files in library (laid
// out as teams/) hash as pinned says they did at the manifest's Gscouter
// commit (TeamHash). skipped lists the approved teams it leaves on gcsim,
// and why. Only the approved teams are hashed.
func New(module, manifest []byte, pinned map[string]string, library fs.FS, approved []Approval) (e *Engine, skipped []string, err error) {
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
		if why := Dropped(m, a); why != "" {
			skipped = append(skipped, a.Team+": "+why)
			continue
		}
		switch {
		case pinned[a.Team] == "":
			skipped = append(skipped, a.Team+": no pinned library files (release/teams.txt)")
		default:
			switch h, err := TeamHash(library, a.Team); {
			case err != nil:
				skipped = append(skipped, a.Team+": "+err.Error())
			case h != pinned[a.Team]:
				skipped = append(skipped, fmt.Sprintf("%s: teams/%s changed since Gscouter %s, which the release verified", a.Team, a.Team, m.Gscouter))
			default:
				e.teams[a.Team] = true
			}
		}
	}
	e.runner = newRunner(e)
	return e, skipped, nil
}

// Dropped says why the approval a does not hold for the release of
// manifest m, or "" when it holds. An approval holds while its team is
// verified and in group A; one given from group B (Release set) holds
// for its release only: cmd/pinengine carries it to a new release, at pin
// time, only when the new module scores the team identically, and writes
// the new release on its line. A dropped approval needs the owner's yes again
// (cmd/pinengine takes it off the list), whatever a later release says.
// A team whose library files changed since the release is not dropped:
// it waits on gcsim for a release that verifies its files (New).
func Dropped(m Manifest, a Approval) string {
	t, ok := m.Teams[a.Team]
	switch {
	case !ok:
		return "not in the manifest"
	case !t.Verified:
		return "not verified"
	case a.Release != "":
		if !strings.HasPrefix(m.Engine.WasmSHA256, a.Release) {
			return "approved from group B for module " + a.Release + " only"
		}
	case t.Group != "A":
		why := "not in group A"
		if t.Group != "" {
			why = "in group " + t.Group
		}
		if len(t.GroupWhy) > 0 {
			why += " (" + strings.Join(t.GroupWhy, "; ") + ")"
		}
		return why
	}
	return ""
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
