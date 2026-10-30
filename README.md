# Gscouter

Genshin Impact team DPS calculator: settles "whose team is better built?" by scoring every team under the same conditions with a fixed rotation. See [docs/design.md](docs/design.md) for the design and [docs/README.md](docs/README.md) for the other documents.

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
| `-iter` | 50 | Iterations per score |
| `-warmup` | 150 | Seconds of the looped fight excluded from the score |
| `-duration` | 450 | Length of the looped fight in seconds |
| `-concurrent` | 1 | Simulations running at once |
| `-queue` | 8 | Simulations waiting before requests are rejected |
| `-static` | `$GSCOUTER_STATIC` | Directory of the browser build; when set, visitors' browsers calculate results |
| `-links` | `$GSCOUTER_LINKS` | SQLite file of short links (`/s/...`); without it (or `none`), copying a result copies its long link |
| `-site` | `$GSCOUTER_SITE` | The site's public URL (e.g. `https://gscouter.example`) for canonical links, sitemaps and IndexNow; without it, they take each request's host |
| `-indexnow` | `$GSCOUTER_INDEXNOW` | State file of [IndexNow](https://www.indexnow.org) submissions (needs `-site`); without it (or `none`), nothing is submitted |
| `-usage` | `$GSCOUTER_USAGE` | JSON file of daily usage counts (front page and result page views, distinct UIDs; counts only, see design §4.6); without it (or `none`), nothing is counted |

Search engines index the library's pages: the front page, `/teams`, a page per team (`/teams/<id>`) and per character (`/characters/<key>`), each in every site language (`?hl=<code>`; English has none). `/robots.txt` keeps crawlers off results and points them to `/sitemap.xml`, which lists a sitemap per language. With `-indexnow`, the server creates a key on first start and, a minute after each start, submits the pages whose library data changed since the last submission to IndexNow (Bing, Naver, Yandex and others): every page the first time, then only what a library update changed.

Without `-static`, the server calculates every result. To have browsers calculate them, as the Docker image does:

```sh
mkdir -p static
GOOS=js GOARCH=wasm go build -o static/gscouter.wasm ./cmd/wasm
gzip -9 -k static/gscouter.wasm
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" static/
go run ./cmd/server -static static
```

Deployment: see [deploy/README.md](deploy/README.md).

## Command line

Scores a build file (gcsim character setup lines) against a library team, with 1000 iterations.

```sh
go run ./cmd/gscouter \
  -team teams/hutao-xingqiu-zhongli-yelan \
  -build teams/hutao-xingqiu-zhongli-yelan/build.txt
```

```
Team:      Hutao / Xingqiu / Zhongli / Yelan (rotation v1, supported)
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
- **Fillers**: when the looped rotation makes a character stand still (1 s/min or more) waiting for burst energy or a skill cooldown, the rotation is run again with that time handled, and the best run is the score. A burst energy wait is either filled (with the character's own skill or attacks, or with another member's skill and an immediate swap back) or the burst is passed when it is not ready, as Hu Tao teams play, and the way out for Skirk's opening burst, which a loop cannot charge again; each character gets its own choice. Skill cooldown waits are filled with attacks. A rotation's own loop that attacks until a burst is ready (`while !.xingqiu.burst.ready { xingqiu attack; }`) is not a wait to gcsim; it is bounded to two attacks so that the wait shows, and the rotation as written stays a candidate. Skills are used as the rotation writes them (hold or tap, following plunges). Every run that still leaves characters standing still is followed by runs that handle them, up to 16 runs per score (`internal/steady`).
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

Each team has a `team.yaml`, a `rotation.gcsl`, a `build.txt` and a `source.txt` (the source's enemy, energy and options lines), taken from a community gcsim config. A team the community database has no entry for gets a rotation written from a published community guide instead, marked experimental (design §5.4); no rotation is written without a source.

```sh
go run ./cmd/harvest -n 10000 -cache /tmp/kqm.json   # every team in db.kqm.gg, saving the download
go run ./cmd/import https://gcsim.app/sh/<key>   # one team from a share (or gcsim.app/db/<id>) link
go run ./cmd/variant -char citlali -cons 6 teams/*citlali*   # variants without Citlali's burst, kept where they score higher at C6
go run ./cmd/validate                            # every team
go run ./cmd/validate -v -detail teams/<id>      # one team, with a breakdown
go run ./cmd/validate -iter 20 -j 4 -shard 1/2 -json a1.jsonl   # half the library, reports as JSON
go run ./cmd/compare 'a*.jsonl' 'b*.jsonl'       # two runs, field by field (the regression run)
go run ./cmd/weights -v teams/<id>              # artifact score weights of one team
go run ./cmd/weights -join teams/weights.json -base teams/weights.json 'w*.jsonl'   # shards into the table
```

- `cmd/harvest` downloads the community gcsim database at db.kqm.gg and ranks teams by how many entries use them. Every entry with four characters and a rotation that ends (no unconditional `while 1`) is usable, whatever its enemies, energy or gcsim tags. For each team it tries the entries from the lowest constellations and refinements up. An entry must reproduce the database's DPS within 15% in one pass under its own conditions, and its looped rotation must not stall (20 iterations); of the first five that pass (`-choose`), it keeps the one whose score keeps the most of its one pass. Flags: `-j` teams in parallel (default: one per CPU), `-cache <file>` saves the download and reuses it, `-min` least entries per team (1), `-tries` entries per team (0, all), `-force` replaces teams already in the library. Each team harvested also gets its alternatives (`teams/<id>/alt/<entry>/`): the database's rotations for higher constellations than its own, which the score tries besides it and keeps the best (`docs/design.md` §5.2); `-alternatives` redoes only those, for every team already in the library. Teams that fail every entry are listed in `teams/harvest-failed.txt` and skipped by later runs unless `-retry-failed` is set.
- `cmd/variant` derives variants of teams' rotations (`docs/cycle.md` §7.2): the team's rotation without a member's bursts, scored against it with the team's build at a higher constellation of that member, and written to `teams/<id>/alt/<char>-c<cons>-no-burst/` as a draft alternative when it scores at least `-min` (1%) more; `cmd/validate -promote` then checks it. The score tries a variant only for builds with that constellation.
- `cmd/import` splits one shared config into the source's build (`build.txt`), character params such as Hu Tao's `start_hp=1` (`params` in `team.yaml`) and the rotation. The source's options, enemy and energy lines go to `source.txt`; the site replaces them with the common conditions. It records the source's DPS as `reference`.
- `cmd/validate` runs each team with its `build.txt`. One pass of the rotation under the source's conditions (`source.txt`; enemy count, level, resistance, energy) must reproduce the source's DPS within 15% (a warning above 5%). It then scores the looped rotation as the site does, and warns about energy and other waits and about members with almost no damage. A team with no source, whose rotation never finishes, or whose looped rotation stops advancing (a loop with no action, such as a counter set once before it) fails. With several teams, each is checked in its own process, `-j` at a time, because gcsim cannot cancel a stuck rotation. Simulation results are cached by config, options and gcsim version (`-cache`, by default under the user's cache directory; `-cache ""` turns it off), so a rerun simulates only what changed. `-shard k/n` checks every n-th team from the k-th on and `-json <file>` writes the reports, so a run can be split across machines; `cmd/compare` compares two runs, each side a file or a glob of shards.
- `cmd/weights` calculates the artifact score's weights (`docs/artifacts.md` §4–§5): per character, the team DPS one maximum roll of each substat gains over the reference build and the DPS with each main stat on sands, goblet and circlet, all on the reference's seeds and fillers, 20 iterations. It takes `-shard`, `-json`, `-j` and `-cache` as `cmd/validate` does, and `-limit n` for n teams spread over the library; `-join` merges shards into `teams/weights.json`. The `weights` workflow runs it on GitHub's runners and pushes the table to a new branch.

The `add-team` skill (`.claude/skills/add-team/SKILL.md`) describes the workflow.

## Layout

```
cmd/gscouter/       CLI
cmd/server/         website
cmd/wasm/           browser build: calculates results in visitors' browsers
cmd/harvest/        team library harvest from the community gcsim database
cmd/variant/        variants of team rotations for higher constellations
cmd/import/         team import from gcsim share links
cmd/validate/       team library checks
cmd/compare/        comparison of two validation runs
cmd/weights/        artifact score weights per library team
internal/config/    common simulation conditions and config assembly
internal/enka/      Enka.Network client and converter to gcsim
internal/kqm/       community gcsim database (db.kqm.gg) client
internal/web/       HTTP handlers and templates
internal/links/     short link store (SQLite)
internal/library/   team library loading and matching
internal/score/     team DPS from gcsim runs
internal/steady/    looped scoring with fillers for waits
internal/share/     gcsim share download, config splitting and team files
internal/validate/  team checks shared by cmd/validate and cmd/harvest
internal/weights/   artifact score weights: build.txt variants, arithmetic, table
internal/batch/     team listing, shards and child processes for library-wide commands
teams/<id>/         team.yaml, rotation.gcsl, source build.txt and source.txt per team
deploy/             VM setup and auto-update
```

## Tests

```sh
go test ./...
```

## License

GNU AGPL v3 or later (`LICENSE`), as gcsim is. Anyone who runs a modified Gscouter as a website must offer its visitors the modified source; the site links this repository from every page. The license covers Gscouter's own code and data; the other dependencies keep their own licenses, and the game images the site loads from Enka.Network are © HoYoverse.
