# Artifacts

> Status: Design draft · Last updated: 2026-10-28

This covers two roadmap nodes (`docs/roadmap.md`): **Artifacts** (ring 1) and **Artifact score** (ring 2). How much each piece contributes to the team's DPS (taking one piece's stats out) is a later step and not part of this design.

## 1. Keeping artifacts piece by piece

Today a build keeps only artifact totals: `enka.Character` has `Sets` (set → pieces) and `Stats` (stat → total), and Enka's per-piece data is dropped during normalization. `enka.Pack` writes the same totals into links. A score for artifacts needs the pieces, since the totals mix main stats with substats.

- `Character` gains the five pieces: slot, set, rarity, level, main stat, substats. `Stats` stays the total the gcsim config is built from, so scores do not change.
- `Pack` gets a new format version that carries the pieces. Links in the old format still open. Their artifact score is not shown, and the page asks the visitor to load the showcase again.
- The showcase holds up to 12 characters. That is three teams of four with no character shared, as content with three parties needs (Stygian Onslaught at its hardest difficulty).

## 2. Display: totals by default

Showing every piece invites bragging, and then attacks on the players who brag. The site keeps the details one click away, so that the visitor who opens them chose to:

- Result pages and shared links show each character's artifact totals and artifact score. The individual pieces stay folded until the visitor opens them.
- When copying a link, the sharer can leave the pieces out. Such a link keeps, for each piece, only its slot, level and main stat: enough to separate main stats from substats and compute the score. Substats per piece are dropped.
- Leaving the pieces out changes nothing in the DPS score, which is calculated from totals.
- Folding is only display. Data in a link can be read by anyone who decodes it, so leaving pieces out is the way to keep them out. A UID already shows every piece on Enka.Network; the aim is that the site does not put bragging first, not secrecy.

## 3. Fairness: verified builds and duplicate artifacts

Builds stay in the visitor's browser after the showcase changes (design §4.8). That lets a team combine builds loaded at different times, and one artifact can then sit on two characters: character A was loaded wearing circlet X, X moved to character B, then B was loaded. The game does not allow that build, and it inflates the team's stats, by mistake or on purpose. More broadly, anyone can write a packed build by hand, and today only the game's limits are checked.

- **Verified build**: every character in the team (all twelve for three parties) comes from the same fresh Enka fetch. The game puts one artifact on one character at a time, so these builds cannot share a piece. **The arena and any ranking accept verified builds only.**
- **Other builds** (from the browser, from a link) are scored as today and never enter a ranking.
- **Duplicate check** on other builds: Enka gives artifacts no instance ID, so two pieces are the same artifact when set, slot, level, main stat and all four substat values match. Two different artifacts matching on all of that by chance is practically impossible. A duplicate gets a warning naming the piece and both characters, and the score is marked as not comparable with standard scores, as a custom enemy's is (design §3.3).
- A link without pieces cannot be checked, so it counts as unverified.

## 4. Artifact score

### 4.1 Substat score

CV (CRIT Rate × 2 + CRIT DMG) counts crit rolls: one maximum CRIT Rate roll (3.89%) is half a maximum CRIT DMG roll (7.77%). It gives nothing to ATK%, HP%, Elemental Mastery or Energy Recharge, and it is the same for every character. Gscouter's score extends it to every substat, weighted for the character in its team:

```
score = Σ over substats (value ÷ maximum roll) × weight × 7.77
```

- Maximum 5★ rolls: HP 298.75, ATK 19.45, DEF 23.15, HP% 5.83%, ATK% 5.83%, DEF% 7.29%, Elemental Mastery 23.31, Energy Recharge 6.48%, CRIT Rate 3.89%, CRIT DMG 7.77%.
- A weight is between 0 and 1. With CRIT Rate and CRIT DMG at 1 and every other weight at 0, the score is CV.
- Only substats count. Main stats are judged separately (§4.3).

### 4.2 Weights

Weights are simulated, which no other site does:

- **Reference build**: the library team's own build (`teams/<id>/build.txt`). It comes from the community config, reproduces the source's DPS (design §5.3), and already writes main stats and substats on separate lines.
- For each character and each of the ten substats, one maximum roll is added to the reference build's substats, and the team is scored again.
- weight = (DPS gained from that roll) ÷ (DPS gained from the best stat's roll), at least 0. The best stat has weight 1; for a crit DPS with balanced crit, CRIT Rate and CRIT DMG are both close to 1 and the score is close to CV.
- Weights belong to a character in a team: Bennett's buff changes what ATK% is worth to the others.

### 4.3 Main stats

Flower and plume are fixed. Sands, goblet and circlet are judged on their own:

- Each possible main stat is put in the reference build in turn and the team scored. A main stat is **right** when the team scores within 5% of the best one, so a CRIT Rate and a CRIT DMG circlet can both be right, as can ATK% and Elemental Mastery sands.
- A **wrong** main stat costs the DPS it loses against the best, converted into score points: (DPS lost ÷ DPS gained from the best stat's roll) × 7.77. The penalty is that converted value, unscaled. A main stat is worth several substat rolls, so it is large: a goblet that loses 18% of the DPS where one roll gains 2% costs 9 rolls, about 70 points.
- The page shows, per character: substat score, main stat penalty, total (e.g. 185 · −70 · 115), and a ✓ or ✗ for sands, goblet and circlet.

### 4.4 Teams outside the library

A team the library does not have has no reference build, so no artifact score is shown.

## 5. Calculating the weights

Weights are calculated once per library team, ahead of time, and embedded in the binary as a table, like the library itself. Visitors' browsers and the server calculate nothing for them, so server cost does not grow.

- Every variant runs with the same seeds as the reference build (design §3.1), so the luck of crits is the same on both sides and the difference between them is far more precise than either score alone; 20 iterations are enough. Every variant uses the fillers chosen for the reference build (design §9), so a small change cannot switch fillers and make the number jump. Each variant is one simulation.
- Per character: 1 reference + 10 substats + about 21 main stats (sands 5, goblet 12, circlet 7, less the 3 in the reference build) ≈ 32 simulations; about 130 per team.
- **`cmd/weights`**: like `cmd/validate`, takes `-shard k/n -json` (and `-limit n`, n teams spread evenly over the library), and writes per team and character the weights, the DPS one roll of each substat gains, the right main stats, the DPS per main stat and the penalty of each wrong one, with the reference DPS, the number of simulations and the wall time. `-join` merges the shards into one table, `teams/weights.json` (one team per line, so a rerun's changes show team by team), to be embedded with the library.
- **Reading `build.txt`**: per character, the first `add stats` line holds the five main stats and the second the substat totals. The substat roll is added to the second line; a main stat is replaced on the first line by position, so a set bonus or another piece with the same key (`em=187 em=187`, a goblet's Pyro DMG beside Crimson Witch's) is left alone. Some configs write the circlet before the goblet (`atk%=0.466 cr=0.311 pyro%=0.466`): each main stat goes to the slot the game allows, the written order when it allows several. A 4★ piece (Instructor, `hp=3571`) is replaced by the 4★ level-16 value, not the 5★ one, so that the comparison is between main stats, not rarities. A team is skipped, with the reason, when a character's stats do not follow this: totals on one line, main stats summed (`em=561`), a value no fully levelled piece has, or a main stat its slot cannot have. On 2026-10-28 that leaves 4,017 of 4,438 teams.
- **`.github/workflows/weights.yml`**: modeled on `regression.yml`. Started by hand; runs the shards in parallel on 4-vCPU runners; a last job joins the shards into one table and pushes it to a branch for the owner to review and merge. After that, only new or changed teams are calculated.
- **Cost (estimate, not measured)**: the regression run on 2026-10-28 (run 40617384146) validated 4,433 teams per side on 16 shards in 81–241 s per shard, about 38 runner-minutes per side, free on a public repository. The weights take an estimated 85 times that (about 130 simulations per team against the 1–2 a validation mostly runs), about 55 runner-hours: under 3 hours per shard on 20 shards. GitHub's limits are 6 hours per job, 72 hours per run and 20 concurrent jobs on the free plan, so more shards queue up and finish in about the same time. A first run on about 100 teams measures the real cost and sets the number of shards.
- **Cost (measured locally, 2026-10-28)**: 12 teams spread over the library on 4 CPUs (`-j 4 -workers 1`): 130.6 simulations per team (125–144) and 40 CPU-seconds per team, 10.3 s per team on the machine. The 4,017 teams would take about 11.5 hours of a 4-vCPU machine, a fifth of the estimate; if a GitHub runner's CPUs are slower, more. The first run on about 100 teams measures the runners.

## 6. On the result page

- The artifact score is calculated only when the visitor asks for it, never automatically. With the weights precomputed, asking costs a table lookup and no simulation.
- Order of work, as agreed with the owner on 2026-10-28:
  1. In parallel: pieces in `Character` and `Pack` (§1), and `cmd/weights` with its workflow, first run on about 100 teams to measure the cost (§5). Neither touches the web layer.
  2. The full weights batch.
  3. After faster DPS phase 1 (`docs/design.md` §4.7) is merged into `main`, since both change the web handlers and templates: display and links (§2), verified builds and the duplicate check (§3), the score on the result page (§4).

## 7. Not in this design

- How much each piece contributes to DPS (one piece's stats taken out).
- Finding the best combination of artifacts (an optimizer).
- Recommendations such as what to farm. Those belong to the artifact resin expectation and the AI planner (roadmap ring 3 and beyond).
- Swapping artifacts on a result page (roadmap ring 2, Artifact swap), which builds on §1.
