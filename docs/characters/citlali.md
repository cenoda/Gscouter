# Citlali

> Last updated: 2026-10-29

How the library's rotations score a C6 Citlali. Citlali is in upstream gcsim; this document records measurements, not a fork implementation. Roadmap context: `docs/cycle.md` §4 and §7.2.

## 1. The report

_2026-10-29._ A community report says a C6 Citlali does not use her burst. `docs/cycle.md` §4 found that dropping it lowered Mavuika / Citlali / Xilonen / Nicole on every build, so the question is which teams it holds for.

In gcsim, C6 starts Opal Fire from her skill alone (no 50 Nightsoul points needed), so the burst is not needed for it. What the burst still brings is its Cryo (Melt, Citlali's RES shred), against its time on field.

## 2. Dropping her burst, over the library

For each of the 180 library teams whose rotation has Citlali's burst (of 221 Citlali teams; 41 never cast it), the team's rotation and the same rotation without Citlali's bursts, each scored as the site does (looped, fillers, 200 iterations, seed 1), with the library build and with that build at Citlali C6 (nothing else changed). Change from dropping the burst:

| Build | Teams | Median | Over +5% | +1% to +5% | Within 1% | Under −1% | Under −10% |
|---|---|---|---|---|---|---|---|
| The library's (Citlali mostly C0) | 180 | −14.4% | 0 | 13 | 13 | 154 | 102 |
| The library's at Citlali C6 | 180 | −3.5% | 22 | 18 | 23 | 117 | 41 |

- The report holds for some teams only, and the carry does not decide which: of 13 Gaming teams, 6 gain at C6 and their median is −14.9%; of 35 Mavuika teams, 6 gain.
- Largest gains at C6: Escoffier / Citlali / Bennett / Xianyun +31.7%, Gaming / Citlali / Nicole / Xianyun +26.2%, Mavuika / Citlali / Charlotte / Bennett +24.8%, Diluc / Citlali / Escoffier / Bennett +19.9%, Gaming / Citlali / Xianyun / Bennett +18.9%.
- Largest losses at C6: Gaming / Citlali / Chevreuse / Diona −48.8%, Mavuika / Citlali / Chongyun / Bennett −43.9%, the Arlecchino teams about −33%, Skirk / Furina / Dahlia / Citlali −31.6%.
- So the rotations stay as they are, and the burst is dropped as a variant (`docs/cycle.md` §7.2), which the score tries only for a C6 Citlali and keeps when it scores higher.

## 3. Variants in the library

_2026-10-29._ `cmd/variant -char citlali -cons 6` over the 221 Citlali teams wrote `alt/citlali-c6-no-burst/` for the 40 teams where dropping the burst gains at least 1% at C6 (gains +1.2% to +31.7%, median +5.5%; the scores are in each `notes`). `cmd/validate -promote` made 39 supported. Gaming / Citlali / Sucrose / Bennett stays draft: its looped score keeps 140% of one pass, over the 130% limit.

Mavuika / Citlali / Xilonen / Bennett has none: at Citlali C6 its own rotation stops with a gcsim error on one seed of 200 ("cannot recast E while not in nightsoul blessing"), so `cmd/variant` could not score it; at 100 iterations the variant scored −22%. That a C6 Citlali can stop this rotation is not looked into here.

A C6 Citlali player of these teams gets the variant when it scores higher; the result says the score uses the team's rotation without her burst, made for C6 Citlali.
