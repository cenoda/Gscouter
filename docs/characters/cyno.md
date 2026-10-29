# Cyno

> Witch's Revelation on the gcsim fork (`docs/fork.md` §7) · Last updated: 2026-10-29

Cyno (5★ Electro, 3.1) is in upstream gcsim. His 6.7 Witch's Revelation, which lets him deal Stellar-Conduct DMG inside a Polestar Field, is in an open upstream PR. The fork carries it, and his Stellar-Conduct teams are written from guides and scored in the experimental tier (design §5.4).

## 1. On the fork

**Source.** Upstream [PR #3250](https://github.com/genshinsim/gcsim/pull/3250) (open), squashed under its author (`fa4cd10` on the fork). The upgrade is on by default (`revelation=1`).

**What changes.**
- Inside a Polestar Field, Judication fires Duststalker Bolts - Starsame instead: 200% of ATK plus 600% of EM (A4) as direct Electro Stellar-Conduct DMG, against 100% plus 250% outside the field. [Icy Veins](https://www.icy-veins.com/genshin-impact/cyno-profile-talents-constellations) and [Prydwen](https://www.prydwen.gg/genshin-impact/characters/cyno) give the same numbers. The PR's comment on `a4BoltSSC` still says 250%, but its code uses 600%, which matches them. A pre-release leak said 80%.
- In or out of the field, casting his skill outside Pactsworn Pathclearer grants 6 s of it. That state cannot be extended.
- C1 to C6 change inside the field (C1's Together We Rise: +200 EM, passed to the next active character; C2: +16% Stellar-Conduct DMG per stack, up to 5).

**Known problems.** The 6 s Pactsworn Pathclearer queues A1 at 234 frames, which the PR marks as `TODO`. The upgrade also changed existing teams (`docs/fork.md` §2, the result for `eb21884`): two pre-6.7 rotations break with it and keep `revelation=0`.

## 2. Library teams

### Batch 1 (2026-10-29)

Every published Cyno Stellar-Conduct team with a rotation, searched 2026-10-29:
- The [KQM Odette quick guide](https://keqingmains.com/q/odette-quickguide/) (updated for 7.0) has two: Odette / Cyno / Yae Miko with Qiqi or Alyosha. It calls them one of Cyno's strongest options. KQM has no Cyno quick guide for Stellar-Conduct.
- [GameWith's Best Teams For Cyno](https://gamewith.net/genshin-impact/article/show/38317) (updated 2026-10-28) has one: Cyno / Odette / Qiqi / Nicole.

GenshinBuilds and AllThingsHow name Cyno / Odette / Ineffa / Columbina and Cyno / Cryo Traveler / Alyosha / Diona without a rotation, so they have no team yet. The KQM Qiqi quick guide and GameWith's buff page call Cyno and Sandrone a poor pair: both need the field.

| Team | Source | Looped | One rotation | Notes |
|---|---|---|---|---|
| Cyno / Odette / Yae Miko / Qiqi | KQM | 89.6k | 84.1k (29.0 s) | Cyno 55%, Yae 22%, Odette 17%, Qiqi 6% |
| Cyno / Odette / Yae Miko / Alyosha | KQM | 101.3k | 100.7k (27.4 s) | Cyno 63%, Yae 19%, Odette 15%, Alyosha 4%. In the one pass, Cyno waits 4.5 s/min for his skill in the 4[N4D N4E] loop; the looped score fills that wait |
| Cyno / Odette / Qiqi / Nicole | GameWith | 67.3k | 78.2k (30.1 s) | Cyno 63%, Odette 28%, Qiqi 7%, Nicole 2%. With no other Electro, Cyno's bursts wait 0.4 s/min for energy |

All results: 100 iterations, fork `eb21884`. In a 20-iteration run of the Qiqi team, Starsame bolts made 77% of Cyno's damage.

**Rotations.**
- KQM: Yae 3[E] > Odette 2[E] > Qiqi E(Q) or Alyosha Q E > Cyno E N1 Q N2E 4[N4D N4E]. Qiqi bursts only at C6, and Odette's burst is skipped (the guide: it tends to extend rotations).
- GameWith: Odette E (Q) > Qiqi E Q > Nicole E Q > Cyno E Q, then normal attacks with his skill whenever Judication is ready (the `cyno-a1` mod), while Pactsworn Pathclearer lasts.
  - GameWith's step says "Use her Skill". Odette casts it twice, as in KQM's 2[E], because her second skill places her Dance Double. With one skill she deals 3.4% of the damage, and the team scores 59.6k against 67.8k (20 iterations).
  - Her optional burst is skipped, as KQM advises.
  - Cyno at 4 to 8 ER rolls in place of CRIT DMG scored lower (64.1k to 62.2k), so he keeps the standard 2.

**Builds.**
- **Cyno:** C0, Staff of the Scarlet Sands R1, 4pc Disenchantment in Deep Shadow, EM/EM/CRIT Rate, KQM-standard substats. These are the best weapon, set and main stats in the GameWith, GenshinBuilds and AllThingsHow guides; the KQM Odette guide gives no Cyno build.
- **Yae Miko, Odette, Qiqi (C6) and Alyosha (C6):** as in the Sandrone teams.
- **Nicole:** as in the Vesna teams, the most common harvested build (C0, Oathsworn Eye R5, 4pc Celestial Gift). GameWith suggests Angelos' Heptades and Noblesse Oblige.

**Cross-check:** none found. The guides give no DPS, and the community gcsim database (6,021 entries) has no Cyno team with Odette, Sandrone or the Cryo Traveler.
