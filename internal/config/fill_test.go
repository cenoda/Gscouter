package config

import (
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
		"if .bennett.skill.ready { bennett skill; } else if .arlecchino.skill.ready { arlecchino skill; } else if .furina.skill.ready { furina skill; } else { bennett attack; }",
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
