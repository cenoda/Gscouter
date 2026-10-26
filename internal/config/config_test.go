package config

import "testing"

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
