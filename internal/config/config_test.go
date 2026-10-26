package config

import (
	"strings"
	"testing"
)

func TestRenameCharacters(t *testing.T) {
	rot := "active travelercryo;\ntravelercryo skill;\nif .travelercryo.burst.ready { travelercryo burst; }\nfaruzan skill;"
	got := RenameCharacters(rot, map[string]string{"travelercryo": "aethercryo", "faruzan": "faruzan"})
	want := "active aethercryo;\naethercryo skill;\nif .aethercryo.burst.ready { aethercryo burst; }\nfaruzan skill;"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestAddParams(t *testing.T) {
	build := "hutao char lvl=90/90 cons=1 talent=10,9,10;\nhutao add weapon=\"staffofhoma\" refine=1 lvl=90/90;\n"
	got := AddParams(build, "hutao", "start_hp=1")
	want := "hutao char lvl=90/90 cons=1 talent=10,9,10 +params=[start_hp=1];\nhutao add weapon=\"staffofhoma\" refine=1 lvl=90/90;\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	if again := AddParams(got, "hutao", "start_hp=1"); again != got {
		t.Errorf("params added twice:\n%s", again)
	}
}

func TestLoop(t *testing.T) {
	got, err := Loop("active zhongli;\nzhongli skill[hold=1];\nhutao skill, attack:2, charge;\n")
	if err != nil {
		t.Fatal(err)
	}
	want := "active zhongli;\nwhile 1 {\nzhongli skill[hold=1];\nhutao skill, attack:2, charge;\n}\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	if _, err := Loop("zhongli skill;"); err == nil {
		t.Error("expected an error without an active statement")
	}
}

func TestLoopKeepsSetupOutside(t *testing.T) {
	rot := `set_particle_delay("bennett", 100);
active varka; // stacks
let walk = true;
fn step() {
  if walk { varka walk; }
}
while .varka.energy < 60 { varka attack; }
if walk {
  step();
} else {
  varka dash;
}
varka burst;
`
	got, err := Loop(rot)
	if err != nil {
		t.Fatal(err)
	}
	want := `set_particle_delay("bennett", 100);
active varka;
let walk = true;
fn step() {
  if walk { varka walk; }
}
while 1 {
while .varka.energy < 60 { varka attack; }
if walk {
  step();
} else {
  varka dash;
}
varka burst;
}
`
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	if _, err := Loop(`active varka; varka walk; print("open`); err != nil {
		t.Errorf("unterminated string: %v", err)
	}
	if _, err := Loop("active varka; let x = 1;"); err == nil {
		t.Error("expected an error without actions")
	}
}

func TestAssembleSource(t *testing.T) {
	conditions := "# Conditions of the source config.\noptions swap_delay=12 iteration=1000 duration=90;\ntarget lvl=95 resist=0.1 hp=1e9;\ntarget lvl=95 resist=0.1 hp=1e9;\n"
	cfg := AssembleSource(conditions, 20, "xingqiu char lvl=90/90 cons=6 talent=9,9,9;", "active xingqiu;\nxingqiu skill;")
	for _, want := range []string{"options swap_delay=12 duration=90 iteration=20;\n", "target lvl=95 resist=0.1 hp=1e9;\ntarget lvl=95", "xingqiu skill;"} {
		if !strings.Contains(cfg, want) {
			t.Errorf("config does not contain %q:\n%s", want, cfg)
		}
	}
	if strings.Contains(cfg, "1000") || strings.Contains(cfg, "#") {
		t.Errorf("source iterations or comments kept:\n%s", cfg)
	}
	if cfg := AssembleSource("target lvl=100 resist=0.1;", 5, "", "active a;"); !strings.HasPrefix(cfg, "target lvl=100 resist=0.1;\noptions iteration=5;\n") {
		t.Errorf("missing options line not added:\n%s", cfg)
	}
}
