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
GOTOOLCHAIN=auto go run ./cmd/harvest -n 64          # keep existing teams
GOTOOLCHAIN=auto go run ./cmd/harvest -n 64 -force   # replace them
```

This reads the community gcsim database at db.kqm.gg. It ranks teams by how many usable entries use them. For each of the top teams it imports the entry with the lowest constellations and refinements that passes validation, trying up to three. Entries are usable when:
- they are accepted by gcsim or a guide;
- they are not random delays or an action priority list;
- they fight one Lv.100 target with 10% resistance;
- their rotation ends.

Teams that fail every try are printed and skipped. To add one particular team, use a share link instead.

## 2. Import (one team)

```sh
GOTOOLCHAIN=auto go run ./cmd/import https://gcsim.app/sh/<key>
# or a database link: https://gcsim.app/db/<id>
```

This writes `teams/<id>/`:

- `build.txt`: the source config's character setup, used for validation.
- `rotation.gcsl`: everything after the setup (`active`, actions,
  conditions), as written in the source.
- `team.yaml`: members, `sources`, `params` (character params such as Hu
  Tao's `start_hp=1`, added to each player's character line) and
  `reference` (the DPS the share reports and its gcsim version).

The source's `options`, `target` and `energy` lines are replaced by the
common conditions (`internal/config`); the import prints them. If they
differ from ours (other enemy level, resistance, several targets, other
energy), expect a different DPS.

Then set `archetype` in `team.yaml` (e.g. `Pyro vaporize hypercarry`).
Leave `status: draft`; only the maintainer promotes a team to
`supported`.

## 3. Validate

```sh
GOTOOLCHAIN=auto go run ./cmd/validate -v teams/<id>
GOTOOLCHAIN=auto go run ./cmd/validate -detail teams/<id>   # actions, damage by source, reactions
```

- The validator runs the rotation once from full energy and compares it
  with the source's `reference.dps`, then loops it (as the site scores
  it) and reports energy and other waits.
- `FAIL` when the one-pass DPS with `build.txt` is more than 5% from the
  source's `reference.dps`, when it has no source, or when the rotation
  never ends.
- A gap usually means the source used other conditions (see the dropped
  lines) or another gcsim version (`reference.gcsim_version` vs. ours in
  `go.mod`). Explain it in `notes`, or find a source that matches.
- The rotation must choose `active <char>;` in its setup and must end.
  Gscouter loops it itself: the setup (`active`, `let`, `fn`, `set_*`)
  runs once and the rest repeats. A source with its own unconditional
  `while 1` loop cannot be used as is.
- Warnings about action waits or a member with almost no damage are
  information; note anything a reviewer should know.

## 4. Finish

`go test ./...` must pass (the library and web tests load every team).
Commit only when asked, with a message such as
`Add Hu Tao / Yelan / Xingqiu / Zhongli team`.
