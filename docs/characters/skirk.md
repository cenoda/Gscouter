# Skirk

> Status: Investigation; the zero score of §4 fixed, the rest open (§6.1) · Last updated: 2026-10-29

How the library's rotations score a C6 Skirk, after a community report that her constellation "doesn't count". Skirk is in upstream gcsim; this document records measurements, not a fork implementation. Roadmap context: `docs/cycle.md` §8–§9.

## 1. C6 in gcsim

_2026-10-29_, fork `b029c45` (`internal/characters/skirk`; unchanged in `eb21884`).

- Constellations reach gcsim (`internal/enka` sets them from `talentIdList`).
- **Void Rifts** come from Cryo reactions (Frozen, Superconduct, Cryo Swirl, Cryo Crystallize; one per 2.5 s, at most 3, 17.6 s). Skirk absorbs them with a held skill, with Havoc: Extinction and with her charged attack in Seven-Phase Flash. Each absorbed rift gives 8 Serpent's Subtlety and, at C1, a Far to Fall hit.
- **C6** gives a Havoc: Sever stack per absorbed rift (at most 3, 15 s each). N3 and N5 in Seven-Phase Flash spend one stack for 3×180% ATK ("Havoc: Sever (Normal)"). Havoc: Ruin, her burst outside Seven-Phase Flash, spends every stack for 750% ATK each ("Havoc: Sever (Burst)"). Inside Seven-Phase Flash the burst is Havoc: Extinction, which spends none.
- Ruin needs 50 Serpent's Subtlety, and leaving Seven-Phase Flash drains it to 0. In a loop, Skirk reaches 50 outside the mode only with a held skill (45, plus 8 per absorbed rift). A rotation that opens with Ruin from her full starting 100 never has it again, and the `pass` filler skips that burst (`docs/fillers.md` §2).
- The held skill's Serpent's Subtlety arrives 18 frames after the action starts. A rotation that tests `.skirk.serpents_subtlety` or `.skirk.burst.ready` right after `skirk skill[hold=1];` reads the old value. The diagnostic in §4 waits 25 frames first.

## 2. Method

Every score in this document is measured the way the site scores a build: `steady.Best` over the team's own rotation and its supported alternatives, with fillers, a 450 s fight with the first 150 s left out, 50 iterations to confirm and 10 to rank, seed 1. A scratch tool, not committed, ran the same pipeline and then ran the chosen looped config again for 50 iterations, adding Skirk's damage by ability from gcsim's damage events between 150 s and 450 s. Rifts absorbed are counted as Far to Fall (C1) hits, and Sever (Normal) procs as its hits / 3. The code was at main `de5477c3` (fork `b029c45`). Main then moved to fork `eb21884`, whose regression runs moved only Vesna and Wriothesley teams (`docs/fork.md`). On it, both Wriothesley / Skirk teams, Skirk / Furina / Shenhe / Escoffier and Skirk / Furina / Citlali / Mika give the same scores to the DPS, at C0 and at C6.

## 3. The two all-C6 alternatives

Of the 150 Skirk teams, two have an alternative written for an all-C6 team: Skirk / Furina / Shenhe / Escoffier (`alt/dJjdJBRLgqD8`) and Skirk / Furina / Citlali / Escoffier (`alt/6JqcdtfQjGbz`). Both alternatives open each pass with Ruin from full Serpent's Subtlety (`if r == 0 { skirk burst, skill, ... }`), and that branch has no Extinction. In the loop the opening Ruin is skipped (`bursts of skirk skipped when short of energy`). **Havoc: Ruin never fires in the scored window, and no stack is ever spent at 750%.** The library rotations play Ruin the same way, and it never fires there either.

Team DPS for each rotation and what `steady.Best` picks:

| Team | Build | Own rotation | Alternative | Site scores with |
|---|---|---|---|---|
| Shenhe | the alternative's (all C6, R5 weapons) | 729.9k | **866.7k** | alternative, +18.7% |
| Shenhe | the alternative's, teammates C0 | 323.4k | **394.6k** | alternative, +22.0% |
| Shenhe | library, only Skirk C6 | 243.5k | **263.6k** | alternative, +8.2% |
| Shenhe | library (Skirk C0, Shenhe C1) | 137.0k | **148.4k** | alternative, +8.3% |
| Citlali | the alternative's (all C6) | 715.6k | **817.3k** | alternative, +14.2% |
| Citlali | the alternative's, teammates C0 | **435.4k** | 422.5k | own rotation (alternative −3.0%) |
| Citlali | library, only Skirk C6 | **267.8k** | 266.2k | own rotation (alternative −0.6%) |
| Citlali | library (all C0) | 140.2k | **143.0k** | alternative, +2.0% |

- A C6 Skirk player gets the alternative on the Shenhe team, with any teammates. On the Citlali team they get it only when Citlali or Furina is high too: with C0 teammates the team's own rotation wins by 0.6–3%, and `steady.Best` keeps it.
- The 137.0k → 243.2k of the first look was the team's own rotation. With the alternative the site scores the Shenhe team 148.4k (Skirk C0) and 263.6k (Skirk C6).

Skirk's damage by ability over the scored window (DPS). Normal attacks include Extinction's bonus, which is a DMG% buff on them:

| | Shenhe, alt. build, alternative | Shenhe, alt. build, own | Shenhe, library + C6 (site: alternative) | Citlali, alt. build, alternative | Citlali, alt. build, own | Citlali, library + C6 (site: own) |
|---|---|---|---|---|---|---|
| Normal attacks (Seven-Phase Flash) | 525.7k | 455.0k | 165.9k | 471.8k | 424.0k | 174.6k |
| Charged attacks | 18.0k | 11.0k | 3.6k | 10.8k | 7.4k | 3.0k |
| Havoc: Sever (Normal) | 113.2k | 91.0k | 33.7k | 94.0k | 88.7k | 37.6k |
| Far to Fall (C1) | 76.8k | 53.6k | 22.9k | 79.1k | 67.3k | 24.3k |
| Havoc: Ruin | 0 | 0 | 0 | 0 | 0 | 0 |
| Havoc: Sever (Burst) | 0 | 0 | 0 | 0 | 0 | 0 |
| **Skirk** | 733.7k | 610.6k | 226.1k | 655.7k | 587.4k | 239.4k |
| Team | 866.7k | 729.9k | 263.6k | 817.3k | 715.6k | 267.8k |
| Extinctions per 300 s | 10.0 | 13.0 | 10.2 | 10.6 | 15.0 | 15.0 |
| Rifts absorbed / Sever (Normal) procs per 300 s | 82 / 50 | 63 / 41 | 77 / 48 | 81 / 51 | 83 / 54 | 79 / 50 |

- The stacks are not idle without Ruin. N3 and N5 spend them, and Sever (Normal) is 12–14% of team DPS. About 60–65% of absorbed rifts become a Sever (Normal) proc. The rest expire, are overwritten at 3 stacks, or are popped while the proc is on its 0.2 s cooldown (gcsim pops the stack before it checks that cooldown).
- The alternatives win by playing more N3–dash strings and charged attacks: more normal-attack damage, more rifts absorbed and more Sever procs. They do not win by playing Ruin.

## 4. All Skirk teams

The 150 library teams with Skirk, each with its library build and Skirk raised to C6, scored as in §2:

| How the rotation plays Skirk | Teams | Ruin per 300 s (scored window) | Stacks per Ruin | Score C6 / C0, median |
|---|---|---|---|---|
| On field, bursts only in Seven-Phase Flash | 131 | 0 | — | 1.82× (1.18–1.99) |
| Held skill and Ruin every rotation, Cryo reactions in the team | 16 | 9–19 | about 3 (Navia / Xilonen / Escoffier 0.2) | 1.64× |
| Held skill and Ruin every rotation, no Cryo reaction (Arlecchino / Citlali, Mavuika / Citlali, Dehya / Escoffier, all with Bennett) | 3 | 12–16 | 0 | 1.22× |

- **131 teams score a C6 Skirk with Ruin never spending stacks.** In their library rotations Skirk bursts only inside Seven-Phase Flash, and 124 of them open with a Ruin the loop skips. Their C6 still counts through Sever (Normal): median 13% of team DPS.
- In the 16 sub-DPS teams (held skill, Ruin each rotation) C6 counts in full: Sever (Burst) is a median 13.6% of team DPS. Skirk / Furina / Mona / Escoffier goes 96.1k → 200.3k.
- The 3 Pyro teams make no Cryo reaction, so no rift forms and C6 has nothing to spend. That is the game's rule, not a rotation problem.
- **Two teams score 0 with a C6 Skirk**: Skirk / Furina / Citlali / Mika and Skirk / Furina / Sigewinne / Citlali, which score 88.4k and 94.3k at C0. Their rotation as written stands still (the opening Ruin, §1) and scores 0. The best filled run ranks at 165k / 177k on 10 iterations. Its 50-iteration confirmation then stops on one seed (12 and 17) with gcsim's "skirk: cannot use skill in seven-phase flash", the filler failure of `docs/fillers.md` §2. `steady.Best` confirmed only each rotation's best outcome, so it fell back to the as-written 0, and the site, with the same settings, gave these builds a score of 0. **Fixed** (2026-10-29): when a confirmation fails, `steady.Best` now confirms the rotation's next-best ranked run in its place (`TestBestConfirmsNextRunAfterFailure`). The two builds score 165.9k and 176.9k, which is what `cmd/gscouter` gives when it runs every variant in full. Rerun with the fix, the other 298 builds of these 150 teams (C0 and C6) score the same to the DPS. `cmd/validate` runs every variant in full and was not affected.

**How much DPS is missing.** This was estimated with a temporary variant, in a scratch directory and not committed. Once per loop pass, Skirk uses a held skill, waits 25 frames and casts Ruin if it is ready (`skirk skill[hold=1]; wait(25); if .skirk.burst.ready { skirk burst; }`), then plays that pass's Seven-Phase Flash as written. In the library rotation of the Shenhe team that flash is the opener's, with its Extinction dropped (Ruin's 15 s cooldown). In the other teams it is the `r == 0` flash, whose 0-stack Ruin the variant replaces. The variant was scored alone and compared with the rotation it was made from:

| Team, build | Rotation as is | With a stacked Ruin per loop | Change | Ruin + Sever (Burst) | Stacks per Ruin |
|---|---|---|---|---|---|
| Skirk / Furina / Shenhe / Escoffier, library + Skirk C6, own rotation | 243.5k | 239.6k | −1.6% | 13.7k | 3 |
| the same, the flash replaced by Ruin | 243.5k | 242.0k | −0.6% | 20.8k | 3 |
| Skirk / Furina / Citlali / Escoffier, library + Skirk C6 | 267.8k | 266.7k | −0.4% | 15.1k | 3 |
| Skirk / Durin / Furina / Nicole, library (all C6) | 1,248.6k | 1,191.6k | −4.6% | 33.9k | 1 |
| Skirk / Xingqiu / Mona / Escoffier, library + Skirk C6 | 169.4k | 173.4k | **+2.4%** | 22.2k | 3 |
| Shenhe team, the alternative's build and rotation | 866.7k | 855.3k | −1.3% | 67.2k | 3 |
| Citlali team, the alternative's build and rotation | 817.3k | 793.3k | −2.9% | 67.7k | 3 |

- A stacked Ruin is worth 3–8% of team DPS on its own, but the held skill costs a skill cooldown (8 s) before the next Seven-Phase Flash, and Ruin's 15 s cooldown, shared with Extinction, delays or drops the next Extinction. On 6 of 7 runs the team loses more than Ruin adds. The cost of the held skill comes from gcsim's model; whether players actually weave a Ruin into a C6 rotation was not checked against a guide.
- So the gap behind "C6 doesn't count" is in Seven-Phase Flash play, not in Ruin. A C6 Skirk scores 1.8× her C0 with the C0 rotation. A rotation written for C6 adds another 8–19% on the one team where the database has one (§3).
- Arlecchino / Citlali and the Mona sub-DPS team were not given a variant: they already cast Ruin every rotation, with 0 stacks (no rifts) and about 3 stacks respectively.

## 5. The database

_2026-10-29_, the database as `cmd/harvest` reads it (`kqm.Cached`, 6,021 entries, the same download as `docs/cycle.md` §7): 180 four-member entries have Skirk, over 143 member sets. Skirk is above C0 in only 4 of them, all at C6:

| Entry | Team | Used as |
|---|---|---|
| `dJjdJBRLgqD8` | Skirk / Furina / Shenhe / Escoffier, all C6 | the team's alternative |
| `ndbLjKpHnL8q` | the same team, all C6 | not kept: same constellation group, `dJjdJBRLgqD8` kept more of its one pass |
| `6JqcdtfQjGbz` | Skirk / Furina / Citlali / Escoffier, all C6 | the team's alternative |
| `WHFJgzbdggJf` | Skirk / Durin / Furina / Nicole, all C6 | the team's own entry |

No database entry has Skirk at C1–C5. **`cmd/harvest -alternatives` gives no more teams an alternative for a C6 Skirk today.** It will when the database gains C6-Skirk entries.

## 6. Options for Skirk

The ways of `docs/cycle.md` §8, for Skirk:

| Option | Cost | Score it recovers for a C6 Skirk |
|---|---|---|
| 1. Say on the result page that the rotation assumes C0 Skirk | one constellation comparison per member, a translated note (`docs/cycle.md` §8, 1) | none; it labels the 8–19% that a C6 rotation added on the Shenhe team (§3) |
| 2. More database entries | none in code; a re-run of `cmd/harvest -alternatives` | 0 teams today (§5); later, whatever C6 entries are submitted |
| 3. A C6 alternative written from a guide (design §5.4) | a guide; alternatives without a source DPS (`library.LoadFS`), validation and review; per team, since a rotation names all four members | up to what the database's C6 rotation gave on its own team: +8% (library build, Skirk C6) to +19% (all C6), on Furina / Escoffier teams like it. Not measured elsewhere |
| 4. A Skirk-specific filler or rewrite that casts Ruin when stacks are up | a new filler in `config.Fill` / `steady`, about one more run per Skirk team | −4.6% to +2.4% (§4); kept only where it wins, so +2.4% on one team of four |

Recommendation:

- ~~**Fix the zero score first.**~~ Done (§4).
- **Do not build 4.** A stacked Ruin loses on most teams, and where it wins the gain is small.
- **Do 1**, as `docs/cycle.md` §8 already proposes for every character. It is the only option that reaches all 131 on-field teams, and it states the gap honestly: the score counts C6's damage (1.8×) but plays the C0 rotation.
- **Keep 2 open.** It needs no work, and C6 entries are what Skirk's C6 needs.
- **Do 3 only with a guide** that writes a C6-specific Seven-Phase Flash, and start with the Furina / Escoffier archetype, where most Skirk teams are.

### 6.1 Open problems

_2026-10-29._ Recorded, not fixed:

1. **Ruin never spends C6 stacks on the 131 on-field teams** (§4). Their rotations burst only inside Seven-Phase Flash, and 124 of them open with a Ruin that needs Skirk's full starting Serpent's Subtlety, which the loop skips. Not a score loss as measured (§4), but it is what players see when they read the breakdown.
2. **The score plays the C0 rotation for a C6 Skirk.** Where a C6 rotation exists it adds 8–19% (§3); on the other 148 teams none exists, and the result page does not say so (option 1).
3. **No more C6-Skirk entries in the database** (§5). The alternatives cannot grow until the community submits some.
4. **The Citlali alternative loses with C0 teammates** (−0.6% to −3.0%, §3). `steady.Best` keeps the team's own rotation then, so nothing is lost, but Skirk / Furina / Citlali / Escoffier has no rotation written for a C6 Skirk with C0 teammates.
5. **About 35–40% of absorbed rifts never become a Sever proc** (§3): they expire, are overwritten at 3 stacks, or are popped during the proc's 0.2 s cooldown. Rotations do not plan around the stacks.
6. **The `r == 0` branch of looped rotations plays a Seven-Phase Flash without Extinction**, because its opening Ruin is skipped and nothing takes its place (§3). It applies to the two alternatives and to the library rotations written the same way.
