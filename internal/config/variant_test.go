package config

import (
	"os"
	"strings"
	"testing"

	"github.com/genshinsim/gcsim/pkg/gcs/ast"
	"github.com/genshinsim/gcsim/pkg/gcs/parser"
)

func TestDropBursts(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"citlali skill, burst, attack;\nmavuika burst;", "citlali skill, attack;\nmavuika burst;"},
		{"citlali burst;\nmavuika skill;", "\nmavuika skill;"},
		{"if .citlali.burst.ready { citlali burst; } citlali skill,burst[x=1];", "if .citlali.burst.ready {  } citlali skill;"},
		{"for let i=0; i<4; i=i+1 {\n  citlali attack:2, burst;\n}", "for let i=0; i<4; i=i+1 {\n  citlali attack:2;\n}"},
		{"xcitlali burst;", "xcitlali burst;"},
		// A comma inside brackets separates parameters, not actions.
		{"citlali skill[a=1, b=2], burst[hits=10, mine_delay=250], attack[c=3, d=4];", "citlali skill[a=1, b=2], attack[c=3, d=4];"},
		{"citlali burst[hits=10, mine_delay=250];\nmavuika skill;", "\nmavuika skill;"},
		// gcsim allows spaces before parameters and repeat counts.
		{"active citlali; citlali burst [counter=1];", "active citlali; "},
		{"citlali skill [hold=1], burst [counter=1], attack :2;", "citlali skill [hold=1], attack :2;"},
		{"citlali burst :2, skill;", "citlali skill;"},
	} {
		if got := DropBursts(c.in, "citlali"); got != c.want {
			t.Errorf("DropBursts(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	// The finding's case: a burst written with a space before its
	// parameters goes as one written without.
	if got, want := DropBursts("active beidou; beidou burst [counter=1];", "beidou"), "active beidou; "; got != want {
		t.Errorf("DropBursts kept a spaced burst: %q, want %q", got, want)
	}
	// The committed Kirara statement (teams/klee-aloy-kirara-kaedeharakazuha).
	in := "kirara skill[short_hold=1], burst[hits=10, mine_delay=250];"
	if got, want := DropBursts(in, "kirara"), "kirara skill[short_hold=1];"; got != want {
		t.Errorf("DropBursts(%q) = %q, want %q", in, got, want)
	}
}

// The no-burst variant of a library team whose burst takes several
// parameters still parses once assembled.
func TestDropBurstsTeamParses(t *testing.T) {
	dir := "../../teams/klee-aloy-kirara-kaedeharakazuha/"
	build, err := os.ReadFile(dir + "build.txt")
	if err != nil {
		t.Fatal(err)
	}
	rot, err := os.ReadFile(dir + "rotation.gcsl")
	if err != nil {
		t.Fatal(err)
	}
	got := DropBursts(string(rot), "kirara")
	if !strings.Contains(got, "\n  kirara skill[short_hold=1];\n") || strings.Contains(got, "hits=10") {
		t.Errorf("Kirara's burst not taken out whole:\n%s", got)
	}
	cfg := Assemble(Conditions{Iterations: 1}, string(build), got)
	if _, _, err := parser.New(ast.NewFile(), cfg).Parse(); err != nil {
		t.Errorf("the variant does not parse: %v", err)
	}
}

// A member may be named under any of gcsim's names, in the rotation and
// in the request: statements are matched by gcsim key and kept under the
// rotation's name.
func TestDropBurstsAliases(t *testing.T) {
	in := "active fischl;\nfischl skill, burst;\namy burst, attack;\nif .amy.burst.ready { amy attack, burst; }\nbeidou burst;\n"
	want := "active fischl;\nfischl skill;\namy attack;\nif .amy.burst.ready { amy attack; }\nbeidou burst;\n"
	for _, char := range []string{"fischl", "amy", "Fischl"} {
		if got := DropBursts(in, char); got != want {
			t.Errorf("DropBursts(%q) = %q, want %q", char, got, want)
		}
	}
	// The committed example: the rotation writes Fischl as "amy" and
	// Alhaitham as "haitham", the team as fischl and alhaitham.
	raw, err := os.ReadFile("../../teams/alhaitham-bennett-fischl-dori/rotation.gcsl")
	if err != nil {
		t.Fatal(err)
	}
	rot := string(raw)
	for _, c := range []struct {
		names      [2]string
		gone, kept string
	}{
		{[2]string{"fischl", "amy"}, "amy attack, burst;", "amy attack;"},
		{[2]string{"alhaitham", "haitham"}, "haitham burst;", "alhaitham attack:4"},
	} {
		canonical, alias := DropBursts(rot, c.names[0]), DropBursts(rot, c.names[1])
		if canonical == rot || canonical != alias || strings.Contains(canonical, c.gone) || !strings.Contains(canonical, c.kept) {
			t.Errorf("%v: bursts not taken out alike:\n%s\n---\n%s", c.names, canonical, alias)
		}
	}
}
