package share

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cenoda/gscouter/internal/library"
	"gopkg.in/yaml.v3"
)

// GcsimCommit is the upstream gcsim commit that the fork in go.mod is
// based on, recorded on each team.
const GcsimCommit = "2350be9"

// GcsimFork is the commit of the fork in go.mod, recorded on experimental
// teams: a fix on the fork changes their scores but not GcsimCommit.
const GcsimFork = "eb21884"

// Team builds the library entry for a split config. Its ID joins the
// members' canonical names in config order.
func (sp Split) Team(sources []string, ref *library.Reference, notes string) (*library.Team, error) {
	if len(sp.Members) != 4 {
		return nil, fmt.Errorf("the config has %d characters, not 4: %v", len(sp.Members), sp.Members)
	}
	var ids, names []string
	for _, m := range sp.Members {
		key := library.Canonical(m)
		ids = append(ids, key)
		names = append(names, displayName(key))
	}
	t := &library.Team{
		ID:              strings.Join(ids, "-"),
		Name:            strings.Join(names, " / "),
		Members:         sp.Members,
		Status:          library.StatusDraft,
		RotationVersion: 1,
		GcsimCommit:     GcsimCommit,
		Sources:         sources,
		Notes:           notes,
		Reference:       ref,
	}
	if len(sp.Params) > 0 {
		t.Params = sp.Params
	}
	if slices.ContainsFunc(ids, func(k string) bool { return library.ForkOnly[k] }) || library.UsesForkOnlyWeapon(sp.Build) {
		t.Tier = library.TierExperimental
		t.GcsimFork = GcsimFork
	}
	return t, nil
}

// Write stores the team in root/<id>/: team.yaml, rotation.gcsl, the
// source's build as build.txt and its conditions as source.txt. A team
// already in the library with the same members, in any order and under
// any alias (library.Key), is refused unless force is set; with force,
// the new team takes that team's ID, name and hand-written archetype, if
// any, and is written over its directory, so its links and its
// alternatives' parent ID stay as they were. A second directory for the same members would stop the
// library from loading.
func (sp Split) Write(root string, t *library.Team, force bool) (string, error) {
	dir := filepath.Join(root, t.ID)
	was, wasDir, err := findTeam(root, t.Members)
	if err != nil {
		return "", err
	}
	if was == nil {
		if _, err := os.Stat(dir); err == nil && !force {
			return "", fmt.Errorf("%s already exists; use -force to replace it", dir)
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		return dir, sp.WriteDir(dir, t)
	}
	if !force {
		return "", fmt.Errorf("%s has the same members; use -force to replace it", wasDir)
	}
	t.ID, t.Name = was.ID, was.Name
	if t.Archetype == "" {
		t.Archetype = was.Archetype
	}
	return wasDir, sp.WriteDir(wasDir, t)
}

// findTeam returns the team in root whose members are members in any
// order, and its directory, or nil. A name starting with a dot is a
// staging directory (cmd/harvest) and is skipped.
func findTeam(root string, members []string) (*library.Team, string, error) {
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", err
	}
	key := library.Key(members)
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		dir := filepath.Join(root, e.Name())
		if _, err := os.Stat(filepath.Join(dir, "team.yaml")); err != nil {
			continue
		}
		t, err := library.Load(dir)
		if err != nil {
			return nil, "", err
		}
		if library.Key(t.Members) == key {
			return t, dir, nil
		}
	}
	return nil, "", nil
}

// WriteDir stores the team's files in dir, as Write does, replacing
// those there. A team's alternative (library.AltDir) is written this way.
func (sp Split) WriteDir(dir string, t *library.Team) error {
	meta, err := yaml.Marshal(t)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	files := map[string]string{
		"team.yaml":     string(meta),
		"rotation.gcsl": sp.Rotation,
		"build.txt":     "# Build from the source config, used to validate the rotation.\n\n" + sp.Build,
		"source.txt":    sp.SourceConditions(),
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// displayName turns a gcsim key into a readable name ("hutao" -> "Hutao").
func displayName(key string) string {
	if key == "" {
		return key
	}
	return strings.ToUpper(key[:1]) + key[1:]
}

// SourceConditions renders the source's own options, target, energy and
// hurt lines for source.txt. Validation runs one pass of the rotation
// under them to compare with the source's DPS; the site always uses the
// common conditions.
func (sp Split) SourceConditions() string {
	return "# Conditions of the source config, used to validate the rotation.\n\n" + strings.Join(sp.Dropped, "\n") + "\n"
}
