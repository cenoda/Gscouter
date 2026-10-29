# Vesna

> Character on the gcsim fork (`docs/fork.md`) · Last updated: 2026-10-28

Vesna (5★ Anemo, 7.1). Upstream gcsim: no PR. Scored in the experimental tier (design §5.4); the fork's rules and regression runs are in `docs/fork.md`.

## 1. On the fork

**Source.** Ours (2026-10-27, fork `3e75b40`): multipliers from the pipeline; mechanics from the game's ability and gadget data (`ConfigAbility_Avatar_Vesna`, `Avatar_Girl_Sword_Vesna_AniEvent_Attack`, `ConfigGadget_SkillObj_Vesna`, `ConfigTalent_Vesna`; Vesna is also her internal name, avatar 10000143). The skill hits at the press and starts Armed for Action (15 s; C1 adds 5 s, `Buff_Duration_Extra` in the data, not in the text) with 60 Sword Essence (max 90). In the mode normal and charged attacks deal Anemo DMG no infusion overrides, and each attack (not each hit) summons a wind pinion that deals Anemo DMG and regenerates 5 Essence, so 12 attacks fill the 60 that, with the skill's 60 and the burst's 30, pay for the five Windborne Swords of a C0 window, as guides count it. Windborne Sword costs 30 Essence and rises from Lv. 1 to Lv. 3 within a window; Lv. 2 adds a Spirit Blade 0.5 s after its gadget appears, Lv. 3 four blades and a final hit, and the window ends after the third Lv. 3 (C1: fourth, the first free). The burst is one Spirit Blade 0.55 s after the press (the text says "multiple"; the data has one gadget with one hit) and +30 Essence. Windborne hits, pinions and non-Radiance blades share the game's `Vesna_MagicSword` ICD group (1 application per 2 hits, 2 s reset). In Radiance: Stellar Swirl, read when the blade's gadget appears or when the Lv. 3 animation starts, Spirit Blades deal Anemo DMG that is Stellar Swirl DMG (`StarSwirlWindDamage`, the engine's direct Stellar Swirl tag with Anemo element) with their own multipliers; the pinion never does (its Radiance bullet runs the same ability). A1: Disciplinary Action, one instance per Windborne Sword or burst, 20 s each, the count capped at 6 and lowered as each instance expires, as the game's global value does; blades ×(1 + 10% per stack); cleared by the skill and by leaving the field. A4: in Radiance, 6% ATK per Cryo or Anemo member and 25 EM per other member (C4 ×3). Stellar Jubilee: Odette's pattern, 0.7% base Stellar Swirl DMG per 100 ATK, max 14%. C1 +20% Stellar Swirl DMG during the window, C2 6 stacks at the skill and +40% ATK at 6, C6 Windborne Sword: Transpose (150% + a 200% blade, with a pinion in the window) for 5 s after each Lv. 3 and 20% elevation on her Stellar Swirl DMG. 1 Anemo particle per 2.5 s

**Frame data.** Every animation length and cancel is estimated: no community frame count exists (searched KQM, the TCL, Prydwen, Icy Veins, HoYoLAB). Normal attacks follow a generic 6-hit sword (N1–N6 hitmarks 12/12/10+21/15/14/21, chains 21–52), charged attack hit at 24 (52 frames), skill 38 frames (swap at 28), Lv. 1 42, Lv. 2 44, Lv. 3 80 (hits at 26, 33, 41, 49 and 66), Transpose 64, burst 100. From the data, not estimated: the skill's hit at the press, Lv. 1 at 20% and Lv. 2 at 35% of their animations, the Lv. 3 hits at 36/47/58/69% of the animation and the final hit at its recovery, the Lv. 2 blade 0.5 s and the burst blade 0.55 s after their gadgets appear, gauges, hit lag and hitboxes. With these frames her window is about 14 s, inside the 15 s mode, for the KQM/Prydwen combo E N1C E N1C E N3C Q E N1C E N1C E. Calibration (no source gives every member's stats, so most comparisons are ratios; builds: KQM-standard substats from gcsim's optimizer, talents 9/9/9, level 100 enemy at 10% RES, every member bursting every rotation): weapons against Silver Light R5 in C0 Vesna / C0 Vodyanitsa / C6 Cryo Traveler / C0 Odette (Icy Veins' team, figures from search extracts): Beyond the Chrysalis 1.319 (Icy Veins 1.272, +3.7%), Whitelake Frostfeather 1.140 (1.201, −5.1%), Absolution 1.130 (1.121, +0.8%), Azurelight 1.022 (1.080, −5.4%), Emberwell R5 1.022 (1.082, −5.5%); against Emberwell R5 (Prydwen): Beyond the Chrysalis 1.291 (1.247), Whitelake 1.116 (1.168), Absolution 1.106 (1.055), Azurelight 1.000 (1.049), Silver Light R5 0.979 (0.978), Primordial Jade Cutter 0.911 (1.061: its 44% CRIT Rate puts this build over 100%). One damage instance, [HoYoLAB 46901646](https://www.hoyolab.com/article/46901646) (Silver Light, sheet 69% CRIT Rate and 171% CRIT DMG, C6 Cryo Traveler, C6 Faruzan; refinement and ATK not stated): burst 240k against 233k (R1) to 300k (R5) in the sim; one Lv. 3 Windborne Sword 340k against 290k–390k, depending on the Disciplinary Action stacks at the cast; Vesna's damage per rotation 1.3M against 1.25M (−4%). Team damage is lower than every source: Icy Veins 215,899 DPS against 135.1k (−37%; rotation and builds not visible); [HoYoLAB 46785086](https://www.hoyolab.com/article/46785086) (Vesna / Odette / Faruzan / Vodyanitsa, its combo E E Q E 2N1C E 2N1C E) 4.23M per 18.48 s rotation against 2.79M per 20.05 s (139.0k DPS, −34%); [HoYoLAB 46785152](https://www.hoyolab.com/article/46785152) (C0 Silver Light Vesna at 47.1%/118.1%, Qiqi, C6 Cryo Traveler, C6 Faruzan) about 2M per rotation against 1.54M. Traced: the per-hit formula matches the engine's direct-reaction formula term by term (checked on a Lv. 3 final hit) and Vesna's own share matches HoYoLAB 46901646; the sim's rotations are longer (19–24 s against about 18 s, from the teammates' frames and Vesna's estimated ones); Vodyanitsa's Lead Vocal and Chorus missed Vesna's blades until fork `87338fd` (with it: Icy Veins' team 128.5k → 135.1k, +5.1%; HoYoLAB 46785086's 115.8k → 123.4k, +6.6%); the first HoYoLAB 46785086 runs had Odette press her skill once and never recast it, so she dealt no Stellar damage (only Coda at Dawn's Tolling and the dance moves after it are Stellar) and her Heart of the Furnace 4pc, the party's +50% Stellar DMG, never triggered: with burst, skill, skill as in the Icy Veins team, 123.4k → 139.0k (2.35M → 2.79M per rotation, −45% → −34%; the set's code and data agree, checked against `Relic_7.0_StarSupport_4`); and none of these sources states its teammates' stats or constellations, so the rest of the gap cannot be attributed yet. Vodyanitsa's weapon is not the gap: HoYoLAB 46785086 gives her Thrilling Tales of Dragon Slayers (as the sim does), HoYoLAB 46785152 has no Vodyanitsa, and Icy Veins' weapon for her was not visible (the sim uses TTDS R5). With Hymn R1 in her place (fork `76e8116`, same substats) the Icy Veins team gives 145.9k (−32% instead of −37%) and HoYoLAB 46785086's 150.4k (3.02M per rotation, −29% instead of −34%); the TTDS runs are unchanged on `76e8116` (135.4k, 138.7k). Second trace (2026-10-28, fork `76e8116`, no fork change): the sources' per-hit numbers were compared with sims of their setups, with a Vesna rebuilt from 46785152 (Scarlet Proof, as "Bloodred set" there; ATK/ATK/CRIT DMG, sheet 69%/171% as in 46901646; Qiqi C0, C6 Cryo Traveler with Heart of the Furnace, C6 Faruzan with Noblesse, Vodyanitsa TTDS R5 with 54.7k HP; KQM combo, best cast of the rotation, crit values). 46901646, burst / one Lv. 3 Windborne Sword (four blades and the final hit): Cryo Traveler, Faruzan, Qiqi with Silver Light 240k / 340k against 262k / 360k (+9%, +6%; Vesna's damage per rotation 1.3M against 1.38M); Qiqi replaced by Vodyanitsa 310k / 500k against 308k / 434k (−1%, −13%; the screenshot's final hit 206,393 and blade 74,298 against 182,259 and 62,925); Odette, Faruzan, Vodyanitsa with Beyond the Chrysalis R1 500k / 800k against 447k / 668k (−11%, −17%). The Vodyanitsa steps undershoot on the Lv. 3 more than on the burst: Lead Vocal adds flat DMG per hit, and the screenshot's final-to-blade ratio implies about 5.4k per hit where her 54.7k HP gives 3,831 (about 61k HP would match). The 25 Lead Vocal stacks also run out before the second Lv. 3, because every Stellar Swirl Vesna triggers spends one; her data counts reaction contributions (`ModifyNewElementReactionDamageCountMixin` beside `ModifyDamageCountMixin` on `StarSwirlWindDamage`/`StarSwirlIceDamage`), so this is modeled as the game does it. Stellar Swirl reaction hits: 34,454 (46785086) and 27,606 (46785152, four five-stars) on screen against 22.9k median and 28.4k highest in the full-team sim. Team totals stay low: 46785086 2.46M per 18.5 s (132.9k DPS with Vesna at 121% ER, which the source recommends; at 111% the third rotation waits for her burst) against 4.23M, −42%; per rotation Vesna's direct hits 1.41M, her other hits 0.17M, Stellar Swirl reactions 0.36M and detonations 0.32M, Odette 0.15M, Faruzan 0.07M, Vodyanitsa 0.01M. 46785152's Qiqi team 1.21M per 18.7 s against about 2M (−40%), its Lv. 3 blade 41.5k against 47,799 on screen (−13%). Icy Veins (403) gives no per-member numbers. So Vesna's hits are within 6–17% of every source and the reactions within range; the missing damage sits in what the sources leave out. The sim's total moves with exactly those builds: Odette C1 +9%, C2 +27%, C6 +87%; Beyond the Chrysalis R5 +22%; Vodyanitsa +30% HP +4% (KQM combo, 100 iterations). Read with an Odette at C2 and a signature at R5, the source totals are within reach, but no source states them, so nothing is tuned to them. The guide's combo (E E Q E 2N1C E 2N1C E) has 8 attacks, so with the data's 5 Sword Essence per attack it fits only two Lv. 3; 46901646's 1.3M (about one burst and three Lv. 3) fits the 12-attack KQM combo. Checked against the game's data and found as modeled: Stellar Swirl's base multiplier 1.0 with level ratios 0.75 for the reaction (`StarSwirlWind` [1.0, 0.75]) and 2/3 for the detonation (`StarSwirlIce` [1, 2, 3]), contributor weights 0.6/0.3/0.05/0.05 (`ConfigGlobalCombat`), the vortex's 3 s life and 3/6 stack thresholds (`ElementAbility_StarSwirl_Ice_Gadget`); Faruzan's A4 excludes Stellar Swirl hits and her C6 +40% CRIT DMG covers direct and reaction Stellar Swirl DMG (`Avatar_Faruzan_PermanentSkill2_WindDamageExtraUp`); Scarlet Proof (`Relic_7.0_StarWind`, 16% CRIT Rate, +40%, 10 s); Stellar Jubilee is `SetNewElementReactionBaseDamageRatioParam`, the same action as the Lunar characters' base DMG bonuses, which upstream adds together (a highest-only reading would lower the sim). One engine difference, not changed: the reaction's attack uses Swirl's ICD tag and `ICDGroupReactionA`, whose damage sequence zeroes a character's third and later hit within 0.5 s (the TODO in `pkg/reactable/stellarswirl.go`), while the game's attack has an empty `attenuationTag`; removing it raises these teams by 0.2–1.2%, and it would move verified Odette, Qiqi and Cryo Traveler teams, so it is left for an upstream fix

**Known problems.** No plunge (her plunging attack is not modeled, and its pinion with it); Coursing is not modeled. Lv. 1 and the wind pinion hit the primary target only. Disciplinary Action timers are not extended by hit lag. The wind pinion travel time (12 frames) is estimated. Stellar Jubilee's base DMG bonus adds to Odette's and the Cryo Traveler's, as the engine already does for those two; whether the game takes the highest instead is not known. Until fork `87338fd`, Vodyanitsa's Lead Vocal and Chorus skipped direct Stellar Swirl hits, which her data (`ModifyDamageCountMixin` on `StarSwirlWindDamage` and `StarSwirlIceDamage`) counts; that commit, made separately from Vesna's, adds them (Vodyanitsa's row)

## 2. Library teams

### From the GameWith Vesna team guide

_2026-10-28, fork `b029c45`, all `status: draft`, 100 iterations, common conditions._

No community gcsim config with Vesna exists (the database cannot have one while upstream lacks her), and KQM has no Vesna quick guide (`/q/vesna-quickguide/` returns 404; searches find none). So the rotations come from the [GameWith Vesna team guide](https://gamewith.net/genshin-impact/article/show/77723), the only guide found that gives a rotation for every team it lists, with per-member gear: its three example teams, one library team each, in its order. The [Icy Veins team guide](https://www.icy-veins.com/genshin-impact/vesna-team-guide) lists eight teams but its [build guide](https://www.icy-veins.com/genshin-impact/vesna-guide-best-builds) gives rotations for two (the premium team below, its supports in the same order, and the same with Faruzan for Vodyanitsa); AllThingsHow gives one F2P rotation (Faruzan / Diona / Cryo Traveler), GenshinBuilds none. Icy Veins' ER table for Vesna (a second Anemo member 120–140%, C6 Faruzan 100–120%, none 150–170%) is the only one found, so Vesna's ER follows it.

| Team | Guide rotation | One pass | Looped (score) |
|---|---|---|---|
| Vesna / Odette / Cryo Traveler / Vodyanitsa | Odette E E > Traveler E C Q > Vodyanitsa E > Vesna | 151.1k | 150.5k |
| Vesna / Cryo Traveler / Diona (C6) / Sucrose | Traveler E C Q > Diona E Q > Sucrose E > Vesna | 67.9k | 76.5k |
| Vesna / Columbina / Ineffa / Sucrose | Ineffa E (Q) > Columbina E Q > Sucrose E > Vesna | 70.4k | 80.5k |

- Rotations: Vesna plays the guide's combo `E [N C E]x2 Q E [N C E]x2` in every team. Its 8 attacks give 40 Sword Essence at the fork's 5 per attack (the data's wind pinion), so with the skill's 60 and the burst's 30 they pay for four Windborne Swords, not the five a C0 window allows; whenever the next sword is not ready she keeps attacking (`N C`, the guide's "use Normal and Charged Attacks to recharge her special Skill") until the fifth ends Armed for Action. gcsim needs a normal attack before the Cryo Traveler's charged attack, so her `E C Q` is `E N1C Q`, as in the Sandrone Stellar Swirl team. Odette's and Sucrose's bursts are not in the guide's rotations.
- Builds: Vesna is the same in every team except her ER and, in the Anemo driver team, her set: C0, Beyond the Chrysalis R1 (the guide's weapon; it names Emberwell or Finale of the Deep for the F2P team), 4pc Scarlet Proof, ATK%/ATK%/CRIT DMG (the guide: CRIT Rate or DMG; CRIT DMG scored 7% higher in the premium team), KQM-standard substats. In the Anemo driver team she wears 4pc Viridescent Venerer, the guide's set there: without Cryo there is no Stellar Swirl, so Scarlet Proof would do nothing. Her ER: with Sucrose she bursts every rotation from 138.6% (Diona team; 133.1% waits 3 s per minute) and from 116.5% (Columbina team, raised to the table's 122%), inside Icy Veins' second-Anemo row. In the premium team she cannot at any ER the substats reach (166.1% still waits 1.1 s per minute; the table's row is 150–170%), so she follows the usual advice for a main DPS: no ER beyond the standard's (111%), burst when ready. Each choice scores highest: premium 150.5k looped against 146.9k at 166.1% with the burst every rotation; Diona team 76.5k against 71.9k at 111% bursting when ready; Columbina team 80.5k against 79.9k.
- Supports reuse the Sandrone batches' builds: Odette (Finale of the Deep R5, Heart of the Furnace; the guide gives Whitelake Frostfeather), Vodyanitsa (Thrilling Tales R5, Tenacity; the guide gives Hymn of the Maelstrom), the Cryo Traveler (Exaiphanes Blade R1, Disenchantment in Deep Shadow; the guide gives Noblesse Oblige), C6 Diona, Sucrose (Viridescent Venerer), Columbina and Ineffa. Two exceptions follow the guide's set where its holder differs: the F2P team's Cryo Traveler wears Heart of the Furnace (no Odette holds it there), and the Anemo driver team's Sucrose wears Instructor, copied from the harvested Alhaitham / Furina / Sucrose / Kuki Shinobu team (Vesna holds Viridescent Venerer).
- No other member can burst every rotation at an ER the rules allow, so every other burst is cast when ready: the Cryo Traveler waits 3.6 s per minute at 166.1% in the premium team and 22 s per minute in the F2P team, C6 Diona 15 s per minute at her harvested 257.2%, and Columbina 18 s per minute with every CRIT DMG roll of her copied build moved to ER (182.7%). Why they run so short (the rotations last 20–21 s, about 14 s of it Vesna's window) was not examined. Ineffa bursts when available, as the guide says.
- Vesna deals 76–86% of the Stellar Swirl teams' damage. In the Anemo driver team, which the guide rates 1 of 5 stars, she deals 18% and Ineffa and Columbina 81%.
- Cross-check: no public number for any of these teams (searched 2026-10-28: GameWith and Icy Veins give team lists, rotations and Icy Veins weapon and constellation percentages but no DPS; AllThingsHow and GenshinBuilds give ratings). §1 quotes 215,899 DPS for Icy Veins' C0 Vesna / C0 Vodyanitsa / C6 Cryo Traveler / C0 Odette team, seen in search extracts on 2026-10-27; the current pages do not show it and its builds and rotation are unknown, so it is not recorded as a cross-check. The premium team, with a C0 Traveler, scores 151.1k one pass (−30%).
- Known problems, left for later (2026-10-28; the engine is unchanged): every Vesna frame is estimated (§1). Three published combos, GameWith's and Icy Veins' (`E EE QE N4 E N4 E`) here and HoYoLAB 46785086's in §1, fit five Windborne Swords into 8 attacks, where the fork's 5 Sword Essence per attack gives four. Icy Veins' kit text says four normal attacks restore one sword's Essence and lists Windborne Sword itself as an Essence source, neither of which the fork models; the data the fork follows was not rechecked. The extra attacks the rotations need lengthen her field time, so a fix may raise these teams. The fork's team totals already sit below every source (§1); these teams carry that gap.
- The Vesna / Vodyanitsa / Cryo Traveler / Faruzan team, `blocked` until then, became a draft written team the same day (below).

### Vesna / Vodyanitsa / Cryo Traveler / Faruzan

_2026-10-28, fork `b029c45`, `status: draft`, 100 iterations._

Blocked until a source for it was found: the [KQM Vodyanitsa quick guide](https://keqingmains.com/q/vodyanitsa-quickguide/) lists Vodyanitsa / Vesna / Odette / Cryo Traveler, says "If one of Odette or Cryo Traveler is unavailable, C6 Faruzan can be a decent replacement for them", and gives the rotation of the Odette / Faruzan variant (`Faruzan E C Q > Odette 2[E] > Vodyanitsa E > Vesna combo`). This team replaces Odette, with Faruzan played as in that variant: `Faruzan E C Q > Cryo Traveler E N1C Q > Vodyanitsa E > Vesna combo`, Vesna's combo as in the other Vesna teams (the guide does not spell it out).

- Builds: Vesna as in the other Vesna teams but at 122% ER (Icy Veins: 100–120% with C6 Faruzan, 120–140% with a second Anemo member), bursting in 18 of about 20 rotations: 116.6k looped, against 109.3k at 111% and 116.9k at 127.5%. The Cryo Traveler wears Heart of the Furnace as in the F2P team (no Odette holds it), Vodyanitsa as in the premium team, and Faruzan the most common C6 build of the harvested library teams (Favonius Warbow R4, Viridescent Venerer); Faruzan and the Traveler burst when ready (Faruzan in 11 of about 21 rotations; 22% more ER adds 0.6%).
- Result: 108.3k one pass, 116.6k looped (kept 108%). Vesna deals 83%, the Traveler 9%, Faruzan 7%, Vodyanitsa 0.6%. No public number (searched 2026-10-28: KQM, GameWith, Icy Veins, Prydwen, Mobalytics, Game8, AllThingsHow, GenshinBuilds). Mobalytics and Prydwen (search extracts; both pages return 403) warn that Vodyanitsa next to Faruzan lowers Stellar Swirl triggers, and the guides rank the team below the ones with Odette; it scores 77% of the premium team.

### Stellar Swirl teams by support

_2026-10-28, fork `b029c45`, all `status: draft`, 100 iterations._

Players field Vesna with Vodyanitsa and any two of Odette, the Cryo Traveler, Diona, Qiqi, Faruzan, Sucrose, Lan Yan and Nicole, or with any three of them, so each combination with at least one Cryo member has a team: 22 with Vodyanitsa and 52 without, 71 of them here and three above (the premium team, the Faruzan team and the F2P team). A team without Cryo has no Stellar Swirl and is left out (a Vesna / Vodyanitsa / Faruzan / Nicole team, added earlier the same day at 28.9k, was removed). The library finds a team by its members in any order. A script wrote them by these rules:

- Rotations follow the order of GameWith's Stellar Swirl rotations: Cryo supports (Odette E E, Cryo Traveler E N1C Q, Diona E (hold) Q, Qiqi E Q), then the others (Faruzan E C Q, Sucrose E, Lan Yan E Q, Nicole E Q, Vodyanitsa E), then Vesna's combo; every burst but Vesna's is cast when ready. Every member is one of GameWith's two Stellar Swirl teams or a substitute it lists for that slot (Cryo: Odette, Cryo Traveler, Diona, Qiqi; fourth slot: Vodyanitsa, Faruzan, Sucrose, Lan Yan, Qiqi); the KQM Vodyanitsa guide also has C6 Faruzan replace Odette or the Cryo Traveler, and its Odette / Faruzan / Vodyanitsa rotation is used as written (Odette Q > Faruzan Q > Odette E when her burst is ready, Faruzan E C Q > Odette 2[E] otherwise). No guide puts Nicole in a Stellar Swirl team; GameWith lists her as an ATK% buffer for Vesna's Anemo driver team, so she takes a support slot. Diona holds her skill (GameWith: tap or hold): it feeds her burst better (with Vodyanitsa and Faruzan, 112.8k tapped against 121.0k held, before Faruzan held Heart of the Furnace).
- Builds are the other Vesna teams': Odette, the Cryo Traveler, Vodyanitsa, C6 Diona, Sucrose; Faruzan the most common harvested C6 build (Favonius Warbow R4), Qiqi the Sandrone teams' (C6, Favonius Sword R3, Scroll; Witch's Revelation on, gcsim's default), Lan Yan the most common harvested one (C6, Sacrificial Fragments R5, Viridescent Venerer), Nicole the most common harvested one (C0, Oathsworn Eye R5, Celestial Gift). Heart of the Furnace's 4pc buff needs its holder to deal Stellar damage, so Odette holds it, else the Cryo Traveler, else the first Anemo support; with only Diona, Qiqi and Nicole beside Vesna nobody does.
- Vesna's ER is whichever of 111%, 122%, 133% and 144% scores highest per team (each team's notes list all four): 111% in 47 teams, higher in 24, mostly teams with Sucrose or Qiqi.
- Teams that keep under 80% of their one pass have Diona as their only steady Cryo source: without a Favonius holder beside her she bursts about every other rotation, and Vodyanitsa's Hydro or the Anemo swirls take over (with Vodyanitsa and Nicole, 143 Hydro Swirls against 66 Stellar Swirls in one run).
- Community rankings in October 2026 put Diona and Faruzan above the Cryo Traveler beside Vesna, as the sim does (Odette / Diona / Faruzan 154.3k without Vodyanitsa, Odette / Cryo Traveler / Faruzan 127.1k).
- No public number for any of them (searched 2026-10-28, the same sources as above).

With Vodyanitsa:

| Vesna / Vodyanitsa + | One pass | Looped (score) | Kept | Vesna ER |
|---|---|---|---|---|
| Odette / Qiqi | 163.2k | 167.0k | 102% | 133% |
| Odette / Faruzan | 183.8k | 154.0k | 84% | 111% |
| Odette / Diona | 175.0k | 152.4k | 87% | 111% |
| Odette / Nicole | 155.2k | 145.8k | 94% | 111% |
| Odette / Sucrose | 140.5k | 141.3k | 101% | 144% |
| Diona / Faruzan | 142.4k | 137.3k | 96% | 111% |
| Qiqi / Faruzan | 131.7k | 136.2k | 103% | 111% |
| Odette / Lan Yan | 143.7k | 128.1k | 89% | 111% |
| Cryo Traveler / Qiqi | 100.7k | 118.0k | 117% | 144% |
| Diona / Qiqi | 122.3k | 111.6k | 91% | 133% |
| Qiqi / Sucrose | 112.5k | 108.8k | 97% | 122% |
| Cryo Traveler / Diona | 108.5k | 107.9k | 99% | 111% |
| Qiqi / Nicole | 98.3k | 102.1k | 104% | 144% |
| Cryo Traveler / Nicole | 96.3k | 101.1k | 105% | 111% |
| Cryo Traveler / Sucrose | 84.6k | 98.3k | 116% | 144% |
| Diona / Sucrose | 120.1k | 97.4k | 81% | 111% |
| Cryo Traveler / Lan Yan | 89.2k | 92.5k | 104% | 111% |
| Qiqi / Lan Yan | 95.0k | 87.0k | 92% | 144% |
| Diona / Lan Yan | 110.0k | 84.4k | 77% | 111% |
| Diona / Nicole | 102.6k | 64.0k | 62% | 144% |

Without Vodyanitsa:

| Vesna + | One pass | Looped (score) | Kept | Vesna ER |
|---|---|---|---|---|
| Odette / Diona / Faruzan | 146.4k | 154.3k | 105% | 111% |
| Odette / Qiqi / Faruzan | 139.0k | 147.6k | 106% | 111% |
| Odette / Faruzan / Sucrose | 131.9k | 134.4k | 102% | 111% |
| Odette / Qiqi / Sucrose | 123.9k | 129.9k | 105% | 122% |
| Odette / Cryo Traveler / Faruzan | 122.9k | 127.1k | 103% | 111% |
| Odette / Faruzan / Nicole | 128.8k | 121.1k | 94% | 111% |
| Odette / Diona / Qiqi | 112.9k | 120.7k | 107% | 133% |
| Odette / Diona / Sucrose | 128.0k | 119.8k | 94% | 122% |
| Odette / Qiqi / Nicole | 106.1k | 117.6k | 111% | 144% |
| Odette / Cryo Traveler / Sucrose | 103.8k | 117.2k | 113% | 144% |
| Odette / Diona / Nicole | 125.5k | 115.8k | 92% | 111% |
| Odette / Cryo Traveler / Nicole | 101.5k | 114.1k | 112% | 111% |
| Diona / Qiqi / Faruzan | 111.5k | 110.2k | 99% | 111% |
| Odette / Cryo Traveler / Diona | 101.4k | 108.9k | 107% | 144% |
| Odette / Cryo Traveler / Qiqi | 93.0k | 108.7k | 117% | 144% |
| Odette / Sucrose / Nicole | 114.6k | 108.4k | 95% | 111% |
| Cryo Traveler / Diona / Faruzan | 84.2k | 103.1k | 122% | 111% |
| Qiqi / Faruzan / Nicole | 95.1k | 102.5k | 108% | 111% |
| Qiqi / Faruzan / Sucrose | 97.8k | 102.2k | 105% | 111% |
| Cryo Traveler / Qiqi / Faruzan | 87.7k | 102.0k | 116% | 111% |
| Odette / Faruzan / Lan Yan | 110.2k | 99.9k | 91% | 111% |
| Diona / Faruzan / Sucrose | 106.3k | 99.0k | 93% | 111% |
| Odette / Cryo Traveler / Lan Yan | 90.3k | 98.9k | 109% | 111% |
| Odette / Qiqi / Lan Yan | 103.3k | 98.5k | 95% | 111% |
| Odette / Diona / Lan Yan | 109.7k | 96.8k | 88% | 111% |
| Diona / Qiqi / Sucrose | 98.7k | 94.7k | 96% | 111% |
| Odette / Lan Yan / Nicole | 96.4k | 90.3k | 94% | 111% |
| Cryo Traveler / Faruzan / Sucrose | 76.5k | 90.0k | 118% | 111% |
| Cryo Traveler / Qiqi / Sucrose | 74.0k | 87.6k | 118% | 122% |
| Diona / Faruzan / Nicole | 102.4k | 87.3k | 85% | 111% |
| Odette / Sucrose / Lan Yan | 92.3k | 84.6k | 92% | 111% |
| Cryo Traveler / Qiqi / Nicole | 69.3k | 84.1k | 121% | 144% |
| Qiqi / Faruzan / Lan Yan | 80.7k | 84.0k | 104% | 111% |
| Cryo Traveler / Faruzan / Nicole | 78.0k | 83.5k | 107% | 111% |
| Diona / Faruzan / Lan Yan | 86.0k | 83.3k | 97% | 111% |
| Cryo Traveler / Diona / Qiqi | 70.8k | 80.6k | 114% | 133% |
| Diona / Qiqi / Nicole | 80.4k | 77.4k | 96% | 133% |
| Qiqi / Sucrose / Nicole | 74.2k | 77.3k | 104% | 133% |
| Cryo Traveler / Diona / Nicole | 71.7k | 73.3k | 102% | 111% |
| Cryo Traveler / Sucrose / Nicole | 64.9k | 71.8k | 111% | 111% |
| Diona / Sucrose / Nicole | 79.0k | 69.9k | 88% | 133% |
| Cryo Traveler / Qiqi / Lan Yan | 64.4k | 68.5k | 106% | 111% |
| Cryo Traveler / Faruzan / Lan Yan | 64.5k | 67.3k | 104% | 122% |
| Diona / Qiqi / Lan Yan | 78.7k | 65.6k | 83% | 111% |
| Cryo Traveler / Diona / Lan Yan | 61.0k | 60.9k | 100% | 111% |
| Cryo Traveler / Lan Yan / Nicole | 42.4k | 60.4k | 143% | 111% |
| Qiqi / Lan Yan / Nicole | 65.7k | 60.0k | 91% | 111% |
| Qiqi / Sucrose / Lan Yan | 58.0k | 58.3k | 100% | 144% |
| Cryo Traveler / Sucrose / Lan Yan | 50.5k | 56.6k | 112% | 111% |
| Diona / Sucrose / Lan Yan | 69.1k | 52.3k | 76% | 111% |
| Diona / Lan Yan / Nicole | 71.8k | 40.2k | 56% | 111% |
- Only team directories and this document changed, so no regression run was needed.
### Before the fork (checked 2026-10-26)

- As of the latest gcsim `main` commit (`2350be9`, 2026-10-23), the Cryo Traveler (`travelercryo`) and Faruzan are implemented. Vesna and Vodyanitsa are not in the character list.
- Both characters were released in 7.1 (2026-09-23). gcsim already has Stellar Swirl: the Stellar Vortex and the Radiance: Stellar Swirl state (`pkg/reactable/stellarswirl.go`), enabled by the Cryo Traveler and Odette. What was missing is the two characters and the Wandering Vortex, which Vodyanitsa's A1 turns Stellar Vortices into (it also drives her C2). Vesna was expected to change how Stellar Swirl works; the game's data shows she does not (she enables it as Odette does, §1). Vodyanitsa and the Wandering Vortex are on the fork since `6fcfc4c`, Vesna since `3e75b40`; the team is `tier: experimental` with `gcsim_fork` and has a draft rotation written from the KQM Vodyanitsa quick guide (§2).
- No upstream PR exists for either. They are planned for the experimental tier (design §5.4, `docs/fork.md` §6).
- This team also serves as the first test case for "teams the engine does not support yet": matched, but reported as `blocked`.
