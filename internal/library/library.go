// Package library loads team definitions from the teams/ directory.
package library

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/genshinsim/gcsim/pkg/shortcut"
	"gopkg.in/yaml.v3"
)

type Status string

const (
	StatusSupported Status = "supported"
	StatusDraft     Status = "draft"
	StatusBlocked   Status = "blocked"
)

// Tier says how far a team's score can be trusted (design §5.4).
type Tier string

const (
	// TierVerified teams use only characters upstream gcsim has and a
	// rotation from a community gcsim config. It is the default, written
	// as no tier at all.
	TierVerified Tier = ""
	// TierExperimental teams have a character only Gscouter's gcsim fork
	// has, or a rotation written from a guide because no community config
	// exists for the team (Written). Their scores carry a badge: they may
	// change.
	TierExperimental Tier = "experimental"
)

// ForkOnly holds the characters that only Gscouter's gcsim fork has, by
// canonical key (design §5.4). A team is experimental exactly when it has
// one of them. Update it whenever a character is added to the fork or
// upstream merges one.
var ForkOnly = map[string]bool{
	"kachina":    true,
	"linnea":     true,
	"lohen":      true,
	"nefer":      true,
	"sandrone":   true,
	"vesna":      true,
	"vodyanitsa": true,
}

type Team struct {
	ID              string   `yaml:"id"`
	Name            string   `yaml:"name"`
	Members         []string `yaml:"members"` // gcsim character keys
	Archetype       string   `yaml:"archetype"`
	Status          Status   `yaml:"status"`
	Tier            Tier     `yaml:"tier,omitempty"`
	RotationVersion int      `yaml:"rotation_version"`
	GcsimCommit     string   `yaml:"gcsim_commit"`
	// GcsimFork is the commit of Gscouter's gcsim fork an experimental
	// team was written for. GcsimCommit stays the upstream base, which a
	// fix on the fork does not change.
	GcsimFork string   `yaml:"gcsim_fork,omitempty"`
	Sources   []string `yaml:"sources"`
	Notes     string   `yaml:"notes"`
	// Params are gcsim character params the rotation assumes, by member
	// key, e.g. {hutao: "start_hp=1"}. They are added to the player's
	// character line.
	Params map[string]string `yaml:"params,omitempty"`
	// Reference is what the source config scored with its own build. The
	// validator checks that the library reproduces it.
	Reference *Reference `yaml:"reference,omitempty"`
	// CrossCheck records how a rotation written from a guide (Written)
	// compares with public calculations or showcases (design §5.4).
	CrossCheck *CrossCheck `yaml:"cross_check,omitempty"`
	// Validation is what cmd/validate -promote last measured with the
	// reference build; it decides the status.
	Validation *Validation `yaml:"validation,omitempty"`

	// Rotation is the contents of rotation.gcsl.
	Rotation string `yaml:"-"`
}

type Reference struct {
	DPS          float64 `yaml:"dps"`                     // mean team DPS reported by the source
	GcsimVersion string  `yaml:"gcsim_version,omitempty"` // gcsim commit the source ran on, if known
	Computed     string  `yaml:"computed,omitempty"`      // date the source last ran, if known
}

// CrossCheck is the public number a written rotation was compared with,
// or, when none was found, what was searched.
type CrossCheck struct {
	DPS    float64 `yaml:"dps,omitempty"`    // the public team DPS, 0 if none was found
	Source string  `yaml:"source,omitempty"` // where it was published
	Note   string  `yaml:"note"`             // its conditions and the difference, or what was searched
}

// Written reports whether the team's rotation was written from a guide
// rather than taken from a community gcsim config: an experimental team
// without a source DPS (design §5.4). cmd/harvest replaces such a team
// once the community database has an entry for it.
func (t *Team) Written() bool {
	return t.Tier == TierExperimental && t.Reference == nil
}

// HasForkOnly reports whether a member is a character only Gscouter's
// gcsim fork has.
func (t *Team) HasForkOnly() bool {
	return slices.ContainsFunc(t.Members, func(m string) bool { return ForkOnly[Canonical(m)] })
}

type Validation struct {
	Date       string `yaml:"date"`
	Iterations int    `yaml:"iterations"`
	// Gap is the one pass under the source's conditions relative to the
	// source's DPS; absent without a reference.
	Gap  *float64 `yaml:"gap,omitempty"`
	Kept float64  `yaml:"kept"` // looped score over one pass under the common conditions
	DPS  int      `yaml:"dps"`  // looped score, as the site scores it
	// Draft says why the team does not meet the criteria for supported.
	Draft string `yaml:"draft,omitempty"`
}

// Load reads team.yaml and rotation.gcsl from dir on disk.
func Load(dir string) (*Team, error) {
	return LoadFS(os.DirFS(dir), ".")
}

// LoadFS reads team.yaml and rotation.gcsl from dir in fsys.
func LoadFS(fsys fs.FS, dir string) (*Team, error) {
	file := path.Join(dir, "team.yaml")
	raw, err := fs.ReadFile(fsys, file)
	if err != nil {
		return nil, err
	}
	var t Team
	if err := yaml.Unmarshal(raw, &t); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	if len(t.Members) != 4 {
		return nil, fmt.Errorf("team %q: expected 4 members, got %d", t.ID, len(t.Members))
	}
	switch t.Status {
	case StatusSupported, StatusDraft, StatusBlocked:
	default:
		return nil, fmt.Errorf("team %q: unknown status %q", t.ID, t.Status)
	}
	switch t.Tier {
	case TierVerified, TierExperimental:
	default:
		return nil, fmt.Errorf("team %q: unknown tier %q", t.ID, t.Tier)
	}
	forkOnly := t.HasForkOnly()
	switch {
	case forkOnly && t.Tier != TierExperimental:
		return nil, fmt.Errorf("team %q: has a character only the gcsim fork has, so it needs tier: experimental", t.ID)
	case t.Tier == TierExperimental && !forkOnly && !t.Written():
		return nil, fmt.Errorf("team %q: tier experimental, but every member is in upstream gcsim and the rotation has a source DPS", t.ID)
	}
	if (t.GcsimFork != "") != forkOnly {
		return nil, fmt.Errorf("team %q: gcsim_fork goes with tier experimental, and only with it", t.ID)
	}
	rotation, err := fs.ReadFile(fsys, path.Join(dir, "rotation.gcsl"))
	if err != nil && (t.Status != StatusBlocked || !errors.Is(err, fs.ErrNotExist)) {
		return nil, err
	}
	t.Rotation = string(rotation)
	return &t, nil
}

// Record rewrites the status and gcsim_commit lines of team.yaml in dir
// on disk and replaces its validation block with v, leaving the rest of
// the file as it is.
func Record(dir string, status Status, gcsimCommit string, v *Validation) error {
	file := filepath.Join(dir, "team.yaml")
	raw, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	block, err := yaml.Marshal(map[string]*Validation{"validation": v})
	if err != nil {
		return err
	}
	var out []string
	var setStatus, inBlock bool
	for _, line := range strings.SplitAfter(string(raw), "\n") {
		if inBlock && (strings.HasPrefix(line, " ") || line == "\n") {
			continue
		}
		inBlock = false
		switch {
		case line == "":
			continue
		case strings.HasPrefix(line, "status:"):
			line = "status: " + string(status) + "\n"
			setStatus = true
		case strings.HasPrefix(line, "gcsim_commit:"):
			line = "gcsim_commit: " + gcsimCommit + "\n"
		case line == "validation:\n":
			inBlock = true
			continue
		}
		if !strings.HasSuffix(line, "\n") {
			line += "\n"
		}
		out = append(out, line)
	}
	if !setStatus {
		return fmt.Errorf("%s: no status line", file)
	}
	out = append(out, string(block))
	return os.WriteFile(file, []byte(strings.Join(out, "")), 0o644)
}

// Library is a set of teams indexed by their member set.
type Library struct {
	Teams []*Team
	byKey map[string]*Team // by Key of the members
}

// LoadLibrary loads every team directory at the root of fsys.
func LoadLibrary(fsys fs.FS) (*Library, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, err
	}
	lib := &Library{byKey: map[string]*Team{}}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		t, err := LoadFS(fsys, e.Name())
		if err != nil {
			return nil, err
		}
		if t.ID != e.Name() {
			return nil, fmt.Errorf("team %q is in directory %q", t.ID, e.Name())
		}
		k := Key(t.Members)
		if other := lib.byKey[k]; other != nil {
			return nil, fmt.Errorf("teams %q and %q have the same members", other.ID, t.ID)
		}
		lib.byKey[k] = t
		lib.Teams = append(lib.Teams, t)
	}
	return lib, nil
}

// Find returns the team whose members are exactly chars, or nil.
func (l *Library) Find(chars []string) *Team {
	return l.byKey[Key(chars)]
}

// Key identifies a set of characters in any order and under any alias:
// their canonical names, sorted and joined by spaces.
func Key(chars []string) string {
	return strings.Join(canonicalSet(chars), " ")
}

// Canonical returns the name used to compare characters: gcsim's key for
// any of its aliases (e.g. "kazuha" -> "kaedeharakazuha"), with both
// Traveler genders folded into one ("aethercryo", "luminecryo" ->
// "travelercryo"). Unknown names are returned lowercased.
func Canonical(name string) string {
	name = strings.ToLower(name)
	key, ok := shortcut.CharNameToKey[name]
	if !ok {
		return name
	}
	s := key.String()
	for _, prefix := range []string{"aether", "lumine"} {
		if elem, ok := strings.CutPrefix(s, prefix); ok {
			return "traveler" + elem
		}
	}
	return s
}

// Matches reports whether chars is exactly the team's 4-character set,
// in any order and under any alias.
func (t *Team) Matches(chars []string) bool {
	return slices.Equal(canonicalSet(t.Members), canonicalSet(chars))
}

func canonicalSet(names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = Canonical(n)
	}
	slices.Sort(out)
	return out
}
