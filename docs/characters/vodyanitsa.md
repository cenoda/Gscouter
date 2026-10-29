# Vodyanitsa

> Character on the gcsim fork (`docs/fork.md`) · Last updated: 2026-10-28

Vodyanitsa (5★ Hydro, 7.1). Upstream gcsim: no PR. Scored in the experimental tier (design §5.4); the fork's rules and regression runs are in `docs/fork.md`.

## 1. On the fork

**Source.** Ours (2026-10-27): multipliers from the pipeline; mechanics from the game's ability data (`ConfigAbility_Avatar_Vodyanitsa`, `ConfigTalent_Vodyanitsa`; Vodyanitsa is also her internal name) and, for the Wandering Vortex, the Stellar Swirl abilities (`ElementAbility_StarSwirl_Ice_Gadget` in `ConfigAbility_Element`, `TeamAbility_StarSwirl_Ice` in `ConfigAbility_Team`). The Wandering Vortex is not a gadget of its own: every Stellar Vortex checks every 0.2 s whether a party member carries her Mark, which Song of Ages Past's party buff (reapplied every 1 s for 2 s) sets with A1, and then stays a Wandering Vortex until it explodes; its damage and detonation are unchanged. Turning and detonating reduce Anemo RES by 35% for 6 s (the data; GenshinBuilds and KQM say 35%, AllThingsHow and genshin-build.com 30%); while it exists and the Song is up, plus 5 s, the party carries the AlterMark, which moves Lead Vocal/Chorus (A4) and C2 from Hydro/Cryo hits to Stellar Swirl DMG (+60% CRIT DMG instead of +50%). Radiance: Stellar Swirl lasts 12 s instead of 8 s while the Mark is on. Skill, horns and burst scale with Max HP; the burst is ×(1 + 86.4%) during the Song. Lead Vocal (25, active character) and Chorus (10, the others) add flat DMG only above 40,000 Max HP (140 per 1000 to Hydro/Cryo, 260 to Stellar Swirl, at most 3,500 and 6,500), one stack per damage instance

**Frame data.** Normal and charged attacks: Sucrose's counted frames (4-hit catalyst, same body type). Skill (tap, 45 frames, swap at 30) and burst (110 frames, hit at 95) estimated: no community count exists, and the KQM quick guide calls her field time low. From the data: first hit and Song of Ages Past at the press (`onAbilityStart`), a horn every 3 s hitting 0.2 s after it appears, heals from 1.7 s every 1.5 s, Lead Vocal granted at 66% of the tap, the burst hit at 95% of the burst state, gauges and ICD. Calibration (no source gives a full build, so ratios, on the library's community Skirk teams): team DPS of Skirk / Furina / Escoffier with C0 Vodyanitsa against C1 Shenhe, [AllThingsHow](https://allthings.how/genshin-impact-how-to-build-vodyanitsa-for-skirk-freeze-teams/) 152.2k against 135.5k (×1.123), with the builds it states (C1 Azurelight Skirk, C6 Favonius / Golden Troupe Furina at 9/10/9, Scroll Escoffier, C1 Calamity Queller / Noblesse Shenhe, Vodyanitsa with R5 Hymn of the Maelstrom and Tenacity at 69,659 HP; both bursts when ready): with Hymn (fork `76e8116`) 256.1k against 182.6k, ×1.403 (+25%). Before Hymn was on the fork, Everlasting Moonglow stood in for it (65.7k HP): 214.9k, ×1.177 (+4.8%). The gap is Hymn's ATK: at R5 with three stacks in its Freeze/Stellar Swirl state it gives Skirk +84% ATK (16% per stack, the data's cap per stack, ×3 ×1.75), and her damage rises 26%. Read with the 16% as a cap on all stacks together (a test build, not the fork), the team gives 229.2k, ×1.255 (+12%), still above the source; the source's ratio is below even Moonglow's, so it seems to count little of Hymn's ATK. One damage instance, Skirk's burst hit with C0 TTDS Vodyanitsa against C0 TTDS Mona, [HoYoLAB 46785492](https://www.hoyolab.com/article/46785492) about 160k against 130k (×1.23; final hit 190k against 160k): 152.0k against 124.1k, ×1.208 (−1.9%); final hit 174.5k against 155.8k, ×1.12 (−5.7%), because Skirk spends the 25 Lead Vocal stacks on normal attacks before her burst, so the last hits get none. The source's numbers fit only without Mona's Omen on the burst (with it Mona's hits are 171.5k), so Mona's burst is left out of that comparison. Weapons: Prydwen ranks Hymn R1 103.7% and TTDS R5 101% (Skirk team), a ratio of 1.027; in the AllThingsHow team (Moonglow's substats, 300 iterations) Hymn R1 scores 235.3k against TTDS R5's 219.3k, 1.073 (+4.5%; 1.012 with the cap on all stacks). Against Prototype Amber R5 in the same team: Hymn R1 1.098, Hymn R5 1.193, TTDS R5 1.024, Sacrificial Jade R5 1.005, Everlasting Moonglow 1.002, Jadefall's Splendor 1.003, A Thousand Floating Dreams 0.966; the creator-server analysis ([HoYoLAB 46781743](https://www.hoyolab.com/article/46781743)) calls the last three on par with Prototype Amber

**Known problems.** No plunge; the out-of-combat swim is not modeled; the Wandering Vortex stays where the vortex was made (in game it wanders toward opponents). Lead Vocal on Stellar Swirl DMG goes to each contributor's share, the way the engine splits Stellar Swirl DMG, so one Stellar Swirl hit can spend up to four stacks; the game's count per reaction instance is not known. Direct Stellar Swirl hits (Odette's, Sandrone's, the Cryo Traveler's, Vesna's) spend one stack each since fork `87338fd`, as her data counts every `StarSwirlWindDamage` and `StarSwirlIceDamage` instance; before, only reaction contributions did. C6's "+60% Hydro/Cryo DMG" (`damagePercentageRatio`) is modeled as a DMG Bonus

## 2. Library teams

Two teams with Sandrone (`docs/characters/sandrone.md`, batch 4) and 22 with Vesna (`docs/characters/vesna.md`) are recorded with those characters. This section has the others: Freeze teams with a Cryo carry, then the rest.

### With Cryo carries

_2026-10-28, fork `b029c45`, all `status: draft`, 100 iterations, common conditions._

Outside Vesna and Sandrone teams, guides use Vodyanitsa mostly in Freeze teams with a Cryo carry. A survey of her team pages (searched 2026-10-28: the [KQM Vodyanitsa quick guide](https://keqingmains.com/q/vodyanitsa-quickguide/), [GenshinBuilds](https://genshin-builds.com/en/teams/vodyanitsa), [GameWith](https://gamewith.net/genshin-impact/article/show/73767), AllThingsHow's [team list](https://allthings.how/genshin-impact-vodyanitsa-best-build-and-teams-version-7-1/) and [Skirk fourth-slot comparison](https://allthings.how/skirk-s-best-fourth-slot-vodyanitsa-shenhe-mona-or-furina/), [LDShop](https://www.ldshop.gg/blog/genshin-impact/vodyanitsa-best-teams.html) and wishgm (the same text), [enjoygm](https://www.enjoygm.com/blog/genshin-impact/vodyanitsa-team-guide), [Gaming ProMax](https://gamingpromax.com/vodyanitsa-best-teams-genshin-impact/), [FandomWire](https://fandomwire.com/best-teams-for-vodyanitsa-in-genshin-impact-7-1/); Icy Veins and Prydwen return 403) found these Cryo carries with her:

- Skirk: Skirk / Furina / Escoffier / Vodyanitsa in every source (KQM: one of Skirk's best teammates in Pure Freeze, with a rotation; GenshinBuilds SS; AllThingsHow 187k). Variants: Mona for Furina (AllThingsHow), Citlali for Escoffier (enjoygm), and the substitutes LDShop lists: Yelan or Xingqiu for Furina, Shenhe, Mika or Citlali for Escoffier.
- Kamisato Ayaka: Ayaka / Furina / Escoffier / Vodyanitsa (GenshinBuilds S, enjoygm; LDShop and AllThingsHow name her as a Skirk substitute), Ayaka / Furina / Shenhe / Vodyanitsa (Gaming ProMax), Ayaka or Lohen / Escoffier / Furina or Mona / Vodyanitsa (enjoygm), and a budget team Ayaka or Kaeya / Xingqiu or Mona / Diona / Vodyanitsa (enjoygm).
- Wriothesley: Wriothesley / Furina / Escoffier / Vodyanitsa (FandomWire; KQM calls her good in his Freeze teams, without a sample team).
- Ganyu: only as a Skirk substitute (LDShop, AllThingsHow).
- Lohen: Lohen / Escoffier / Mona / Vodyanitsa (KQM, with a rotation; AllThingsHow 164k with C4 Mona) and Lohen / Furina / Prune or Escoffier / Vodyanitsa (enjoygm).
- KQM also names Freminet, Shenhe, Citlali and Rosaria as Cryo teammates, with no team.

Each of these teams with a rotation to build on has a library team, 15 in all. No guide gives a rotation for most of them, so each one starts from the library's harvested community rotation for the same carry and two of the same supports, with Vodyanitsa's skill in the slot the guides name (the KQM guide's rotations for Skirk and Lohen put her among the supports before the carry). Where she holds Thrilling Tales she is the last swap before the carry (Gaming ProMax); her burst is cast when ready (KQM: lowest priority), and so is Escoffier's (KQM's Skirk rotation alternates it; Vodyanitsa gives her fewer particles than Citlali, and the source's unconditional burst made the one pass wait for energy). The Lohen teams copy Lohen / Escoffier / Furina / Mona and keep his Freeze combo; Lohen / Escoffier / Mona / Vodyanitsa plays KQM's rotation as written. Vodyanitsa's build is her KQM one (C0, Thrilling Tales R5, 4pc Tenacity, HP%/HP%/HP%, KQM-standard substats, 111% ER), with Prototype Amber R5 where Mona or Citlali holds Thrilling Tales and 4pc Scroll of the Hero of Cinder City where a teammate holds Tenacity (the guide's alternative), or 2pc Tenacity + 2pc Vourukasha's Glow where both are held.

| Team | Built on (replaced member) | One pass | Looped (score) | Kept | Source team, looped |
|---|---|---|---|---|---|
| Skirk / Furina / Escoffier / Vodyanitsa | Skirk / Furina / Citlali / Escoffier (Citlali) | 161.2k | 160.7k | 100% | 140.2k |
| Skirk / Furina / Citlali / Vodyanitsa | same (Escoffier) | 136.8k | 138.1k | 101% | 140.2k |
| Skirk / Yelan / Escoffier / Vodyanitsa | Skirk / Yelan / Citlali / Escoffier (Citlali) | 126.4k | 125.5k | 99% | 104.8k |
| Skirk / Escoffier / Mona / Vodyanitsa | Skirk / Xingqiu / Mona / Escoffier (Xingqiu) | 127.9k | 121.3k | 95% | 97.2k |
| Skirk / Furina / Shenhe / Vodyanitsa | Skirk / Furina / Shenhe / Escoffier (Escoffier) | 126.4k | 120.9k | 96% | 137.0k |
| Lohen / Escoffier / Furina / Vodyanitsa | Lohen / Escoffier / Furina / Mona (Mona) | 106.2k | 120.9k | 114% | 98.3k |
| Skirk / Furina / Mika / Vodyanitsa | Skirk / Furina / Mika / Escoffier (Escoffier) | 107.6k | 103.8k | 96% | 108.5k |
| Skirk / Xingqiu / Escoffier / Vodyanitsa | Skirk / Xingqiu / Citlali / Escoffier (Citlali) | 103.7k | 100.1k | 97% | 108.6k |
| Lohen / Escoffier / Mona / Vodyanitsa | Lohen / Escoffier / Furina / Mona (Furina) | 108.3k | 97.3k | 90% | 98.3k |
| Wriothesley / Furina / Escoffier / Vodyanitsa | Wriothesley / Furina / Escoffier / Citlali (Citlali) | 95.4k | 95.2k | 100% | 72.5k |
| Kamisato Ayaka / Furina / Escoffier / Vodyanitsa | Kamisato Ayaka / Furina / Citlali / Escoffier (Citlali) | 73.3k | 90.7k | 124% | 81.3k |
| Ganyu / Furina / Escoffier / Vodyanitsa | Ganyu / Furina / Escoffier / Citlali (Citlali) | 76.1k | 87.3k | 115% | 71.2k |
| Kamisato Ayaka / Escoffier / Mona / Vodyanitsa | Kamisato Ayaka / Furina / Mona / Escoffier (Furina) | 67.7k | 66.5k | 98% | 79.5k |
| Kamisato Ayaka / Furina / Shenhe / Vodyanitsa | Kamisato Ayaka / Furina / Shenhe / Escoffier (Escoffier) | 66.9k | 65.9k | 98% | 106.0k |
| Kamisato Ayaka / Mona / Diona / Vodyanitsa | Kamisato Ayaka / Diona / Mona / Kaedehara Kazuha (Kazuha) | 58.3k | 57.0k | 98% | 52.0k |

The source teams' looped scores are their recorded validations (20 iterations; Lohen's 100).

- In Citlali's slot Vodyanitsa gains 15% with Skirk / Furina / Escoffier, 20% with Yelan, 12% with Ayaka, 23% with Ganyu and 31% with Wriothesley; AllThingsHow puts her about 7% above Citlali with Skirk. Against C0 Shenhe in the Skirk / Furina / Escoffier team she adds 23.7k, inside AllThingsHow's 23–25k over C2 Shenhe or C4 Mona. With Xingqiu she loses 8% (the source's Citlali bursts every rotation there).
- In Escoffier's slot she loses: 1.5% (Skirk with Citlali), 4% (Mika), 12% (Skirk with Shenhe), 38% (Ayaka with Shenhe), in line with AllThingsHow and LDShop (taking out Escoffier costs damage).
- Cross-checks: Skirk / Furina / Escoffier / Vodyanitsa is 13.8% below AllThingsHow's 187k (no builds stated; the harvested 4★ supports and a C0 Thrilling Tales Vodyanitsa here). Lohen / Escoffier / Mona / Vodyanitsa is 33.9% below AllThingsHow's 164k with C4 Mona, in line with the low Lohen scores already known (`docs/characters/lohen.md` §2). No public number exists for the other teams.
- Not added: Lohen / Furina / Prune / Vodyanitsa (no harvested team or guide gives a Prune build), the Ayaka / Xingqiu / Diona and Kaeya budget teams (no harvested rotation for that core), and Freminet, Rosaria or Citlali as carries (no source names a team).
- Only team directories and this document changed, so no regression run was needed.

### Kamisato Ayato / Citlali / Escoffier / Vodyanitsa

_2026-10-28, same fork and conditions, `status: draft`._

A Hydro carry, but a Freeze team, and the one Freeze team in the KQM Vodyanitsa quick guide without a Cryo carry; no other team page found lists Ayato with her. It plays the guide's rotation as written: Ayato Q > Escoffier E > Vodyanitsa E Q > Citlali EQ > Ayato E N15 > Escoffier Q E > Vodyanitsa E Q > Citlali E > Ayato E N15. Ayato, Citlali and Escoffier are the harvested Kamisato Ayato / Citlali / Furina / Escoffier team's; Vodyanitsa wears Prototype Amber R5 (the guide's weapon when she bursts every rotation beside a Hydro damage dealer) and 4pc Tenacity, at 155.1% ER (10 ER rolls in place of CRIT rolls), the least at which she bursts twice per rotation (the guide's double Hydro range with Amber is 145–155%; at 144.1% she waits 7 s/min for energy, and at 111% the looped score skips her bursts, −2.4%).

- Result: 68.3k one pass, 69.4k looped (kept 102%); Ayato deals 39%, Escoffier 39%, Vodyanitsa 14%, Citlali 8%. The source team with Furina in her place scores 70.0k (20 iterations), so in the sim she is about even with C0 Furina there.
- No public number for this team (searched 2026-10-28: the KQM guide gives no DPS or gcsim links; GenshinBuilds, AllThingsHow, enjoygm, FandomWire and Mobalytics list no Ayato team with her; Icy Veins and Prydwen return 403).

### Other teams

_2026-10-28, same fork and conditions, all `status: draft`, 100 iterations._

Every remaining team in the KQM Vodyanitsa quick guide (Hydro carries, Stellar Swirl and Stellar-Conduct, Bloom) and the ones other team pages add (GenshinBuilds, AllThingsHow, enjoygm, LDShop, wishgm, Gaming ProMax, GameWith), 18 in all. Where the KQM guide gives a rotation it is played as written; otherwise the rotation is the closest library team's (the same carry and two of the same supports) with Vodyanitsa in the replaced member's slot, as in the Cryo batch above. Her build follows the guide: Prototype Amber R5 where she bursts every rotation beside Hydro damage dealers or a teammate holds Thrilling Tales, Thrilling Tales R5 otherwise; 2pc Tenacity + 2pc Vourukasha's Glow beside Neuvillette and Mizuki (not ATK-scaling), 4pc Tenacity or 4pc Scroll otherwise; EM mains, Blackmarrow Lantern R5 and 4pc Flower of Paradise Lost in the Bloom team (the guide's EM build; Kokomi's weapon and set in the source team); ER raised only where the guide's rotation has her burst every rotation. Bursts the guide does not give her are cast when ready (its advice outside Hydro teams). Supports' bursts that stalled the one pass once she replaced a particle source are cast when ready, and each team's notes say which.

| Team | Rotation | Vodyanitsa | Looped (score) | Kept | Source team, looped |
|---|---|---|---|---|---|
| Sandrone / Alyosha / Cryo Traveler / Vodyanitsa | Sandrone / Alyosha / Odette / Vodyanitsa's, Traveler for Odette (enjoygm) | TTDS, Tenacity | 146.0k | 99% | 182.2k (with Odette) |
| Sandrone / Yae Miko / Cryo Traveler / Vodyanitsa | Sandrone / Yae Miko / Odette / Vodyanitsa's, Traveler for Odette (enjoygm) | TTDS, Tenacity | 130.4k | 108% | 175.2k (with Odette) |
| Cryo Traveler / Yae Miko / Odette / Vodyanitsa | KQM, as written | TTDS, Tenacity | 122.8k | 112% | – |
| Varka / Prune / Mona / Vodyanitsa | KQM, as written (Varka's combo from Varka / Prune / Mona / Furina) | Amber, Tenacity | 119.8k | 100% | 116.2k (Furina for Vodyanitsa) |
| Neuvillette / Furina / Escoffier / Vodyanitsa | KQM, as written | Amber, 2+2, 133.1% ER | 111.5k | 107% | 89.3k (Neuvillette / Furina / Xilonen / Escoffier) |
| Neuvillette / Furina / Xilonen / Vodyanitsa | KQM, as written | Amber, 2+2, 133.1% ER | 109.3k | 109% | 89.3k; with Ineffa 94.3k |
| Sandrone / Odette / Sucrose / Vodyanitsa | Sandrone / Odette / Sucrose / Cryo Traveler's, Vodyanitsa for the Traveler (LDShop, enjoygm, AllThingsHow) | Amber, Tenacity | 108.0k | 89% | 95.9k |
| Varka / Prune / Furina / Vodyanitsa | Varka / Prune / Mona / Furina's, Vodyanitsa for Mona (GenshinBuilds, AllThingsHow) | TTDS, Scroll | 107.8k | 101% | 116.2k |
| Neuvillette / Xilonen / Kazuha / Vodyanitsa | KQM, as written | Amber, 2+2, 155.1% ER | 95.5k | 116% | 94.4k (Furina for Vodyanitsa) |
| Lohen / Furina / Prune / Vodyanitsa | Lohen / Escoffier / Furina / Vodyanitsa's, Prune for Escoffier (enjoygm) | TTDS, Scroll | 91.5k | 92% | 120.9k (with Escoffier) |
| Mizuki / Odette / Cryo Traveler / Vodyanitsa | KQM, as written | TTDS, 2+2 | 82.6k | 151% | – |
| Chasca / Furina / Mavuika / Vodyanitsa | KQM, as written | TTDS, 2+2 | 81.0k | 103% | 79.4k (Chasca / Furina / Mavuika / Bennett) |
| Chasca / Mavuika / Bennett / Vodyanitsa | Chasca / Furina / Mavuika / Bennett's, Vodyanitsa for Furina (enjoygm) | TTDS, Tenacity | 67.2k | 98% | 79.4k |
| Nilou / Lauma / Nahida / Vodyanitsa | KQM, as written (Bloom) | Blackmarrow Lantern, Flower of Paradise Lost, EM | 67.1k | 102% | 78.0k (Kokomi for Vodyanitsa) |
| Mizuki / Odette / Sucrose / Vodyanitsa | KQM's Mizuki rotation, Sucrose for the Traveler (enjoygm) | Amber, 2+2 | 60.0k | 125% | – |
| Chasca / Iansan / Bennett / Vodyanitsa | Chasca / Iansan / Bennett / Furina's, Vodyanitsa for Furina (enjoygm) | TTDS, Tenacity | 52.5k | 92% | 57.8k |
| Mizuki / Cryo Traveler / Diona / Vodyanitsa | KQM, as written (limited roster) | TTDS, 2+2 | 43.2k | 168% | – |
| Hydro Traveler / Mona / Albedo / Vodyanitsa | KQM, as written | Amber, Tenacity | 37.4k | 98% | – |

ER is 111% unless stated. "2+2" is 2pc Tenacity + 2pc Vourukasha's Glow.

- The two KQM Mizuki teams keep 151% and 168% of their one pass, outside the 80–130% range. The one pass is a single weak first rotation (the guide skips the Traveler's N1C Q there, and Stellar Swirl has not built up); three rotations from full energy keep 91% and 102%.
- Cross-checks against AllThingsHow's tested numbers (no builds stated; C0 4★ supports here): Neuvillette / Furina / Xilonen / Vodyanitsa −24.8% one pass from 133k ("Neuvillette core"), Varka / Prune / Furina / Vodyanitsa −32.6% from 159k, Mizuki / Odette / Cryo Traveler / Vodyanitsa −70.8% from 187k, Sandrone / Odette / Sucrose / Vodyanitsa −50.7% from 247k (with the fork's −11.9% Sandrone calibration gap). AllThingsHow says she adds 1–5k over Ineffa for Neuvillette; here she adds 15k (+16%). No public number for the others.
- Not added, no rotation to build on: Chasca / Odette / Nicole / Vodyanitsa (GenshinBuilds SS; no Chasca Stellar Swirl rotation anywhere, no library team with Chasca and Odette); Sucrose / Cryo Traveler / Diona / Vodyanitsa (GameWith's F2P team, a general support order only); enjoygm's starter Stellar teams (Mizuki or Anemo Traveler / Kaeya or Rosaria / Sucrose / Vodyanitsa; no rotation, and in the fork Kaeya and Rosaria do not enable Stellar Swirl); the budget Freeze teams Ayaka / Xingqiu / Diona and Kaeya / Xingqiu or Mona / Diona with Vodyanitsa (no library team shares two supports, no Kaeya Freeze rotation found).
- Only team directories and this document changed, so no regression run was needed.
