package library

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cenoda/gscouter/teams"
	"gopkg.in/yaml.v3"
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

func TestRecord(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "team.yaml")
	before := "id: a-b-c-d\nmembers: [a, b, c, d]\nstatus: draft\ngcsim_commit: 1111111\nnotes: 'status: draft stays'\n"
	if err := os.WriteFile(file, []byte(before), 0o644); err != nil {
		t.Fatal(err)
	}
	gap := -0.012
	v := &Validation{Date: "2026-10-27", Iterations: 20, Gap: &gap, Kept: 0.94, DPS: 52120}
	if err := Record(dir, StatusSupported, "2350be9", v); err != nil {
		t.Fatal(err)
	}
	want := "id: a-b-c-d\nmembers: [a, b, c, d]\nstatus: supported\ngcsim_commit: 2350be9\nnotes: 'status: draft stays'\n" +
		"validation:\n    date: \"2026-10-27\"\n    iterations: 20\n    gap: -0.012\n    kept: 0.94\n    dps: 52120\n"
	got, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
	// A second record replaces the block.
	v = &Validation{Date: "2026-11-01", Iterations: 20, Kept: 0.5, DPS: 100, Draft: "keeps too little"}
	if err := Record(dir, StatusDraft, "2350be9", v); err != nil {
		t.Fatal(err)
	}
	want = "id: a-b-c-d\nmembers: [a, b, c, d]\nstatus: draft\ngcsim_commit: 2350be9\nnotes: 'status: draft stays'\n" +
		"validation:\n    date: \"2026-11-01\"\n    iterations: 20\n    kept: 0.5\n    dps: 100\n    draft: keeps too little\n"
	if got, _ = os.ReadFile(file); string(got) != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
	var back Team
	if err := yaml.Unmarshal(got, &back); err != nil {
		t.Fatal(err)
	}
	if back.Validation == nil || back.Validation.DPS != 100 || back.Validation.Draft != "keeps too little" {
		t.Errorf("read back %+v", back.Validation)
	}
}
