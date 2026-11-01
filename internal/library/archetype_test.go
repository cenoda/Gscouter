package library

import "testing"

func TestDeriveArchetype(t *testing.T) {
	for _, c := range []struct {
		members []string
		want    string
	}{
		{[]string{"hutao", "xingqiu", "zhongli", "yelan"}, "Vaporize"},
		{[]string{"raiden", "xiangling", "xingqiu", "bennett"}, "Vaporize, Electro-Charged, Overload"},
		{[]string{"neuvillette", "furina", "kazuha", "zhongli"}, "Hydro"},
		{[]string{"aratakiitto", "gorou", "zhongli", "albedo"}, "Geo"},
		{[]string{"travelercryo", "ganyu", "mona", "kazuha"}, "Freeze"},
	} {
		if got := DeriveArchetype(c.members); got != c.want {
			t.Errorf("DeriveArchetype(%v) = %q, want %q", c.members, got, c.want)
		}
	}
	written := &Team{Members: []string{"hutao", "xingqiu", "zhongli", "yelan"}, Archetype: "Pyro vaporize hypercarry"}
	if got := written.ArchetypeLabel(); got != written.Archetype {
		t.Errorf("ArchetypeLabel() = %q, want the written %q", got, written.Archetype)
	}
}
