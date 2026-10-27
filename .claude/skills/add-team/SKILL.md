---
name: add-team
description: Fill the Gscouter team library from the community gcsim database (cmd/harvest) or a single gcsim share link (cmd/import), validate teams against their source's DPS (cmd/validate), or fix an imported team. Use when asked to add, harvest, import or fix library teams or their rotations.
---

# Add a team to the library

Library rotations come only from community gcsim configs. Never write a
rotation from your own idea of the team: a batch written that way scored
a third of the community numbers and was deleted. If there is no source
config for a team, say so and stop.

Run Go commands with `GOTOOLCHAIN=auto` (gcsim needs Go 1.27).

## 1. Harvest (many teams)

```sh
GOTOOLCHAIN=auto go run ./cmd/harvest -n 10000 -cache /tmp/kqm.json   # every team, keep existing ones
GOTOOLCHAIN=auto go run ./cmd/harvest -n 64 -force                    # replace them
```

This reads the community gcsim database at db.kqm.gg (`-cache <file>` saves the download and reuses it). It ranks teams by how many entries use them. Every entry is usable except those without four characters, with a rotation that never ends on its own (`while 1`) or without an `active` setup; gcsim tags, enemies and energy do not matter. For each team it tries the entries from the lowest constellations and refinements up. An entry must pass the reference check (`validate.CheckReference`: one pass under the entry's own conditions, 20 iterations, within 15% of the database's DPS) and the full check (the looped rotation must not stall); of the first five that pass (`-choose`), it imports the one whose score keeps the most of its one pass, as `supported` if it meets the criteria in step 3 and as `draft` otherwise. Teams run in parallel (`-j`, one per CPU by default).

Teams that fail every entry are written to `teams/harvest-failed.txt` and skipped by the next run; `-retry-failed` tries them again. Run `cmd/validate -j 4` afterwards as a final check. To add one particular team, use a share link instead.

## 2. Import (one team)

```sh
GOTOOLCHAIN=auto go run ./cmd/import https://gcsim.app/sh/<key>
# or a database link: https://gcsim.app/db/<id>
```

This writes `teams/<id>/`:

- `build.txt`: the source config's character setup, used for validation.
- `source.txt`: the source config's `options`, `target`, `energy` and
  `hurt` lines, used only for the reference check.
- `rotation.gcsl`: everything after the setup (`active`, actions,
  conditions), as written in the source.
- `team.yaml`: members, `sources`, `params` (character params such as Hu
  Tao's `start_hp=1`, added to each player's character line) and
  `reference` (the DPS the share reports and its gcsim version).

The site replaces the source's `options`, `target` and `energy` lines
with the common conditions (`internal/config`); the import prints them.
If they differ from ours (other enemy level, resistance, several
targets, other energy), the site's DPS differs from the source's, but
validation still compares like with like.

Then set `archetype` in `team.yaml` (e.g. `Pyro vaporize hypercarry`).
Leave `status: draft`; `cmd/validate -promote` sets `supported` once the
team meets the criteria (step 3).

## 3. Validate

```sh
GOTOOLCHAIN=auto go run ./cmd/validate -v teams/<id>
GOTOOLCHAIN=auto go run ./cmd/validate -detail teams/<id>   # actions, damage by source, reactions
```

- The validator runs the rotation once from full energy under
  `source.txt` (the common conditions if the team has none) and compares
  it with the source's `reference.dps`, then loops it under the common
  conditions (as the site scores it) and reports energy and other waits.
- `FAIL` when the one-pass DPS with `build.txt` is more than 15% from the
  source's `reference.dps` (`WARN` above 5%), when it has no source, when
  the rotation never ends, or when the looped rotation stops advancing
  (e.g. `while rotations { ... }` with the counter set once in the
  setup). Such a team cannot be served: remove it.
- With several teams, `cmd/validate` checks each in its own process
  (`-j 4` runs four at a time).
- A gap usually means another gcsim version (`reference.gcsim_version`
  vs. ours in `go.mod`) or a random rotation. Explain it in `notes`, or
  find a source that matches.
- CI fully validates the team directories a push changes (a random 30
  of them when it changes more, so validate a harvest in full locally),
  or 10 random teams when it changes none.
- The rotation must choose `active <char>;` in its setup and must end.
  Gscouter loops it itself: the setup (`active`, `let`, `fn`, `set_*`)
  runs once and the rest repeats. A source with its own unconditional
  `while 1` loop cannot be used as is.
- Warnings about action waits or a member with almost no damage are
  information; note anything a reviewer should know.
- `-promote` sets each team's status from the result: `supported` when
  the check passes, the one pass is within 5% of `reference.dps` and the
  looped score keeps at least 80% of the one pass under the common
  conditions (`validate.Report.Unpromotable`), `draft` otherwise.
  Result pages warn on draft teams. Blocked teams are left alone. Run it
  on every team you add, and on the whole library after a gcsim update:

  ```sh
  GOTOOLCHAIN=auto go run ./cmd/validate -promote teams/<id>
  GOTOOLCHAIN=auto go run ./cmd/validate -promote -iter 20 -j 4   # every team
  ```

## 4. Finish

`go test ./...` must pass (the library and web tests load every team).
Commit only when asked, with a message such as
`Add Hu Tao / Yelan / Xingqiu / Zhongli team`.
