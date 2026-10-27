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
}
