# Lohen

> Character on the gcsim fork (`docs/fork.md`) · Last updated: 2026-10-28

Lohen (5★ Cryo, 6.6). Upstream gcsim: no PR. Scored in the experimental tier (design §5.4); the fork's rules and regression runs are in `docs/fork.md`.

## 1. On the fork

**Source.** Ours (2026-10-27, fork `af3eb82`): multipliers from the pipeline; mechanics from the game's ability data (`ConfigAbility_Avatar_Lohen`, `ConfigAvatar_Lohen`): Will to Win multiplies the talent multiplier by 1 + 0.004 per point (snapshotted at cast), the Hexerei passive is +40% normal and charged attack DMG for 6 s at Will to Win ≥ 50% of its limit with 2+ Hexerei members, and normal and charged hits share a game ICD group of 6 Cryo applications per 5 s

**Frame data.** Every frame is estimated: no community frame count exists (KQM quick guide, TCL). The N1C loop (N1 → charged attack → N1) is 43 frames, calibrated so that a 13 s Masterstroke fits 15–16 loops after the skill and 17 only with near frame-perfect input, as the KQM Lohen quick guide reports; other normal attacks follow Flins; skill, Etched Into Bone and Soul and burst animations are estimates. From the ability data, not estimated: hit lag, defense halt, hit counts and hitboxes, Etched's hit order and the burst's hit spacing. Each 5 frames of the N1C loop move team DPS by about 3–5%. Calibration: an Etched Into Bone and Soul crit hits 28.5k, against 28k+ on the creator server ([HoYoLAB 45375045](https://www.hoyolab.com/article/45375045), all C0 with signature weapons); Lohen / Durin / Nicole / Citlali (C0, single target, 10% RES, KQM's average combo, real ER: the least at which every member bursts every rotation) scores 139.1k one pass with the library team's build (Disaster and Remorse, Wolf-Fang R1 Durin, Oathsworn Eye R5 Nicole), 169.5k with the supports' signatures (Athame Artis R1 Durin, Angelos Heptades R1 Nicole; Durin 163% ER, Nicole 139%, Citlali 179%) and 149.5k with those and Deathmatch R5 on Lohen, against community figures of about 190k and 160k (−27%, −11%, −7%). The earlier figures here, 180.6k with signatures and 162.9k with Deathmatch R5, gave every member a free +80% ER (`add stats er=0.8`) besides the build, and are withdrawn. In the recovered config, the free ER alone took the team from 106.3k to 134.1k; a CRIT DMG crown on Lohen then gave 144.6k and the signatures 180.6k

**Known problems.** No plunging attacks; C2's Evilsbane Blade and the C6 Joy refill after the burst fire at a fixed frame 95 (the burst animation's end is estimated); the particle count (1 per Cryo hit, every 2 s) is inferred from the data, not checked in game; teammates' transformative reaction damage grants no Will to Win (direct Lunar and Stellar damage does), a guess where the data is unclear

## 2. Library teams

### From the KQM Lohen quick guide

_2026-10-28, fork `b029c45`, all `status: draft`, 100 iterations, common conditions._

No community gcsim config with Lohen exists (the database cannot have one while upstream lacks him; web searches found no share link or config written for a fork or test build), so the rotations come from the [KQM Lohen quick guide](https://keqingmains.com/q/lohen-quickguide/): one team per archetype in its example list, most common first, each with the guide's rotation for it.

| Team | Guide team | Lohen's combo | One pass | Looped (score) |
|---|---|---|---|---|
| Lohen / Durin / Nicole / Citlali | 1 (Hexerei forward melt, "typically best overall") | average: `E 2[N1C] E/Q 4[N1C] E 5[N1C] E` | 139.1k | 141.6k |
| Lohen / Mavuika / Nicole / Citlali | 2 | average | 136.5k | 129.7k |
| Lohen / Durin / Nicole / Sucrose | 1 with Sucrose | average | 125.6k | 112.9k |
| Lohen / Durin / Escoffier / Furina | 4 | average | 97.9k | 109.0k |
| Lohen / Escoffier / Furina / Mona | 8 (Freeze) | Freeze: `E 15[N1C] N1 E` | 94.4k | 98.3k |
| Lohen / Xiangling / Bennett / Kaeya | 5 (limited roster, 4★ teammates) | no Hexerei teammate: `E 15[N1C] N1 E` | 50.1k | 47.8k |

- Builds: Lohen is the same in every team: C0, Disaster and Remorse R1 and 4pc Rising Winds (the guide's best), ATK%/Cryo/CRIT DMG, KQM-standard substats, no ER (the guide: his burst does not justify ER; it is cast when ready). Each support is copied from a harvested library team with the same supports, named in `build.txt` (Chongyun / Durin / Citlali / Nicole for the Citlali team), with ER raised to the least at which every member bursts every rotation, as the guide's rotations do. That puts Durin at 163% (an ER sands with no ER substats beyond the standard's fixed two; the guide gives 170–185% for the Pyro unit with Citlali), Nicole at 128–150% (guide 125–175% by team), and Xiangling at 248%, with Favonius Lance as the guide assumes. No team waits for energy or cooldowns, and every Etched Into Bone and Soul the combo calls for lands: the average combo's three per Masterstroke, each from 100 Joy, which two N1C strings give.
- The long combo `E 15–17[N1C] (N1 E)` is the guide's for Freeze and for teams without a Hexerei teammate. Without a second Hexerei member, Lohen gets neither his +40% nor Disaster and Remorse's Hexerei boost, and his Will to Win stays low (no teammate hits for 1000% of his base ATK), so the 4★ team's Etched hits are about a fifth of those in the Durin teams.
- Cross-check: the community's about 190k for Lohen / Durin / Nicole / Citlali (Disaster and Remorse, signature weapons on Nicole and Durin) against 139.1k one pass here (−27%) with the harvested 4★ support weapons, and 169.5k (−11%) with Athame Artis R1 on Durin and Angelos Heptades R1 on Nicole at real ER (Durin 163%, Nicole 139%, Citlali 179%); with Deathmatch R5 on Lohen that variant gives 149.5k against the community's about 160k (−7%). The fork's earlier calibration (180.6k, 162.9k) gave every member a free +80% ER and is withdrawn (§1). Prydwen's team chart (seen only through search snippets; the page returns 403) rates the Freeze team at 88.7% of the Citlali team, and here it is 68–69%. No public number exists for the other teams.
- Known problem, left for later (2026-10-28): the scores sit at or below the low end of every community range found. A Korean community guide (DCInside, no build or enemy stated) gives C0 Freeze with 4★/standard weapons 110–130k (here 94.4k with Lohen on Disaster and Remorse, so −15% or more), C0 Lohen / Nicole / Citlali / Durin 140–170k (139.1k), Lohen and Durin with signatures 180–200k (169.3k), and all four with signatures 200–230k at best (173.9k with Starcaller's Watch R1 on Citlali, −13 to −24%; Lohen 99.4k, Durin 64.8k, Citlali 8.2k, Nicole 1.5k). Part of it is the substats (KQM standard here, the guide's figures read as good artifacts), and part is likely Lohen's estimated frames: each 5 frames of the N1C loop move team DPS by 3–5%, and the loop is set from KQM's 15–16 strings per Masterstroke, not a frame count. The Freeze team's lower ratio may also come from its copied Escoffier / Furina / Mona builds and rotation. To fix: a frame count or a timed video of Lohen's N1C loop, then rerun these teams.
- The site scores a showcase with Lohen and these teammates in the experimental tier (`TestScoreForkOnlyCharacter`). Only team directories, a server test and this document changed, so no regression run was needed.
With Vodyanitsa: see `docs/characters/vodyanitsa.md` §2.
