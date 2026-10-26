package library

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAllTeams(t *testing.T) {
	dirs, err := filepath.Glob("../../teams/*")
	if err != nil {
		t.Fatal(err)
	}
	if len(dirs) == 0 {
		t.Fatal("no teams found")
	}
	for _, dir := range dirs {
		team, err := Load(dir)
		if err != nil {
			t.Errorf("%s: %v", dir, err)
			continue
		}
		if team.ID != filepath.Base(dir) {
			t.Errorf("%s: id %q does not match directory name", dir, team.ID)
		}
		if team.Status != StatusBlocked && team.Rotation == "" {
			t.Errorf("%s: %s team has no rotation", dir, team.Status)
		}
	}
}

func TestMatchesIgnoresOrder(t *testing.T) {
	team := &Team{Members: []string{"skirk", "yelan", "furina", "escoffier"}}
	if !team.Matches([]string{"furina", "escoffier", "skirk", "yelan"}) {
		t.Error("same set in a different order should match")
	}
	if team.Matches([]string{"furina", "escoffier", "skirk", "xingqiu"}) {
		t.Error("a different member should not match")
	}
}

func TestLoadRejectsUnknownStatus(t *testing.T) {
	dir := t.TempDir()
	yaml := "id: x\nmembers: [a, b, c, d]\nstatus: done\n"
	if err := os.WriteFile(filepath.Join(dir, "team.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err == nil {
		t.Error("expected an error for an unknown status")
	}
}
