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
