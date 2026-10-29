# First written teams

> Status: all `draft` · Last updated: 2026-10-28

The first rotations written from community guides (design §5.4, Rotations): teams for Nefer, Linnea and Zibai on the fork, and for Iansan, an upstream character the community database has no entry for. Later characters' teams are recorded in their own documents in `docs/characters/`.

## 1. First experimental teams

_2026-10-27, fork `91c5de4`, all `status: draft`._

Chosen from community guides and showcases, one or two per character, preferring teams with a published rotation. A first version used the KQM quick guides' second-choice teams with 4-star weapons and KQM-standard artifacts, the level of the database's lowest-investment entries, and scored 95k (Nefer / Lauma / Aino / Nahida), 127k and 146k. Community numbers for these characters are about 200k for Nefer teams and 240k for Zibai's, so the teams and builds were checked against them:

- The damage formula holds: a Phantasm Performance shade hit, traced in gcsim's debug log, gets Pale Hymn, the Moonsign base bonus, NotSU, SMS and the Dendro shred, and follows gcsim's direct Lunar-Bloom formula.
- The gap was the builds and the team. The strongest Nefer team by community account (below) with its own rotation scored 127.9k one pass with KQM-standard artifacts and Blackmarrow Lantern, and 179.3k with Dawning Frost R5 and artifacts well above KQM standard, against 183k in game. Zibai's team went from 137.6k to 235.9k with the community's conditions (C2 Columbina with her signature, Zibai's signature, strong artifacts).

So the validation builds (`build.txt`) are now at the level of each team's cross-check source, and `cross_check` compares like with like as far as the sources tell. A visitor's score uses their own build in any case.

| Team | Why | Rotation source | One pass | Looped (score) | Waits |
|---|---|---|---|---|---|
| Nefer / Lauma / Columbina / Sucrose | Called the strongest Nefer team by a C0R0 showcase (183k in game) that gives the rotation | [HoYoLAB 44026962](https://www.hoyolab.com/article/44026962); same combo as the [Nefer quick guide](https://keqingmains.com/q/nefer-quickguide/) | 179.3k | 168.7k | none above 2 s/min; Nefer's skill cooldown filled with attacks |
| Nefer / Linnea / Lauma / Columbina | The [Linnea quick guide](https://keqingmains.com/q/linnea-quickguide/)'s team with Nefer: covers both characters | Linnea quick guide | 150.1k | 166.5k | Linnea 10.6 s/min for her 18 s skill |
| Zibai / Linnea / Columbina / Gorou | The Linnea guide's first team; Gorou rather than Illuga (185% ER at C0; the guides call C6 Gorou close) | [Zibai quick guide](https://keqingmains.com/q/zibai-quickguide/) and the Linnea guide | 235.9k | 252.4k | none |

1000 iterations, common conditions; no energy waits, no failed actions, no stalls. Rotation notes:
- Nefer's charged attacks are held (`charge[hold=150]`, Slither) until Verdant Dew turns them into Phantasm Performance, as the guides say she may wait for Dew.
- Lauma holds her skill only with Dew, checked after her burst: checked before it, Columbina's charged attack can spend the Dew in between, and gcsim rejects the hold.
- Linnea's burst is never used: the guides use it only for healing, and on the fork a burst while Lumi is out stops Lumi's attacks (the known problem above). Her Ultimate Power Form (the guide's `5[E]`) is one press and three recasts in gcsim; a fifth press would wait for the cooldown.
- Zibai's Spirit Steed's Stride needs 70 Radiance: the rotation attacks until she has it (in the checks she always had it after N4).

### Cross-check

_Searched 2026-10-27; recorded in each `team.yaml`._

- Nefer / Lauma / Columbina / Sucrose: 183k in game with C0R0 builds the author calls top 5% (200k with top 1% stats); 179.3k one pass here, −2.0%. The post gives no artifacts, so ours are an approximation.
- Zibai: an arca.live player reports about 200k at C0, 240k at C1 and 310k at C2, with C2 Columbina and her signature, in Zibai / Illuga / Columbina + Zhongli or Gorou (seen through search results; the page returns 403). Ours, with Linnea for Illuga, is 235.9k one pass at C0: for scale only. With the same builds, Zibai / Illuga / Linnea / Columbina scores 214.9k one pass with C0 Illuga, 226.6k at C2 and 257.7k at C6; the Gorou team is 91.5% of the C6 one (Prydwen: 85% of the best team), and KQM says C6 Gorou performs like a pre-C4 Illuga. Checked on the way: Illuga's buff is an additive bonus applied after the EM and reaction multiplier, as the wiki's direct Lunar formula has it, so it is small next to a Spirit Steed's Stride (about 3.9k of 344k); Illuga's team gives up the Ascendant Gleam bonus that a non-Nod-Krai member such as Gorou grants; and his burst counted the three Moondrifts present at cast (36 stacks).
- Nefer / Linnea / Lauma / Columbina: no public number. The KQM quick guides and TCL publish none, the community database has no entry with these characters, and Prydwen returns 403.

### Found while writing them

_Reported, not fixed._

- Upstream gcsim: Illuga's burst is meant to gain 5 stacks per Geo construct created during it, but its `OnConstructSpawned` handler type-asserts `construct.Construct` while the event carries `*construct.Construct`, so it never fires. Constructs present at cast are counted, as in these teams' rotations, so their scores are unaffected.
- Fork: Nefer's Phantasm Performance checks the charged attack only for its first hit; the later hits are queued at fixed delays and land even when a dash or swap cancels it. The Nefer guide says a dash cancel loses Nefer's second hit, so dash-cancelled rotations keep it on the fork: about 1.5% of a Nefer team's damage.

## 2. Second batch

_2026-10-27, all `status: draft`, 100 iterations, common conditions._

Written from the example teams of the KQM quick guides, so that every character in `docs/fork.md` §6 with an implementation and every upstream character the database lacks has teams. Builds: the Nefer, Lauma, Linnea, Zibai and Columbina builds of the first teams verbatim, so teams with the same carry compare; new supports at C0 (C6 for common 4-stars) with 4-star weapons. Iansan's teams copy each carry's and support's build from a harvested library team with that character, named in its `build.txt`. No team has a public DPS; each `cross_check` note says what was searched and gives the closest library team for scale. A reviewer who plays the characters judged the Nefer and Linnea numbers normal for C0 (higher constellations would score more; a visitor's score uses their own build).

| Group | Teams | Looped (score) | Notes |
|---|---|---|---|
| Zibai / Linnea | with Illuga + Columbina, Xilonen + Columbina, Columbina + Ineffa, Illuga + Aino, Illuga + Yelan, Illuga + Xingqiu | 130.7k–224.8k | Zibai quick guide rotations; Illuga C0 at 204% ER, bursts every rotation. Without Columbina the teams lose about 40% |
| Linnea | Navia / Columbina / Furina, Itto / Gorou / Columbina, Noelle / Aino + Gorou or Geo Traveler | 44.8k–87.5k | Linnea quick guide; Linnea's burst replaced by her tapped skill in the Navia team (the fork problem above). Linnea out-damaging Itto is expected |
| Nefer | Aino / Lauma + Kuki, Zhongli, Nahida or Sucrose; Kokomi / Lauma / Nahida; Furina / Lauma + Ineffa or Kuki; Aino / Nahida / Kuki; Aino / Collei / Kuki | 85.7k–139.9k | The guide gives no per-team rotation: the first Nefer team's pattern with the guide's `E 3[CD] E CD CQ CD`. Without Columbina's Moonridge Dew, one pass from a standing start gets about one Phantasm Performance and the loop carries Dew over, so the looped score keeps 105–184% of one pass |
| Iansan (upstream, no database entry) | Mavuika / Citlali + Bennett or Xilonen; Kinich / Mavuika + Bennett or Emilie; Varesa / Furina / Xianyun; Varesa / Chevreuse / Mavuika; Clorinde / Chevreuse / Mavuika; Chasca / Bennett / Furina; Gaming / Bennett / Citlali; Flins / Aino / Sucrose | 55.2k–82.3k | Iansan quick guide order (her `ECQ` is `skill, charge, burst`), carry combos in the form of the closest harvested config. Movement reported where the guide says the carry keeps Iansan's buff: see below |

- Iansan's burst restores Nightsoul points from the distance the active character moves (0.67 points per metre, counted every second; her points drain 6 per second), which gcsim learns only from `movement(n)` calls or `movement=` action params in the rotation. No community config or guide gives distances, so the rotations report none, and her ATK buff falls to its weaker rate (below 42 points) a few seconds after her skill. Its effect is small: with every action reporting a constant movement speed (an `execute_action` wrapper calling `movement(speed × elapsed time)`), four of the teams (20 iterations) score 1–2% more at 3 m/s, 4–5% at 6 m/s and 5–6% at 9 m/s, where the gain saturates (Mavuika / Citlali / Iansan / Bennett 63.1k → 68.4k, Varesa / Chevreuse / Iansan / Mavuika 82.3k → 86.3k). So these teams understate Iansan by at most about 6%. Against the library's teams with the same carry they rank near the middle (Varesa / Chevreuse / Iansan / Mavuika 30th of 57 Varesa teams, the Clorinde team 33rd of 144); comparisons with a single harvested team mostly reflect that team's investment, since library builds range from C0 to full constellations. The rotations follow the guide's statements on movement: it says Varesa and Gaming keep Iansan's higher buff easily, Chasca while she keeps moving and Flins thanks to his combo, so those five teams wrap `execute_action` to report movement at 9 m/s since every previous action, the lowest speed tried (6, 9, 12) above which the score stops rising, so the buff stays up. The speed stands for the guide's statement, not a measurement. For Mavuika the guide has no statement; a reviewer who plays the characters says she keeps the buff nearly always at its maximum. The Kinich quick guide says Kinich easily keeps Iansan's points high, but that Bennett locks him into his circle. So the other five teams report 9 m/s only for the time after Mavuika's actions, and after Kinich's in his Emilie team (a wrapper that checks the previous action's character against `.keys.char`): Clorinde, whom the guide says does not keep the buff before Iansan's C4, and Kinich beside Bennett report none. Kinich / Mavuika / Iansan / Bennett is unchanged, since there Mavuika acts only before Iansan's burst. Looped scores: Mavuika / Iansan / Citlali / Bennett 64.0k, with Xilonen 69.0k, Kinich / Mavuika / Iansan / Emilie 76.5k, Kinich / Mavuika / Iansan / Bennett 63.0k, Clorinde 82.2k. Looped scores with movement: Varesa / Chevreuse / Iansan / Mavuika 86.3k, Varesa / Iansan / Furina / Xianyun 70.0k, Gaming 70.0k, Flins 65.9k, Chasca 57.8k. A measured speed per carry, or a community convention, would replace the assumption.

## 3. Added on request: Nefer / Lauma / Columbina / Nahida

Nefer / Lauma / Columbina / Nahida, with the rotation the Lauma and Columbina quick guides both give for it, `(Nefer E) > Columbina E Q > Nahida E Q > Lauma hE Q > Nefer E 3[CD] E 3[CD] (Q)`. Nefer, Lauma and Columbina as in the Sucrose team (Lauma with 4pc Gilded Dreams, since Nahida holds Deepwood), Nahida Widsith R5 and 4pc Deepwood. 140.5k one pass, 165.3k looped (1000 iterations), against 168.7k for the Sucrose team, which the community calls the strongest. No public number for this team.

## 4. Added on request: Nilou

_All draft; a request: Nefer / Lauma / Columbina / Nilou is played, and Nilou can take Columbina's place in Nefer teams._

Rotations from the KQM Nilou quick guide, which lists Nilou / Nefer / Lauma / Columbina, Nahida or Sucrose with a rotation each; Nilou plays `E N2 E`, C0 with Favonius Sword R5, since the guide says her stats matter little and she needs no ER. The Linnea team has no guide: the Linnea guide's rotation with Nilou's `E N2 E` in Columbina's place. Looped (1000 iterations), next to the Columbina version at the same Nefer build:

| Team | Looped | With Columbina instead |
|---|---|---|
| Nefer / Lauma / Columbina / Nilou | 156.3k | — |
| Nefer / Lauma / Nilou / Sucrose | 113.2k | 168.7k |
| Nefer / Lauma / Nilou / Nahida | 104.9k | 165.3k |
| Nefer / Linnea / Lauma / Nilou | 93.9k | 166.5k |

- Nefer's Seeds of Deceit take every Dendro Core, so Nilou makes no Bountiful Cores, as the guide says.
- Replacing Columbina costs 32–44%. Columbina deals about 12–15% of her teams' damage herself, and her Moonridge Dew adds to the Verdant Dew: in a 120 s fight Nefer gets 39 Phantasm Performances with Columbina and Nahida, 32 with Nilou and Nahida.
- Nilou's A1 EM buff (+100 when a member under Golden Chalice is hit by Dendro attacks) comes from Dendro Cores bursting on the party; with the cores turned into Seeds it is rare (about 1 shade hit in 8 has it). Whether the game behaves the same is not checked. Golden Chalice is off with Sucrose or Linnea in the team.
- No public number for any of them.
- Nilou's constellations (looped, 500 iterations, validation builds otherwise unchanged): in Nefer / Lauma / Columbina / Nilou, C0 156.3k, C2 177.2k, C4 175.8k, C6 177.6k, against 165.2k with Nahida in her place: Nilou overtakes Nahida from C2, whose Hydro and Dendro RES shred works only under Golden Chalice (all members Hydro or Dendro). With Nahida instead of Columbina: C0 104.9k, C4 120.8k, C6 121.8k. With Sucrose (no Golden Chalice): 113.2k, 113.6k, 114.8k. With Linnea: 93.9k, 82.0k, 83.3k, already 81k at C1, whose 18 s Tranquility Aura (12 s before) lets Linnea's Geo turn Nilou's Hydro into Lunar-Crystallize (20 reactions per fight against 1) and Nefer's damage falls from 72k to 58k. Columbina's versions stay well ahead in every team.
