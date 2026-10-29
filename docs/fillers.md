# Fillers

> Status: Implemented (`internal/steady`, `config.Fill`) · Last updated: 2026-10-29

What a character does when the looped rotation (design §3.1) makes it stand still, waiting for burst energy or a skill cooldown. The score keeps the best of the rotation as written and the filled runs, so a filler can only raise a score.

## 1. Why

- gcsim makes a character wait until its burst has the energy or its skill is off cooldown. A rotation that spends more energy than it gains lives off its starting energy, then stalls; the stall also stops particle generation (Flins / Ineffa / Aino / Sucrose: 111k one pass, 31k looped).
- A player fills that time, or passes the burst: Hu Tao teams often never use hers, and a rotation that waits for it loses half its damage (Hu Tao / Zhongli / Yelan / Furina, a player build with no Energy Recharge on Hu Tao: 19.8k waiting, 42.8k passing; one pass 36.1k). The score therefore runs the looped rotation as written first. If a character waits 1 s/min or more for energy or for its skill, it runs the rotation again with that wait handled and keeps the best of all runs. A character short of energy is handled one of three ways, and the best run is then tried a fourth way (§2).

## 2. The ways to fill

- **own skill**: before each burst, the character uses its skill when ready, otherwise attacks, until the burst is ready (at most 20 actions);
- **party skills**: as above, but when its skill is on cooldown, another member whose skill is ready swaps in and uses it, and the character's next action swaps back to catch the particles;
- **pass**: the character bursts only when the burst is ready, and the rotation moves on otherwise (a one-frame wait keeps time passing). Some rotations wait for a burst they can never charge once looped: Skirk's opening burst is written for her full starting Serpent's Subtlety (gcsim reports her wait as `insufficient_energy`), so a loop that repeats the opener stood still for good, and the whole team with it. The other fillers fail there: their skill puts Skirk into Seven-Phase Flash, and the rotation's own next skill is then rejected by gcsim (whose `.skirk.skill.ready` still reads true in that state).
- **brief**, tried only on the best run once the search is done: for the characters whose energy that run handles (all of them, then each alone), skills as in **party skills**, at most 2 attacks, then the burst is passed if still not ready. A player swaps out rather than stand on a support until its burst charges: a low-ER Ineffa filling with party skills attacked up to 10 times a cycle on field (Ayato / Columbina / Lauma / Ineffa, all ER substats removed: 28 attacks in 2 minutes), where she is played with two skills to a burst; brief, she attacks 6 times and the team scores 97.7k instead of 94.4k. Tried as a fourth way in the search, it crowded the search's own runs out of the 16-run budget of the time and lowered 58 library teams (Hu Tao / Yelan / Chevreuse / Furina −19%); capping the attacks of every filler instead lowered 357 (Mavuika / Escoffier / Kaeya / Bennett −40%: a main DPS should attack until its burst is ready). Tried after the search it can only raise a score: on the library builds (2026-10-28) 117 teams rose (mean +1.5%, Bennett / Furina / Jean / Mona +45%) and none fell. A near tie still goes to the higher score, so a support can still stand on field when that scores higher by noise (Varesa / Furina / Ineffa / Xilonen, ER removed: Ineffa's 22 attacks win by 0.05%).
- In every variant a character waiting for its skill's cooldown attacks meanwhile (at most 30 attacks).
- A filler uses each member's skill as the rotation first writes it: its parameters (hold or tap) and the plunges that follow it (Kazuha, Xianyun). Burst-reset skills such as Kazuha C1 come from gcsim.

## 3. The search

- The right way differs per character and depends on the others: once Hu Tao passes, the rotation is shorter and Furina runs short of energy, a wait the run as written never showed. So every run that leaves characters standing still is followed by runs that handle them: all of them each way, and, when there are several, each one alone each way. Follow-ups of the better runs go first.
- Filling every burst hurt teams that wait on purpose (Skirk / Furina / Mona / Escoffier went from −2% to −79%). That is why only characters that stood still are filled, and why the unfilled run stays a candidate.

## 4. Results on the first library batch

- With fillers, 46 of 64 teams stay within 5% of one pass, 16 lose 5–20%, and 2 lose more:
  - Kazuha C6 plunge −27%, down from −61% without fillers;
  - Arlecchino / Furina / Kazuha / Bennett −21%. The site shows a known-issue note on Arlecchino teams: their damage is calculated too low, by about 20%, and the cause is under investigation.

## 5. Cost, cap and ranking iterations

- Scoring a team that stood still costs up to 24 simulations in the search (`steady.maxRuns`; brief's runs come on top) instead of one. On the site the rotation as written runs with the full iterations; the filled variants are ranked on 10 (`web.exploreIterations`), and only the best is run again with the full iterations (kept if it still beats the rotation as written). If gcsim rejects that full run on a seed the ranking did not run, the next-best ranked run is confirmed in its place (2026-10-29; before, the rotation as written stood, which scored 0 for two C6 Skirk builds, `docs/characters/skirk.md` §4). `cmd/validate` runs every variant with its own iterations. An Arlecchino / Citlali / Skirk / Bennett build went from 900 iterations to 440 with the same result (100 iterations, ranked on 20, 16 runs).
- The cap and the ranking iterations (2026-10-28; until then 16 runs, ranked on 20). Measured natively with the site's pipeline (seed 1, 50 iterations, warmup 150 s, duration 450 s) on 80 random supported library teams, each with its library build and with its ER substats removed; 116 of the 160 cases search. "Best known" is an exhaustive search (every run at 50 iterations, cap 64).
  - 16 runs, ranked on 20: 290 simulated iterations per searching case on average; 12 of 116 more than 1% below the best known (Yanfei / Durin / Fischl / Nicole without ER −12.4%, Durin / Xingqiu / Nicole / Bennett −11.2%, Rosaria / Yelan / Kazuha / Bennett −10.4%, Chasca / Mavuika / Durin / Nicole −9.6%, all without ER). The cap stopped the search before it tried the best filler (search order); the ranking iterations were not the cause.
  - 16 runs, ranked on 10: 188 iterations (−35%), still 12 missed; 15 of 116 chose another filler, −0.87% to +2.67%.
  - 24 runs, ranked on 10 (chosen): 211 iterations (−27%), 6 missed; against 16 / 20, 24 of 116 chose another filler, −0.87% to +14.2%. Over 5 seed bases, 43 of 580 runs more than 1% below the best known, against 64 at 16 / 20.
  - Ranked on 5: one team −8.5%, rejected.
  - The library rerun with the cap at 24 (`cmd/validate -promote`, each team at its recorded iterations; `cmd/validate` ranks at full iterations, so only the cap matters there): 162 of 4,451 recorded scores rose (61 by more than 1%, 11 by more than 5%; Bennett / Furina / Jean / Mona +71%, Wanderer / Faruzan / Tartaglia / Bennett +32%, Gaming / Layla / Rosaria / Bennett +29%, Mavuika / Iansan / Citlali / Bennett +17%), none fell. Two teams change status: Gaming / Layla / Rosaria / Bennett now keeps 81% of its one pass (62% before) and meets the criteria, and Kaveh / Baizhu / Xingqiu / Kazuha keeps 137% (124% before), over the 130% limit.
  - In the browser: `docs/speed.md` §1.
