# Gscouter

Genshin Impact team DPS calculator: settles "whose team is better built?" by scoring every team under the same conditions with a fixed rotation. See [docs/design.md](docs/design.md) for the design.

Status: prototype. The website looks up a UID on Enka.Network, lets you pick four showcase characters (or one of the library teams the showcase can build), and scores or compares teams. Results and comparisons have shareable links; `/teams` lists the library and finds teams by character. Character face icons are HoYoverse's, loaded from Enka.Network (`enka.network/ui/<icon>.png`, the icon name from gcsim's character data); element icons are drawn for the site. Gscouter is a non-commercial fan project with no ads or donations, which HoYoverse's fan content guidance allows.

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
| `-warmup` | 150 | Seconds of the looped fight excluded from the score |
| `-duration` | 450 | Length of the looped fight in seconds |
| `-concurrent` | 1 | Simulations running at once |
| `-queue` | 8 | Simulations waiting before requests are rejected |

Deployment: see [deploy/README.md](deploy/README.md).

## Command line

Scores a build file (gcsim character setup lines) against a library team, with 1000 iterations.

```sh
go run ./cmd/gscouter \
  -team teams/hutao-xingqiu-zhongli-yelan \
  -build teams/hutao-xingqiu-zhongli-yelan/build.txt
```

```
Team:      Hutao / Xingqiu / Zhongli / Yelan (rotation v1, draft)
Score:     72363 DPS ± 77 (1 s.e.; s.d. 2430)
Window:    150s–450s of the looped rotation, 1000 iterations, 9.882s
One pass:  67851 DPS over 87.5s from full energy (community gcsim measure)

Character           DPS   Share    Energy wait
hutao             45212   62.5%        0.00s/m
xingqiu            9823   13.6%        0.00s/m
zhongli            1473    2.0%        0.00s/m
yelan             15855   21.9%        0.00s/m
```

- **Score**: team DPS with the community rotation repeated endlessly, over 150–450 s once the starting energy has run out. Too little Energy Recharge shows up as energy waits and a lower score.
- **Fillers**: when the looped rotation makes a character stand still (1 s/min or more) waiting for burst energy or a skill cooldown, the rotation is run again with that time filled, and the best run is the score. Burst energy waits are filled with the character's own skill or attacks, or with another member's skill and an immediate swap back. Skill cooldown waits are filled with attacks. Skills are used as the rotation writes them (hold or tap, following plunges). Up to three rounds add characters that still stand still (`internal/steady`).
- **One pass**: the rotation once from full energy, as community gcsim configs report it. Validation compares this with the source.
- **Energy wait**: seconds per minute that a character's actions waited for energy. Too little Energy Recharge for the rotation lowers the score.

Flags:

| Flag | Default | Description |
|---|---|---|
| `-team` | | Team library directory |
| `-build` | | Build file |
| `-iter` | 1000 | Iterations |
| `-warmup` | 150 | Seconds of the looped fight excluded from the score |
| `-duration` | 450 | Length of the looped fight in seconds |
| `-seed` | 1 | Seed of the first iteration |
| `-dump` | false | Print the assembled gcsim config and exit |

## Team library

Each team has a `team.yaml`, a `rotation.gcsl`, a `build.txt` and a `source.txt` (the source's enemy, energy and options lines), all taken from a community gcsim config. Rotations are never written from scratch.

```sh
go run ./cmd/harvest -n 10000 -cache /tmp/kqm.json   # every team in db.kqm.gg, saving the download
go run ./cmd/import https://gcsim.app/sh/<key>   # one team from a share (or gcsim.app/db/<id>) link
go run ./cmd/validate                            # every team
go run ./cmd/validate -v -detail teams/<id>      # one team, with a breakdown
```

- `cmd/harvest` downloads the community gcsim database at db.kqm.gg and ranks teams by how many entries use them. Every entry with four characters and a rotation that ends (no unconditional `while 1`) is usable, whatever its enemies, energy or gcsim tags. For each team it tries the entries from the lowest constellations and refinements up and keeps the first whose one pass, under the entry's own conditions, is within 15% of the database's DPS (20 iterations). Flags: `-j` teams in parallel (default: one per CPU), `-cache <file>` saves the download and reuses it, `-min` least entries per team (1), `-tries` entries per team (0, all), `-force` replaces teams already in the library. Teams that fail every entry are listed in `teams/harvest-failed.txt` and skipped by later runs unless `-retry-failed` is set.
- `cmd/import` splits one shared config into the source's build (`build.txt`), character params such as Hu Tao's `start_hp=1` (`params` in `team.yaml`) and the rotation. The source's options, enemy and energy lines go to `source.txt`; the site replaces them with the common conditions. It records the source's DPS as `reference`.
- `cmd/validate` runs each team with its `build.txt`. One pass of the rotation under the source's conditions (`source.txt`; enemy count, level, resistance, energy) must reproduce the source's DPS within 15% (a warning above 5%). It then scores the looped rotation as the site does, and warns about energy and other waits and about members with almost no damage. A team with no source, whose rotation never finishes, or whose looped rotation stops advancing (a loop with no action, such as a counter set once before it) fails. With several teams, each is checked in its own process, `-j` at a time, because gcsim cannot cancel a stuck rotation.

The `add-team` skill (`.claude/skills/add-team/SKILL.md`) describes the workflow.

## Layout

```
cmd/gscouter/       CLI
cmd/server/         website
cmd/harvest/        team library harvest from the community gcsim database
cmd/import/         team import from gcsim share links
cmd/validate/       team library checks
internal/config/    common simulation conditions and config assembly
internal/enka/      Enka.Network client and converter to gcsim
internal/kqm/       community gcsim database (db.kqm.gg) client
internal/web/       HTTP handlers and templates
internal/library/   team library loading and matching
internal/score/     team DPS from gcsim runs
internal/steady/    looped scoring with fillers for waits
internal/share/     gcsim share download, config splitting and team files
internal/validate/  team checks shared by cmd/validate and cmd/harvest
teams/<id>/         team.yaml, rotation.gcsl, source build.txt and source.txt per team
deploy/             VM setup and auto-update
```

## Tests

```sh
go test ./...
```
