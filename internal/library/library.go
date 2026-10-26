// Package library loads team definitions from the teams/ directory.
package library

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
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

type Team struct {
	ID              string   `yaml:"id"`
	Name            string   `yaml:"name"`
	Members         []string `yaml:"members"` // gcsim character keys
	Archetype       string   `yaml:"archetype"`
	Status          Status   `yaml:"status"`
	RotationVersion int      `yaml:"rotation_version"`
	GcsimCommit     string   `yaml:"gcsim_commit"`
	Sources         []string `yaml:"sources"`
	Notes           string   `yaml:"notes"`

	// Rotation is the contents of rotation.gcsl.
	Rotation string `yaml:"-"`
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
	rotation, err := fs.ReadFile(fsys, path.Join(dir, "rotation.gcsl"))
	if err != nil && (t.Status != StatusBlocked || !errors.Is(err, fs.ErrNotExist)) {
		return nil, err
	}
	t.Rotation = string(rotation)
	return &t, nil
}

// Library is a set of teams indexed by their member set.
type Library struct {
	Teams []*Team
}

// LoadLibrary loads every team directory at the root of fsys.
func LoadLibrary(fsys fs.FS) (*Library, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, err
	}
	lib := &Library{}
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
		lib.Teams = append(lib.Teams, t)
	}
	return lib, nil
}

// Find returns the team whose members are exactly chars, or nil.
func (l *Library) Find(chars []string) *Team {
	for _, t := range l.Teams {
		if t.Matches(chars) {
			return t
		}
	}
	return nil
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
