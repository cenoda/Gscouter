# Nefer

> Character on the gcsim fork (`docs/fork.md`) · Last updated: 2026-10-28

Nefer (5★ Dendro, 6.1). Upstream gcsim: Open PR #3045 (2026-09-20), no review yet; an earlier draft, #2587, was closed. Scored in the experimental tier (design §5.4); the fork's rules and regression runs are in `docs/fork.md`.

## 1. On the fork

**Source.** Upstream PR #3045 (dersario), open since 2026-09, no review

**Frame data.** The PR cites a frame count by caramielle; normal attack, charged attack, skill and burst numbers are irregular and match no other catalyst, so counted. Slither's stamina drain and movement, the Seed absorb radius (6) and the Slither entry (0 frames) are assumptions

**Known problems.** Changes Verdant Dew generation for every team (below); no hitlag or poise; documented hitboxes differ from the code; Phantasm and charged hits are mostly 25U with no ICD; `.nefer.veilStacks` and `.nefer.phantasmCharges` are documented but do not resolve; C4 re-adds its resistance shred to every enemy every frame (slow); Slither moves the player. Fixed on the fork: N4's animation (51 → 63, its walk cancel), plunges with no frames (removed: Nefer cannot plunge), the burst's Veil bonus expiring before either hit

## 2. Library teams

Written in the first two batches with Linnea, Zibai and Iansan, and on request (with Nahida, with Nilou): see `docs/characters/first-batches.md`.
