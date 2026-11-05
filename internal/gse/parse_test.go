package gse

import (
	"errors"
	"strings"
	"testing"
)

// The contract of gscouter-engine docs/parse-api.md, parse_version 1,
// decoded without a module.
func TestDecodeParse(t *testing.T) {
	ok := `{"parse_version": 1, "ok": true,
	 "characters": [{"key": "kaedeharakazuha", "level": 90, "max_level": 90, "ascension": 6, "cons": 0,
	   "talents": [9, 9, 9], "params": {},
	   "weapon": {"key": "favoniussword", "level": 90, "max_level": 90, "refine": 3, "params": {"stacks": 5}},
	   "sets": [{"key": "viridescentvenerer", "count": 4, "params": {}}],
	   "stats": {"hp": 4780, "em": 561, "def%": 0.124}},
	   {"key": "luminecryo", "level": 80, "max_level": 90, "ascension": null, "cons": 6, "talents": [1, 8, 10],
	    "params": {}, "weapon": null, "sets": [], "stats": {}}],
	 "active": null, "targets": [], "energy": null, "options": {}, "hurt": false,
	 "errors": [{"class": "no_active_character", "message": "no active char set"}, {"class": "no_targets", "message": "no targets"}]}`
	p, err := decodeParse(0, []byte(ok))
	if err != nil {
		t.Fatal(err)
	}
	k := p.Characters[0]
	if len(p.Characters) != 2 || k.Key != "kaedeharakazuha" || k.MaxLevel != 90 || *k.Ascension != 6 || k.Talents != [3]int{9, 9, 9} ||
		k.Weapon.Key != "favoniussword" || k.Weapon.Refine != 3 || k.Weapon.Params["stacks"] != 5 ||
		k.Sets[0].Key != "viridescentvenerer" || k.Sets[0].Count != 4 || k.Stats["em"] != 561 {
		t.Errorf("parsed %+v", p.Characters)
	}
	if p.Characters[1].Weapon != nil || p.Characters[1].Ascension != nil {
		t.Errorf("Traveler: %+v", p.Characters[1])
	}
	if len(p.Errors) != 2 || p.Errors[0].Class != "no_active_character" {
		t.Errorf("errors %v", p.Errors)
	}

	for _, tc := range []struct{ body, want string }{
		{`{"parse_version": 1, "ok": false, "error": {"class": "unknown_weapon", "line": 2, "message": "unrecognized weapon \"nope\""}}`, `unknown weapon (line 2): unrecognized weapon "nope"`},
		{`{"parse_version": 1, "ok": false, "error": {"class": "unsupported", "line": 3, "message": "add stats random is not supported"}}`, "not supported by Gscouter's parser (line 3): add stats random is not supported"},
		{`{"parse_version": 1, "ok": false, "error": {"class": "syntax", "line": null, "message": "unexpected end of input"}}`, "syntax: unexpected end of input"},
	} {
		p, err := decodeParse(1, []byte(tc.body))
		var pe *ParseError
		if !errors.As(err, &pe) || p == nil || err.Error() != tc.want {
			t.Errorf("%s: %v, want %q", tc.body, err, tc.want)
		}
	}
	if _, err := decodeParse(1, []byte("config is not UTF-8: invalid utf-8")); err == nil || !strings.Contains(err.Error(), "UTF-8") {
		t.Errorf("plain error: %v", err)
	}
	if _, err := decodeParse(0, []byte(`{"parse_version": 2, "ok": true}`)); err == nil {
		t.Error("parse_version 2 is read")
	}
}

// With a pinned module that has the parse API, a build parses and a
// malformed one gives its class.
func TestParsePinned(t *testing.T) {
	if err := Available(); err != nil {
		t.Skipf("skipped until a release with gse_parse is pinned: %v", err)
	}
	p, err := Parse("ht char lvl=90/90 cons=1 talent=10,10,10;\nht add weapon=\"homa\" refine=1 lvl=90/90;\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Characters) != 1 || p.Characters[0].Key != "hutao" || p.Characters[0].Weapon.Key != "staffofhoma" {
		t.Errorf("parsed %+v", p.Characters)
	}
	for cfg, class := range map[string]string{
		"hutao char lvl=90/90 cons=1 talent=10,10,10;\nhutao add weapon=\"nope\" refine=1 lvl=90/90;\n":                     "unknown_weapon",
		"hutao char lvl=90/90 cons=1 talent=10,10,10;\nhutao add set=\"nope\" count=4;\n":                                   "unknown_set",
		"hutao char lvl=90/90 cons=1 talent=10,10,10;\nhutao add stats random rarity=5 sand=hp% goblet=pyro% circlet=cd;\n": "unsupported",
		"hutao char lvl=90/90 cons=1 talent=10,10,10\n":                                                                     "syntax",
	} {
		_, err := Parse(cfg)
		var pe *ParseError
		if !errors.As(err, &pe) || pe.Class != class {
			t.Errorf("%q: %v, want class %s", cfg, err, class)
		}
	}
}
