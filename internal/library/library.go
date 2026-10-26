// Package library loads team definitions from the teams/ directory.
package library

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

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

// Load reads team.yaml and rotation.gcsl from dir.
func Load(dir string) (*Team, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "team.yaml"))
	if err != nil {
		return nil, err
	}
	var t Team
	if err := yaml.Unmarshal(raw, &t); err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Join(dir, "team.yaml"), err)
	}
	if len(t.Members) != 4 {
		return nil, fmt.Errorf("team %q: expected 4 members, got %d", t.ID, len(t.Members))
	}
	switch t.Status {
	case StatusSupported, StatusDraft, StatusBlocked:
	default:
		return nil, fmt.Errorf("team %q: unknown status %q", t.ID, t.Status)
	}
	rotation, err := os.ReadFile(filepath.Join(dir, "rotation.gcsl"))
	if err != nil && t.Status != StatusBlocked {
		return nil, err
	}
	t.Rotation = string(rotation)
	return &t, nil
}

// Matches reports whether chars is exactly the team's 4-character set,
// in any order.
func (t *Team) Matches(chars []string) bool {
	a := slices.Clone(t.Members)
	b := slices.Clone(chars)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}
