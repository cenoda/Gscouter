---
name: add-team
description: Add a team to the Gscouter team library (teams/<id>/ with team.yaml, rotation.gcsl and a reference build.txt) and validate it with cmd/validate. Use when asked to add, write or fix a library team or its rotation.
---

# Add a team to the library

A library team is four characters, a gcsim rotation that loops forever,
and a reference build used only to check that the rotation works. Read
`docs/design.md` §3 (scoring conditions) and §4 (library) once if you
have not.

Run Go commands with `GOTOOLCHAIN=auto` (gcsim needs Go 1.27). The gcsim
source is the pinned module:

```sh
GCSIM=$(GOTOOLCHAIN=auto go list -m -f '{{.Dir}}' github.com/genshinsim/gcsim)
```

## 1. Check the characters

- Implemented characters: `ls $GCSIM/internal/characters`. All Traveler
  elements live in the `traveler` directory; in configs the Traveler is
  `aether<element>` (e.g. `aetheranemo`).
- If a member is not implemented, the team is `blocked`: write only
  `team.yaml` with `rotation_version: 0` and a note naming the missing
  character, then stop.

## 2. Name the team

- `members`: gcsim keys, main damage dealer first. The keys are the
  directory names under `$GCSIM/internal/characters`, except the Traveler
  (`aether<element>`).
- `id` and the directory name: the members joined by `-`, in the same
  order, e.g. `hutao-xingqiu-yelan-zhongli`.
- Check that no team with the same four members exists:
  `grep -l <char> teams/*/team.yaml`.

## 3. team.yaml

```yaml
id: hutao-xingqiu-yelan-zhongli
name: Hu Tao / Xingqiu / Yelan / Zhongli
members: [hutao, xingqiu, yelan, zhongli]
archetype: Pyro vaporize hypercarry
status: draft
rotation_version: 1
gcsim_commit: 2350be9
sources: []
notes: "One line on the rotation and anything a reviewer should know."
```

- `status` is always `draft` for new rotations. Only the maintainer
  promotes a team to `supported`.
- `sources`: URLs of community rotations or gcsim configs you based the
  rotation on, if any.

## 4. rotation.gcsl

The rotation is the part of a gcsim config after the character setup:
`active <char>;` then an infinite loop. The common conditions (enemy,
energy, duration) are added by the program; do not write `options`,
`target` or `energy` lines.

```
active xingqiu;
while 1 {
  xingqiu skill;
  if .xingqiu.burst.ready { xingqiu burst; }
  if .yelan.burst.ready { yelan burst; }
  yelan skill;
  zhongli skill[hold=1];
  hutao skill;
  while .hutao.status.paramita > 0 {
    hutao attack, charge;
  }
}
```

Write the rotation real players use for the team (the standard community
rotation), not one tuned to the simulator. Keep it simple and robust:

- gcsim waits until an action is ready. A fixed sequence that waits for
  burst energy stalls the whole team, so use
  `if .<char>.burst.ready { <char> burst; }` where players would skip a
  burst, as above.
- Loop on the carry's state where it is natural, as above or the Skirk
  rotation (`while .skirk.status.seven-phase-flash > 0 { skirk attack; }`).
  Status names are in the character's source (`AddStatus("<name>", ...)`
  or the key constants).
- Prefer `<char> attack:4;` style repeats where they read better.

Reference for syntax, actions, params and fields:

- Config language: `$GCSIM/ui/packages/docs/docs/reference/config.md`
  and `system_functions.md` in the same directory.
- Legal actions per character: `$GCSIM/ui/packages/docs/src/components/Actions/character.dm.json`
- Action params (e.g. `skill[hold=1]`): `.../components/Params/character.dm.json`
- Fields (e.g. `.<char>.status.<name>`): `.../components/Fields/character.dm.json`
- When the docs are not enough, read the character source in
  `$GCSIM/internal/characters/<name>/`.
- Existing rotations in `teams/*/rotation.gcsl`.

## 5. build.txt

A reference build of a well-built account, only for validation. Write
gcsim character setup lines; no other lines.

```
hutao char lvl=90/90 cons=0 talent=9,9,9;
hutao add weapon="staffofhoma" refine=1 lvl=90/90;
hutao add set="crimsonwitchofflames" count=4;
hutao add stats hp=4780 atk=311 hp%=0.565 pyro%=0.466 cd=0.886 cr=0.33 em=79 er=0.11;
```

- Level 90/90, talents 9,9,9 (1,9,9 for characters who never use normal
  attacks), 5-star characters C0, 4-star characters C6.
- Weapons: the main damage dealer gets their usual 5-star (signature) at
  R1; supports get common 4-star weapons (Favonius, Sacrificial, craftable
  R5) unless the team is defined by a 5-star support weapon.
- Artifacts: usual 4-piece set; main stats flower `hp=4780`, feather
  `atk=311`, sands/goblet/circlet as players use (atk%/hp% 0.466,
  er 0.518, em 186.5, elemental% 0.466, phys% 0.583, cr 0.311, cd 0.622,
  heal 0.359). Substats about 20 useful rolls: cr 0.033, cd 0.066,
  atk%/hp% 0.0496, def% 0.062, er 0.0551, em 19.82 per roll.
- Give each character enough Energy Recharge for the rotation, as
  players would: the validator warns at 2 s/min of energy wait.
- Weapon and set keys: `$GCSIM/internal/weapons/` and
  `$GCSIM/internal/artifacts/` directory names, or the shortcut tables in
  `$GCSIM/pkg/shortcut/`.

## 6. Validate

```sh
GOTOOLCHAIN=auto go run ./cmd/validate -v teams/<id>
```

Add `-detail` to see what each character did in one run: actions,
damage by source and reactions per minute. Use it to find a stalled loop
(few actions per minute), missing reactions or an action that does
nothing.

- `FAIL` must be fixed: the config does not parse, a member deals no
  damage, or the rotation does not loop steadily (drift over 10%).
- `WARN` should be fixed. Waits are listed by reason:
  `insufficient_energy` (add ER to the build or make the burst
  conditional), `insufficient_stamina` (use jump or dash cancels as
  players do), `skill_cd`/`burst_cd` (the loop is shorter than the
  cooldown; reorder or make it conditional). If a warning is inherent to the team (for example a
  support with almost no personal damage), say so in `notes`.
- Sanity-check the numbers: the main damage dealer should have the
  largest share in a hypercarry, and team DPS should be plausible next
  to similar teams already in the library.
- Simulation errors name the line and the character; check the action
  against the legal actions and the character's source.

## 7. Finish

`go test ./...` must still pass (the library test loads every team).
Commit only when asked, one commit per team or batch, with a message
such as `Add Hu Tao / Xingqiu / Yelan / Zhongli team`.
