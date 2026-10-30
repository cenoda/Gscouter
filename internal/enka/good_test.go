package enka

import (
	"errors"
	"os"
	"slices"
	"strings"
	"testing"
)

func TestParseGOOD(t *testing.T) {
	raw, err := os.ReadFile("testdata/good.json")
	if err != nil {
		t.Fatal(err)
	}
	chars, skipped, err := ParseGOOD(raw)
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, c := range chars {
		keys = append(keys, c.Key)
	}
	// Both Travelers get the shared "Traveler" equipment.
	if want := []string{"hutao", "xingqiu", "zhongli", "yelan", "aetheranemo", "aethergeo"}; !slices.Equal(keys, want) {
		t.Errorf("characters = %v, want %v", keys, want)
	}
	wantSkipped := []GOODSkipped{
		{"Manekina", "character Manekina is not supported by gcsim yet"},
		{"Chongyun", "weapon SomeSwordFromTheFuture is not supported by gcsim yet"},
		{"Bennett", "character Bennett has no weapon"},
	}
	if !slices.Equal(skipped, wantSkipped) {
		t.Errorf("skipped = %+v, want %+v", skipped, wantSkipped)
	}
	if len(chars) == 0 {
		return
	}

	// Main stats come from the main stat table, substats are summed as
	// the game shows them, and talents are base levels.
	want := `hutao char lvl=90/90 cons=1 talent=10,10,9;
hutao add weapon="staffofhoma" refine=1 lvl=90/90;
hutao add set="crimsonwitchofflames" count=4;
hutao add set="gladiatorsfinale" count=1;
hutao add stats def=23 hp=5079 hp%=0.705 atk=363 atk%=0.058 er=0.052 em=105 cr=0.377 cd=1.454 pyro%=0.466;
`
	if got := chars[0].Config(); got != want {
		t.Errorf("config:\n%s\nwant:\n%s", got, want)
	}
	if chars[0].PieceDetail != PiecesFull || len(chars[0].Pieces) != 5 {
		t.Errorf("Hu Tao pieces = %v %+v, want five full pieces", chars[0].PieceDetail, chars[0].Pieces)
	}
	// An unactivated substat does not count.
	yelan := chars[3]
	if yelan.Stats["atk"] != 0 || yelan.Stats["hydro%"] == 0 {
		t.Errorf("Yelan stats = %v", yelan.Stats)
	}
	// A set gcsim does not have keeps its stats.
	zhongli := chars[2]
	if zhongli.UnknownSets != 1 || len(zhongli.Sets) != 0 || zhongli.Stats["er"] != 0.11 {
		t.Errorf("Zhongli = %+v", zhongli)
	}
	if w := chars[2].Weapon; w != (Weapon{Key: "favoniuslance", Refine: 3, Level: 80, MaxLevel: 90}) {
		t.Errorf("Zhongli weapon = %+v", w)
	}

	// Every character is a build a link can carry, unchanged.
	back, err := Unpack(Pack(chars))
	if err != nil {
		t.Fatal(err)
	}
	for i := range chars {
		if back[i].Config() != chars[i].Config() {
			t.Errorf("%s changed when packed", chars[i].Key)
		}
	}
}

func TestParseGOODRejects(t *testing.T) {
	for _, raw := range []string{``, `{}`, `[]`, `{"format":"GO"}`, `not json`} {
		if _, _, err := ParseGOOD([]byte(raw)); !errors.Is(err, ErrNotGOOD) {
			t.Errorf("%q: err = %v, want ErrNotGOOD", raw, err)
		}
	}
	// A character the game could not have is skipped, not the file.
	chars, skipped, err := ParseGOOD([]byte(`{"format":"GOOD","characters":[
		{"key":"HuTao","level":90,"constellation":9,"ascension":6,"talent":{"auto":1,"skill":1,"burst":1}},
		{"key":"Xingqiu","level":90,"constellation":0,"ascension":6,"talent":{"auto":1,"skill":1,"burst":1}}],
		"weapons":[{"key":"StaffOfHoma","level":90,"ascension":6,"refinement":1,"location":"HuTao"},
		{"key":"SacrificialSword","level":90,"ascension":6,"refinement":1,"location":"Xingqiu"}]}`))
	if err != nil || len(chars) != 1 || chars[0].Key != "xingqiu" || len(skipped) != 1 || skipped[0].Key != "HuTao" {
		t.Errorf("chars = %+v, skipped = %+v, err = %v", chars, skipped, err)
	}
}

// GOOD keys are English names in PascalCase without punctuation, so
// goodKey finds a gcsim key whose English name gives the same letters.
// This checks that every gcsim key in the data is one (the Travelers,
// named "Traveler" in English, aside).
func TestGOODKeysMatchNames(t *testing.T) {
	CharacterName("hutao", "en") // loads names
	for _, kind := range []string{"characters", "weapons", "artifacts"} {
		for key, byLang := range names[kind] {
			if kind == "characters" && (strings.HasPrefix(key, "aether") || strings.HasPrefix(key, "lumine")) {
				continue
			}
			if got := goodKey(byLang["en"]); got != key {
				t.Errorf("%s %q: the GOOD key of %q is %q", kind, key, byLang["en"], got)
			}
		}
	}
}

func TestGOODCharKey(t *testing.T) {
	for in, want := range map[string][2]string{
		"KaedeharaKazuha": {"kaedeharakazuha", "KaedeharaKazuha"},
		"TravelerDendro":  {"aetherdendro", "Traveler"},
		"TravelerHydroF":  {"luminehydro", "Traveler"},
		"TravelerHydroM":  {"aetherhydro", "Traveler"},
		"Traveler":        {"traveler", "Traveler"},
	} {
		if key, loc := goodCharKey(in); key != want[0] || loc != want[1] {
			t.Errorf("goodCharKey(%q) = %q, %q, want %q, %q", in, key, loc, want[0], want[1])
		}
	}
}
