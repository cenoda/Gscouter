package library

import (
	"os"
	"path/filepath"
	"strings"
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
		// A rotation written from a guide must name the guide and say how
		// it compares with public numbers (design §5.4). A blocked team has
		// no rotation yet.
		if team.Status != StatusBlocked && team.Written() && (len(team.Sources) == 0 || team.CrossCheck == nil || team.CrossCheck.Note == "") {
			t.Errorf("%s: written rotation without sources or a cross_check note", team.ID)
		}
		// An alternative is another community rotation (design §5.2).
		for _, alt := range team.Alternatives {
			if len(alt.Sources) == 0 || strings.Join(strings.Fields(alt.Rotation), " ") == strings.Join(strings.Fields(team.Rotation), " ") {
				t.Errorf("%s: alternative %v without a source or with the team's own rotation", team.ID, alt.Sources)
			}
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

// Every Traveler is one roster identity, while Canonical keeps the
// element apart.
func TestIdentity(t *testing.T) {
	for _, name := range []string{"aetheranemo", "aethergeo", "lumineanemo", "luminecryo", "travelercryo", "TravelerDendro"} {
		if got := Identity(name); got != "traveler" {
			t.Errorf("Identity(%q) = %q, want traveler", name, got)
		}
	}
	for name, want := range map[string]string{"kazuha": "kaedeharakazuha", "hutao": "hutao", "Xingqiu": "xingqiu"} {
		if got := Identity(name); got != want {
			t.Errorf("Identity(%q) = %q, want %q", name, got, want)
		}
	}
	if Canonical("aetheranemo") == Canonical("aethergeo") {
		t.Error("Canonical folded two Traveler elements")
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

func TestLoadTier(t *testing.T) {
	for _, c := range []struct {
		members, tier string
		ok            bool
	}{
		{"a, b, c, d", "", true},
		{"nefer, b, c, d", "experimental", true},
		{"Linnea, b, c, d", "experimental", true}, // any alias
		{"nefer, b, c, d", "experimental-nofork", false},
		{"a, b, c, d", "fork", false},               // gcsim_fork on a verified team
		{"nefer, b, c, d", "", false},               // fork-only member, no tier
		{"a, b, c, d", "experimental", false},       // experimental without one
		{"a, b, c, d", "experimental-nofork", true}, // a rotation written from a guide
		{"a, b, c, d", "written-ref", false},        // experimental with a source DPS
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
		case "written-ref":
			yaml += "tier: experimental\nreference: {dps: 1000}\n"
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
		if want := Tier(strings.TrimSuffix(c.tier, "-nofork")); team.Tier != want {
			t.Errorf("[%s] tier %q: loaded %q", c.members, c.tier, team.Tier)
		}
	}
}

// TestLibraryBuilds loads the library from disk, with each team's
// build.txt, which the embedded copy leaves out: a team whose build uses a
// weapon only the gcsim fork has must be experimental with gcsim_fork.
func TestLibraryBuilds(t *testing.T) {
	lib, err := LoadLibrary(os.DirFS("../../teams"))
	if err != nil {
		t.Fatal(err)
	}
	builds := 0
	for _, team := range lib.Teams {
		if team.Build != "" {
			builds++
		}
	}
	if builds < len(lib.Teams)/2 {
		t.Errorf("only %d of %d teams have a build.txt", builds, len(lib.Teams))
	}
}

func TestLoadForkOnlyWeapon(t *testing.T) {
	for _, c := range []struct {
		build, tier string
		ok          bool
	}{
		{`a add weapon="hymnofthemaelstrom" refine=1;`, "experimental", true},
		{`a add weapon="hymn" refine=5;`, "experimental", true}, // a gcsim shortcut
		{`a add weapon="hymnofthemaelstrom" refine=1;`, "", false},
		{`a add weapon="hymnofthemaelstrom" refine=1;`, "experimental-nofork", false},
		{`a add weapon="staffofhoma" refine=1;`, "", true},
		{`a add weapon="staffofhoma" refine=1;`, "experimental", false},
	} {
		dir := t.TempDir()
		yaml := "id: x\nmembers: [a, b, c, d]\nstatus: draft\nreference: {dps: 1000}\n"
		switch c.tier {
		case "experimental":
			yaml += "tier: experimental\ngcsim_fork: abc1234\n"
		case "experimental-nofork":
			yaml += "tier: experimental\n"
		}
		for name, content := range map[string]string{"team.yaml": yaml, "rotation.gcsl": "a attack;", "build.txt": c.build} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		team, err := Load(dir)
		if (err == nil) != c.ok {
			t.Errorf("%s, tier %q: error %v, want ok %v", c.build, c.tier, err, c.ok)
			continue
		}
		if err == nil && team.HasForkOnly() != (c.tier == "experimental") {
			t.Errorf("%s: HasForkOnly %v", c.build, team.HasForkOnly())
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

func TestLoadAlternatives(t *testing.T) {
	write := func(dir, yaml, build string) {
		t.Helper()
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for name, content := range map[string]string{"team.yaml": yaml, "rotation.gcsl": "active a;\n", "build.txt": build} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	team := "id: x\nmembers: [furina, xilonen, c, d]\nstatus: supported\n"
	alt := "id: x\nmembers: [xilonen, Furina, d, c]\nreference: {dps: 1000}\nstatus: "
	root := t.TempDir()
	write(root, team, "furina char lvl=90/90 cons=0 talent=9,9,9;\nxilonen char lvl=90/90 cons=2 talent=9,9,9;\n")
	write(filepath.Join(root, AltDir, "e1"), alt+"supported\n", "furina char lvl=90/90 cons=6 talent=9,9,9;\nxilonen char lvl=90/90 cons=2 talent=9,9,9;\n")
	write(filepath.Join(root, AltDir, "e2"), alt+"draft\n", "")
	got, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Alternatives) != 2 {
		t.Fatalf("loaded %d alternatives, want 2", len(got.Alternatives))
	}
	// A draft alternative is not tried.
	if rs := got.Rotations(); len(rs) != 2 || rs[0] != got || rs[1] != got.Alternatives[0] {
		t.Errorf("Rotations() = %v, want the team and its supported alternative", rs)
	}
	if r := got.Alternatives[0].Raised(got); len(r) != 1 || r[0] != (Cons{"furina", 6}) {
		t.Errorf("Raised = %v, want [{furina 6}]", r)
	}
	if n := DirName(filepath.Join(root, AltDir, "e1")); n != filepath.Base(root)+"/alt/e1" {
		t.Errorf("DirName = %q", n)
	}

	// An alternative must be the same team, with a source DPS.
	for _, bad := range []string{
		"id: y\nmembers: [furina, xilonen, c, d]\nreference: {dps: 1000}\nstatus: supported\n",
		"id: x\nmembers: [furina, xilonen, c, e]\nreference: {dps: 1000}\nstatus: supported\n",
		"id: x\nmembers: [furina, xilonen, c, d]\nstatus: supported\n",
	} {
		write(filepath.Join(root, AltDir, "e2"), bad, "")
		if _, err := Load(root); err == nil {
			t.Errorf("expected an error for alternative %q", bad)
		}
	}
}

func TestRotationsForVariant(t *testing.T) {
	write := func(dir, yaml, build string) {
		t.Helper()
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for name, content := range map[string]string{"team.yaml": yaml, "rotation.gcsl": "active a;\n", "build.txt": build} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	root := t.TempDir()
	write(root, "id: x\nmembers: [mavuika, citlali, c, d]\nstatus: supported\n", "citlali char lvl=90/90 cons=0 talent=9,9,9;\n")
	write(filepath.Join(root, AltDir, "e1"), "id: x\nmembers: [mavuika, citlali, c, d]\nreference: {dps: 1000}\nstatus: supported\n", "mavuika char lvl=90/90 cons=2 talent=9,9,9;\n")
	write(filepath.Join(root, AltDir, "v"), "id: x\nmembers: [mavuika, citlali, c, d]\nvariant: {drop: burst, char: citlali}\nstatus: supported\n", "citlali char lvl=90/90 cons=6 talent=9,9,9;\n")
	got, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	// A variant, unlike a community alternative, is tried only for the
	// constellations it was made for.
	if n := len(got.RotationsFor(map[string]int{"citlali": 5})); n != 2 {
		t.Errorf("RotationsFor(citlali C5) = %d rotations, want 2", n)
	}
	if n := len(got.RotationsFor(map[string]int{"citlali": 6})); n != 3 {
		t.Errorf("RotationsFor(citlali C6) = %d rotations, want 3", n)
	}

	// A variant drops a known action of a member.
	for _, bad := range []string{
		"id: x\nmembers: [mavuika, citlali, c, d]\nvariant: {drop: skill, char: citlali}\nstatus: supported\n",
		"id: x\nmembers: [mavuika, citlali, c, d]\nvariant: {drop: burst, char: xilonen}\nstatus: supported\n",
	} {
		write(filepath.Join(root, AltDir, "v"), bad, "")
		if _, err := Load(root); err == nil {
			t.Errorf("expected an error for variant %q", bad)
		}
	}
}
