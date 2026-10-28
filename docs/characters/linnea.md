# Linnea

> Character on the gcsim fork (`docs/fork.md`) · Last updated: 2026-10-28

Linnea (5★ Geo). Upstream gcsim: Open PR #2596 (2026-04-19). Scored in the experimental tier (design §5.4); the fork's rules and regression runs are in `docs/fork.md`.

## 1. On the fork

**Source.** Upstream PR #2596 (TheSast), open since 2026-04 with requested changes

**Frame data.** Skill (first press) and burst cancels counted by caramielle; normal attack hitmarks counted, but their cancels are Gorou's; aimed shots are the generic bow template; skill recast a placeholder (10); plunge missing

**Known problems.** The burst while Lumi is out stops Lumi's attacks and the A1 shred for the rest of its duration; C1 consumes stacks once per Lunar-Crystallize contributor instead of once; C2's extra attack does not emit Moondrift Harmony; C4 may give Linnea +50% DEF on field instead of +25%; the A4 EM buff is not refreshed on swap. Fixed on the fork: a duplicate registration that panicked at startup, frame invariants (skill animation 14 → 44, the longest cancel, as the upstream review asked; N3 animation 43 → 47, its hitmark; burst queue at the earliest cancel), C3 raising normal attacks instead of the skill, C2 skipping members after the first non-Geo/Hydro one

## 2. Library teams

Written in the first two batches with Nefer, Zibai and Iansan: see `docs/characters/first-batches.md`.
