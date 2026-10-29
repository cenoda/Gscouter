# Faster DPS

> Status: Paused (2026-10-28, `docs/roadmap.md`) · Last updated: 2026-10-28

Measurements of the time to a score, and what was tried to shorten it. The site calculates results in the visitor's browser (design §4.7); the library is validated on the command line (design §5.3). Both run the same simulations.

## 1. A result page in the browser

_2026-10-28._ The page records each stage as a performance measure named `gscouter:*` (nothing is logged). Headless Chromium on 4 CPUs, a fresh profile each time, median of 3; "after" comes through the front page, "before" opens the result directly (the front page did nothing for it). Scores and the whole result were identical.

| Stage | Hu Tao / Yelan / Xingqiu / Zhongli, before → after | Arlecchino / Citlali / Skirk / Bennett (19 simulations), before → after |
|---|---|---|
| Download and compile, local / 50 Mbit/s | 0.5 / 3.4 s → 0.6 / 0.6 s | 0.5 / 3.4 s → 0.6 / 0.6 s |
| Starting the workers | 2.6 s → 0.24 s | 2.7 s → 0.28 s |
| Score simulation (100 iterations) | 9.0 s → 8.0 s | 6.5 s → 5.4 s |
| Filler search (16 runs of 20 iterations) | — | 12.4 s → 12.5 s |
| Best filler again, one pass (100 iterations each) | 1.3 s → 1.4 s (one pass) | 4.1 s → 4.2 s |
| Two timelines, then rendering | 0.2 s + 0.4 s | 0.14 s + 0.3 s |
| Total, local | 14.1 s → 10.8 s | 26.7 s → 23.6 s |
| Total, 50 Mbit/s | 16.9 s → 10.1 s | 30.1 s → 23.3 s |

- The calculation is bound by the CPUs. The first simulation runs about twice as slow as the same work later (Arlecchino: 6.5 s as written, 3.4 s for the best filler at the same 100 iterations), because the browser starts on quickly compiled code and optimizes what runs; the warm-up's code cache covers part of that.
- The filler search runs one simulation after another (each follows from the last, `docs/fillers.md` §3), but each is already split among all helpers (7 chunks on 4), which are busy about 90% of the time. Splitting into single iterations, as tight as packing gets, took the same 12.2 s: the 4 CPUs are full, so running the search's simulations side by side would not make it faster, and it was not done.
- The table was measured at 100 iterations per score. With 50, the site default since 2026-10-28 (design §3.1), a result page opened directly (no warm-up, a fresh profile, two runs each) took 11.4–11.8 s instead of 14.7 s for Hu Tao and 24.1–24.4 s instead of 27.7–29.6 s for Arlecchino, with the same score in the browser and on the server. That is 3–5 s, not half: download, compile, worker start and the filler search (20 iterations per run) do not depend on the iterations. The two changes together, through the front page, are not measured yet.
- Filler search at 10 iterations per run and up to 24 runs (2026-10-28, `docs/fillers.md` §5), the low-ER Arlecchino / Citlali / Skirk / Bennett showcase link opened directly, two runs each: before (16 runs, ranked on 20) the page made 18 filler runs (search and brief) in 8.4–8.8 s and took 14.5–16.0 s in all; after, 26 filler runs, since the cap had stopped its search, in 7.0–7.5 s, and 12.9–13.6 s in all. The score was 70,370 both ways, in the browser and on the server (`server=1`). At the old cap of 16 runs, 10 iterations alone brought its search from 8.0–9.1 s to 4.9–5.1 s (5 iterations: 2.8–2.9 s) and the page from 13.4–15.1 s to 10.2–11.1 s, same score. A filler run costs about 0.08 s plus 0.02 s per iteration.
- Not done either: showing the score before the timelines (at most the 0.4–0.7 s after the last simulation), the garbage collector at twice the default heap growth (3% faster for 11 MB more per worker), and Go's newer WebAssembly instructions (`GOWASM=satconv,signext`: no change).

## 2. Iterations per score

The site scores with 50 iterations since 2026-10-28, down from 100, to halve the time to a score; time is roughly linear in the iterations. Measured natively on 6 library teams, 31 seeds per count: the standard deviation of a score across seeds was 0.04–0.64% of its mean at 50 (worst seen 1.2%, apart from filler-dependent outliers) against 0.03–0.51% at 100, and with the site's seed every team chose the same filler at both counts. Shared links are calculated again with the new count, so their numbers move slightly; with the site's seed, 50 against 100: Hu Tao / Yelan / Xingqiu / Zhongli −0.53%, Hu Tao / Furina (no ER) −0.04%, an Arlecchino team −0.04%, a Skirk freeze team −0.04%, Raiden National +0.13%, a hyperbloom team +0.29%. Fillers are ranked on 10 iterations (20 until the same day, `docs/fillers.md` §5) and the best run again on 50; warmup, duration and seeds are unchanged.

## 3. Validation runs

A full run (`cmd/validate -iter 20 -j 4`, 4 CPUs) simulates per team one pass under the source's conditions, one under the common conditions, one probe fight and the looped score, which `steady.Score` reruns with fillers up to 24 times (16 until 2026-10-28, `docs/fillers.md` §5). Profiled on 100 random teams (2026-10-27, cap 16): the looped score took 87% of the time (its first run 22%, the filler runs 64%; 11 of the 100 teams ran all 16), the two passes 5% each, the probe 3%; half the teams need one looped run. What makes a run faster without changing any result to the last bit:
- each check's process runs the garbage collector a tenth as often (GOGC 1000, a 1 GiB soft limit; at most about 260 MB per process): 40% less time. A fight allocates fast and keeps little alive;
- gcsim collects only the stats the score needs (`score.Run`): damage, failed actions, and shields, which is not only an observer: on every hit on the player it queues a task, which reorders the frame's other tasks, so without it Dehya / Furina / Albedo / Xilonen scored 1.6 DPS lower under its source's conditions (they have hurt lines). About 5% less;
- the probe is the looped score's first iteration (same config, seed and warmup; gcsim does not read the iteration count): 3% less;
- results are cached (`score.Cache`, `validate -cache`, by default under the user's cache directory; `-cache ""` turns it off): keyed by config and options, in a directory per gcsim module with its replace directive, Go version and format name (`cacheFormat`, to change with `summarize` or the collectors). A rerun on the same gcsim simulates only the teams that changed. The library takes about 210 MB (28,000 entries) per gcsim version.

The whole library took 41 minutes (148 CPU minutes) instead of 67 (233), every report identical to the last bit (`cmd/compare`, 4,433 teams); a rerun with the cache took 2 minutes. `-shard k/n` splits a run across machines. A cached run is not identical in one way: a team near the 2-minute limit of a check, which is measured in wall time and depends on the machine's load, finishes once enough of its simulations are cached. Ningguang / Xingqiu / Bennett / Chiori timed out on both cold runs and passed with the cache. With the search's cap at 24 runs (2026-10-28, `docs/fillers.md` §5), a cold run took 37 minutes for the 4,398 teams recorded at 20 iterations and 1 minute for the 53 recorded at 100; the same team timed out again and passed with the cache.

Rejected: ranking filler candidates on fewer iterations and running only the winner fully. Which follow-ups `steady.Score` tries, in what order, and which wins all depend on each run's full result. Ranked on 5 iterations (7 minutes for the library), 748 of 4,431 teams chose another filler or none, so it would change scores.
