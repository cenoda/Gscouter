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

## 5. Contribution above replacement

The problems that a tier by character DPS cannot handle without exceptions (owner, 2026-11-03):

- a C2 Cyno and a C0 Odette in one row;
- what groups characters into one tier, and by what rule;
- comparing, say, a C2 Citlali with a C0 Mavuika;
- Kamisato Ayato: among the lowest DPS but among the most flexible, so he lands in B and the tier says nothing of why;
- telling supports from damage dealers;
- Escoffier: strong only under particular conditions.

Proposal: measure every character the same way, like baseball's WAR.

- **Value of a character in a team = team DPS with it − team DPS with that slot given to a replacement-level character** (for example the best 4★ that fits the slot). Damage dealers and supports are measured in the same unit, team DPS. A character has one value per team it is in; the distribution of those values describes it.
- **Two axes, not one grade**: peak (its highest value) and breadth (how many archetypes it reaches, say, some share of its peak in). Escoffier is high peak, narrow breadth; Ayato low peak, wide breadth. Neither needs an exception; they sit in different places.
- **Support or dealer** is not an input. If a label is wanted, it comes from the character's share of team damage.
- **Entries are character@constellation.** Values are computed per constellation, so a C2 Cyno and a C0 Odette stand in one row with a constellation badge. The selector sets each character's default constellation; the visitor's own constellations change only their badges.
- **Comparing two entries** (C2 Citlali and C0 Mavuika): their peak and breadth, their values in the teams they share, and the teams each one makes possible.
- **Tier boundaries from the data**: break the sorted values where the gaps are large (natural breaks) instead of fixed cuts, and never separate entries whose difference is below what matters (about 1–2%, and the simulation's noise).
- The unit of a tier is character × archetype, not character (§6): a single grade averaged over archetypes gives a grade no team has (Cyno: S in Stellar-Conduct teams, B or lower elsewhere at C0, much stronger from C1, and an A for him draws "why?" from both sides).

## 6. Rejected along the way

- **One grade per character**, averaging its archetypes (§5).
- **A per-character default constellation** (4★ C6, 5★ C0): the C0 selection would then show 5★ at C0 beside 4★ at C6.
- **A grade with the constellation where it changes** (`B → S (C1)`): most visitors would not read it and would take the site for a C0 tier list.
- **Grouped constellation tabs** (C0 / C1–2 / C6): C1 and C2 differ too much for some characters (Cyno); C3 to C5 are needed too.
- **The visitor's own builds in the tier**: that is the score pages' job (§1).

## 7. Open questions

1. **Replacement teams need rotations.** Swapping a character changes the rotation, so a value needs a library team with the same other three members. How many such pairs the library has, and so which values can be measured, is to be counted.
2. **The replacement level per slot** (damage dealer, applier, buffer) must come from a rule on the data (for example the 4★ most often used in that slot), not by hand, or it becomes an exception again.
3. **Breadth follows the library's bias**: popular characters have more harvested teams. Breadth has to count archetypes, not teams, so archetypes must be classified (who names them, from what).
4. **Cost**: every member of every team replaced, at 7 constellations, precomputed. Larger than the weights table (about 11.5 h of a 4-vCPU machine for 4,017 teams).
5. **Weapons**: which weapon each entry uses (signature R1, best 4★, free), or a weapon axis.
6. **How the two axes are shown**: a chart of peak against breadth, a grade from peak with breadth beside it, or both.
7. **Teammates' constellations** when an entry's value is measured: the selector's, the visitor's, or a fixed one.
