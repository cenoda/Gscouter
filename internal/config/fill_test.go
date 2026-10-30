package config

import (
	"maps"
	"os"
	"strings"
	"testing"

	"github.com/genshinsim/gcsim/pkg/gcs/ast"
	"github.com/genshinsim/gcsim/pkg/gcs/parser"
)

func TestFillSelf(t *testing.T) {
	rot := "active ineffa;\nfor let i=0; i<2; i=i+1 {\n  flins skill, attack, burst, attack:4;\n  ineffa burst;\n}\n"
	got := Fill(rot, map[string]Filler{"flins": FillSelf}, nil, []string{"ineffa", "flins"})
	want := "active ineffa;\nfor let i=0; i<2; i=i+1 {\n  flins skill, attack; " +
		"let gs_fill1 = 0; while !.flins.burst.ready && gs_fill1 < 20 { if .flins.skill.ready { flins skill; } else { flins attack; } gs_fill1 = gs_fill1 + 1; } " +
		"flins burst, attack:4;\n  ineffa burst;\n}\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestFillPartyAndSkill(t *testing.T) {
	rot := "active arlecchino;\nbennett burst; arlecchino skill[hold=1], attack:5;\n"
	got := Fill(rot, map[string]Filler{"bennett": FillParty}, []string{"arlecchino"}, []string{"arlecchino", "bennett", "furina"})
	for _, s := range []string{
		"if .bennett.skill.ready { bennett skill; } else if .arlecchino.skill.ready { arlecchino skill[hold=1]; } else if .furina.skill.ready { furina skill; } else { bennett attack; }",
		"while !.arlecchino.skill.ready && gs_fill2 < 30 { arlecchino attack;",
		"arlecchino skill[hold=1], attack:5;",
	} {
		if !strings.Contains(got, s) {
			t.Errorf("missing %q in:\n%s", s, got)
		}
	}
	if _, _, err := parser.New(ast.NewFile(), "bennett char lvl=90/90 cons=0 talent=9,9,9;\n"+got).Parse(); err != nil {
		t.Errorf("filled rotation does not parse: %v\n%s", err, got)
	}
}

// A rotation that writes members under gcsim aliases ("chev", "xl") is
// filled for them, under the party's names.
func TestFillAliases(t *testing.T) {
	rot := "active fischl;\nchev burst, skill[hold=1], attack;\nxl burst, skill;\nchev attack:3;\n"
	party := []string{"fischl", "beidou", "xiangling", "chevreuse"}
	got := Fill(rot, map[string]Filler{"chevreuse": FillParty, "xiangling": FillSkip}, nil, party)
	for _, s := range []string{
		"if .chevreuse.skill.ready { chevreuse skill[hold=1]; } else if .fischl.skill.ready { fischl skill; } else if .beidou.skill.ready { beidou skill; } else if .xiangling.skill.ready { xiangling skill; } else { chevreuse attack; }",
		"chevreuse burst, skill[hold=1], attack;",
		"if .xiangling.burst.ready { xiangling burst; } else { wait(1); } xiangling skill;",
		"chevreuse attack:3;",
	} {
		if !strings.Contains(got, s) {
			t.Errorf("missing %q in:\n%s", s, got)
		}
	}
	if _, _, err := parser.New(ast.NewFile(), "chevreuse char lvl=90/90 cons=0 talent=9,9,9;\n"+got).Parse(); err != nil {
		t.Errorf("filled rotation does not parse: %v\n%s", err, got)
	}
}

func TestFillLeavesOthers(t *testing.T) {
	rot := "active a;\nlet x = 1;\nif .a.burst.ready { b burst; }\n"
	if got := Fill(rot, map[string]Filler{"a": FillSelf}, nil, []string{"a", "b"}); got != rot {
		t.Errorf("changed a rotation without bursts of a:\n%s", got)
	}
}

func TestFillUsesSkillAsWritten(t *testing.T) {
	rot := "active bennett;\nxianyun skill, low_plunge, burst;\nkazuha burst;\nkazuha skill[hold=1], high_plunge[collision=1], dash;\n"
	got := Fill(rot, map[string]Filler{"kazuha": FillParty}, nil, []string{"bennett", "xianyun", "kazuha"})
	for _, s := range []string{
		"if .kazuha.skill.ready { kazuha skill[hold=1], high_plunge[collision=1]; }",
		"else if .xianyun.skill.ready { xianyun skill, low_plunge; }",
		"else if .bennett.skill.ready { bennett skill; }",
	} {
		if !strings.Contains(got, s) {
			t.Errorf("missing %q in:\n%s", s, got)
		}
	}
}

func TestFillSkip(t *testing.T) {
	rot := "active candace;\nskirk burst;\nskirk skill, attack:2, burst[x=1], attack:5;\n"
	got := Fill(rot, map[string]Filler{"skirk": FillSkip}, nil, []string{"skirk", "candace"})
	want := "active candace;\nif .skirk.burst.ready { skirk burst; } else { wait(1); }\nskirk skill, attack:2; if .skirk.burst.ready { skirk burst[x=1]; } else { wait(1); } skirk attack:5;\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	if _, _, err := parser.New(ast.NewFile(), "skirk char lvl=90/90 cons=0 talent=9,9,9;\n"+got).Parse(); err != nil {
		t.Errorf("filled rotation does not parse: %v\n%s", err, got)
	}
}

func TestFillSkipPerMember(t *testing.T) {
	rot := "active furina;\nyelan skill, burst, skill;\nhutao skill, attack:2, charge, dash, burst;\n"
	got := Fill(rot, map[string]Filler{"hutao": FillSkip, "yelan": FillSelf}, nil, []string{"hutao", "yelan", "furina"})
	for _, s := range []string{
		"hutao skill, attack:2, charge, dash; if .hutao.burst.ready { hutao burst; } else { wait(1); }\n",
		"while !.yelan.burst.ready && gs_fill1 < 20 {",
		"yelan burst, skill;",
	} {
		if !strings.Contains(got, s) {
			t.Errorf("missing %q in:\n%s", s, got)
		}
	}
}

func TestFillBrief(t *testing.T) {
	rot := "active ineffa;\nineffa skill, burst;\n"
	got := Fill(rot, map[string]Filler{"ineffa": FillBrief}, nil, []string{"ineffa", "ayato"})
	want := "active ineffa;\nineffa skill; " +
		"let gs_fill1 = 0; let gs_fill1a = 0; while !.ineffa.burst.ready && gs_fill1 < 20 && gs_fill1a < 2 { " +
		"if .ineffa.skill.ready { ineffa skill; } else if .ayato.skill.ready { ayato skill; } else { ineffa attack; gs_fill1a = gs_fill1a + 1; } gs_fill1 = gs_fill1 + 1; } " +
		"if .ineffa.burst.ready { ineffa burst; } else { wait(1); }\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	if _, _, err := parser.New(ast.NewFile(), "ineffa char lvl=90/90 cons=0 talent=9,9,9;\n"+got).Parse(); err != nil {
		t.Errorf("filled rotation does not parse: %v\n%s", err, got)
	}
}

func TestWaitLoops(t *testing.T) {
	rot := "active raiden;\nraiden skill;\nwhile !.xingqiu.burst.ready {\n  xingqiu attack;\n}\nxingqiu burst;\nwhile !.yelan.burst.ready { bennett attack; }\nyelan burst;\n"
	party := []string{"raiden", "xingqiu", "yelan", "bennett"}
	loop := "while !.xingqiu.burst.ready {\n  xingqiu attack;\n}"
	other := "while !.yelan.burst.ready { bennett attack; }"
	bounded := "let gs_wait1 = 0; while !.xingqiu.burst.ready && gs_wait1 < 2 { xingqiu attack; gs_wait1 = gs_wait1 + 1; }"
	for _, c := range []struct {
		energy      map[string]Filler
		has, hasNot []string
	}{
		{nil, []string{bounded, other}, []string{loop}},
		{map[string]Filler{"xingqiu": FillSelf}, []string{loop}, []string{bounded}},
		{map[string]Filler{"xingqiu": FillSkip}, []string{"if .xingqiu.burst.ready { xingqiu burst; }"}, []string{loop, bounded, "xingqiu attack"}},
		{map[string]Filler{"xingqiu": FillBrief}, []string{"gs_fill1a < 2"}, []string{loop, bounded}},
	} {
		got := Fill(WaitLoops(rot, c.energy, party), c.energy, nil, party)
		for _, s := range c.has {
			if !strings.Contains(got, s) {
				t.Errorf("%v: missing %q in:\n%s", c.energy, s, got)
			}
		}
		for _, s := range c.hasNot {
			if strings.Contains(got, s) {
				t.Errorf("%v: unexpected %q in:\n%s", c.energy, s, got)
			}
		}
		if _, _, err := parser.New(ast.NewFile(), got).Parse(); err != nil {
			t.Errorf("%v: filled rotation does not parse: %v\n%s", c.energy, err, got)
		}
	}
}

// An action with several parameters keeps its brackets whole: a comma
// inside them separates parameters, not actions.
func TestFillMultiParamSkill(t *testing.T) {
	rot := "active klee;\nklee attack;\nklee skill[bounce=3, mine=8];\nklee burst;\n"
	got := Fill(rot, map[string]Filler{"klee": FillSelf}, nil, []string{"klee"})
	if s := "if .klee.skill.ready { klee skill[bounce=3, mine=8]; }"; !strings.Contains(got, s) {
		t.Errorf("missing %q in:\n%s", s, got)
	}
	if _, _, err := parser.New(ast.NewFile(), "klee char lvl=90/90 cons=0 talent=9,9,9;\n"+got).Parse(); err != nil {
		t.Errorf("filled rotation does not parse: %v\n%s", err, got)
	}
}

func TestFillPartyMultiParamSkill(t *testing.T) {
	rot := "active klee;\nklee skill[bounce=3, mine=8], attack;\nbennett burst;\n"
	got := Fill(rot, map[string]Filler{"bennett": FillParty}, nil, []string{"bennett", "klee"})
	if s := "else if .klee.skill.ready { klee skill[bounce=3, mine=8]; }"; !strings.Contains(got, s) {
		t.Errorf("missing %q in:\n%s", s, got)
	}
	if _, _, err := parser.New(ast.NewFile(), "klee char lvl=90/90 cons=0 talent=9,9,9;\n"+got).Parse(); err != nil {
		t.Errorf("filled rotation does not parse: %v\n%s", err, got)
	}
}

func TestFillSkipMultiParamBurst(t *testing.T) {
	rot := "active xiangling;\nxiangling burst[a=1, b=2], skill;\n"
	got := Fill(rot, map[string]Filler{"xiangling": FillSkip}, nil, []string{"xiangling"})
	if s := "if .xiangling.burst.ready { xiangling burst[a=1, b=2]; } else { wait(1); } xiangling skill;"; !strings.Contains(got, s) {
		t.Errorf("missing %q in:\n%s", s, got)
	}
	if _, _, err := parser.New(ast.NewFile(), "xiangling char lvl=90/90 cons=0 talent=9,9,9;\n"+got).Parse(); err != nil {
		t.Errorf("filled rotation does not parse: %v\n%s", err, got)
	}
}

// Statements written on one line are all seen when the rotation's skill
// forms are taken.
func TestFillSameLineStatements(t *testing.T) {
	rot := "active zhongli; zhongli skill[hold=1]; zhongli burst;\n"
	got := Fill(rot, map[string]Filler{"zhongli": FillSelf}, nil, []string{"zhongli"})
	if s := "if .zhongli.skill.ready { zhongli skill[hold=1]; }"; !strings.Contains(got, s) {
		t.Errorf("missing %q in:\n%s", s, got)
	}
}

// gcsim's lexer skips spaces between an action's name, its parameters
// and its repeat count, so each spelling reads as the compact one.
func TestParseAction(t *testing.T) {
	for _, c := range []struct {
		in   string
		want action
	}{
		{"skill", action{name: "skill"}},
		{" skill[counter=1] ", action{name: "skill", params: "[counter=1]"}},
		{"skill [counter=1]", action{name: "skill", params: "[counter=1]"}},
		{"skill[hold=1]:2", action{name: "skill", params: "[hold=1]", repeat: "2"}},
		{"skill [hold=1] : 2", action{name: "skill", params: "[hold=1]", repeat: "2"}},
		{"attack :4", action{name: "attack", repeat: "4"}},
		{"burst [a=1, b=2]", action{name: "burst", params: "[a=1, b=2]"}},
		{"high_plunge[collision=1]", action{name: "high_plunge", params: "[collision=1]"}},
	} {
		if got := parseAction(c.in); got != c.want {
			t.Errorf("parseAction(%q) = %+v, want %+v", c.in, got, c.want)
		}
	}
}

// The committed Beidou team writes "beidou skill [counter=1]": its skill
// form is the one written without the space, parameter and all.
func TestSkillFormsSpaceBeforeParams(t *testing.T) {
	rot, err := os.ReadFile("../../teams/aratakiitto-bennett-fischl-beidou/rotation.gcsl")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rot), "beidou skill [counter=1]") {
		t.Fatal("the team no longer writes Beidou's skill with a space")
	}
	party := []string{"itto", "bennett", "fischl", "beidou"}
	spaced := skillForms(string(rot), party)
	compact := skillForms(strings.ReplaceAll(string(rot), "skill [", "skill["), party)
	if !maps.Equal(spaced, compact) {
		t.Errorf("forms differ by spelling:\n%v\n%v", spaced, compact)
	}
	if got := spaced["beidou"]; got != "skill[counter=1]" {
		t.Errorf("Beidou's form = %q, want skill[counter=1]", got)
	}
	for _, c := range []struct{ rot, want string }{
		{"beidou skill [hold=1] :2;", "skill[hold=1]:2"},
		{"beidou skill[hold=1]:2;", "skill[hold=1]:2"},
	} {
		if got := skillForms(c.rot, []string{"beidou"})["beidou"]; got != c.want {
			t.Errorf("skillForms(%q) = %q, want %q", c.rot, got, c.want)
		}
	}
}

// A space before an action's parameters fills the same as no space: the
// burst and skill fillers are put in and the action is kept as written.
func TestFillSpaceBeforeParams(t *testing.T) {
	party := []string{"itto", "bennett", "fischl", "beidou"}
	spaced := "active beidou;\nbeidou attack, skill [counter=1], burst [x=1];\nbennett skill;\n"
	compact := "active beidou;\nbeidou attack, skill[counter=1], burst[x=1];\nbennett skill;\n"
	unspace := strings.NewReplacer("skill [", "skill[", "burst [", "burst[")
	for _, how := range []Filler{FillSelf, FillParty, FillSkip, FillBrief} {
		energy := map[string]Filler{"beidou": how}
		skill := []string{"beidou"}
		got := Fill(spaced, energy, skill, party)
		want := Fill(compact, energy, skill, party)
		if !strings.Contains(got, "while !.beidou.skill.ready") || unspace.Replace(got) != want {
			t.Errorf("%v: the spelling changes the fill:\n%s\nwant (spaced as written):\n%s", how, got, want)
		}
		if _, _, err := parser.New(ast.NewFile(), "beidou char lvl=90/90 cons=0 talent=9,9,9;\n"+got).Parse(); err != nil {
			t.Errorf("%v: filled rotation does not parse: %v\n%s", how, err, got)
		}
	}
}
