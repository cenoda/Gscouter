# Gscouter

Genshin Impact team DPS calculator: settles "whose team is better built?" by scoring every team under the same conditions with a fixed rotation. See [docs/design.md](docs/design.md) for the design.

Status: prototype. Builds are read from a file of gcsim character setup lines; Enka.Network input is not implemented yet.

## Requirements

- Go 1.27 or newer (with `GOTOOLCHAIN=auto`, an older Go downloads it automatically)

Gscouter uses [gcsim](https://github.com/genshinsim/gcsim) as a Go library, pinned in `go.mod`.

## Usage

```sh
go run ./cmd/gscouter \
  -team teams/skirk-yelan-furina-escoffier \
  -build examples/builds/skirk-yelan-furina-escoffier.txt
```

```
Team:      Skirk / Yelan / Furina / Escoffier (rotation v1, draft)
Score:     96611 DPS ± 89 (1 s.e.; s.d. 2815)
Window:    300s–900s, 1000 iterations, 11.972s
Drift:     -0.34% (second half vs. first half of the window)

Character           DPS   Share    Energy wait
skirk             56014   58.0%        0.00s/m
yelan             10516   10.9%        0.00s/m
furina            17457   18.1%        0.01s/m
escoffier         12624   13.1%        8.25s/m
```

- **Score**: steady-state team DPS over the window, excluding the opener.
- **Drift**: how much the DPS still changes within the window. Near 0 means the team has settled.
- **Energy wait**: seconds per minute that a character's actions waited for energy. Here the sample Escoffier lacks Energy Recharge for the rotation, which lowers the score.

Flags:

| Flag | Default | Description |
|---|---|---|
| `-team` | | Team library directory |
| `-build` | | Build file |
| `-iter` | 1000 | Iterations |
| `-warmup` | 300 | Seconds excluded from the score |
| `-duration` | 900 | Fight duration in seconds |
| `-seed` | 1 | Seed of the first iteration |
| `-dump` | false | Print the assembled gcsim config and exit |

## Layout

```
cmd/gscouter/       CLI
internal/config/    common simulation conditions and config assembly
internal/library/   team library loading and matching
internal/score/     steady-state DPS from gcsim runs
teams/<id>/         team.yaml + rotation.gcsl per team
examples/builds/    sample builds
```

## Tests

```sh
go test ./...
```
