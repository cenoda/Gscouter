# Gscouter Design Document

> Status: Draft (prototype in progress)
> Last updated: 2026-10-26 · Game version baseline: 7.1

## 1. Overview

Gscouter is a Genshin Impact team DPS calculator that settles **"whose team is better built?"**
A user submits the builds of their party members, and Gscouter computes the team's **theoretical DPS** under fixed conditions with a fixed rotation, so it can be compared against other teams.

### Goals

- Treat team choice (meta), constellations, weapon refinements, levels, talents, and artifacts all as part of "skill", and express them as a single score (DPS).
- Compute every team under identical conditions so that even different teams can be compared fairly.
  - Example: "my Nahida team vs. your Neuvillette team"

### Non-goals

- Evaluating player control/execution. Rotations are fixed by the system and cannot be changed by users.
- Predicting clear times for actual content (Spiral Abyss, Imaginarium Theater, etc.)
- Build recommendations or artifact optimization

## 2. Key Decisions

| Item | Decision | Rationale |
|---|---|---|
| Score | A single absolute DPS value | Team choice (meta) counts as skill. No score relative to a baseline build. |
| Constellations / refinements | Included in the score | Investment is part of building |
| Enemy | Lv. 100, 10% resistance to all elements and physical, single target | Matches the KQM standard, so results can be cross-checked against community calcs |
| Fight duration | One rotation starting with full energy | How community gcsim configs and players on a training dummy measure team DPS, so scores match community numbers |
| Rotation | A fixed rotation per team, taken from a community gcsim config | Removes player control from the equation; community configs are the accepted standard |
| Teams not in the library | Marked "unsupported" and not computed | Early on, library quality comes first |
| Library management | Manual authoring/approval + automated validation | Rotation quality *is* fairness |
| Simulation engine | [gcsim](https://github.com/genshinsim/gcsim) (MIT license) | Frame-level simulator, community standard, fast to add new characters |
| Build input | Enka.Network UID (primary), GOOD JSON (secondary) | Manual stat entry tends to go unused |

### Rules these decisions create (to be communicated to users)

- A single target favors **hypercarry teams**. The best team in Gscouter may differ from the team that feels strongest in multi-target content. That is a rule, not a bug.
- If insufficient Energy Recharge delays the rotation, DPS drops accordingly. Managing ER is part of building.

## 3. Score Definition

### 3.1 Score

```
score = team DPS over one pass of the team's rotation, starting with full energy
```

This is how community gcsim configs measure a team, and how players measure on a training dummy with infinite HP. The enemy is given practically infinite HP, which puts gcsim in damage mode: the run ends when the rotation's action list ends, and DPS is the damage over that time.

- Each iteration `i` uses seed `base + i`, so the same build always gets the same score. This is required for caching and for repeatable comparisons.
- Randomness such as crits is handled by averaging iterations. The result stores the standard deviation and the standard error.
- Check: the imported Hu Tao / Yelan / Xingqiu / Zhongli config scores 103.8k with 1000 iterations against the 103.2k its gcsim share reports.

| Setting | Standard error | Time (4 cores) |
|---|---|---|
| 1000 iterations (CLI) | ±0.2% | 0.65 s |
| **100 iterations (site default)** | ±0.5% | about 0.07 s |

History: the prototype scored steady-state DPS over infinite looping rotations, then 90 s from full energy. Both scored community teams far below what players and community configs report, mainly because the looping rotations were not the community ones.

### 3.2 Common simulation conditions

| Item | Value |
|---|---|
| Enemy level | 100 |
| Enemy resistance | 10% for all elements + physical |
| Number of targets | 1 |
| Enemy HP | Practically infinite (`hp=999999999`); the run ends with the rotation |
| Fight duration | One rotation from full energy (§3.1) |
| Iterations | Site: 100 (±0.5%). CLI: 1000 (±0.2%) |
| Energy | 1 clear particle every 8–12 s (`energy every interval=480,720 amount=1`). Provisional, see §9 |
| Swap delay | 12 frames |

As a gcsim config, roughly:

```
options iteration=100 duration=450 swap_delay=12;
target lvl=100 resist=0.1;
energy every interval=480,720 amount=1;
```

## 4. System Architecture

```
[Input]               [Normalize]       [Match]               [Compute]           [Output]
Enka UID ──┐
           ├─► build normalization ─► team matching ─┬─► config assembly ─► gcsim ×2 ─► score / report
GOOD JSON ─┘   (to gcsim keys)        (library)      │   (build + rotation)   (1 rotation)
                                                     └─► "unsupported" + request tally
```

### 4.1 Input collection

- **Enka.Network**
  - Fetches the characters in the profile showcase (up to 8) by UID: level, constellations, talents, weapon, refinement, artifact main stats and substats.
  - Terms of use: set a unique User-Agent, respect rate limits, keep a response cache (TTL).
  - Constraint: all 4 party members must be in the showcase with "Show Character Details" enabled. Otherwise, tell the user which characters are missing.
- **GOOD JSON**
  - The common format of inventory scanners; used for characters outside the showcase.

### 4.2 Build normalization

- Map character, weapon, and artifact set IDs to gcsim keys (e.g. `nahida`, `travelercryo`).
- **Talent levels**: pass base levels without the constellation bonus (+3).
  - gcsim adds the constellation bonus itself.
  - Check whether the Enka data includes the bonus and handle it accordingly.
- Sum artifact main stats and substats into gcsim `add stats` lines.
- If anything lacks a mapping (new weapon, new set, etc.), skip the calculation and report it as "unsupported item".

### 4.3 Team matching

- **Match only when the 4-character set matches exactly** (order-independent).
  - Changing even one support changes the rotation.
  - Variants are registered as separate library entries.
- The Traveler is distinguished by element (`travelercryo` ≠ `traveleranemo`).
- On a failed match, mark the team "unsupported" and record the combination in the request tally (§5.3).

### 4.4 Config assembly and execution

- Combine the common conditions (§3.2) + the user's build (§4.2) + the library rotation (§5) into a gcsim config.
- Run the rotation once per iteration from full energy and average the team DPS (§3.1).
- **Cache key** = hash of the assembled config. The config contains the build, the rotation and the conditions, and the gcsim version is fixed per deployment, so the same build is never recomputed.
- Characters are compared by canonical name: gcsim aliases resolve to one key (`kazuha` = `kaedeharakazuha`), and both Traveler genders fold into `traveler<element>`. When the player's key differs from the library's (e.g. `aethercryo` vs. `travelercryo`), the rotation is rewritten to the player's key.

### 4.5 Results

- **Score**: team DPS over one rotation from full energy
- **Per-character contribution**: DPS and share
- **Warnings**
  - Insufficient ER: time each character spent waiting for energy, in seconds per minute of the scoring window
  - Failed actions
- **Metadata**: rotation version, gcsim version, simulation conditions
- **Versus view**: two UIDs/teams side by side with score, contributions, and warnings. A difference under 2% is shown as a tie, since rotation error is larger than that.

### 4.6 Deployment

- **One Go binary** (`cmd/server`) serves the site, calls Enka, runs gcsim in-process, and caches Enka responses (for Enka's TTL) and scores (24 h) in memory. The team library is embedded in the binary.
- **Concurrency**: one simulation at a time, up to 8 waiting; beyond that the request gets a "busy" message. Peak memory per simulation is about 35 MB.
- **Hosting**: a single Azure B2ats v2 VM (2 vCPU, 1 GB) with Docker. Caddy terminates HTTPS for the owner's domain. Expected cost is roughly KRW 15–20k per month.
  - Alternatives considered: in-browser WASM on static hosting (works, about 1.5 s per score single-threaded, but Enka does not allow CORS, so a proxy is still needed); Cloud Run, Oracle Always Free, Hugging Face Spaces (ruled out by the owner's constraints or uncertain terms).
- **Delivery**: pushing to `main` runs tests and publishes `ghcr.io/cenoda/gscouter:latest`. A systemd timer on the VM re-downloads `deploy/` from `main` and pulls the image every 5 minutes, so no deploy credentials live in GitHub. See `deploy/README.md`.

## 5. Team Library

### 5.1 Structure

One directory per team.

```
teams/
  hutao-yelan-xingqiu-zhongli/
    team.yaml        # metadata, character params, source and its DPS
    rotation.gcsl    # rotation from the source config (actions only)
    build.txt        # the source config's build, used for validation
```

Example `team.yaml` (written by `cmd/import`):

```yaml
id: hutao-yelan-xingqiu-zhongli
name: Hutao / Yelan / Xingqiu / Zhongli
members: [hutao, yelan, xingqiu, zhongli]
archetype: Pyro vaporize hypercarry
status: draft            # supported | draft | blocked
rotation_version: 1
gcsim_commit: 2350be9    # gcsim version used for validation
sources:
  - https://gcsim.app/db/fjPG7F9FpwDQ
  - https://gcsim.app/sh/bKtD798PKDfW
notes: Imported from a gcsim share.
params:
  hutao: start_hp=1      # added to the player's character line
reference:
  dps: 103157            # what the source reported with its own build
  gcsim_version: 4ed9d50
```

### 5.2 Rotation authoring rules

- The rotation is the source config's action list, run once. It must not loop forever.
- Character params the rotation assumes (e.g. Hu Tao `start_hp=1`) are stored in `team.yaml` and added to the player's build.
- One rotation is used regardless of the user's build.
  - If an action can't be performed due to insufficient ER, rely on gcsim's wait/delay behavior and surface the delay as a warning.
- Aim for a near-optimal standard rotation for the team. Uneven rotation quality across teams is itself unfairness.

### 5.3 Addition workflow (semi-automated)

| Step | Mode | Description |
|---|---|---|
| 1. Candidate selection | Automated | Tally "unsupported" requests per 4-character set into a priority list |
| 2. Import | Automated | `cmd/import <gcsim share link>`; never write rotations from scratch |
| 3. Validation | Automated | Check the items below |
| 4. Approval | Manual | After PR review, set `status: supported` |
| 5. Regression tests | Automated | Re-run everything on gcsim updates or game patches; flag teams whose DPS shifts more than ±N% for review |

Step 3 automated checks:
- The team reproduces the source's DPS with the source's build within 5%
- The rotation finishes
- No failed actions or burst delays with the reference build

## 6. Initial Library

### 6.1 Test teams (top priority)

| Team | Archetype | gcsim support | Status |
|---|---|---|---|
| Skirk / Yelan / Furina / Escoffier | Cryo hypercarry | ✅ All 4 supported | First implementation target |
| Vesna / Vodyanitsa / Cryo Traveler / Faruzan | Anemo (Cryo Swirl → Stellar Swirl) | ❌ Vesna and Vodyanitsa unsupported | `blocked` |

**Vesna team status (checked 2026-10-26)**
- As of the latest gcsim `main` commit (`2350be9`, 2026-10-23), the Cryo Traveler (`travelercryo`) and Faruzan are implemented. Vesna and Vodyanitsa are not in the character list.
- Both characters were released in 7.1 (2026-09-23). Vesna has a new mechanic that converts Cryo Swirl into "Stellar Swirl", so the engine may need new reaction handling, not just new characters.
- Options:
  1. **Wait for upstream gcsim support** (recommended). Start testing with the Skirk team.
  2. Implement it ourselves and contribute to gcsim. Requires frame data and multiplier validation; substantial work.
- This team also serves as the first test case for "teams the engine does not support yet": matched, but reported as `blocked`.

### 6.2 First library batch (8 teams)

All characters confirmed supported in gcsim (commit `2350be9`).

| # | Team | Reaction / archetype |
|---|---|---|
| 1 | Neuvillette / Furina / Kazuha / Zhongli | Hydro hypercarry |
| 2 | Nahida / Raiden / Xingqiu / Kuki Shinobu | Hyperbloom |
| 3 | Tighnari / Nahida / Yae Miko / Zhongli | Spread / Aggravate |
| 4 | Clorinde / Nahida / Fischl / Zhongli | Aggravate |
| 5 | Mavuika / Citlali / Xilonen / Bennett | Pyro hypercarry / Overloaded |
| 6 | Arlecchino / Yelan / Bennett / Kazuha | Vaporize |
| 7 | Ayaka / Shenhe / Kazuha / Kokomi | Freeze |
| 8 | Raiden / Xiangling / Xingqiu / Bennett (Raiden National) | Overloaded / Vaporize mix. Baseline for engine validation |

### 6.3 Second batch (later)

- Add teams with 6.x–7.x characters. Select them after checking usage stats (YShelper, Teyvat Assistant, etc.) and gcsim support.
- Factor in the ranking from the "unsupported" request tally in §5.3.

## 7. Accuracy and Limitations

- With the same rotation, gcsim generally matches the actual game within a few percent.
- Error mostly comes from:
  - A rotation that is not optimal for the team
  - Implementation errors for new characters
- **New character lag**: until gcsim supports a character, teams using it are `blocked`.
  - Vesna and Vodyanitsa are still unsupported more than a month after release.
  - Not supporting new meta teams right at launch is a structural limitation.
- **Fixed rotation limits**: the optimal rotation can vary with the build (e.g. high ER enables a more aggressive cycle). Initially we ignore this and use one standard rotation.

## 8. Risks

| Risk | Impact | Mitigation |
|---|---|---|
| gcsim lags on new characters | Latest meta teams unsupported | Report as `blocked`, track upstream, contribute if needed |
| Enka API changes / blocking | No input | Caching, GOOD JSON fallback |
| Uneven rotation quality | Fairness compromised | Automated validation + cross-checks with public calcs + PR review |
| Game patches change numbers | Old scores not comparable | Record gcsim and rotation versions with results; consider seasons |
| Simulation cost | Server load | Build-hash cache, tune iteration count |

## 9. Open Questions

- [x] Where to run: server-side Go on one small VM (§4.6)
- [x] Fight length: one rotation from full energy, as in community configs (§3.1)
- [ ] Enemy energy particle drops
  - The prototype uses `energy every interval=480,720 amount=1`, as many community gcsim configs do.
  - This directly affects ER requirements, so the same value must apply across the whole library. Confirm before the first library batch.
- [ ] Whether to support additional rulesets such as an AoE league or a short-fight league (out of scope for now)
- [ ] How to share results (versus card image, link, etc.)

## 10. Next Steps

1. ~~gcsim execution prototype~~ Done: `cmd/gscouter` scores a build file against a library team (see README).
2. ~~Enka UID → gcsim config converter~~ Done (`internal/enka`). Artifact substats come from Enka's rounded display values; crit rate differs from Enka's final stats by under 0.1%p.
3. ~~Website with versus view~~ Done (`cmd/server`, `internal/web`), deployed at https://gscouter.altsky.info (§4.6).
4. ~~Library format, import and validation~~ Done: `cmd/import` adds a team from a gcsim share link; `cmd/validate` checks that each team reproduces its source's DPS (§5.3). CI runs it. The `add-team` skill describes the workflow.
5. First library batch from gcsim share links. A batch of 64 agent-written rotations was removed: written without sources, they scored a third of the community numbers.
6. Display names (Korean character and weapon names instead of gcsim keys)
