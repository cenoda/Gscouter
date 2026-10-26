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
