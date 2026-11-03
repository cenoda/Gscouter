# Tier & Wiki

> Status: Draft, under discussion (nothing here is decided) · Last updated: 2026-11-03

Roadmap node Tier & Wiki (`docs/roadmap.md`): team and character tiers from scores. This document records the direction discussed with the owner on 2026-11-03 and the questions still open. Every point is a proposal until the owner decides it.

## 1. What the wiki is for

- **The wiki shows general performance; the score pages show the player's own build.** A well-built Skirk beating a Vesna is normal, and showing that the two can swap places is the DPS page's job. The wiki tells which character is better in general (owner, 2026-11-03).
- **The goal is a tier per constellation.** Reflecting the visitor's own constellations is a convenience on top of it (§3).
- **Written by the developer, not the community.** The site does not have enough visitors for community editing (owner, 2026-11-03).

## 2. What other sites teach (research, 2026-11-03)

### 2.1 KQM and Game8

From general community perception, not from sources gathered for this document:

- KQM is trusted because it publishes its method (the KQM Standards, §2.2), backs mechanics with evidence (the Theorycrafting Library), states conditions instead of a single letter ("in this team, at this constellation, this weapon is x% better"), reviews among theorycrafters, does not rank new characters before testing, and does not publish an official tier list.
- Game8 is criticised for grades without a stated method, grades that change often (new characters rated high at release, read as promoting the current banner), one scale across roles and constellations, SEO-driven content with errors, and reaching casual players first, which spreads its errors.
- Lesson: criticism cannot be avoided, so aim for criticism the page can answer with numbers: every placement traces to a team, build, rotation and score.

### 2.2 KQM Standards

[KQM Standards](https://compendium.keqingmains.com/) revision 2.0.2, last updated 2024-05-03 (no later revision found on 2026-11-03). They fix only artifact substat distribution, weapon and character levels and a dummy enemy; everything else must be stated.

- **Artifacts**: all 5★ at +20. 40 substat rolls: 20 fixed (2 per each of the 10 substat types) and 20 distributed as well as the tool allows (one roll of error accepted). A substat takes at most 2 distributed rolls per piece whose main stat is a different stat, so at most 10, less 2 per piece with that main stat. A roll is the average of the low and high roll: CRIT Rate 3.31%, CRIT DMG 6.62%, ATK% and HP% 4.96%, DEF% 6.20%, EM 19.82, ER 5.51%, flat ATK 16.54, HP 253.94, DEF 19.68. A 4★ piece gives ×0.8 and 2 fewer distributed rolls. Sets, set stacks, main stats and energy needs must be stated.
- **Weapons**: level 90; weapon, refinement and passive uptime stated.
- **Characters**: level 90, talents 9/9/9 (plus constellation +3); constellations stated; start at 100% HP with the burst charged.
- **Enemy**: level 100, 10% resistance to all. gcsim single target `radius=2 pos=0,2.4`; two targets are two Ruin Guard sized enemies, many targets one Ruin Guard and two Hilichurl sized ones (`radius=1`) on either side.
- **Energy**: gcsim `energy every interval=480,720 amount=1` (one particle every 8–12 s); sheet ER calculations use 3 clear orbs over 90 s on safe particle RNG.
- **gcsim runs**: at least 90 s and 1,000 iterations, with one of two prescribed random delay functions.
- **Must be stated**: rotation length, reaction uptime, buff durations and uptime, AoE modifiers (with the single-target damage beside them), the team rotation as a comment, clips if possible. Any deviation is stated and explained.

Gscouter already shares the enemy (design §1). It differs in using the player's builds rather than standard rolls, a 150–450 s steady-state loop instead of at least 90 s, and one fixed rotation per team with fillers.

### 2.3 Project Dodoco (dodoco.gg)

A Korean Genshin guide site. Checked 2026-11-03; its arca.live introduction and status reports returned 403 and were not read.

- Tagline "모두가 함께 만들어가는 프로젝트 도도코" ("built by everyone"), but no edit button, contributor page or credits were found. A search summary of its own announcement says its build guides are reviewed by "professional ambassadors" and published every version (not read in the original).
- No author or review line on the pages checked. The Vesna guide was last edited 2026-09-27; the tier list is labelled 7.1 (new: Vodyanitsa SS, Vesna SS) but shows a last edit of 2025-06-17. Tier lists are kept per version, 4.3 to 7.1.
- The tier list states its basis ("current version; future not considered"; pick rate differs from absolute performance) but not who ranks or how.
- Content is CC BY-NC-ND 4.0. The GitHub organisation has no public repositories or members. No ads were seen.
- Reading: a small team writes and reviews, updated each version, which fits §1; the missing method and authorship is where Gscouter can do better.

## 3. Constellations

- A selector at the top of the tier page, C0 to C6, default C0. It applies the same to 5★ and 4★ characters: with C0 selected, 4★ characters are C0 too, although C6 4★ is common, because one page should use one rule (owner, 2026-11-03).
- Characters the visitor owns use their own constellation, read in the browser from the showcase builds already kept there (design §4.8). Only the constellation is read, never the build. Characters never put in the showcase are covered by the selector.
- One URL for everyone (`/tier`), no share links; nothing about the visitor goes into the URL or to the server. The page is the same precomputed data for everyone, and the browser applies the visitor's constellations. No switch to turn this off: the visitor changes constellations with the selector (owner, 2026-11-03).
- Needs constellation-dependent rotations first (roadmap "Next", `docs/cycle.md`): with one C0 rotation per team, a constellation that changes the cycle (Cyno C1) scores too low.

## 4. Artifacts and weapons

- Artifacts: a fixed roll budget, not fixed stat values, as in the KQM Standards (§2.2). Each character in each team gets its best main stats and its 40 rolls distributed as well as possible, so an ATK scaler gets ATK%, an HP scaler HP%, and crit is balanced for each. The artifact weights table (`docs/artifacts.md` §5) gives the DPS per roll and the right main stats per team and character; a greedy distribution from it, checked by a few simulations (the weights ignore diminishing returns, such as the crit ratio), would do. ER is one of the stats rolls can go to, since waits for energy cost score in the looped score.
- Weapons: not discussed yet (§7).

## 5. From library teams to a tier

The problems that a tier by character DPS cannot handle without exceptions (owner, 2026-11-03): a C2 Cyno and a C0 Odette in one row; what groups characters into one tier; comparing, say, a C2 Citlali with a C0 Mavuika; Kamisato Ayato (among the lowest DPS, among the most flexible); telling supports from damage dealers; Escoffier (strong only under particular conditions); double-carry and sub-DPS teams. Handling each as an exception is not wanted, and the tier is one grade, not two (owner).

### 5.1 Steps

1. **Library teams**, each named with its archetype (Hu Tao vaporize, Stellar-Conduct, …).
2. **Fixed conditions**: constellation from the selector (§3), artifacts by the roll budget (§4), weapon (open, §7), the usual enemy.
3. **Simulate every team** under those conditions: one team DPS each. Precomputed for every constellation.
4. **Team grades**: sort the team DPS values and break them where the gaps are large (natural breaks) instead of at fixed cuts, never separating teams whose difference is below what matters (about 1–2%, and the simulation's noise).
5. **Free-rider check**: for each member of each team, if the library has a team that differs only in that member and is stronger (beyond the same margin), the member is not credited with that team.
6. **Character grade per archetype**: the best grade among the teams of that archetype the character is credited with.
7. **Tier**: each character@constellation's count of archetypes per grade, sorted by the count in the highest grade, then the next. The tier page shows only these counts per grade (owner, 2026-11-03); everything else is on a character's detail.

### 5.2 Example

Illustrative teams and numbers, not measured. Hu Tao vaporize at C0:

```
Team 1  Hu Tao / Xingqiu / Yelan / Zhongli    52k  S
Team 2  Hu Tao / Xingqiu / Yelan / Noelle     47k  A
Team 3  Hu Tao / Xingqiu / Albedo / Zhongli   44k  A
```

Noelle is not credited with team 2, since team 1 differs only in her slot and is stronger; Albedo likewise in team 3. No member of team 1 has a stronger one-member variant, so all four are credited. In this archetype Hu Tao, Xingqiu, Yelan and Zhongli are S; Noelle and Albedo have no grade. Over every archetype:

```
                 S   A   B
Zhongli C0       6   4   2    best choice in many teams
Hu Tao C0        2   1   0
Ayato C0         0   0  12    low peak, credited in many teams
Escoffier C0     2   1   3    top only in a few teams
```

### 5.3 How it answers the problems

- **Double carry and sub-DPS**: the grade is the team's, so two damage dealers splitting a team's damage both get its grade.
- **Supports** get the grade of the teams they are the best choice in, and their count of archetypes shows their flexibility.
- **Free riders** are removed by step 5, a comparison of library scores, not an exception. Where the library has no team differing in that member alone, a free rider keeps the team's grade; the owner judges such pairs common enough (2026-11-03), and the tier improves as the library fills.
- **Ayato and Escoffier** differ in their counts per grade: nothing is summed or averaged, so the peak is never diluted. Breadth counts against the common grade boundaries, not the character's own peak (counted against its own peak, a character weak everywhere would look the most flexible).
- **The unit is character@constellation**, so a C2 Cyno and a C0 Odette stand in one row with a constellation badge. The selector sets each character's default constellation; the visitor's own constellations change only their badges. Changing the selector shows the precomputed results for that constellation (Cyno rises to S in Stellar-Conduct teams at C1).
- **One grade per archetype, never averaged over archetypes**: averaged, Cyno (S in Stellar-Conduct teams, B or lower elsewhere at C0) would get an A that draws "why?" from both sides.
- **Comparing two entries** (C2 Citlali and C0 Mavuika): their counts per grade, their grades in the archetypes they share, and the teams each is credited with.

### 5.4 Shapley shares, for explanation only

On a character's detail, its share of a team can be shown: the Shapley value over the team's members, with an empty slot as the baseline (the team DPS of every subset of the four, 15 non-empty ones, and each member's added DPS averaged over every order of joining). The four shares add up to the team DPS, and synergy is split between the members that make it. It is not a grade input: a damage dealer in a double-carry team gets about half the share of one in a hypercarry team with the same team DPS (alone 20k and 20k, together 45k: 22.5k each; against a dealer alone 30k with a support adding 50%: 37.5k and 7.5k), and supports' shares are small.

## 6. Rejected along the way

- **One grade per character**, averaging its archetypes (§5.3).
- **A per-character default constellation** (4★ C6, 5★ C0): the C0 selection would then show 5★ at C0 beside 4★ at C6.
- **A grade with the constellation where it changes** (`B → S (C1)`): most visitors would not read it and would take the site for a C0 tier list.
- **Grouped constellation tabs** (C0 / C1–2 / C6): C1 and C2 differ too much for some characters (Cyno); C3 to C5 are needed too.
- **The visitor's own builds in the tier**: that is the score pages' job (§1).
- **Contribution above a replacement character** (as baseball's WAR): a member's value as team DPS minus the team's with a replacement-level character in its slot. Baseball's nine roles are separable, so a minor-league player can fill one; a Genshin team's four are not: no character replaces Hu Tao in her team, and nothing explains why her replacement would be Xiangling rather than Noelle (owner, 2026-11-03). Taking the best 4★ as the replacement also gives Bennett, a 4★, no value.
- **Breadth as the area of the value distribution**, or as a count against the character's own peak: the first averages the peak away, the second rewards being weak everywhere (§5.3).
- **Grades from Shapley shares**: double-carry and sub-DPS characters, and supports, come out low (§5.4).
- **Team grade without the free-rider check**: Noelle would get S from Hu Tao's team.
- **Two grades per archetype** (team grade and share grade side by side): the tier is one grade (owner, 2026-11-03).

## 7. Open questions

1. **Archetypes**: the library's teams must be classified into archetypes: who names them, from what.
2. **Cost**: every library team at 7 constellations with the roll budget, precomputed. The free-rider check compares existing scores and needs no simulation. Shapley shares (§5.4), if shown, add 15 subsets per team, and need a rule to remove members from a rotation (swap timings and buff order can break; whether the fillers, `docs/fillers.md`, can fill the freed time is to be checked).
3. **Weapons** (§4): which weapon each entry uses, or a weapon axis. Proposed so far, not decided: a second selector (free / 4★ / 5★ R1), the weapon in each class picked by team DPS on the whole team; the refinement of gacha 4★ weapons; whether the visitor's own weapons count (proposed: no, they are build).
4. **Teammates' constellations** when a team is simulated for an entry: the selector's, the visitor's, or a fixed one.
