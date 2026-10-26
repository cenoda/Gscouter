# Gscouter Design Document

> Status: Draft (pre-implementation)
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
| Fight duration | Infinite cycles (steady-state DPS excluding the opener) | Teams with different cycle lengths become comparable; insufficient Energy Recharge shows up honestly |
| Rotation | A fixed standard rotation per team | Removes player control from the equation |
| Teams not in the library | Marked "unsupported" and not computed | Early on, library quality comes first |
| Library management | Manual authoring/approval + automated validation | Rotation quality *is* fairness |
| Simulation engine | [gcsim](https://github.com/genshinsim/gcsim) (MIT license) | Frame-level simulator, community standard, fast to add new characters |
| Build input | Enka.Network UID (primary), GOOD JSON (secondary) | Manual stat entry tends to go unused |

### Rules these decisions create (to be communicated to users)

- Infinite cycles + single target favor **hypercarry teams and teams that ramp up over time**, e.g. teams that need buffs to stack, like Furina's Fanfare.
  - The best team in Gscouter may differ from the team that feels strongest in real content. That is a rule, not a bug.
- If insufficient Energy Recharge delays the rotation, DPS drops accordingly. Managing ER is part of building.

## 3. Score Definition

### 3.1 Steady-state DPS

```
score = average DPS over the interval after the opener (first cycle)
```

gcsim runs fixed-duration simulations, so infinite cycles are approximated.

- **Difference method (default)**
  - Run the same config with two durations, T1 and T2 (T1 < T2).
  - Use `(total_damage(T2) − total_damage(T1)) / (T2 − T1)` as the score.
  - Example: T1 = 60 s, T2 = 360 s
  - The opener cancels out between the two runs, so this does not depend on gcsim internals.
- **Alternative**
  - Sum only the damage after the first cycle from gcsim's damage event log.
  - Gives exact cycle boundaries, but depends on the log format.
- Randomness such as crits is handled by averaging gcsim iterations. Store the standard deviation with the result.

> To verify: how large the error gets when T1/T2 fall mid-cycle in the difference method. Measure with representative library teams, then finalize the T values.

### 3.2 Common simulation conditions

| Item | Value |
|---|---|
| Enemy level | 100 |
| Enemy resistance | 10% for all elements + physical |
| Number of targets | 1 |
| Enemy HP | Infinite (time-based termination) |
| Iterations | TBD (e.g. 1000), based on precision vs. cost |
| Enemy energy particle drops | TBD (see §9) |

As a gcsim config, roughly:

```
options iteration=1000 duration=360 swap_delay=12;
target lvl=100 resist=0.1;
```

## 4. System Architecture

```
[Input]               [Normalize]       [Match]               [Compute]           [Output]
Enka UID ──┐
           ├─► build normalization ─► team matching ─┬─► config assembly ─► gcsim ×2 ─► score / report
GOOD JSON ─┘   (to gcsim keys)        (library)      │   (build + rotation)   (T1, T2)
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
- Run the same config at both durations, T1 and T2.
- **Cache key** = build hash + team ID + rotation version + gcsim version. Never recompute the same build.

### 4.5 Results

- **Score**: steady-state DPS
- **Per-character contribution**: DPS and share
- **Warnings**
  - Insufficient ER (e.g. "Furina's burst delayed by N s per cycle on average")
  - Failed actions
- **Metadata**: rotation version, gcsim version, simulation conditions
- **Versus view**: two UIDs/teams side by side with score, contributions, and warnings.

## 5. Team Library

### 5.1 Structure

One directory per team.

```
teams/
  skirk-yelan-furina-escoffier/
    team.yaml        # metadata
    rotation.gcsl    # rotation (actions only, no character stats)
    reference.yaml   # reference build and expected DPS range for validation
```

Example `team.yaml`:

```yaml
id: skirk-yelan-furina-escoffier
name: Skirk / Yelan / Furina / Escoffier
members: [skirk, yelan, furina, escoffier]
archetype: Cryo hypercarry
status: supported        # supported | draft | blocked
rotation_version: 1
gcsim_commit: 2350be9    # gcsim version used for validation
sources:
  - <links to referenced community configs/calcs>
notes: ""
```

### 5.2 Rotation authoring rules

- After the first cycle (opener), **the same cycle must repeat**. This is a precondition for steady-state DPS.
- One rotation is used regardless of the user's build.
  - If an action can't be performed due to insufficient ER, rely on gcsim's wait/delay behavior and surface the delay as a warning.
- Aim for a near-optimal standard rotation for the team. Uneven rotation quality across teams is itself unfairness.

### 5.3 Addition workflow (semi-automated)

| Step | Mode | Description |
|---|---|---|
| 1. Candidate selection | Automated | Tally "unsupported" requests per 4-character set into a priority list |
| 2. Draft | Semi-automated | Import shared gcsim community configs for the same set and refine them into infinite-cycle form |
| 3. Validation | Automated | Check the items below |
| 4. Approval | Manual | After PR review, set `status: supported` |
| 5. Regression tests | Automated | Re-run everything on gcsim updates or game patches; flag teams whose DPS shifts more than ±N% for review |

Step 3 automated checks:
- The cycle actually repeats (variance of per-cycle damage)
- No failed actions or burst delays with the reference build
- The result falls within the expected range in `reference.yaml` (compared against public calcs such as KQM)

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

- [ ] Where to run: server-side in Go vs. in-browser via WASM (server-side is better if we need match history and rankings)
- [ ] Iteration count and T1/T2 values (after the §3.1 verification)
- [ ] Whether to include enemy energy particle drops
  - Community gcsim configs commonly include settings like `energy every`.
  - It directly affects ER requirements, so the same value must apply across the whole library.
- [ ] Whether to support additional rulesets such as an AoE league or a short-fight league (out of scope for now)
- [ ] How to share results (versus card image, link, etc.)

## 10. Next Steps

1. gcsim execution prototype: compute the T1/T2 difference score for Skirk / Yelan / Furina / Escoffier with a hardcoded build.
2. Enka UID → gcsim config converter
3. Library format and automated validation script
4. Author the 8 teams of the first library batch
5. Versus view
