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
| `-concurrent` | 1 | Simulations running at once |
| `-queue` | 8 | Simulations waiting before requests are rejected |

Deployment: see [deploy/README.md](deploy/README.md).

## Command line

Scores a build file (gcsim character setup lines) against a library team, with 1000 iterations.

```sh
go run ./cmd/gscouter \
  -team teams/hutao-yelan-xingqiu-zhongli \
  -build teams/hutao-yelan-xingqiu-zhongli/build.txt
```

```
Team:      Hutao / Yelan / Xingqiu / Zhongli (rotation v1, draft)
Score:     103764 DPS ± 177 (1 s.e.; s.d. 5606)
Rotation:  20.7s from full energy, 1000 iterations, 667ms

Character           DPS   Share    Energy wait
hutao             56451   54.4%        0.00s/m
yelan             36443   35.1%        0.00s/m
xingqiu           10239    9.9%        0.00s/m
zhongli             630    0.6%        0.00s/m
```

- **Score**: team DPS over one rotation, starting with full energy, against a single enemy with infinite HP. This is how community gcsim configs and players on a training dummy measure.
- **Energy wait**: seconds per minute that a character's actions waited for energy. Too little Energy Recharge for the rotation lowers the score.

Flags:

| Flag | Default | Description |
|---|---|---|
| `-team` | | Team library directory |
| `-build` | | Build file |
| `-iter` | 1000 | Iterations |
| `-seed` | 1 | Seed of the first iteration |
| `-dump` | false | Print the assembled gcsim config and exit |

## Team library

Each team has a `team.yaml`, a `rotation.gcsl` and a `build.txt`, all taken from a community gcsim config. Rotations are never written from scratch.

```sh
go run ./cmd/import https://gcsim.app/sh/<key>   # or a gcsim.app/db/<id> link
go run ./cmd/validate                            # every team
go run ./cmd/validate -v -detail teams/<id>      # one team, with a breakdown
```

- `cmd/import` splits the shared config into the source's build (`build.txt`), character params such as Hu Tao's `start_hp=1` (`params` in `team.yaml`) and the rotation. The source's options, enemy and energy lines are replaced by the common conditions. It records the source's DPS as `reference`.
- `cmd/validate` runs each team with its `build.txt` and fails a team that does not reproduce the source's DPS within 5%, has no source, or never finishes its rotation. It warns about action waits and members with almost no damage.

The `add-team` skill (`.claude/skills/add-team/SKILL.md`) describes the workflow.

## Layout

```
cmd/gscouter/       CLI
cmd/server/         website
cmd/import/         team import from gcsim share links
cmd/validate/       team library checks
internal/config/    common simulation conditions and config assembly
internal/enka/      Enka.Network client and converter to gcsim
internal/web/       HTTP handlers and templates
internal/library/   team library loading and matching
internal/score/     team DPS from gcsim runs
internal/share/     gcsim share download and config splitting
teams/<id>/         team.yaml, rotation.gcsl and source build.txt per team
deploy/             VM setup and auto-update
```

## Tests

```sh
go test ./...
```
