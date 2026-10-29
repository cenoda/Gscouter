# Cycle

> Status: In progress (§5.1, §7 and §10 implemented) · Last updated: 2026-10-29

Roadmap node Cycle (`docs/roadmap.md`): what the fixed rotation (design §5.2) does with builds other than the one it was written for, and where the score stops being trustworthy because of it. §5.1, §7 and §10 are implemented; §6 proposes the rest, §8 says what teams without an alternative still need, and §10 adds variants of a team's own rotation (Citlali C6 without her burst).

## 1. The report

_2026-10-29._ A community post (arca.live) ran Mavuika / Citlali / Xilonen / Nicole and other teams on the site:

- Citlali's burst is a damage loss for the poster, and the rotation uses it, "probably because the gcsim build was taken as it is";
- Vesna teams score far too low (experimental, `docs/characters/vesna.md`);
- some teams match what players expect and some do not; recent teams mostly do not, older characters' teams mostly do.

The owner's reading: rotations change with constellations, and the site does not follow players with high constellations.

## 2. How a team's rotation is chosen

- `cmd/harvest` tries a team's database entries from the lowest total constellations and refinements up (design §5.3, step 4). One rotation then scores every build (design §5.2).
- The chosen entries are mostly C0 for 5-star characters and C6 for 4-star ones. Over the 4,421 harvested teams, the members of the chosen entries are at C0 10,285 times, at C6 7,027 times and at C1–C5 372 times.
- _2026-10-29_, the database (6,021 entries): 928 of the 4,421 harvested teams have more than one entry, and **217** have an entry with a member at a higher constellation than the chosen one. For the other 4,204 teams no community rotation for a higher-constellation build exists.
- 13 of the 4,569 library rotations branch on a constellation (`.mavuika.cons == 0`, `.arle.cons >= 2`, ...). Every other rotation plays the same actions at C0 and at C6. gcsim does model every constellation's effect, so the damage of a constellation is counted; what does not change is the order of actions a constellation calls for.

## 3. Higher-constellation entries against the library rotation

_2026-10-29._ For each of the 217 teams, its database entry with the most added constellations was imported (`cmd/import`, not validated against its own DPS) and four scores were taken with `cmd/gscouter` (looped score, 50 iterations, fillers as on the site): the library build and the entry's build, each with the library rotation and with the entry's rotation. 21 teams lost a score (the import or a run failed), and 5 more had a run that collapsed under 5k DPS (three from the weapon problem in §5, two Ayaka / Ganyu / Shenhe entries whose rotation scores 0 under the common conditions); 191 remain.

| Build | Entry rotation against library rotation, median | Over +5% | Over +20% | Under −5% |
|---|---|---|---|---|
| The entry's higher-constellation build | **+12.4%** | 136 | 61 | 15 |
| The library's build | **−15.0%** | 15 | 3 | 141 |

- In 103 teams the entry's rotation is more than 5% better for its own build and more than 5% worse for the library's. Neither rotation is right for every build: a rotation fits the constellations it was written for.
- Keeping the better of the two rotations for each build: the higher-constellation build gains a median of 12.4% (mean 20.9%); the library build gains a median of 0.0% (21 of 191 more than 1%).
- By the character whose constellation the entry adds (the entry's build, entry rotation against library rotation; teams counted once per such character):

| Character | Teams | Median | Over +10% |
|---|---|---|---|
| Furina | 66 | +20% | 48 |
| Xilonen | 40 | +17% | 29 |
| Nahida | 25 | +2% | 8 |
| Kazuha | 20 | +7% | 9 |
| Yelan | 20 | +18% | 13 |
| Raiden | 18 | +5% | 5 |
| Citlali | 16 | +27% | 13 |
| Hu Tao | 16 | +17% | 11 |
| Mavuika | 12 | +13% | 7 |
| Neuvillette | 8 | +35% | 8 |

- The largest single cases come from a constellation that changes a character's role. Furina C6 makes her an on-field Hydro attacker: on Bennett / Xiangling / Furina / Kazuha the C6 entry plays her normal and charged attacks, which the library's C0 rotation never does, and the C6 build scores 174k with it against 85k with the library rotation (+105%). Kazuha / Furina / Nahida / Bennett +156%, Xianyun / Furina / Xiangling / Bennett +115%.

### 3.1 The reported team

Mavuika / Citlali / Xilonen / Nicole: the library rotation is the C0 entry `N9CT6r6b7LGC`. The database also has a C6R5 entry, `npQhNPRFDqhn` (all four C6, signature weapons): Citlali bursts every second rotation, with no Energy Recharge, and Mavuika plays a held skill and dash-cancelled charged attacks. The library build with constellations raised (50 iterations):

| Constellations | Library rotation | C6 entry's rotation | |
|---|---|---|---|
| all C0 | 118.0k | 76.2k | −35.5% |
| Mavuika C2 | 178.7k | 125.9k | −29.5% |
| Mavuika C6 | 264.2k | 196.9k | −25.5% |
| Citlali C6 | 161.6k | 119.6k | −26.0% |
| Mavuika C2, Citlali C2 | 236.8k | 153.2k | −35.3% |
| Mavuika C6, Citlali C1–C5 | 271.5–348.1k | 201.7–256.7k | −26% to −32% |
| Mavuika C1, Citlali C6 | 177.2k | 221.8k | **+25.2%** |
| Mavuika C2–C5, Citlali C6 | 235.7–283.9k | 297.9–364.0k | **+25.5% to +28.2%** |
| Mavuika C6, Citlali C6 | 341.0k | 433.5k | **+27.1%** |
| all four C6 | 696.8k | 855.0k | **+22.7%** |
| the C6 entry's own build | 1,592k | 1,892k | **+18.8%** |

The C6 rotation pays once Citlali is C6 and Mavuika at least C1, and loses a quarter or more otherwise. Which rotation fits depends on two characters' constellations together, so it is best measured on the player's build rather than decided by a table.

## 4. Citlali's burst

The library rotation with Citlali's burst on every second rotation only (otherwise unchanged), against the rotation as it is, on the builds of §3.1: lower on every build, from −2.7% (Mavuika C6, Citlali C6) to −35.5% (Mavuika C2, Citlali C2); −30.9% on the library build. Without the burst Mavuika herself deals 31% less and nobody waits for energy: in gcsim the burst's Cryo feeds her Melts and Citlali's Pyro RES shred. gcsim does not support "Citlali's burst is a loss" for this rotation. The C6 entry does burst every second rotation, but what makes it better at C6 is the whole rotation, not the skipped burst.

## 5. Rotations that only work with the source's weapon

Found while checking collapsed runs in §3, and a worse problem than constellations because it hits ordinary builds.

- 329 library rotations (325 `supported`) wait in a loop for a Favonius weapon's cooldown: `while !.xilonen.mods.favonius-cd { xilonen attack; }`. The source's build gives that character a Favonius weapon. With any other weapon the mod never appears, the character attacks for the rest of the fight, and the team's score collapses. No energy or cooldown wait shows, so fillers (`docs/fillers.md`) do not see it.
- Yelan / Sigewinne / Furina / Xilonen, the library build with only Xilonen's Favonius Sword changed to a Sacrificial Sword: 74k → 1.3k.
- Every such team, its library build with each looping character's Favonius weapon changed to the Sacrificial weapon of its type (Dragon's Bane for a polearm), 2 iterations, against the library build as it is. 301 of the 329 could be tested (in 28 the swap did not apply: the build names the character by another alias, such as `rosa`, or the loop is already guarded):

| Score with the weapon swapped, of the score as it is | Teams |
|---|---|
| under 20% (median 3.5%, 0.9–19.7%) | 271 |
| 20–80% | 18 |
| 80–95% | 11 |
| 95% or more | 1 |

  The loops that do not collapse have a second way out (`while !.faruzan.mods.favonius-cd && !.faruzan.burst.ready`), and still lose time attacking.
- A few library rotations already guard the read the right way: `if .furina.weapon == .keys.weapon.favsword && ...` (Wanderer / Furina / Faruzan / Bennett).
- Other `while` conditions on mods are on a character's own state (`razor-q` 57, `xingqiuburst` 20, `bennett-field` 17, `yelanburst` 16, ...), which every build has; they were not tested. One rotation reads `sacrificial-cd` the same way, and two misspell the mod (`favnoius-cd`) in an `if`, which costs one action and is the same for every build.

### 5.1 The fix

_2026-10-29._ Implemented.

- `config.Assemble` and `config.AssembleSource` rewrite a rotation's reads of `.X.mods.favonius-cd` and `.X.mods.sacrificial-cd` to `1` (the cooldown always running) for a character whose weapon does not give that mod (`config.GuardWeaponMods`). Character aliases (`xilo`, `rosa`) and weapon shortcuts resolve through gcsim. A build with the weapon gets the config it had, so its score and cache key do not change.
- 600 library rotations read one of these mods. The rewrite leaves the library builds' configs as they were except in 21 teams whose build does not give that character the weapon; all 21 scored as recorded, to the DPS, apart from Razor / Xingqiu / Raiden / Nahida (−0.2%).
- Yelan / Sigewinne / Furina / Xilonen with Xilonen's Favonius Sword swapped out: 1.3k before, 68.1k after (74.2k with it).
- **Validation check**: `cmd/validate` scores every team whose rotation reads a weapon mod once more, as the site does (fillers included), with those weapons swapped for a weapon of the same type without one (`config.SwapModWeapons`: The Alley Flash, Rainslasher, The Stringless, The Widsith, Dragon's Bane). It records the ratio as `SwapKept` in `-json`. Under 20% fails the team, a loop waiting for a passive that never comes; under 50% is a warning.
- The 600 teams (`cmd/validate -iter 20`), 585 with a weapon to swap:

| Score with the weapons swapped, of the score | Teams |
|---|---|
| under 20% | 0 (271 of 301 before the fix, §5) |
| 20–50% | 12 |
| 50–80% | 182 |
| 80–95% | 295 |
| 95% or more | 96 |

  Median 86%. What remains is energy: the swapped weapon gives no particles. A lower score there is the rule of design §2 ("managing ER is part of building"). The 12 under 50% get the warning. Two of them wait for a burst or skill in a loop (Alhaitham / Xingqiu / Yelan / Raiden, Tighnari / Collei / Fischl / Zhongli; §5.2); the other ten were not looked into further.

### 5.2 Loops that wait for a burst

Found while checking the 12 teams above, not fixed. 101 library rotations wait for energy by attacking: `while !.xingqiu.burst.ready { xingqiu attack; }`. gcsim does not count that as waiting for energy, so the fillers (`docs/fillers.md`) never see it, and a build short of Energy Recharge attacks on a support instead of passing the burst as a player would. Alhaitham / Xingqiu / Yelan / Raiden without its Favonius weapons keeps 24% of its score this way. A fix would make these loops visible to the fillers, for instance by turning each into a bounded wait the filler search can replace; it needs its own measurement on the library.

## 6. Proposal

(1)–(4) in order of how much trust each buys for its cost; (5) was found later. (1) and (2) are done; (3) and (4) are what teams without an alternative need (§8); (5) is not decided.

1. ~~**Weapon-dependent loops** (§5).~~ Done (§5.1): weapon mods are read as always on without the weapon, and validation fails a collapse.
2. ~~**Keep the best of a team's rotations.**~~ Done (§7). Harvest keeps, besides the rotation it picks now, the best passing entry for each higher-constellation profile the database has; the score runs the player's build with each and keeps the best, as fillers keep the best run (design §3.1). On the 191 teams measured it raises higher-constellation builds by a median of 12.4% and leaves the library builds where they are. Cost: one more scoring (with its filler search) per extra rotation, on about 217 teams; the result page would say which rotation scored. A rotation could be skipped when the player's constellations are far below its entry's, but §3.1 shows the boundary depends on several members at once, so that needs measuring first.
3. **Say what the rotation assumes.** The result page shows the constellations the rotation was written for, and a note when a member's constellation is higher: "This rotation was written for C0 Furina; a C6 Furina is played differently, and the score may be low." Cheap, and it covers the 4,204 teams with no higher-constellation entry.
4. **Rotations for constellations the database lacks.** For a constellation that changes a role (Furina C6, and the characters of the table in §3), a rotation written from a community guide under the rules of design §5.4, as a second rotation of the team kept by (2). Needed now that (2) exists (§8).
5. **Loops that wait for a burst** (§5.2): make them visible to the fillers.

## 7. Keeping the best rotation

_2026-10-29._ (2) of §6, implemented.

- **Alternatives**: `cmd/harvest` keeps, for each team, the database's rotations for higher constellations than its own entry's in `teams/<id>/alt/<entry>/` (design §5.1). The entries with a member at a higher constellation are grouped by their constellations; each group's entries are tried as the team's are (reference check, full check, the one that keeps most of its one pass among the first five that pass), and the pick is kept when its rotation differs from the team's and it meets the criteria for supported. `cmd/harvest -alternatives` redoes only this for every team in the library.
- The run of 2026-10-29 (database of 6,021 entries, 1 min 49 s on 4 CPUs) wrote **207 alternatives for 191 teams**. 12 picks were not kept for missing the criteria (11 keeping under 80% or over 130% of one pass when looped, 1 more than 5% from the source), 5 groups had no entry that passed, and 1 group's rotation was the team's own.
- **The score** (`steady.Best`, used by the site and `cmd/gscouter`) runs the build with the team's rotation and with each supported alternative, each with its fillers. As before, only the first run of the team's own rotation uses the full iterations and the rest rank with 10; the best ranked run is then confirmed with the full iterations, and if it does not beat the team's own rotation as written, the next is tried, down to that one. An alternative gcsim rejects is left out.
- The result says when an alternative scored, with the constellations it was written for and its source; the library team page lists the alternatives. `cmd/validate` checks each alternative after its team, so CI covers them.
- Mavuika: 15 teams have an alternative, mostly from all-C6 entries, among them the reported Mavuika / Citlali / Xilonen / Nicole (§3.1) with its C6 entry `npQhNPRFDqhn`.
- A rotation that fails to run is left out, the team's own included, as long as another one runs. With a C6 Chasca, the library rotations of most Chasca teams stopped with a gcsim error (§8, now fixed by PR #55); the team's alternative then scored the build instead of the score failing (Chasca / Citlali / Nicole / Durin with its C6 entry's build: an error before, 1.85M with the alternative).

### 7.1 What it changes

_2026-10-29._ For each of the 191 teams, every build it has (the library's and each alternative's own) was scored as the site does (`steady.Best`: fillers, 10 iterations to rank, 50 to confirm; 450 s fight, the first 150 s excluded), once with the team's rotation alone and once with all its rotations. 394 builds; 3 are left out (two C6 Chasca builds whose team rotation fails, now scored as above, and one Traveler build the measuring tool did not rename), and one whose team rotation scores 0 (Mualani / Chiori / Xiangling / Xilonen with its C6 entry's build: 78.9k with the alternative).

| Build | Builds | Median | Mean | Over +5% | Over +20% | Under −1% | Alternative scored |
|---|---|---|---|---|---|---|---|
| The library's | 191 | **+0.0%** | +1.1% | 15 | 2 | 0 | 24 |
| An alternative's own | 203 | **+13.7%** | +24.8% | 149 | 70 | 0 | 170 |

- No build scores lower: the team's rotation is still run and kept unless an alternative beats it in a full run.
- The largest gains are where the team's rotation breaks down with the constellations, rather than merely fitting them less well: Clorinde / Mavuika / Ororon / Chevreuse with Clorinde and Mavuika C6, 52k with the library rotation (below its 99k with the library build) and 450k with the alternative; Kazuha / Furina / Nahida / Bennett with Furina C6, +187%; Furina C6 teams generally +100% or more (§3).
- Mavuika teams, the alternative's build: Mavuika C6 on Mavuika / Chevreuse / Ororon / Bennett +17.4%, Mavuika / Yelan / Furina / Xilonen (all C6) +15.2%, the reported Mavuika / Citlali / Xilonen / Nicole (all C6) +9.3%, Raiden / Chevreuse / Mavuika / Sara (Raiden and Mavuika C2) +10.8%, Mavuika / Chasca / Citlali / Bennett (Chasca, Mavuika, Citlali C6) +69.3%.
- **Cost**: on these 191 teams, the iterations simulated for a score grow by a median 1.64 times (mean 2.0–2.3), mostly the alternatives' filler searches at 10 iterations. The other 4,375 teams run as before.

## 8. Teams without alternatives

_2026-10-29._ Alternatives (§7) come only from database entries. Of the 4,566 library teams that are not blocked, **4,375 have none**:

- 4,204 harvested teams have no database entry at higher constellations than their own (§2);
- 171 experimental teams have a rotation written from a guide (design §5.4), which harvest never gives alternatives, and an alternative needs a source DPS today (`library.LoadFS`). The Vesna teams are among them: a Vesna C2 / Odette C6 / Nicole C6 / Vodyanitsa C2 build scored 7–11% low (design §5.2).

Their high-constellation builds are still scored with one rotation, written for the source's constellations, so they can score low by the margins §3 measured where an entry existed: a median 12.4%, and twice the score for a constellation that changes a role (Furina C6 on Bennett / Xiangling / Furina / Kazuha, +105%). Nothing on the result says so.

The characters whose higher constellations changed the rotation most in §3, with the library teams that have them and how many of those have no alternative:

| Character | Teams | Without an alternative |
|---|---|---|
| Furina | 1,324 | 1,244 |
| Nahida | 683 | 654 |
| Yelan | 632 | 603 |
| Xilonen | 479 | 430 |
| Raiden | 366 | 344 |
| Neuvillette | 245 | 236 |
| Mavuika | 244 | 230 |
| Citlali | 221 | 202 |
| Hu Tao | 128 | 110 |

**Needed: a way to score these teams' high-constellation builds with a rotation that fits them.** Not decided. The ways found, cheapest first:

1. **Say what the rotation assumes** (§6, (3)). The result names the constellations the rotation was written for and notes when a member of the build is higher: "This rotation was written for C0 Furina; a C6 Furina plays differently, and the score may be low." It does not raise the score, but it stops a low score from passing as the build's worth. Covers every team, and costs one comparison of constellations per member (`library.Team.Constellations`).
2. **More database entries.** `cmd/harvest -alternatives` picks up any entry the database gains at higher constellations, with no code change. High-constellation configs submitted to the database (db.kqm.gg) for the characters above become alternatives on the next run.
3. **Alternatives written from a guide** (§6, (4)). For a constellation that changes a role (Furina C6, Neuvillette, Citlali C6, Mavuika C1–C2 riding longer), a rotation written from a published community guide under the rules of design §5.4, stored as an alternative. It needs: alternatives without a source DPS when written (`sources` and a `cross_check` note, as a written team), validation as for a written team, and review by a person before it is `supported`, as for any experimental team. One archetype at a time, from the characters above.

**Rotations that fail at higher constellations.** _2026-10-29._ Found while measuring §7.1: a rotation can stop with a gcsim error at a constellation it was not written for, and the score then fails instead of scoring low. With Chasca at C6 (the library build otherwise unchanged), **128 of the 145 Chasca teams without an alternative fail**: 127 with "bullets must be 6 when c6 instant charge is active" (their rotation aims with fewer bullets, which gcsim rejects once C6 makes the charge instant), 1 with "chasca jump in nightsoul blessing not implemented". A team with an alternative falls back to it (§7). Fixed by PR #55 (`internal/score/chasca.go`, docs/fork.md §7): for a C6 Chasca, an aim with fewer than 6 bullets fires 6 while her instant charge is active, as the game loads all six at once, and `chasca-diona-furina-bennett` jumps only outside Nightsoul's Blessing; all 150 Chasca teams run at C0–C6, and C0–C5 scores are unchanged. A rewrite in the rotation cannot do it: gcsim asks for the next action before the previous aim sets the status. Other characters were not swept.

Not usable: a high-constellation rotation of another team. A rotation names all four members and times its actions on them, so moving it to another party is writing a new rotation, which design §5.4 allows only from a guide.

Since the score keeps the best rotation, an added alternative cannot lower a build's score (beyond the noise of ranking on fewer iterations); what it costs is the time of its runs (§7).

## 10. Variants: Citlali C6 without her burst

_2026-10-29._ A community report says a C6 Citlali does not use her burst. §4 found that dropping it lowered Mavuika / Citlali / Xilonen / Nicole on every build. Measured over the whole library:

- **Measurement.** For each of the 180 library teams whose rotation has Citlali's burst (221 Citlali teams, 41 without it), the team's rotation and the same rotation without Citlali's bursts, each scored as the site does (looped, fillers, 200 iterations), with the library build and with that build at Citlali C6. Change from dropping the burst:

| Build | Teams | Median | Over +5% | +1% to +5% | Within 1% | Under −1% | Under −10% |
|---|---|---|---|---|---|---|---|
| The library's (Citlali mostly C0) | 180 | −14.4% | 0 | 13 | 13 | 154 | 102 |
| The library's at Citlali C6 | 180 | −3.5% | 22 | 18 | 23 | 117 | 41 |

- So the report holds for some teams only, and the carry does not decide which: of 13 Gaming teams, 6 gain at C6 and the median is −14.9%. Largest gains: Escoffier / Citlali / Bennett / Xianyun +31.7%, Gaming / Citlali / Nicole / Xianyun +26.2%, Mavuika / Citlali / Charlotte / Bennett +24.8%, Diluc / Citlali / Escoffier / Bennett +19.9%. Largest losses: Gaming / Citlali / Chevreuse / Diona −48.8%, Mavuika / Citlali / Chongyun / Bennett −43.9%, the Arlecchino teams about −33%. In gcsim, C6 starts Opal Fire from her skill alone (no 50 Nightsoul points needed), so the burst is not needed for it; what it still brings is its Cryo (Melt, Citlali's RES shred) against its time on field.
- **Variants.** A variant is an alternative (§7) derived from the team's own rotation instead of a database entry: `variant: {drop: burst, char: citlali}` in its `team.yaml` (`library.Variant`), with the team's rotation minus that member's bursts (`config.DropBursts`) and the team's build with her constellation raised. It has no source DPS: `cmd/validate` skips the reference check ("variant of the team's rotation") and applies the other criteria for supported. `cmd/variant -char citlali -cons 6 teams/...` measures each team as above and writes `alt/citlali-c6-no-burst/` only when the variant scores at least 1% more (`-min`), with both scores in `notes`. `cmd/harvest -alternatives` keeps variants when it replaces a team's alternatives.
- **Only for the constellations it was made for.** Unlike a database alternative, the score tries a variant only when the build reaches the constellations its build raises (`library.Team.RotationsFor`): a C0–C5 Citlali never runs it, so it costs nothing there and cannot win by ranking noise.
- **Run of 2026-10-29**: 40 variants written (gains +1.2% to +31.7%, median +5.5%); `cmd/validate -promote` made 39 supported, and Gaming / Citlali / Sucrose / Bennett stays draft (looped score 140% of one pass, over 130%). Mavuika / Citlali / Xilonen / Bennett got none: at Citlali C6 its own rotation stops with a gcsim error on one seed ("cannot recast E while not in nightsoul blessing"); at 100 iterations the variant scored −22%.
- On the site, a C6 Citlali player of these teams gets the variant when it scores higher; the result says the score uses the team's rotation without her burst, made for C6 Citlali, and the library team page lists it.
