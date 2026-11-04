# Other Calculators

> Status: Research · Last updated: 2026-11-04

Other Genshin Impact damage calculators and simulators: what each does well, what Gscouter does not have yet, and where Gscouter is behind. The purpose is to learn from tools that solved problems before Gscouter did. It is not a ranking of those tools.

Method: web searches and the public pages and repositories linked in §7, read on 2026-11-04. Statements the sources do not support are marked *unverified*. Where a site could not be read, the document says so. Screenshots of other tools are not stored in this repository. Their content is described here in text and credited to its author.

## 1. LUNABASE Simulator

LUNABASE Simulator, by Lunnoa (X: `@lunnoa1013`), comes from the same author as the Japanese fan database lunabase.wiki. It was announced on 2026-06-20, with updates 1.0.1 (2026-08-19) and 1.1 (2026-08-25). The owner was told that it is the simulator most used by players in Korea. That claim is *unverified*: Korean-language searches on 2026-11-04 found almost nothing about it. lunabase.wiki offers Japanese, English and Chinese, and the simulator's own languages were not checked.

### 1.1 What it does well

- **One input, the whole calculation.** The announcement says: enter characters, weapons, artifacts, constellations and the rotation, and the damage is calculated automatically. It also offers weapon, team and constellation comparisons, plus a mode that uses the artifacts a player owns, read from their UID.
- **A published artifact standard with an energy model behind it.** The simulator offers the KQM standard and its own LBS standard, which a player can switch between in its settings. LBS is documented in full on lunabase.wiki/gi/lbs:
  - **Fixed rolls:** 20, two in each of the ten substats.
  - **Free rolls:** 22 (KQM has 20), at most 3 per piece for one substat (KQM has 2). A substat that matches a piece's main stat cannot roll on that piece, so the main stats chosen set the real limits.
  - **ER first:** the Energy Recharge needed to cast every burst in the rotation on schedule is computed before anything else. The page says it computes "particle generation, receipt, flat restoration and burst cost over the whole rotation". The needed ER gets the free rolls it requires. Only when ER would fall short does the limit rise to 5 rolls per piece for ER.
  - **The rest maximizes damage:** the remaining free rolls go to the combination with the highest damage per rotation (DPR).
  - **4★ sets:** 14 free rolls, at 0.84 of a 5★ roll's value.
  - **Stated aim:** a uniform build that is reachable in the current game, not a theoretical maximum.
- **Results made to be shared.** A result image the owner received (2026-11-04) has the following:
  - Five teams side by side, each as a percentage of the first.
  - Each character's constellation, refinement and ER.
  - Damage stacked by character.
  - The rotation's length (e.g. 39.08 s) and the rotation itself in compact notation (`E`, `Q`, `CA`, `2E`, swaps).
  - The standard used (KQM) and the author's credit.

  It answers "which of these options is better, and by how much" at a glance.

### 1.2 What Gscouter can learn from it

- **Standard artifacts make options comparable.** Gscouter scores the artifacts a player actually has. That answers "whose team is better built" but not "which weapon or constellation is worth it". LUNABASE answers the second question with a standard that anyone can read and check. An assumed build (`docs/pull.md` §2) could use the same approach: an explicit roll budget, ER solved from the rotation, the rest searched for damage.
- **Required ER as a number.** Gscouter shows a shortfall only afterwards, as seconds per minute of waiting. LUNABASE states the ER the rotation needs. Players can act on that number directly.
- **Comparison as the main output.** A sweep over weapons, constellations or teams, each shown relative to a baseline, is something players share. Gscouter's versus view compares two teams only.

### 1.3 Not known

The sources do not say:
- how damage is computed (expected value or random iterations, frame by frame or not), or whether hitlag, auras and reaction ICD are modelled;
- how many characters are supported;
- how fast a result is;
- what the site is built with.

lunabase.wiki is plain HTML and JavaScript. The simulator's own address was not found, so none of this could be checked.

## 2. Other tools

| Tool | What it is | Built with | What it does well |
|---|---|---|---|
| gcsim web app (gcsim.app) | The community's frame-level Monte Carlo simulator. Gscouter's engine, as a library | Go; WebAssembly in the browser | Accepted as the standard for team rotations. DPS distribution, a frame-by-frame log of a run, a community database of configs |
| ClickySim (Chrome extension) | Runs gcsim inside Genshin Optimizer's teams page | gcsim as WebAssembly, on the device | Joins an optimizer's artifact inventory to a rotation simulator. Version 0.0.3, 2026-05-09 |
| Genshin Optimizer (frzyc.github.io/genshin-optimizer) | Artifact optimizer with teams and loadouts | TypeScript, React, MUI, nx monorepo; MIT | Real-time optimization over the player's own inventory. The GOOD import format, which Gscouter also reads. Its roadmap marks these as done: Teams + Loadouts, an optimal substat solver for its theorycraft mode, an artifact-upgrade damage probability calculator |
| Mona Uranai / 莫娜占卜铺 (mona-uranai.com) | Chinese damage calculator and artifact optimizer | Rust compiled to WebAssembly with wasm-pack, Vue front end; MIT | Virtual-artifact optimization, heuristic search, automatic artifact assignment across a team, substat gain curves, artifact potential and score, statistics of weapons and artifacts from users' results. The closest match to gscouter-engine's own stack (Rust in the browser) |
| Akasha System (akasha.cv) | Leaderboards of players' showcase builds | *Not read: the site returned 403* | Ranks real builds against other players' builds of the same character. A third-party guide says it reads showcase data and calculates without team buffs (*unverified*) |
| げんかるく / gencalc (asagume.github.io/gencalc) | Japanese damage calculator | Vue 3, TypeScript, vue-i18n; MIT | A team manager with an ER calculator, a rotation visualizer, crit target calculation, Enka.Network import, Google Drive sync |
| 原神シミュ2 (ii.gensim.work) | Japanese team damage calculator | SolidJS, Tailwind (from its asset names); ad-supported | Artifact input by OCR, team input |
| 原神ダメージ (damage.paimon.app) | Japanese damage calculator | Vue with Vuetify (from its styles) | Build and artifact management, damage compared as graphs |

Enka.Network is not a calculator. It is the source of showcase data for Gscouter and most of the tools above.

## 3. What Gscouter does not have

| Capability | Who has it | Where it would go in Gscouter |
|---|---|---|
| Substat solver: a roll budget, ER solved from the rotation, the remaining rolls searched for damage | LUNABASE (LBS, KQM), Genshin Optimizer (theorycraft mode), Mona Uranai | `docs/pull.md` §2.2: "substat quality" in assumed builds; `docs/artifacts.md` |
| Builds the player does not own, entered freely | LUNABASE, gcsim, gencalc, Mona Uranai | Pull a character (`docs/pull.md`; presets done, the rest not started) |
| Sweeps: many weapons, constellations or teams scored against one baseline | LUNABASE, Genshin Optimizer | None yet. Would sit on assumed builds |
| Required ER for a rotation, as a number | LUNABASE, gencalc | None yet. Gscouter reports waits only |
| A compact rotation notation in results and shared images | LUNABASE | Cycle (`docs/cycle.md`). The result page already has a timeline with action chips |
| Artifact input by OCR | 原神シミュ2, scanners that write GOOD | Covered indirectly by GOOD import |
| Artifact upgrade and substat gain analysis | Genshin Optimizer, Mona Uranai | Artifact score, Artifact resin expectation (roadmap) |
| Leaderboards of real builds | Akasha | Arena (roadmap ring 2) |

## 4. Where Gscouter is behind

- **Time to a result.** A score takes about 11–14 s in the browser (`docs/speed.md` §1). A formula-based calculator answers almost at once. That LUNABASE is such a calculator is *unverified* (§1.3). gscouter-engine shortens this only for the teams approved for it (`docs/engine.md`).
- **Teams outside the library.** A team with no library rotation gets no score unless the player writes a rotation (`docs/cycle-swap.md`). Tools that take a rotation as input have no such limit.
- **Numbers players already know.** Community numbers are one rotation from full energy, usually under the KQM standard. Gscouter's score is a looped 150–450 s window on the player's own artifacts, so the same team shows a different number. Gscouter computes the one pass (design §3.1), but it is not the headline number. A player who compares the two sites can read the difference as an error.
- **Rotations by constellation.** Gscouter scores every build with one rotation per team and under-scores higher constellations (roadmap, "Next"; `docs/cycle.md` §3). A constellation comparison is only as good as the rotation used for each constellation.
- **Reach.** LUNABASE's results circulate as images with the author's name on them, through an author with a following. Gscouter has image sharing, but no image that compares several options, and no author presence.

## 5. Where Gscouter takes a different approach

These are deliberate choices (design §2), listed so that adopting a lesson does not undo them by accident:

- **Frame-level simulation.** Scores come from gcsim's frame-level random fights, or from gscouter-engine within 1% of them. Hitlag, auras, reaction ICD and crit variance are modelled.
- **ER sustained over a looped rotation.** Missing ER shows as waiting, which fillers then handle (`docs/fillers.md`). Nothing is solved ahead of time.
- **The player's real builds, in a fixed library rotation.** Rotations are validated against their community sources (design §5.3).
- **15 languages, open source (AGPL-3.0), no ads or accounts.**

## 6. Order to take the lessons in

1. Standard artifacts for assumed builds: an explicit roll budget with ER solved first, as LBS does, run on Gscouter's simulator. This also gives a Gscouter number that can be compared with community numbers.
2. Sweeps over weapons and constellations with a baseline percentage, and a shareable image of them. This depends on 1 and on rotations by constellation.
3. Required ER and one-pass DPS beside the score on the result page.

Each of these needs many simulations per answer, so gscouter-engine's speed is what makes them affordable.

## 7. Sources

Read on 2026-11-04.

- LUNABASE Simulator announcement: https://x.com/lunnoa1013/status/2068345646135066969 (from search results; X did not serve the page)
- LUNABASE Simulator 1.0.1 and 1.1: https://x.com/lunnoa1013/status/2090011566548881826, https://x.com/lunnoa1013/status/2092278970943517164 (titles only)
- LBS standard: https://lunabase.wiki/gi/lbs
- lunabase.wiki: https://lunabase.wiki/gi/home
- Japanese overview of Genshin sites: https://note.com/so_dire/n/n13a69c49b9ba
- gcsim: https://github.com/genshinsim/gcsim, https://gcsim.app
- ClickySim: https://chromewebstore.google.com/detail/ggpghddaendldkonodckfljabblplajh
- Genshin Optimizer: https://github.com/frzyc/genshin-optimizer (README, roadmap last updated 2025-02)
- Mona Uranai: https://github.com/wormtql/genshin_artifact
- Akasha System: https://akasha.cv (returned 403); guide: https://landofgames.ru/articles/feature/28926-akashacv-kak-polzovatsja-sajtom-dlja-analiza-bildov-v-genshin-impact.html
- gencalc: https://github.com/asagume/gencalc
- 原神シミュ2: https://ii.gensim.work
- 原神ダメージ: https://damage.paimon.app
