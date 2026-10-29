# Kachina

> Character on the gcsim fork (`docs/fork.md`) · Last updated: 2026-10-28

Kachina (4★ Geo, 5.0). Upstream gcsim: Open PR #2697 (2026-07-25). Scored in the experimental tier (design §5.4); the fork's rules and regression runs are in `docs/fork.md`.

## 1. On the fork

**Source.** Upstream PR #2697 (danieldlc01), open since 2026-07 with requested changes

**Frame data.** The PR cites a frame count by caramielle. Normal attacks look counted (the review asked for a recount); charged attack estimated (every cancel 49–50); skill: counted cancels; its animation ended at 733, the time of Turbo Twirly's last independent attack, and now ends at 57, the longest counted cancel (estimated). The community confirms the skill as about 12 s of damage over time, which Turbo Twirly models on its own (143, then every 118 frames, 10 of 60 points per hit); mount/dismount recast all 30 (placeholder); burst not counted

**Known problems.** A1 gives +20% DMG to the active character on any Geo hit instead of Geo DMG to Kachina only; C1 triggers on crystallize, not shard pickup, and its shard absorption is missing; C4 is not polled; no Geo construct; Twirly loses no points while idle and loses 10 per enemy hit; swapping out while mounted can leave Twirly mounted forever; no plunge. Fixed on the fork: compile fix for current upstream, 500-frame charge cancels, Twirly's independent attacks firing every 236 frames instead of 118, the 733-frame skill animation

## 2. Library teams

None yet: her teams are scored as unsupported. The first is planned to be Kachina / Furina / Qiqi / Noelle ("Cappuccino", `docs/fork.md` §6).
