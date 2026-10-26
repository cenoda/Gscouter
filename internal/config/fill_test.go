package config

import (
	"strings"
	"testing"

	"github.com/genshinsim/gcsim/pkg/gcs/ast"
	"github.com/genshinsim/gcsim/pkg/gcs/parser"
)

func TestFillSelf(t *testing.T) {
	rot := "active ineffa;\nfor let i=0; i<2; i=i+1 {\n  flins skill, attack, burst, attack:4;\n  ineffa burst;\n}\n"
	got := Fill(rot, []string{"flins"}, nil, []string{"ineffa", "flins"}, FillSelf)
	want := "active ineffa;\nfor let i=0; i<2; i=i+1 {\n  flins skill, attack; " +
		"let gs_fill1 = 0; while !.flins.burst.ready && gs_fill1 < 20 { if .flins.skill.ready { flins skill; } else { flins attack; } gs_fill1 = gs_fill1 + 1; } " +
		"flins burst, attack:4;\n  ineffa burst;\n}\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestFillPartyAndSkill(t *testing.T) {
	rot := "active arlecchino;\nbennett burst; arlecchino skill[hold=1], attack:5;\n"
	got := Fill(rot, []string{"bennett"}, []string{"arlecchino"}, []string{"arlecchino", "bennett", "furina"}, FillParty)
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
	if got := Fill(rot, []string{"a"}, nil, []string{"a", "b"}, FillSelf); got != rot {
		t.Errorf("changed a rotation without bursts of a:\n%s", got)
	}
}
