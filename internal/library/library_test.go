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
		// A rotation written from a guide must name the guide and say how
		// it compares with public numbers (design §5.4).
		if team.Written() && (len(team.Sources) == 0 || team.CrossCheck == nil || team.CrossCheck.Note == "") {
			t.Errorf("%s: written rotation without sources or a cross_check note", team.ID)
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
		{"nefer, b, c, d", "experimental-nofork", false},
		{"a, b, c, d", "fork", false},         // gcsim_fork on a verified team
		{"nefer, b, c, d", "", false},         // fork-only member, no tier
		{"a, b, c, d", "experimental", false}, // experimental without one
		{"nefer, b, c, d", "beta", false},
	} {
		dir := t.TempDir()
		yaml := "id: x\nmembers: [" + c.members + "]\nstatus: blocked\n"
		switch c.tier {
		case "":
		case "experimental":
			yaml += "tier: experimental\ngcsim_fork: abc1234\n"
		case "experimental-nofork":
			yaml += "tier: experimental\n"
		case "fork":
			yaml += "gcsim_fork: abc1234\n"
		default:
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

func TestWritten(t *testing.T) {
	ref := &Reference{DPS: 1}
	for _, c := range []struct {
		team Team
		want bool
	}{
		{Team{}, false},               // verified, even without a reference
		{Team{Reference: ref}, false}, // verified from the database
		{Team{Tier: TierExperimental}, true},
		{Team{Tier: TierExperimental, Reference: ref}, false}, // from a database entry or a share
	} {
		if got := c.team.Written(); got != c.want {
			t.Errorf("tier %q, reference %v: Written() = %v", c.team.Tier, c.team.Reference != nil, got)
		}
	}
}
