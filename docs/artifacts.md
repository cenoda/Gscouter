# Artifacts

> Status: Implemented, except verified builds (§3), which wait for the arena · Last updated: 2026-10-28

This covers two roadmap nodes (`docs/roadmap.md`): **Artifacts** (ring 1) and **Artifact score** (ring 2). How much each piece contributes to the team's DPS (taking one piece's stats out) is a later step and not part of this design.

## 1. Keeping artifacts piece by piece

Today a build keeps only artifact totals: `enka.Character` has `Sets` (set → pieces) and `Stats` (stat → total), and Enka's per-piece data is dropped during normalization. `enka.Pack` writes the same totals into links. A score for artifacts needs the pieces, since the totals mix main stats with substats.

- `Character` gains the five pieces: slot, set, rarity, level, main stat, substats. `Stats` stays the total the gcsim config is built from, so scores do not change.
- `Pack` gets a new format version that carries the pieces. Links in the old format still open. Their artifact score is not shown, and the page asks the visitor to load the showcase again.
  - Each build says which it carries (`Character.PieceDetail`): full pieces, stripped pieces (§2), or none (old format). A stripped piece also keeps its rarity, since the main stat's value depends on rarity and level.
- The showcase holds up to 12 characters. That is three teams of four with no character shared, as content with three parties needs (Stygian Onslaught at its hardest difficulty).

## 2. Display: totals by default

Showing every piece invites bragging, and then attacks on the players who brag. The site keeps the details one click away, so that the visitor who opens them chose to:

- Result pages and shared links show each character's total stats (as the character screen shows them, see below), artifact sets and artifact score. The artifact totals and the individual pieces stay folded until the visitor opens them.
- When copying a link, the sharer can leave the pieces out. Such a link keeps, for each piece, only its slot, level and main stat: enough to separate main stats from substats and compute the score. Substats per piece are dropped.
- Leaving the pieces out changes nothing in the DPS score, which is calculated from totals.
- Folding is only display. Data in a link can be read by anyone who decodes it, so leaving pieces out is the way to keep them out. A UID already shows every piece on Enka.Network; the aim is that the site does not put bragging first, not secrecy.
- **On the page** (2026-10-28): the totals (sets, then each stat) sit under each character in the result table; the pieces are in a closed "Artifact pieces" section below it, set and stat names in the game's words (Enka.Network's store data). Beside the copy button, a "Leave artifact pieces out" box copies a link with stripped builds instead. A link from before pieces were kept says, where the pieces would be, that the showcase has to be loaded again.
- **Total stats** (2026-10-28, on a player's request: artifact totals alone, e.g. CRIT Rate without the base 5%, the ascension stat and the weapon's substat, do not say what a build reaches): the line under each character shows the sets, then the stats gcsim gives the character before the fight starts (`Simulation.CharacterDetails`, `SnapshotStats`: base stats, ascension, weapon, artifacts and the set bonuses and passives that are always on; HP, ATK and DEF as flat totals, Energy Recharge with its base 100%, CRIT with the base 5% and 50%). `score.Details` sets the fight up without running it, once per result, alongside the talent levels it already worked out, so it costs a fraction of one iteration; it runs wherever the result is calculated (server or browser), so nothing new is sent or cached. Only the stats the character screen has are kept (not attack speed, generic DMG Bonus or the base values), and zero ones are left out. This is close to the in-game character screen and comes from the same config as the score, so it works for showcase builds, links and library builds alike. Buffs that need a condition (a set's 4-piece effect after a skill, a weapon's stacks, team buffs) are not in it, as they are not on the character screen; a bonus gcsim gives only to the character on the field can show on the first character alone. Enka.Network's final stats (`fightPropMap`) were not used: only a showcase just loaded has them, not a packed build. When the fight cannot be set up the line shows the sets alone. The artifact totals moved to the head of the closed "Artifact pieces" section, an "Artifact totals" line per character above its pieces; a link from before pieces were kept shows them too, under its notice.
- **Link sizes** (measured, `TestPackedSizes`): four characters with full pieces take about 860 URL characters (stripped 580, totals only 360), a versus link about 1,720. The largest four characters can get (every substat at its most rolls, a 63-byte nickname) is about 900 characters of packed text, well under the 4 KB a link's packed text may have, so no limit on links, short links or the showcase changed. A pick sends up to 12 kept builds (8 before), as many as the showcase holds: about 3,300 URL characters.

## 3. Fairness: verified builds and duplicate artifacts

Builds stay in the visitor's browser after the showcase changes (design §4.8). That lets a team combine builds loaded at different times, and one artifact can then sit on two characters: character A was loaded wearing circlet X, X moved to character B, then B was loaded. The game does not allow that build, and it inflates the team's stats, by mistake or on purpose. More broadly, anyone can write a packed build by hand, and today only the game's limits are checked.

- **Verified build**: every character in the team (all twelve for three parties) comes from the same fresh Enka fetch. The game puts one artifact on one character at a time, so these builds cannot share a piece. **The arena and any ranking accept verified builds only.**
- **Other builds** (from the browser, from a link) are scored as today and never enter a ranking.
- **Duplicate check** on other builds: Enka gives artifacts no instance ID, so two pieces are the same artifact when set, slot, level, main stat and all four substat values match. Two different artifacts matching on all of that by chance is practically impossible. A duplicate gets a warning naming the piece and both characters, and the score is marked as not comparable with standard scores, as a custom enemy's is (design §3.3).
- A link without pieces cannot be checked, so it counts as unverified.
- The duplicate check runs with every team's score (2026-10-28): the warning sits with the score's other notes and says the score does not compare with standard scores. A team with any stripped build, or one from before pieces were kept, is not checked and gets no note. Verified builds matter only to the arena and rankings, so their check is implemented with the arena.

## 4. Artifact score

### 4.1 Substat score

CV (CRIT Rate × 2 + CRIT DMG) counts crit rolls: one maximum CRIT Rate roll (3.89%) is half a maximum CRIT DMG roll (7.77%). It gives nothing to ATK%, HP%, Elemental Mastery or Energy Recharge, and it is the same for every character. Gscouter's score extends it to every substat, weighted for the character in its team:

```
score = Σ over substats (value ÷ maximum roll) × weight × 7.77
```

- Maximum 5★ rolls: HP 298.75, ATK 19.45, DEF 23.15, HP% 5.83%, ATK% 5.83%, DEF% 7.29%, Elemental Mastery 23.31, Energy Recharge 6.48%, CRIT Rate 3.89%, CRIT DMG 7.77%.
- A weight is between 0 and 1. With CRIT Rate and CRIT DMG at 1 and every other weight at 0, the score is CV to within 0.13%: the maximum CRIT Rate roll, 3.89%, is slightly more than half of 7.77%, so a CRIT Rate point counts 1.997 times instead of 2.
- Only substats count. Main stats are judged separately (§4.3).

### 4.2 Weights

Weights are simulated, which no other site does:

- **Reference build**: the library team's own build (`teams/<id>/build.txt`). It comes from the community config, reproduces the source's DPS (design §5.3), and already writes main stats and substats on separate lines.
- For each character and each of the ten substats, one maximum roll is added to the reference build's substats, and the team is scored again.
- weight = (DPS gained from that roll) ÷ (DPS gained from the best stat's roll), at least 0. The best stat has weight 1; for a crit DPS with balanced crit, CRIT Rate and CRIT DMG are both close to 1 and the score is close to CV.
- Weights belong to a character in a team: Bennett's buff changes what ATK% is worth to the others.

### 4.3 Main stats

Flower and plume are fixed. Sands, goblet and circlet are judged on their own:

- Each possible main stat is put in the reference build in turn and the team scored. A main stat is **right** when the team scores within 2% of the best one, so a CRIT Rate and a CRIT DMG circlet can both be right, as can ATK% and Elemental Mastery sands.
- When every main stat of a slot is right, the slot has **little effect** on the team's DPS. The page shows that instead of ✓ or ✗, with the main stat that scored best, and gives no penalty: the choice can follow what the simulation does not measure, such as survival or healing. The threshold is the same for every character, since Genshin does not split supports and damage dealers strictly. In a 12-team trial a 5% threshold left 14 of 44 characters with every slot free; 2% leaves 6 (Aino twice, Barbara, Sucrose, Yaoyao, Zhongli), which is how little their main stats move the team's DPS. The first run on about 100 teams checks the threshold again.
- A **wrong** main stat costs the DPS it loses against the best, converted into score points: (DPS lost ÷ DPS the team's most valuable roll gains) × 7.77. That roll is the best substat's of whichever character gains the team the most from one, usually the main damage dealer's CRIT DMG. The penalty is that converted value, unscaled. A main stat is worth several substat rolls, so it is large: a goblet that loses 18% of the DPS where one roll gains 2% costs 9 rolls, about 70 points.
  - The unit is the team's, not the character's own best roll as first designed: a support's own roll gains the team little, so a sands without Energy Recharge that starves the rotation came to thousands of points (44,000 for Mika in skirk-xingqiu-mika-escoffier). Counted in the character's own rolls, 12,384 of 234,117 wrong main stats cost more than 200 points and 4,196 more than 1,000; counted in the team's, 3,007 and 7, the largest 1,283 (Kokomi's sands without Energy Recharge, which loses 82% of the team's DPS). The substat score stays in the character's own rolls, so the page says the penalty is counted in the team's.
- A main stat the rotation cannot be played with (gcsim rejects an action, say for lack of energy) has no DPS and is left out of that slot, as neither right nor wrong; the slot is then never of little effect. A substat roll that does the same gets no weight.
- The page shows, per character: substat score, main stat penalty, total (e.g. 185 · −70 · 115), and a ✓ or ✗ (or "little effect") for sands, goblet and circlet.

### 4.4 Teams outside the library

A team the library does not have has no reference build, so no artifact score is shown.

## 5. Calculating the weights

Weights are calculated once per library team, ahead of time, and embedded in the binary as a small table, like the library itself. Visitors' browsers and the server calculate nothing for them, so server cost does not grow.

- Every variant runs with the same seeds as the reference build (design §3.1), so the luck of crits is the same on both sides and the difference between them is far more precise than either score alone; 20 iterations are enough. Every variant uses the fillers chosen for the reference build (design §9), so a small change cannot switch fillers and make the number jump. Each variant is one simulation.
- Per character: 1 reference + 10 substats + about 21 main stats (sands 5, goblet 12, circlet 7, less the 3 in the reference build) ≈ 32 simulations; about 130 per team.
- **`cmd/weights`**: like `cmd/validate`, takes `-shard k/n -json` (and `-limit n`, n teams spread evenly over the library), and writes per team and character the weights, the DPS one roll of each substat gains, the right main stats, the DPS per main stat and the penalty of each wrong one, with the reference DPS, the number of simulations and the wall time. `-join` merges the shards into one table, `teams/weights.json` (one team per line, so a rerun's changes show team by team). `-slim` derives the small table from it.
- **Two tables**: the full table (17 MB, about 58 MB of heap parsed) is kept in Git LFS, as the source the small table is derived from, so a derived value (the 2% threshold, a rounding) can change without simulating again. A checkout without LFS has a pointer in its place; nothing the server, the browser build or the other workflows build or test reads it, and the test that compares it with the small table skips. The small table, `internal/artifact/table.json`, is an ordinary file embedded in the binary by `internal/artifact`, which needs only the standard library. It keeps what the page needs: the format and gcsim version, and per team and character the weights above 0 (3 decimals) and, per slot, the right main stats, the penalty of each wrong one (1 decimal; the penalty most wrong ones share, such as off-element goblets', written once as `others`), whether the slot has little effect and then its best main stat, and the options the rotation cannot be played with. On 2026-10-28 it is 5.0 MB (0.7 MB gzipped, which the binary does not use: the server would then keep the unpacked table on its heap) for 4,030 teams, one per line; the server indexes it at the first lookup (0.5 MB of heap, 65 ms) and parses only the team looked up (about 45 µs). The server binary grows by 5 MB.
- **Reading `build.txt`**: per character, the first `add stats` line holds the five main stats and the second the substat totals. The substat roll is added to the second line; a main stat is replaced on the first line by position, so a set bonus or another piece with the same key (`em=187 em=187`, a goblet's Pyro DMG beside Crimson Witch's) is left alone. Some configs write the circlet before the goblet (`atk%=0.466 cr=0.311 pyro%=0.466`): each main stat goes to the slot the game allows, the written order when it allows several. A 4★ piece (Instructor, `hp=3571`) is replaced by the 4★ level-16 value, not the 5★ one, so that the comparison is between main stats, not rarities. A team is skipped, with the reason, when a character's stats do not follow this: totals on one line, main stats summed (`em=561`), a value no fully levelled piece has, or a main stat its slot cannot have. On 2026-10-28 that leaves 4,017 of 4,438 teams.
- **`.github/workflows/weights.yml`**: modeled on `regression.yml`. Started by hand; runs the shards in parallel on 4-vCPU runners; a last job, the only one that checks out LFS files, joins the shards into one table, derives the small table, and pushes both to a branch for the owner to review and merge. After that, only new or changed teams are calculated.
- **Cost (estimate, not measured)**: the regression run on 2026-10-28 (run 40617384146) validated 4,433 teams per side on 16 shards in 81–241 s per shard, about 38 runner-minutes per side, free on a public repository. The weights take an estimated 85 times that (about 130 simulations per team against the 1–2 a validation mostly runs), about 55 runner-hours: under 3 hours per shard on 20 shards. GitHub's limits are 6 hours per job, 72 hours per run and 20 concurrent jobs on the free plan, so more shards queue up and finish in about the same time. A first run on about 100 teams measures the real cost and sets the number of shards.
- **Cost (measured locally, 2026-10-28)**: 12 teams spread over the library on 4 CPUs (`-j 4 -workers 1`): 130.6 simulations per team (125–144) and 40 CPU-seconds per team, 10.3 s per team on the machine. The 4,017 teams would take about 11.5 hours of a 4-vCPU machine, a fifth of the estimate; if a GitHub runner's CPUs are slower, more. The first run on about 100 teams measures the runners.

## 6. On the result page

- The artifact score is calculated only when the visitor asks for it, never automatically. With the weights precomputed, asking costs a table lookup and no simulation.
- As built (2026-10-28): the score is worked out on the server when the page is prepared, for pages the server calculates and pages the browser does, and sits in a closed "Artifact score" section under the result table, above the pieces: per character the three numbers in whole points (the total is the other two as shown) and a line per slot: ✓ and the main stat, ✗ with its penalty, or "little effect" with the best main stat; a main stat the rotation cannot be played with, and a missing piece, are said so. Substats come from the pieces, or for stripped pieces from the totals less the main stats; a 4★ piece simply scores less. A library team the table has no weights for (skipped when they were calculated) shows nothing, as a team outside the library does (§4.4).
- The browser build does not embed the table: the scores reach it with the page's calculation (`web.Job`), and `internal/web` uses `internal/artifact` only outside `js/wasm` builds. The WebAssembly build grew from 67.94 MB to 68.05 MB, for the set and stat names and the templates.
- Order of work, as agreed with the owner on 2026-10-28:
  1. In parallel: pieces in `Character` and `Pack` (§1), and `cmd/weights` with its workflow, first run on about 100 teams to measure the cost (§5). Neither touches the web layer.
  2. The full weights batch.
  3. After faster DPS phase 1 (`docs/design.md` §4.7) is merged into `main`, since both change the web handlers and templates: display and links (§2), verified builds and the duplicate check (§3), the score on the result page (§4).

## 7. Not in this design

- How much each piece contributes to DPS (one piece's stats taken out).
- Finding the best combination of artifacts (an optimizer).
- Recommendations such as what to farm. Those belong to the artifact resin expectation and the AI planner (roadmap ring 3 and beyond).
- Swapping artifacts on a result page (roadmap ring 2, Artifact swap), which builds on §1.
