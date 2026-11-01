package share

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/cenoda/gscouter/internal/library"
)

func TestSplitConfig(t *testing.T) {
	raw, err := os.ReadFile("testdata/hutao.txt")
	if err != nil {
		t.Fatal(err)
	}
	sp := SplitConfig(string(raw))

	if want := []string{"hutao", "yelan", "xingqiu", "zhongli"}; !slices.Equal(sp.Members, want) {
		t.Errorf("members %v, want %v", sp.Members, want)
	}
	if got := sp.Params["hutao"]; got != "start_hp=1" {
		t.Errorf("hutao params %q", got)
	}
	if strings.Contains(sp.Build, "+params") {
		t.Errorf("build still has params:\n%s", sp.Build)
	}
	if n := strings.Count(sp.Build, ";"); n != 17 {
		t.Errorf("build has %d statements, want 17:\n%s", n, sp.Build)
	}
	if len(sp.Dropped) != 3 {
		t.Errorf("dropped %q, want options, energy and target", sp.Dropped)
	}
	if !strings.HasPrefix(sp.Rotation, "active zhongli;") {
		t.Errorf("rotation starts with %q", sp.Rotation[:min(40, len(sp.Rotation))])
	}
	for _, s := range []string{"options", "target", "energy", " char ", " add "} {
		if strings.Contains(sp.Rotation, s) {
			t.Errorf("rotation contains %q:\n%s", s, sp.Rotation)
		}
	}
	if !strings.Contains(sp.Rotation, "attack:2, charge, dash;") {
		t.Errorf("rotation lost the Hu Tao string:\n%s", sp.Rotation)
	}
}

func TestResolve(t *testing.T) {
	for link, want := range map[string]string{
		"https://gcsim.app/sh/bKtD798PKDfW":   "bKtD798PKDfW",
		"gcsim.app/sh/bKtD798PKDfW?tab=x":     "bKtD798PKDfW",
		"bKtD798PKDfW":                        "bKtD798PKDfW",
		"https://simpact.app/sh/bKtD798PKDfW": "bKtD798PKDfW",
	} {
		got, err := resolve(t.Context(), link)
		if err != nil || got != want {
			t.Errorf("resolve(%q) = %q, %v; want %q", link, got, err, want)
		}
	}
}

// TestGcsimVersions keeps the recorded versions in step with go.mod: the
// upstream version required, and the fork that replaces it.
func TestGcsimVersions(t *testing.T) {
	mod, err := os.ReadFile("../../go.mod")
	if err != nil {
		t.Fatal(err)
	}
	var require, replace string
	for line := range strings.Lines(string(mod)) {
		switch {
		case strings.HasPrefix(line, "replace github.com/genshinsim/gcsim "):
			replace = line
		case strings.HasPrefix(strings.TrimSpace(line), "github.com/genshinsim/gcsim "):
			require = line
		}
	}
	if !strings.Contains(require, "-"+GcsimCommit) {
		t.Errorf("go.mod requires gcsim %q, GcsimCommit is %s", strings.TrimSpace(require), GcsimCommit)
	}
	if !strings.Contains(replace, "-"+GcsimFork) {
		t.Errorf("go.mod replaces gcsim with %q, GcsimFork is %s", strings.TrimSpace(replace), GcsimFork)
	}
}

func TestTeamTier(t *testing.T) {
	for members, fork := range map[string]bool{"hutao,xingqiu,zhongli,yelan": false, "nefer,lauma,nahida,xingqiu": true} {
		team, err := Split{Members: strings.Split(members, ",")}.Team(nil, nil, "")
		if err != nil {
			t.Fatal(err)
		}
		if got := team.Tier == library.TierExperimental && team.GcsimFork == GcsimFork; got != fork {
			t.Errorf("%s: tier %q, fork %q", members, team.Tier, team.GcsimFork)
		}
		if !fork && (team.Tier != library.TierVerified || team.GcsimFork != "") {
			t.Errorf("%s: tier %q, fork %q, want neither", members, team.Tier, team.GcsimFork)
		}
	}
	// A build with a weapon only the fork has makes the team experimental.
	sp := Split{Members: strings.Split("hutao,xingqiu,zhongli,yelan", ","), Build: `xingqiu add weapon="hymnofthemaelstrom" refine=1 lvl=90/90;`}
	team, err := sp.Team(nil, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if team.Tier != library.TierExperimental || team.GcsimFork != GcsimFork {
		t.Errorf("Hymn build: tier %q, fork %q", team.Tier, team.GcsimFork)
	}
}

// A share of a team already in the library, with its members declared in
// another order, is refused, and with force replaces that team in its own
// directory: a second directory for the same members would stop the
// library from loading.
func TestWriteSameMembers(t *testing.T) {
	root := t.TempDir()
	first := Split{Members: []string{"hutao", "xingqiu", "zhongli", "yelan"}, Rotation: "hutao attack;\n"}
	team, err := first.Team(nil, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	team.Archetype = "vape"
	if _, err := first.Write(root, team, false); err != nil {
		t.Fatal(err)
	}

	again := Split{Members: []string{"zhongli", "hutao", "yelan", "xingqiu"}, Rotation: "zhongli skill;\n"}
	team2, err := again.Team(nil, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := again.Write(root, team2, false); err == nil || !strings.Contains(err.Error(), "same members") {
		t.Fatalf("write without force: err %v, want same members", err)
	}
	dir, err := again.Write(root, team2, true)
	if err != nil {
		t.Fatal(err)
	}
	if want := root + "/hutao-xingqiu-zhongli-yelan"; dir != want {
		t.Errorf("forced write went to %s, want %s", dir, want)
	}
	lib, err := library.LoadLibrary(os.DirFS(root))
	if err != nil {
		t.Fatal(err)
	}
	if len(lib.Teams) != 1 {
		t.Fatalf("library has %d teams, want 1", len(lib.Teams))
	}
	got := lib.Teams[0]
	if got.ID != "hutao-xingqiu-zhongli-yelan" || got.Archetype != "vape" {
		t.Errorf("replaced team id %q archetype %q", got.ID, got.Archetype)
	}
	rot, err := os.ReadFile(dir + "/rotation.gcsl")
	if err != nil || string(rot) != "zhongli skill;\n" {
		t.Errorf("rotation %q, %v", rot, err)
	}
}
