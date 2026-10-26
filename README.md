# Gscouter

Genshin Impact team DPS calculator: settles "whose team is better built?" by scoring every team under the same conditions with a fixed rotation. See [docs/design.md](docs/design.md) for the design.

Status: prototype. The website looks up a UID on Enka.Network, lets you pick four showcase characters, and scores or compares teams.

## Requirements

- Go 1.27 or newer (with `GOTOOLCHAIN=auto`, an older Go downloads it automatically)

Gscouter uses [gcsim](https://github.com/genshinsim/gcsim) as a Go library, pinned in `go.mod`.

## Website

```sh
go run ./cmd/server          # http://localhost:8080
```

| Flag | Default | Description |
|---|---|---|
| `-addr` | `:8080` (or `$PORT`) | Listen address |
| `-iter` | 100 | Iterations per score |
| `-warmup` | 0 | Seconds excluded from the score |
| `-duration` | 90 | Fight duration in seconds |
| `-concurrent` | 1 | Simulations running at once |
| `-queue` | 8 | Simulations waiting before requests are rejected |

Deployment: see [deploy/README.md](deploy/README.md).

## Command line

Scores a build file (gcsim character setup lines) against a library team, with 1000 iterations.

```sh
go run ./cmd/gscouter \
  -team teams/skirk-yelan-furina-escoffier \
  -build teams/skirk-yelan-furina-escoffier/build.txt
```

```
Team:      Skirk / Yelan / Furina / Escoffier (rotation v1, draft)
Score:     107372 DPS ± 76 (1 s.e.; s.d. 2390)
Window:    0s–90s, 1000 iterations, 1.719s
Drift:     -1.80% (second half vs. first half of the window)

Character           DPS   Share    Energy wait
skirk             61269   57.1%        0.00s/m
yelan             14150   13.2%        0.00s/m
furina            19374   18.0%        0.28s/m
escoffier         12579   11.7%        0.18s/m
```

- **Score**: team DPS over 90 s, starting with full energy, as players measure on a training dummy.
- **Drift**: second-half vs. first-half DPS. Slightly negative is normal; far below means the rotation stalls.
- **Energy wait**: seconds per minute that a character's actions waited for energy. Too little Energy Recharge for the rotation lowers the score.

Flags:

| Flag | Default | Description |
|---|---|---|
| `-team` | | Team library directory |
| `-build` | | Build file |
| `-iter` | 1000 | Iterations |
| `-warmup` | 0 | Seconds excluded from the score |
| `-duration` | 90 | Fight duration in seconds |
| `-seed` | 1 | Seed of the first iteration |
| `-dump` | false | Print the assembled gcsim config and exit |

## Team library

Each team has a `team.yaml`, a `rotation.gcsl` and a reference `build.txt` used only for validation. `cmd/validate` runs every team with its reference build and fails a team whose config does not parse, whose member deals no damage, whose character stands idle 10 s/min or more, or whose rotation stalls. It warns about action waits and members with almost no damage.

```sh
go run ./cmd/validate                 # every team
go run ./cmd/validate -v teams/<id>   # one team, per-character results
```

The `add-team` skill (`.claude/skills/add-team/SKILL.md`) describes how to add a team.

## Layout

```
cmd/gscouter/       CLI
cmd/server/         website
cmd/validate/       team library checks
internal/config/    common simulation conditions and config assembly
internal/enka/      Enka.Network client and converter to gcsim
internal/web/       HTTP handlers and templates
internal/library/   team library loading and matching
internal/score/     team DPS from gcsim runs
teams/<id>/         team.yaml, rotation.gcsl and reference build.txt per team
deploy/             VM setup and auto-update
```

## Tests

```sh
go test ./...
```
