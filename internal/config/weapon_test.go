package config

import (
	"slices"
	"strings"
	"testing"

	"github.com/genshinsim/gcsim/pkg/gcs/ast"
	"github.com/genshinsim/gcsim/pkg/gcs/parser"
)

const weaponBuild = `xilonen char lvl=90/90 cons=0 talent=9,9,9;
xilonen add weapon="favsword" refine=3 lvl=90/90;
zhongli char lvl=90/90 cons=0 talent=9,9,9;
zhongli add weapon="staffofhoma" refine=1 lvl=90/90;
dehya char lvl=90/90 cons=0 talent=9,9,9;
dehya add weapon="sacrificialgreatsword" refine=5 lvl=90/90;
`

const weaponRotation = `active xilonen;
while !.xilo.mods.favonius-cd { xilonen attack; }
while !.zhongli.mods.favonius-cd && !.zhongli.burst.ready { zhongli attack; }
if !.dehya.mods.sacrificial-cd { dehya skill; }
if .dehya.mods.favonius-cd { dehya attack; }
if .furina.mods.favonius-cd { furina attack; }
`

func TestGuardWeaponMods(t *testing.T) {
	got := GuardWeaponMods(weaponBuild, weaponRotation)
	want := `active xilonen;
while !.xilo.mods.favonius-cd { xilonen attack; }
while !1 && !.zhongli.burst.ready { zhongli attack; }
if !.dehya.mods.sacrificial-cd { dehya skill; }
if 1 { dehya attack; }
if .furina.mods.favonius-cd { furina attack; }
`
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	// gcsim reads the rewritten rotation.
	cfg := Assemble(Conditions{Iterations: 1, Duration: 10}, weaponBuild, weaponRotation)
	if _, _, err := parser.New(ast.NewFile(), cfg).Parse(); err != nil {
		t.Errorf("parse: %v\n%s", err, cfg)
	}
	if !strings.Contains(cfg, "while !1 && ") {
		t.Errorf("Assemble does not guard the rotation:\n%s", cfg)
	}
	// Kokomi's Favonius weapon never procs; her Sacrificial one does.
	kokomi := `kokomi char lvl=90/90 cons=0 talent=9,9,9;
kokomi add weapon="favoniuscodex" refine=5 lvl=90/90;
`
	if got := GuardWeaponMods(kokomi, "while !.kokomi.mods.favonius-cd { kokomi attack; }"); got != "while !1 { kokomi attack; }" {
		t.Errorf("Kokomi's Favonius wait not guarded: %s", got)
	}
	kokomi = strings.Replace(kokomi, "favoniuscodex", "sacrificialfragments", 1)
	if r := "if .kokomi.mods.sacrificial-cd { kokomi attack; }"; GuardWeaponMods(kokomi, r) != r {
		t.Errorf("Kokomi's Sacrificial read changed: %s", GuardWeaponMods(kokomi, r))
	}
	if got := GuardWeaponMods(weaponBuild, "active xilonen;\nxilonen skill;\n"); got != "active xilonen;\nxilonen skill;\n" {
		t.Errorf("a rotation without weapon mods changed:\n%s", got)
	}
}

func TestSwapModWeapons(t *testing.T) {
	got, chars := SwapModWeapons(weaponBuild, weaponRotation)
	if want := []string{"xilonen", "dehya"}; !slices.Equal(chars, want) {
		t.Errorf("changed %v, want %v", chars, want)
	}
	for _, want := range []string{`xilonen add weapon="thealleyflash" refine=3`, `zhongli add weapon="staffofhoma"`, `dehya add weapon="rainslasher" refine=5`} {
		if !strings.Contains(got, want) {
			t.Errorf("build does not contain %q:\n%s", want, got)
		}
	}
	// Nothing reads a mod the swapped weapons give.
	if g := GuardWeaponMods(got, weaponRotation); !strings.Contains(g, "while !1 { xilonen attack; }") || !strings.Contains(g, "if !1 { dehya skill; }") {
		t.Errorf("swapped build still reads the mods:\n%s", g)
	}
	if _, chars := SwapModWeapons(weaponBuild, "active xilonen;\nxilonen skill;\n"); chars != nil {
		t.Errorf("swapped %v for a rotation without weapon mods", chars)
	}
}
