package config

import "testing"

func TestDropBursts(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"citlali skill, burst, attack;\nmavuika burst;", "citlali skill, attack;\nmavuika burst;"},
		{"citlali burst;\nmavuika skill;", "\nmavuika skill;"},
		{"if .citlali.burst.ready { citlali burst; } citlali skill,burst[x=1];", "if .citlali.burst.ready {  } citlali skill;"},
		{"for let i=0; i<4; i=i+1 {\n  citlali attack:2, burst;\n}", "for let i=0; i<4; i=i+1 {\n  citlali attack:2;\n}"},
		{"xcitlali burst;", "xcitlali burst;"},
	} {
		if got := DropBursts(c.in, "citlali"); got != c.want {
			t.Errorf("DropBursts(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
