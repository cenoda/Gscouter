package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cenoda/gscouter/internal/library"
)

const variantName = "hutao-c6-no-burst"

var hutaoVariant = library.Variant{Char: "hutao", Drop: "burst"}

// teamDir copies the committed Hu Tao team (without its alternatives) into
// a temporary directory.
func teamDir(t *testing.T) string {
	t.Helper()
	src := "../../teams/hutao-xingqiu-zhongli-yelan"
	dir := t.TempDir()
	for _, name := range []string{"team.yaml", "build.txt", "rotation.gcsl"} {
		raw, err := os.ReadFile(filepath.Join(src, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// stub stands in for the simulation for the rest of the test.
func stub(t *testing.T, f func(rotation string) (float64, error)) {
	t.Helper()
	prev := measure
	measure = func(_ *library.Team, _, rotation string, _ int) (float64, error) { return f(rotation) }
	t.Cleanup(func() { measure = prev })
}

// existing writes a variant into dir as an earlier run would have and
// returns its files' contents.
func existing(t *testing.T, dir string) map[string]string {
	t.Helper()
	team, err := library.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	stub(t, func(rotation string) (float64, error) {
		if rotation == team.Rotation {
			return 1000, nil
		}
		return 1100, nil
	})
	if _, err := derive(dir, variantName, hutaoVariant, 6, 0.01, 1); err != nil {
		t.Fatal(err)
	}
	measure = simulate
	return read(t, filepath.Join(dir, library.AltDir, variantName))
}

func read(t *testing.T, dir string) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, e := range entries {
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		files[e.Name()] = string(raw)
	}
	return files
}

// kept checks that the variant in dir is still the earlier one and that
// nothing was left beside it.
func kept(t *testing.T, dir string, want map[string]string) {
	t.Helper()
	alt := filepath.Join(dir, library.AltDir)
	got := read(t, filepath.Join(alt, variantName))
	if len(got) != len(want) {
		t.Fatalf("the variant holds %d files, want %d", len(got), len(want))
	}
	for name, content := range want {
		if got[name] != content {
			t.Errorf("%s changed", name)
		}
	}
	if entries, _ := os.ReadDir(alt); len(entries) != 1 {
		t.Errorf("alt holds %d entries, want only the variant", len(entries))
	}
}

func TestDeriveKeepsVariantOnInputError(t *testing.T) {
	for _, tc := range []struct {
		name  string
		spoil func(t *testing.T, dir string)
		v     library.Variant
	}{
		{"team.yaml unreadable", func(t *testing.T, dir string) {
			if err := os.WriteFile(filepath.Join(dir, "team.yaml"), []byte("members: [\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}, hutaoVariant},
		{"no such member", func(*testing.T, string) {}, library.Variant{Char: "bennett", Drop: "burst"}},
		{"no burst to drop", func(t *testing.T, dir string) {
			if err := os.WriteFile(filepath.Join(dir, "rotation.gcsl"), []byte("hutao skill, attack:3;\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}, hutaoVariant},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := teamDir(t)
			want := existing(t, dir)
			stub(t, func(string) (float64, error) {
				t.Fatal("measured despite the input error")
				return 0, nil
			})
			tc.spoil(t, dir)
			if _, err := derive(dir, variantName, tc.v, 6, 0.01, 1); err == nil {
				t.Fatal("no error")
			}
			kept(t, dir, want)
		})
	}
}

// The audit's reproduction: a rotation gcsim cannot parse, with the real
// simulation.
func TestDeriveKeepsVariantOnParseError(t *testing.T) {
	dir := teamDir(t)
	want := existing(t, dir)
	raw, err := os.ReadFile(filepath.Join(dir, "rotation.gcsl"))
	if err != nil {
		t.Fatal(err)
	}
	rotation := strings.Replace(string(raw), "active zhongli;\n", "active zhongli;\nhutao audit_invalid_action;\n", 1)
	if rotation == string(raw) {
		t.Fatal("the rotation does not start as expected")
	}
	if err := os.WriteFile(filepath.Join(dir, "rotation.gcsl"), []byte(rotation), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = derive(dir, variantName, hutaoVariant, 6, 0.01, 1)
	if err == nil || !strings.Contains(err.Error(), "the team's rotation") {
		t.Fatalf("error %v, want gcsim's on the team's rotation", err)
	}
	t.Log(err)
	kept(t, dir, want)
}

func TestDeriveKeepsVariantOnSimulationFailure(t *testing.T) {
	dir := teamDir(t)
	want := existing(t, dir)
	team, err := library.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, side := range []string{"team's rotation", "variant"} {
		t.Run(side, func(t *testing.T) {
			stub(t, func(rotation string) (float64, error) {
				if (rotation == team.Rotation) == (side == "team's rotation") {
					return 0, errors.New("simulation failed")
				}
				return 1000, nil
			})
			if _, err := derive(dir, variantName, hutaoVariant, 6, 0.01, 1); err == nil || !strings.Contains(err.Error(), side) {
				t.Fatalf("error %v, want one about the %s", err, side)
			}
			kept(t, dir, want)
		})
	}
}

func TestDeriveRemovesVariantOnLowGain(t *testing.T) {
	dir := teamDir(t)
	existing(t, dir)
	stub(t, func(string) (float64, error) { return 1000, nil })
	gain, err := derive(dir, variantName, hutaoVariant, 6, 0.01, 1)
	if err != nil || gain != 0 {
		t.Fatalf("gain %v, error %v; want 0, nil", gain, err)
	}
	if _, err := os.Stat(filepath.Join(dir, library.AltDir, variantName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the variant is still there (%v)", err)
	}
}

func TestDeriveReplacesVariant(t *testing.T) {
	dir := teamDir(t)
	before := existing(t, dir)
	team, err := library.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	stub(t, func(rotation string) (float64, error) {
		if rotation == team.Rotation {
			return 1000, nil
		}
		return 1234, nil
	})
	gain, err := derive(dir, variantName, hutaoVariant, 6, 0.01, 1)
	if err != nil || gain < 0.2 {
		t.Fatalf("gain %v, error %v", gain, err)
	}
	after := read(t, filepath.Join(dir, library.AltDir, variantName))
	if after["team.yaml"] == before["team.yaml"] || !strings.Contains(after["team.yaml"], "1234 DPS") {
		t.Errorf("team.yaml not replaced:\n%s", after["team.yaml"])
	}
	if after["rotation.gcsl"] != before["rotation.gcsl"] || strings.Contains(after["rotation.gcsl"], "hutao burst") {
		t.Errorf("rotation.gcsl:\n%s", after["rotation.gcsl"])
	}
	if entries, _ := os.ReadDir(filepath.Join(dir, library.AltDir)); len(entries) != 1 {
		t.Errorf("alt holds %d entries, want only the variant", len(entries))
	}
	if got, err := library.Load(dir); err != nil || len(got.Alternatives) != 1 || got.Alternatives[0].Variant == nil {
		t.Errorf("the team does not load with its variant: %v", err)
	}
}

func TestReplaceDirKeepsDirOnWriteFailure(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "alt", "v")
	if err := replaceDir(dir, map[string]string{"a": "old"}); err != nil {
		t.Fatal(err)
	}
	// A file name that cannot be created fails the second write.
	if err := replaceDir(dir, map[string]string{"a": "new", "no/such/dir": "x"}); err == nil {
		t.Fatal("no error")
	}
	if got := read(t, dir); len(got) != 1 || got["a"] != "old" {
		t.Errorf("dir holds %v, want the old file", got)
	}
	if entries, _ := os.ReadDir(filepath.Dir(dir)); len(entries) != 1 {
		t.Errorf("staging left behind: %d entries", len(entries))
	}
}
