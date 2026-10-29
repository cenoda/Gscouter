package score

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/cenoda/gscouter/internal/config"
	"github.com/genshinsim/gcsim/pkg/core"
	"github.com/genshinsim/gcsim/pkg/core/keys"
)

// chascaConfig is the library's Chasca / Bennett / Durin / Nicole team,
// whose rotation, written for C0, fires aim[bullets=4], with Chasca at
// cons.
func chascaConfig(cons string) string {
	build := `
chasca char lvl=90/90 cons=` + cons + ` talent=9,9,9;
chasca add weapon="hamayumi" refine=5 lvl=90/90;
chasca add set="obsidiancodex" count=4;
chasca add stats hp=4780 atk=311 atk%=0.466 pyro%=0.466 cd=0.622;
chasca add stats atk%=0.297 cr=0.331 cd=0.662;
bennett char lvl=90/90 cons=6 talent=9,9,9;
bennett add weapon="alleyflash" refine=1 lvl=90/90;
bennett add set="no" count=4;
bennett add stats hp=4780 atk=311 er=0.518 pyro%=0.466 cr=0.311;
durin char lvl=90/90 cons=0 talent=9,9,9;
durin add weapon="wolffang" refine=1 lvl=90/90;
durin add set="risingwinds" count=4;
durin add stats hp=4780 atk=311 atk%=0.466 pyro%=0.466 cd=0.622;
nicole char lvl=90/90 cons=0 talent=9,9,9;
nicole add weapon="oathsworn" refine=5 lvl=90/90;
nicole add set="gift" count=4;
nicole add stats hp=4780 atk=311 atk%=0.466 atk%=0.466 atk%=0.466;
`
	rotation := `
active bennett;
for let i = 0; i < 4; i = i + 1 {
  bennett skill, burst;
  if .nicole.burst.ready {
    nicole burst;
  }
  nicole skill;
  durin skill:2, burst;
  chasca skill, aim:4, aim[bullets=4];
  if .chasca.burst.ready && .chasca.energy == .chasca.energymax {
    chasca burst;
  }
}
`
	return config.Assemble(config.Conditions{Iterations: 5}, build, rotation)
}

// withGcsimChasca runs f with gcsim's own Chasca, unwrapped.
func withGcsimChasca(t *testing.T, f func()) {
	t.Helper()
	wrapped := core.NewCharFuncMap[keys.Chasca]
	core.NewCharFuncMap[keys.Chasca] = gcsimChasca
	defer func() { core.NewCharFuncMap[keys.Chasca] = wrapped }()
	f()
}

// A C6 Chasca fires six shells while her instant charge is ready, so a
// rotation written for C0 runs instead of failing.
func TestChascaC6InstantChargeFiresSix(t *testing.T) {
	cfg := chascaConfig("6")
	withGcsimChasca(t, func() {
		_, err := Run(context.Background(), cfg, Options{Iterations: 5, Seed: 1})
		if err == nil || !strings.Contains(err.Error(), "bullets must be 6") {
			t.Fatalf("gcsim's own Chasca: err = %v, want the C6 bullets error this fix is for", err)
		}
	})
	res, err := Run(context.Background(), cfg, Options{Iterations: 5, Seed: 1})
	if err != nil {
		// chascaC6Key no longer names gcsim's status, or the rule moved.
		t.Fatalf("C6 Chasca: %v", err)
	}
	c0, err := Run(context.Background(), chascaConfig("0"), Options{Iterations: 5, Seed: 1})
	if err != nil {
		t.Fatal(err)
	}
	if res.DPS <= c0.DPS {
		t.Errorf("C6 DPS %.0f is not above C0's %.0f", res.DPS, c0.DPS)
	}
}

// Below C6 Chasca is gcsim's own: every result is the same to the last
// bit.
func TestChascaBelowC6Unchanged(t *testing.T) {
	for _, cons := range []string{"0", "2", "5"} {
		cfg := chascaConfig(cons)
		got, err := RunIterations(context.Background(), cfg, Options{Iterations: 5, Seed: 1})
		if err != nil {
			t.Fatal(err)
		}
		var want []Iteration
		withGcsimChasca(t, func() {
			want, err = RunIterations(context.Background(), cfg, Options{Iterations: 5, Seed: 1})
		})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("C%s: results differ from gcsim's own Chasca", cons)
		}
	}
}
