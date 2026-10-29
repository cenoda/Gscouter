# Wriothesley

> Witch's Revelation on the gcsim fork (`docs/fork.md` §7) · Last updated: 2026-10-29

Wriothesley (5★ Cryo, 4.1) is in upstream gcsim. His 6.7 Witch's Revelation, which lets him deal Stellar-Conduct DMG inside a Polestar Field, merged upstream after the fork's base. The fork carries it until the next rebase. His Stellar-Conduct teams are written from a guide and scored in the experimental tier (design §5.4).

## 1. On the fork

**Source.** Upstream [PR #3212](https://github.com/genshinsim/gcsim/pull/3212) (`8e0bc71` upstream, `108f379` on the fork), unchanged. Two more commits come with it: upstream's Cashflow Supervision 6.7 update ([PR #3247](https://github.com/genshinsim/gcsim/pull/3247), `6ca5a4d`), which adds 14% Stellar-Conduct DMG per stack at R1, and our fix `ee3fc92`. The upgrade is on by default (`revelation=1`).

**What changes.** Inside a Polestar Field, in his skill state:
- N3 and N5 deal 60% and 80% of their Cryo DMG as direct Stellar-Conduct DMG.
- Below 60% HP his charged attack becomes Luster: Vaulting Fist: no Stamina cost, heals 30% of Max HP at most once per 2 s, and deals 100% as Stellar-Conduct DMG.
- He deals +30% Stellar-Conduct DMG.

C1, C2, C4 and C6 each switch to a Stellar-Conduct version. Outside the field, C2 at 5 Prosecution Edict stacks multiplies enhanced normal attacks by 1.25 and charged attacks by 1.3 in his skill state.

**Our fix (`ee3fc92`).** The PR gave the out-of-field C2 bonus without the 5-stack condition that its own comment quotes. Without the fix, the two C6 library teams score about 0.8% above the fixed value (Wriothesley / Emilie / Mavuika / Xilonen: 414,172 bugged against 410,947 fixed, looped, 20 iterations). To be offered upstream.

**Known problems.** Damage he takes is not modeled (the PR's C4 `TODO`: 25% less DMG taken), and the enemy deals none, so his HP falls only through his own skill, as the KQM combo assumes.

## 2. Library teams

### Batch 1 (2026-10-29)

From the [KQM Wriothesley quick guide](https://keqingmains.com/q/wriothesley-quickguide/), Stellar-Conduct section (updated for 7.0). The community gcsim database has no Wriothesley team with Odette, Sandrone or the Cryo Traveler.

| Team | Looped | One rotation | Notes |
|---|---|---|---|
| Wriothesley / Odette / Yae Miko / Escoffier | 149.1k | 119.0k (14.8 s) | The guide's sample rotation; Wriothesley 51%, Yae Miko 25%, Odette 18%, Escoffier 6% |

Rotation: Yae Miko 3[E] > Odette 2[E] > Escoffier E > Wriothesley combo, Odette first from the second rotation on. Neither Odette nor Wriothesley bursts (the guide: Odette's burst is often a DPS loss here, and he builds no ER). His combo is E N5 2[N5CD] in the first rotation and E 3[N5CD] N2 after. In one run of the looped score (5 iterations), Luster: Vaulting Fist, N5 and N3 deal Stellar-Conduct DMG.

Builds:
- **Wriothesley:** the guide's best for this archetype. C0, Cashflow Supervision R1, 4pc Disenchantment in Deep Shadow, ATK%/ATK%/CRIT Rate (no Cryo DMG goblet: Stellar-Conduct DMG does not scale with it), KQM-standard substats.
- **Yae Miko:** from a harvested team, with the guide's 4pc Disenchantment in Deep Shadow.
- **Odette:** as in the Sandrone teams.
- **Escoffier:** from a harvested team, with the guide's 4pc Tenacity of the Millelith.

Cross-check: none found (searched 2026-10-29).
