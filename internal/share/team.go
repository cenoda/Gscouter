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
const GcsimFork = "b029c45"

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
// source's build as build.txt and its conditions as source.txt. It refuses to replace an existing team
// unless force is set.
func (sp Split) Write(root string, t *library.Team, force bool) (string, error) {
	dir := filepath.Join(root, t.ID)
	if _, err := os.Stat(dir); err == nil && !force {
		return "", fmt.Errorf("%s already exists; use -force to replace it", dir)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	meta, err := yaml.Marshal(t)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	files := map[string]string{
		"team.yaml":     string(meta),
		"rotation.gcsl": sp.Rotation,
		"build.txt":     "# Build from the source config, used to validate the rotation.\n\n" + sp.Build,
		"source.txt":    sp.SourceConditions(),
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			return "", err
		}
	}
	return dir, nil
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
