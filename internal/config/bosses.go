package config

// Boss is a preset enemy: a boss's base resistances. A preset keeps the
// level it is given. Phase changes in resistance, shields and immunity
// are not modeled.
type Boss struct {
	Key    string // gcsim's monster name, also the URL's boss parameter
	NameID int    // the monster ID the game's encyclopedia names the boss by
	Weekly bool   // a weekly boss, else a world boss
	Resist [len(Elements)]int
}

// Bosses are the presets, weekly bosses first, each by gcsim name, the ID
// of its name and its resistances in percent, in the order of Elements
// (pyro, hydro, anemo, electro, dendro, cryo, geo, physical). Some weekly
// bosses' data is a later phase of the fight, with another ID than the
// encyclopedia's.
//
// Provenance: legacy_gcsim. The resistances were generated once
// (2026-11-05) from gcsim's monster data, through enemy.ConfigureTarget
// at the fork commit go.mod pins (cenoda/gcsim eb21884), and are kept
// under the owner's legacy-data exception of 2026-11-04
// (gscouter-engine docs/independence.md §4). gcsim's data is generated
// from the game's files, so a new boss's resistances come from official
// or measured sources instead, recorded beside its row.
// bosses_test.go checks that every row still equals gcsim's.
var Bosses = []Boss{
	{Key: "stormterror", NameID: 29010101, Weekly: true, Resist: [len(Elements)]int{10, 10, 10, 10, 10, 10, 10, 10}},
	{Key: "lupusboreasdominatorofwolves", NameID: 29020101, Weekly: true, Resist: [len(Elements)]int{10, 10, 30, 10, 10, 50, 10, 10}},
	{Key: "childe", NameID: 29030101, Weekly: true, Resist: [len(Elements)]int{0, 70, 0, 70, 0, 0, 0, 0}},
	{Key: "azhdaha", NameID: 29040101, Weekly: true, Resist: [len(Elements)]int{10, 10, 10, 10, 10, 10, 70, 40}},
	{Key: "lasignora", NameID: 29050101, Weekly: true, Resist: [len(Elements)]int{70, 10, 10, 10, 10, 10, 10, 10}},
	{Key: "magatsumitakenarukaminomikoto", NameID: 29060201, Weekly: true, Resist: [len(Elements)]int{10, 10, 10, 10, 10, 10, 10, 10}},
	{Key: "shoukinokamitheprodigal", NameID: 29070101, Weekly: true, Resist: [len(Elements)]int{30, 30, 30, 90, 30, 30, 30, 30}},
	{Key: "guardianofapepsoasis", NameID: 29080101, Weekly: true, Resist: [len(Elements)]int{10, 10, 10, 10, 70, 10, 10, 10}},
	{Key: "alldevouringnarwhal", NameID: 29090101, Weekly: true, Resist: [len(Elements)]int{10, 70, 10, 10, 10, 10, 10, 10}},
	{Key: "theknave", NameID: 29091001, Weekly: true, Resist: [len(Elements)]int{70, 10, 10, 10, 10, 10, 10, 10}},
	{Key: "lordoferodedprimalfire", NameID: 29091101, Weekly: true, Resist: [len(Elements)]int{10, 10, 10, 10, 10, 10, 10, 10}},
	{Key: "thegamebeforethegate", NameID: 29122000, Weekly: true, Resist: [len(Elements)]int{10, 70, 10, 70, 10, 10, 10, 10}},
	{Key: "thedoctor", NameID: 29130201, Weekly: true, Resist: [len(Elements)]int{10, 10, 10, 10, 10, 10, 10, 10}},
	{Key: "ildottore", NameID: 29150101, Weekly: true, Resist: [len(Elements)]int{10, 10, 10, 10, 10, 10, 10, 10}},

	{Key: "oceanid", NameID: 20050101, Resist: [len(Elements)]int{10, 10, 10, 10, 10, 10, 10, 10}},
	{Key: "hydrotulpa", NameID: 20051201, Resist: [len(Elements)]int{10, 10, 10, 10, 10, 10, 10, 10}},
	{Key: "thundermanifestation", NameID: 20070101, Resist: [len(Elements)]int{10, 10, 10, 10, 10, 10, 10, 10}},
	{Key: "frostnightherra", NameID: 20081001, Resist: [len(Elements)]int{10, 10, 10, 10, 10, 10, 10, 10}},
	{Key: "goldenwolflord", NameID: 22060101, Resist: [len(Elements)]int{25, 25, 25, 25, 25, 25, 25, 25}},
	{Key: "ruinserpent", NameID: 24010401, Resist: [len(Elements)]int{10, 10, 10, 10, 10, 10, 10, 70}},
	{Key: "perpetualmechanicalarray", NameID: 24021101, Resist: [len(Elements)]int{10, 10, 10, 10, 10, 10, 10, 70}},
	{Key: "aeonblightdrake", NameID: 24030301, Resist: [len(Elements)]int{10, 10, 10, 10, 10, 10, 10, 70}},
	{Key: "maguukenki", NameID: 25090101, Resist: [len(Elements)]int{10, 10, 10, 10, 10, 10, 10, 10}},
	{Key: "cryoregisvine", NameID: 26020101, Resist: [len(Elements)]int{10, 10, 10, 10, 10, 70, 10, 30}},
	{Key: "pyroregisvine", NameID: 26020201, Resist: [len(Elements)]int{70, 10, 10, 10, 10, 10, 10, 30}},
	{Key: "electroregisvine", NameID: 26020301, Resist: [len(Elements)]int{10, 10, 10, 70, 10, 10, 10, 30}},
	{Key: "primogeovishap", NameID: 26050101, Resist: [len(Elements)]int{10, 10, 10, 10, 10, 10, 50, 30}},
	{Key: "jadeplumeterrorshroom", NameID: 26110101, Resist: [len(Elements)]int{25, 25, 25, 25, 80, 25, 25, 25}},
	{Key: "setekhwenut", NameID: 26130101, Resist: [len(Elements)]int{25, 25, 60, 25, 25, 25, 25, 25}},
	{Key: "millennialpearlseahorse", NameID: 26180101, Resist: [len(Elements)]int{10, 10, 10, 60, 10, 10, 10, 10}},
	{Key: "solitarysuanni", NameID: 26190101, Resist: [len(Elements)]int{10, 70, 70, 10, 10, 10, 10, 10}},
	{Key: "gluttonousyumkasaurmountainking", NameID: 26220301, Resist: [len(Elements)]int{10, 10, 10, 10, 70, 10, 10, 10}},
	{Key: "goldflamequcusaurtyrant", NameID: 26230301, Resist: [len(Elements)]int{70, 10, 10, 10, 10, 10, 10, 10}},
	{Key: "lavadragonstatue", NameID: 26270301, Resist: [len(Elements)]int{10, 10, 10, 10, 10, 10, 10, 10}},
	{Key: "radiantmoonfly", NameID: 26301101, Resist: [len(Elements)]int{70, 10, 10, 10, 70, 10, 10, 10}},
	{Key: "radiantmoongecko", NameID: 26302101, Resist: [len(Elements)]int{10, 10, 10, 10, 10, 10, 70, 10}},
	{Key: "chimericwingedlion", NameID: 26310501, Resist: [len(Elements)]int{10, 10, 50, 50, 10, 10, 10, 10}},
}

// FindBoss returns the preset with the given key.
func FindBoss(key string) (Boss, bool) {
	for _, b := range Bosses {
		if b.Key == key {
			return b, true
		}
	}
	return Boss{}, false
}
