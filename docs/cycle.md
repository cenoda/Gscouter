# Cycle

> Status: Investigation · Last updated: 2026-10-29

Roadmap node Cycle (`docs/roadmap.md`): what the fixed rotation (design §5.2) does with builds other than the one it was written for, and where the score stops being trustworthy because of it. Nothing here changes the site yet. §6 proposes what to change.

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
- Every such team, its library build with each looping character's Favonius weapon changed to the Sacrificial weapon of its type (Dragon's Bane for a polearm), 2 iterations (run in progress): all of the first 36 teams collapse, to 2–15% of their score (median about 3%).
- One library rotation already guards the loop the right way: `if .furina.weapon == .keys.weapon.favsword && ...` (Wanderer / Furina / Faruzan / Bennett).
- Other `while` conditions on mods are on a character's own state (`razor-q` 57, `xingqiuburst` 20, `bennett-field` 17, `yelanburst` 16, ...), which every build has; they were not tested.

## 6. Proposal

Not decided; in order of how much trust each buys for its cost.

1. **Weapon-dependent loops** (§5). Guard each `while !.X.mods.favonius-cd` with `.X.weapon` being a Favonius weapon, either as a rewrite when the rotation is loaded (like the key rewrite of design §4.4) or in the rotation files with a library test that rejects an unguarded one. Add a validation check that scores each team with its Favonius weapons swapped out and fails a collapse, so a new harvest cannot bring the problem back. Editing rotations changes their cache keys and moves those teams' recorded scores only where the guard matters.
2. **Keep the best of a team's rotations.** Harvest keeps, besides the rotation it picks now, the best passing entry for each higher-constellation profile the database has; the score runs the player's build with each and keeps the best, as fillers keep the best run (design §3.1). On the 191 teams measured it raises higher-constellation builds by a median of 12.4% and leaves the library builds where they are. Cost: one more scoring (with its filler search) per extra rotation, on about 217 teams; the result page would say which rotation scored. A rotation could be skipped when the player's constellations are far below its entry's, but §3.1 shows the boundary depends on several members at once, so that needs measuring first.
3. **Say what the rotation assumes.** The result page shows the constellations the rotation was written for, and a note when a member's constellation is higher: "This rotation was written for C0 Furina; a C6 Furina is played differently, and the score may be low." Cheap, and it covers the 4,204 teams with no higher-constellation entry.
4. **Rotations for constellations the database lacks.** For a constellation that changes a role (Furina C6, and the characters of the table in §3), a rotation written from a community guide under the rules of design §5.4, as a second rotation of the team kept by (2). Later, once (2) exists.
