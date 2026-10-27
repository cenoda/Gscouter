package config

import (
	"fmt"
	"math"

	"github.com/genshinsim/gcsim/pkg/core/attributes"
	"github.com/genshinsim/gcsim/pkg/core/info"
	"github.com/genshinsim/gcsim/pkg/enemy"
)

// Boss is a preset enemy: a boss's base resistances, from gcsim's monster
// data. A preset keeps the level it is given. Phase changes in
// resistance, shields and immunity are not modeled.
type Boss struct {
	Key    string // gcsim's monster name, also the URL's boss parameter
	NameID int    // the monster ID the game's encyclopedia names the boss by
	Weekly bool   // a weekly boss, else a world boss
	Resist [len(Elements)]int
}

// bossList lists the presets, weekly bosses first, each by gcsim name and
// the ID of its name. Some weekly bosses' gcsim data is a later phase of
// the fight, with another ID than the encyclopedia's.
var bossList = []Boss{
	{Key: "stormterror", NameID: 29010101, Weekly: true},
	{Key: "lupusboreasdominatorofwolves", NameID: 29020101, Weekly: true},
	{Key: "childe", NameID: 29030101, Weekly: true},
	{Key: "azhdaha", NameID: 29040101, Weekly: true},
	{Key: "lasignora", NameID: 29050101, Weekly: true},
	{Key: "magatsumitakenarukaminomikoto", NameID: 29060201, Weekly: true},
	{Key: "shoukinokamitheprodigal", NameID: 29070101, Weekly: true},
	{Key: "guardianofapepsoasis", NameID: 29080101, Weekly: true},
	{Key: "alldevouringnarwhal", NameID: 29090101, Weekly: true},
	{Key: "theknave", NameID: 29091001, Weekly: true},
	{Key: "lordoferodedprimalfire", NameID: 29091101, Weekly: true},
	{Key: "thegamebeforethegate", NameID: 29122000, Weekly: true},
	{Key: "thedoctor", NameID: 29130201, Weekly: true},
	{Key: "ildottore", NameID: 29150101, Weekly: true},

	{Key: "oceanid", NameID: 20050101},
	{Key: "hydrotulpa", NameID: 20051201},
	{Key: "thundermanifestation", NameID: 20070101},
	{Key: "frostnightherra", NameID: 20081001},
	{Key: "goldenwolflord", NameID: 22060101},
	{Key: "ruinserpent", NameID: 24010401},
	{Key: "perpetualmechanicalarray", NameID: 24021101},
	{Key: "aeonblightdrake", NameID: 24030301},
	{Key: "maguukenki", NameID: 25090101},
	{Key: "cryoregisvine", NameID: 26020101},
	{Key: "pyroregisvine", NameID: 26020201},
	{Key: "electroregisvine", NameID: 26020301},
	{Key: "primogeovishap", NameID: 26050101},
	{Key: "jadeplumeterrorshroom", NameID: 26110101},
	{Key: "setekhwenut", NameID: 26130101},
	{Key: "millennialpearlseahorse", NameID: 26180101},
	{Key: "solitarysuanni", NameID: 26190101},
	{Key: "gluttonousyumkasaurmountainking", NameID: 26220301},
	{Key: "goldflamequcusaurtyrant", NameID: 26230301},
	{Key: "lavadragonstatue", NameID: 26270301},
	{Key: "radiantmoonfly", NameID: 26301101},
	{Key: "radiantmoongecko", NameID: 26302101},
	{Key: "chimericwingedlion", NameID: 26310501},
}

// elementAttrs are gcsim's elements in the order of Elements.
var elementAttrs = [len(Elements)]attributes.Element{
	attributes.Pyro, attributes.Hydro, attributes.Anemo, attributes.Electro,
	attributes.Dendro, attributes.Cryo, attributes.Geo, attributes.Physical,
}

// Bosses are the presets, with their resistances.
var Bosses = func() []Boss {
	out := make([]Boss, len(bossList))
	for i, b := range bossList {
		p := info.EnemyProfile{Level: StandardEnemy.Level}
		if err := enemy.ConfigureTarget(&p, b.Key, enemy.TargetParams{Particles: true}); err != nil {
			panic(fmt.Sprintf("config: boss %s: %v", b.Key, err))
		}
		for j, el := range elementAttrs {
			b.Resist[j] = int(math.Round(p.Resist[el] * 100))
		}
		out[i] = b
	}
	return out
}()

// FindBoss returns the preset with the given key.
func FindBoss(key string) (Boss, bool) {
	for _, b := range Bosses {
		if b.Key == key {
			return b, true
		}
	}
	return Boss{}, false
}
