package enka

import (
	"bytes"
	"log/slog"
	"os"
	"strings"
	"testing"
)

// A showcase with a character and a weapon missing from the mapping
// records both, with their level, ascension and base stats and nothing
// of the player's, and is otherwise read as before.
func TestObserveNewIDs(t *testing.T) {
	raw := []byte(`{"uid": "812345678", "playerInfo": {"nickname": "Secret"}, "avatarInfoList": [
	  {"avatarId": 10000999, "skillDepotId": 9901,
	   "propMap": {"4001": {"val": "80"}, "1002": {"val": "5"}},
	   "talentIdList": [1, 2], "skillLevelMap": {"1": 9},
	   "fightPropMap": {"1": 12345.5, "4": 987.25, "7": 765.125, "2000": 30000},
	   "equipList": [{"itemId": 19999, "weapon": {"level": 70, "promoteLevel": 4, "affixMap": {"1": 2}},
	     "flat": {"itemType": "ITEM_WEAPON", "weaponStats": [
	       {"appendPropId": "FIGHT_PROP_BASE_ATTACK", "statValue": 510},
	       {"appendPropId": "FIGHT_PROP_CRITICAL", "statValue": 27.6}]}}]},
	  {"avatarId": 10000046, "skillDepotId": 4601,
	   "propMap": {"4001": {"val": "90"}, "1002": {"val": "6"}},
	   "fightPropMap": {"1": 15552.3, "4": 1200, "7": 876},
	   "equipList": [{"itemId": 19998, "weapon": {"level": 90, "promoteLevel": 6},
	     "flat": {"itemType": "ITEM_WEAPON", "weaponStats": [
	       {"appendPropId": "FIGHT_PROP_BASE_ATTACK", "statValue": 608}]}}]}
	]}`)

	var got []NewID
	old := observe
	observe = func(n NewID) { got = append(got, n) }
	t.Cleanup(func() { observe = old })

	p, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	// Both characters are skipped, as without the observation.
	if len(p.Characters) != 0 || len(p.Skipped) != 2 ||
		!strings.Contains(p.Skipped[0].Reason, "character 10000999 is not supported") ||
		!strings.Contains(p.Skipped[1].Reason, "weapon 19998 is not supported") {
		t.Errorf("profile = %+v", p)
	}
	want := []NewID{
		{Kind: "weapon", ID: 19999, Level: 70, Ascension: 4, BaseATK: 510, SubProp: "FIGHT_PROP_CRITICAL", SubValue: 27.6},
		{Kind: "character", ID: 10000999, SkillDepotID: 9901, Level: 80, Ascension: 5,
			BaseHP: 12345.5, BaseATK: 987.25, BaseDEF: 765.125, WeaponBaseATK: 510},
		{Kind: "weapon", ID: 19998, Level: 90, Ascension: 6, BaseATK: 608},
	}
	if len(got) != len(want) {
		t.Fatalf("observed %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("observation %d = %+v, want %+v", i, got[i], want[i])
		}
	}

	// A known character with a known weapon records nothing.
	got = nil
	if raw, err = os.ReadFile("testdata/skirk.json"); err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(raw); err != nil {
		t.Fatal(err)
	}
	for _, n := range got {
		if n.ID != 10000999 {
			t.Errorf("known content observed: %+v", n)
		}
	}
}

// The log line carries the fixed message and the observation, and never
// a UID or nickname; the same observation is logged once.
func TestLogNewID(t *testing.T) {
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })

	n := NewID{Kind: "character", ID: 10000998, SkillDepotID: 9801, Level: 90, Ascension: 6,
		BaseHP: 14000.25, BaseATK: 900.5, BaseDEF: 800.125, WeaponBaseATK: 674}
	observe(n)
	observe(n)
	out := buf.String()
	if strings.Count(out, `msg="enka new id"`) != 1 {
		t.Fatalf("log:\n%s", out)
	}
	for _, s := range []string{"kind=character", "avatar_id=10000998", "skill_depot_id=9801", "level=90",
		"ascension=6", "base_hp=14000.25", "base_atk=900.5", "base_def=800.125", "weapon_base_atk=674"} {
		if !strings.Contains(out, s) {
			t.Errorf("log lacks %s:\n%s", s, out)
		}
	}
	if strings.Contains(strings.ToLower(out), "uid") {
		t.Errorf("log names a UID:\n%s", out)
	}
}
