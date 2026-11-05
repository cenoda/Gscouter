package steady

import (
	"github.com/cenoda/gscouter/internal/config"
	"github.com/cenoda/gscouter/internal/library"
)

// LibraryConfig is the config of a team's library build as cmd/gscouter
// -dump prints it, which gscouter-engine's compare runs to verify a
// release: build with the params of the team's first rotation for its
// constellations, and the team's own rotation looped, over a fight of
// duration seconds.
func LibraryConfig(team *library.Team, build string, iterations int, duration float64) (string, error) {
	rotations := team.RotationsFor(library.BuildConstellations(build))
	for m, p := range rotations[0].Params {
		build = config.AddParams(build, m, p)
	}
	looped, err := Looped(team.Rotation, team.Members)
	if err != nil {
		return "", err
	}
	return config.Assemble(config.Conditions{Iterations: iterations, Duration: duration}, build, looped), nil
}
