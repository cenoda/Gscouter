package config

import (
	"math"
	"testing"

	"github.com/genshinsim/gcsim/pkg/core/attributes"
	"github.com/genshinsim/gcsim/pkg/core/info"
	"github.com/genshinsim/gcsim/pkg/enemy"
)

// TestBossesMatchGcsim checks that the committed boss resistances equal
// what gcsim's monster data gives, as Bosses was built before it was
// committed as data (legacy_gcsim). It goes with the gcsim tests in step 5
// of gscouter-engine's docs/independence.md.
func TestBossesMatchGcsim(t *testing.T) {
	elements := [len(Elements)]attributes.Element{
		attributes.Pyro, attributes.Hydro, attributes.Anemo, attributes.Electro,
		attributes.Dendro, attributes.Cryo, attributes.Geo, attributes.Physical,
	}
	for _, b := range Bosses {
		p := info.EnemyProfile{Level: StandardEnemy.Level}
		if err := enemy.ConfigureTarget(&p, b.Key, enemy.TargetParams{Particles: true}); err != nil {
			t.Errorf("%s: %v", b.Key, err)
			continue
		}
		var want [len(Elements)]int
		for j, el := range elements {
			want[j] = int(math.Round(p.Resist[el] * 100))
		}
		if b.Resist != want {
			t.Errorf("%s: resist %v, gcsim gives %v", b.Key, b.Resist, want)
		}
	}
}
