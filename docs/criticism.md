# Criticism and Community Risks

> Status: Analysis · Last updated: 2026-11-01

This document analyzes the primary vulnerabilities, potential community backlash, and user friction points for Gscouter, excluding calculation accuracy issues in the DPS engine itself (which is currently undergoing reimplementation).

## 1. Overview & Context

Genshin Impact is fundamentally a casual, PvE, character-collector action RPG. Its player community is notoriously sensitive to meta toxicity, tier list drama, and character elitism ("갈드컵", toxic community faction wars). While technical tools like gcsim and Genshin Optimizer enjoy strong standing among theorycrafters, customer-facing tools that rank, compare, or score players face severe scrutiny.

Gscouter's premise—"settles 'whose team is better built?' by scoring every team under the same conditions with a fixed rotation"—directly intersects with these sensitivities.

---

## 2. Competitive Framing & PvP Toxicity in a PvE Game

### 2.1 The "Versus" Winner/Loser Presentation
- **The problem**: The versus view (`/versus`) and result sharing explicitly declare winners and losers (e.g., *"Left team is 35.2% higher"*), accompanied by comparison progress bars.
- **Community reaction**: Players view this as an antagonistic "combat power meter" (전투력 측정기). Unlike single-player stat checkers, direct head-to-head comparison generates resentment, particularly when a player with high investment loses to a lower-investment meta team.
- **The "Meta is skill" friction**: The design document states: *"Team choice (meta) counts as skill."* For casual or character-loyal players who invested heavily in older or non-meta characters (e.g., Eula, Klee, Albedo, Cyno), being told that their choice of character represents a lack of skill is viewed as hostile elitism.

### 2.2 Contrast with Akasha.cv
- **Why Akasha succeeds without severe backlash**: Akasha.cv ranks artifacts *strictly within the same character and archetype category* (e.g., "Top 1% Hu Tao - Staff of Homa, Vaporize"). It validates the player's dedication to their specific character without comparing Hu Tao to Neuvillette.
- **Gscouter's vulnerability**: By placing completely different teams in head-to-head DPS comparisons on identical terms, Gscouter invites cross-character meta wars that most community members actively disdain.

### 2.3 The Proposed "Arena" (Leaderboards)
- Roadmap node **Arena** (ring 2, `docs/roadmap.md`) envisions public leaderboards ranking player builds against each other.
- HoYoverse explicitly avoids in-game leaderboards to prevent player toxicity and anxiety. Introducing a public third-party ranking ladder risks branding Gscouter as a toxic gatekeeping hub.

---

## 3. Simulation Ruleset vs. Practical In-Game Reality

Even when raw DPS calculation formulas are mathematically sound, the *environmental assumptions* of the simulation create strong perceived unfairness.

### 3.1 The 150–450 s Steady-State Loop vs. 20–30 s Real Clears
- **The rule**: Gscouter discards the first 150 seconds and averages DPS over 150–450 s to measure whether Energy Recharge can sustain the rotation indefinitely.
- **Player frustration**: In actual gameplay (Spiral Abyss Floor 12), combat encounters are decided within 1 to 2 rotations (20–40 seconds). Burst-heavy teams (e.g., Raiden Hypercarry, International) often build lower ER specifically to trade sustainable looping for explosive 1-cycle clear speed.
- In Gscouter, such practical speedrun builds are penalized with severe "energy wait" penalties, leading players to protest: *"My team clears Abyss Floor 12 in 25 seconds without issue, but your site rates it as trash because of a 7-minute infinite loop rule that doesn't exist in the game."*

- The one-rotation number from full energy is already shown next to the score (design §3.1); the complaint is that readers do not notice it.

#### 3.1.1 Review: a 120 s window instead of 150–450 s (2026-11-01)

Proposed: score the first 120 s from full energy, the length of a timed endgame fight, instead of the loop.

Measured with `cmd/validate -iter 20 -shard 1/20` (241 library teams, fork `eb218847`), once as today and once with `-warmup 0 -duration 120`:

| Measure | Result |
|---|---|
| 120 s score / looped score | median 0.990, 5th–95th percentile 0.947–1.060, worst 0.904 |
| Rank correlation (Spearman) | 0.993; 9 of the top 10 stay in the top 10 |
| Largest rank moves | ±22–35 places of 241 |
| Energy waits | the biggest drop from 2–7 s/min to 0–2 s/min (Kuki / Mika / Yun Jin / Furina 7.0 → 0.2), but their scores move only 0–6%: fillers already make up most of the wait in the loop |

What it means:

- **The ranking barely changes.** For most teams the loop and the 120 s window give the same order, so the switch would answer the complaint mostly in wording, not in numbers.
- **Energy Recharge stops counting.** Starting with full energy, a team bursts at least once without earning it, and in 120 s the deficit hardly shows. That drops the rule "managing ER is part of building" (design §2) and the energy warnings with it.
- **The cut falls mid-rotation.** One rotation takes 64–135 s on these teams, so 120 s holds one to two rotations, and a team whose payoff lands just after 120 s loses it. The teams that fall most (Xingqiu / Dehya / Bennett / Xiangling −10%, Sucrose / Xiangling / Yelan / Hu Tao −9%) and rise most (Neuvillette / Furina / Ororon / Lan Yan +13%) are moved by where the cut lands, not by their builds. The loop averages that out.
- **What a timed fight starts with is the mode's rule.** If it starts with empty bursts, the 120 s window would need `energy` at 0, which changes numbers far more than the window does, and every rotation's opener assumes full energy.

Options: keep the loop and show the one-rotation number as prominently as the score; or show a 120 s number as a second figure, which costs one more simulation per team (about 25% more simulation time, since a 120 s run is about a quarter of a 450 s one). Not decided.

### 3.2 Fixed Library Rotations & Constellation Mismatch
- **C0 rotations forced on high-constellation builds**: 4,339 of 4,570 teams have only a single community rotation (almost always authored for C0 5★ characters). High constellations (C2/C6) fundamentally change skill priorities, cooldown timings, and field times (e.g., Furina C6 on-field attacks, Chasca C6 instant charges, Citlali C6 burst skipping).
- Running a C0 rotation on a C6 account frequently results in wasted buffs, idle time, or rejected inputs, drastically under-scoring invested accounts (`docs/cycle.md`). High-spending players are the quickest to complain publicly when their investment is under-represented.

### 3.3 Artificial Algorithmic "Fillers"
- When insufficient ER or cooldowns stall a character, the steady-state engine inserts synthetic filler actions (`internal/steady`, `docs/fillers.md`): swapping to another character to cast skill, or using unscripted normal attacks.
- Players reviewing their rotation breakdown will observe bizarre action patterns they would never perform in reality, blaming the site for fabricating suboptimal play and depressing their score.

### 3.4 Single-Target Sandbox Bias
- The standard enemy is a single Lv. 100 target with 10% universal resistance.
- This creates an inherent structural bias toward single-target hypercarries. AoE specialists, crowd-controllers, and multi-target reaction teams (e.g., Venti, Kazuha Swirl, Nilou Bountiful Bloom) score substantially lower than their actual combat value. Users frequently dismiss single-target-only calculators as "spreadsheet gaming" (허수아비 딜딸).

---

## 4. Artifact Scoring Sensitivities

### 4.1 Divergence from Established Community Metrics (CV)
- Genshin players overwhelmingly evaluate artifacts using Crit Value ($2 \times \text{CR} + \text{CD}$) and counted substat rolls.
- Gscouter's artifact score uses simulated team DPS weights (`docs/artifacts.md`). While theoretically rigorous, it yields counterintuitive results when a universally prized 45+ CV piece receives a mediocre score because the specific library rotation does not scale with that stat.

### 4.2 Severe Main Stat Penalties
- Non-optimal main stats incur severe point deductions based on team DPS loss (e.g., an incorrect sands causing over 1,000 points of penalty).
- Legitimate hybrid or survival builds (e.g., HP/Healing Bonus on Kokomi or Kuki Shinobu, HP shield-bot Zhongli, Favonius-trigger Crit circlets on supports) are heavily penalized because defensive utility is completely ignored by the DPS metric.

---

## 5. Performance & Mobile Experience

### 5.1 Heavy Client-Side WebAssembly (WASM) Load
- **Technical specifications**: The WASM binary is ~62 MB (~10 MB gzipped), spawning up to 6 Web Workers utilizing 100% CPU for 11–14 seconds per calculation (`docs/design.md` §4.7).
- **Mobile reality**: Over 70% of Genshin community traffic arrives via mobile devices (in-app browsers from Reddit, Arca.live, Twitter, LINE).
- On mobile devices, downloading 10 MB and pinning the CPU across multiple cores triggers **instant thermal throttling, high battery drain, unresponsiveness, and frequent browser tab crashes (OOM)**. Users will immediately label the site "broken" or "malware-like lag."

### 5.2 Showcase Constraints & Refresh Delay
- Calculations require all 4 team members to be currently placed in the in-game profile showcase (maximum 12 slots).
- If a player changes their showcase in-game, Enka.Network's cache TTL enforces a mandatory waiting period (up to several minutes) before Gscouter can load the updated characters.
- Unsupported team combinations (parties not matching one of the pre-harvested library entries) are completely rejected from standard scoring, leaving casual users unable to evaluate custom or creative teams without manually learning the rhythm-game-like chart editor.

---

## 6. Privacy, Doxing & Griefing Vectors

### 6.1 Unconsented UID & Nickname Exposure
- Shared result links (`/s/<id>`) and social preview cards prominently display the player's in-game nickname and UID.
- Anyone who knows a player's UID can look up their showcase, generate a comparison, and share it publicly.
- **Weaponized mockery (박제/조리돌림)**: Toxic players frequently use third-party scouters in community arguments to pull up an opponent's profile, post their low score or suboptimal artifact build, and ridicule them publicly.

### 6.2 Permanent Short Links & Manual Deletion Friction
- Short links are stored permanently in SQLite (`/data/links.db`).
- The only way for a player to remove an unwanted link containing their UID and nickname is to manually join the project's Discord server and request deletion (`internal/web/messages_ko.go` `about.delete`). Requiring Discord engagement to delete personal data is a significant community pain point.

### 6.3 Handcrafted Build URL Manipulation
- Packed build strings can be crafted manually with arbitrary weapons, constellations, and substats.
- Without a strict "Verified Build" gate, bad actors can share fake C6R5 screenshot links claiming absurd stats or defaming specific player UIDs.

---

## 7. Fork Legitimacy & Bias Accusations

### 7.1 The Unofficial gcsim Fork (`cenoda/gcsim`)
- New and unreleased characters (e.g., Vesna, Vodyanitsa, Nefer) run on a private fork with custom-authored rotations.
- If an unverified character performs unexpectedly high or low, community members readily accuse the developer of bias, favoritism (접대), or agenda-pushing.

### 7.2 Hardcoded Issue Notices
- Disclaimers such as `issue.arlecchino` (*"Known issue: Arlecchino's damage is calculated about 20% too low... add about 20% to this result"*) undermine technical credibility and invite ridicule regarding tool competence.

---

## 8. Summary of Mitigations

| Vulnerability | Recommended Mitigation |
|---|---|
| **Toxic Versus screen** | Soften comparative language; highlight intra-team character contributions rather than winner/loser banners. Provide an optional "Mask UID/Nickname" toggle for link sharing. |
| **Mobile freeze & crash** | Offer an automatic fallback to server-side calculation or low-iteration mode on mobile user-agents. |
| **Steady-state loop backlash** | Prominently display the 1-cycle burst DPS alongside the loop score, clarifying that both represent valid, different playstyles. |
| **Support artifact penalties** | Exempt defensive and utility main stats (HP%, DEF%, Healing Bonus) on designated supports from team DPS penalties. |
| **Doxing / link persistence** | Implement self-service link reporting or optional UID masking on generated share cards. |
