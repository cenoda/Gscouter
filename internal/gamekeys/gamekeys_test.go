package gamekeys

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/genshinsim/gcsim/pkg/shortcut"
	"gopkg.in/yaml.v3"
)

func TestDecode(t *testing.T) {
	c := All()
	if c.Version != CatalogVersion || len(c.Characters) == 0 || len(c.Weapons) == 0 || len(c.Sets) == 0 {
		t.Fatalf("catalog: version %d, %d characters, %d weapons, %d sets", c.Version, len(c.Characters), len(c.Weapons), len(c.Sets))
	}
	if _, err := Decode([]byte(`{"catalog_version": 2}`)); err == nil {
		t.Error("a catalog of another version decodes")
	}
	if _, err := Decode([]byte(`{"catalog_version": 1, "sets": [{"key": "a", "aliases": ["b"]}, {"key": "b"}]}`)); err == nil {
		t.Error("a name of two keys decodes")
	}
	for _, tc := range [][3]string{
		{"char", "kazuha", "kaedeharakazuha"},
		{"char", "kokomi", "sangonomiyakokomi"},
		{"char", "travelercryo", "luminecryo"},
		{"char", "luminecryo", "luminecryo"},
		{"weapon", "favlance", "favoniuslance"},
		{"set", "sms", "silkenmoonsserenade"},
	} {
		var got string
		switch tc[0] {
		case "char":
			got, _ = Char(tc[1])
		case "weapon":
			got, _ = Weapon(tc[1])
		case "set":
			got, _ = Set(tc[1])
		}
		if got != tc[2] {
			t.Errorf("%s %s = %q, want %q", tc[0], tc[1], got, tc[2])
		}
	}
}

// The catalog resolves exactly the names gcsim's pkg/shortcut accepts,
// each to the key gcsim's keys print, so Gscouter's keys stay the same
// for every name and alias. It goes with the gcsim tests in step 5 of
// gscouter-engine's docs/independence.md.
func TestSameAsGcsimShortcut(t *testing.T) {
	check := func(what string, gcsim map[string]string, ours map[string]string) {
		for name, key := range gcsim {
			if got, ok := ours[name]; !ok || got != key {
				t.Errorf("%s %q: catalog %q (%v), gcsim %q", what, name, got, ok, key)
			}
		}
		for name, key := range ours {
			if _, ok := gcsim[name]; !ok {
				t.Errorf("%s %q (%s): in the catalog, not in gcsim's shortcut", what, name, key)
			}
		}
	}
	gc := map[string]string{}
	for n, k := range shortcut.CharNameToKey {
		gc[n] = k.String()
	}
	gw := map[string]string{}
	for n, k := range shortcut.WeaponNameToKey {
		gw[n] = k.String()
	}
	gs := map[string]string{}
	for n, k := range shortcut.SetNameToKey {
		gs[n] = k.String()
	}
	check("character", gc, chars)
	check("weapon", gw, weapons)
	check("set", gs, sets)
	if n := len(gc) + len(gw) + len(gs); n != len(chars)+len(weapons)+len(sets) {
		t.Errorf("%d names in gcsim, %d in the catalog", n, len(chars)+len(weapons)+len(sets))
	}
}

var (
	buildChar   = regexp.MustCompile(`(?m)^\s*([A-Za-z0-9]+)\s+(char|add)\b`)
	buildWeapon = regexp.MustCompile(`\badd\s+weapon\s*=\s*"?([^"\s;]+)`)
	buildSet    = regexp.MustCompile(`\badd\s+set\s*=\s*"?([^"\s;]+)`)
)

// Every character, weapon and set the team library names (team.yaml's
// members and every build.txt, alternatives included) is in the catalog.
func TestCoversLibrary(t *testing.T) {
	root := filepath.Join("..", "..", "teams")
	n := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		raw, err := os.ReadFile(path)
		switch d.Name() {
		case "team.yaml":
			if err != nil {
				return err
			}
			var team struct{ Members []string }
			if err := yaml.Unmarshal(raw, &team); err != nil {
				return err
			}
			for _, m := range team.Members {
				if _, ok := Char(strings.ToLower(m)); !ok {
					t.Errorf("%s: member %s is not in the catalog", path, m)
				}
			}
			n++
		case "build.txt":
			if err != nil {
				return err
			}
			for _, m := range buildChar.FindAllStringSubmatch(string(raw), -1) {
				if _, ok := Char(strings.ToLower(m[1])); !ok {
					t.Errorf("%s: character %s is not in the catalog", path, m[1])
				}
			}
			for _, m := range buildWeapon.FindAllStringSubmatch(string(raw), -1) {
				if _, ok := Weapon(strings.ToLower(m[1])); !ok {
					t.Errorf("%s: weapon %s is not in the catalog", path, m[1])
				}
			}
			for _, m := range buildSet.FindAllStringSubmatch(string(raw), -1) {
				if _, ok := Set(strings.ToLower(m[1])); !ok {
					t.Errorf("%s: set %s is not in the catalog", path, m[1])
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if n < 1000 {
		t.Errorf("read %d teams, want the library", n)
	}
}
