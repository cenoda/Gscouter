package library

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cenoda/gscouter/teams"
)

func TestLoadAllTeams(t *testing.T) {
	lib, err := LoadLibrary(teams.FS)
	if err != nil {
		t.Fatal(err)
	}
	if len(lib.Teams) == 0 {
		t.Fatal("no teams found")
	}
	for _, team := range lib.Teams {
		if team.Status != StatusBlocked && team.Rotation == "" {
			t.Errorf("%s: %s team has no rotation", team.ID, team.Status)
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

func TestMatchesAliasesAndTraveler(t *testing.T) {
	team := &Team{Members: []string{"travelercryo", "kazuha", "furina", "raiden"}}
	if !team.Matches([]string{"aethercryo", "kaedeharakazuha", "furina", "raidenshogun"}) {
		t.Error("aliases and either Traveler gender should match")
	}
	if team.Matches([]string{"aetheranemo", "kaedeharakazuha", "furina", "raidenshogun"}) {
		t.Error("a Traveler of another element should not match")
	}
}

func TestFind(t *testing.T) {
	lib, err := LoadLibrary(teams.FS)
	if err != nil {
		t.Fatal(err)
	}
	if lib.Find([]string{"zhongli", "xingqiu", "yelan", "hutao"}) == nil {
		t.Error("Hu Tao team not found")
	}
	if lib.Find([]string{"amber", "xingqiu", "yelan", "hutao"}) != nil {
		t.Error("unexpected match")
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

func TestLoadTier(t *testing.T) {
	for _, c := range []struct {
		members, tier string
		ok            bool
	}{
		{"a, b, c, d", "", true},
		{"nefer, b, c, d", "experimental", true},
		{"Linnea, b, c, d", "experimental", true}, // any alias
		{"nefer, b, c, d", "", false},             // fork-only member, no tier
		{"a, b, c, d", "experimental", false},     // experimental without one
		{"nefer, b, c, d", "beta", false},
	} {
		dir := t.TempDir()
		yaml := "id: x\nmembers: [" + c.members + "]\nstatus: blocked\n"
		if c.tier != "" {
			yaml += "tier: " + c.tier + "\n"
		}
		if err := os.WriteFile(filepath.Join(dir, "team.yaml"), []byte(yaml), 0o644); err != nil {
			t.Fatal(err)
		}
		team, err := Load(dir)
		if !c.ok {
			if err == nil {
				t.Errorf("[%s] tier %q: expected an error", c.members, c.tier)
			}
			continue
		}
		if err != nil {
			t.Fatalf("[%s] tier %q: %v", c.members, c.tier, err)
		}
		if team.Tier != Tier(c.tier) {
			t.Errorf("[%s] tier %q: loaded %q", c.members, c.tier, team.Tier)
		}
	}
}
