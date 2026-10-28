# Gscouter Roadmap

> Last updated: 2026-10-28

The design document (`docs/design.md`) describes the MVP: a team DPS score. This roadmap is where the site goes after it, drawn from the owner's mind map.

## Rules

- **DPS is the center.** Every feature grows outward from the team DPS score.
- **A branch starts only when the nodes it depends on are complete.** Outer nodes come last.
- **Server cost is a constraint on every node** and an open question. Outer nodes run many more simulations per page (one per candidate), so they depend on calculation staying in the visitor's browser (§4.7 of the design) and on it getting faster.

## Map

Rings count the steps from DPS. A node depends on every node it is drawn from.

```
                               AI planner
                          (needs both branches)
                         /                     \
       Primogem calculator                      Resin per DPS gained
               |                                          |
       Primogem priority ──────────── Breakthrough guide   |
               |                    Weapon guide ── Character building guide
       Pull planning                                     /       |
               \                                        /        |
                Pull a character ──────────────────────          |
                          \                                      |
  Tier & Wiki ─────────────────────── DPS ───────────────────────┘
          \                      /   |   \
           Arena ───────────────     |    faster DPS
                                     |
                 Artifacts ──────────┼──────────── Cycle
                 /   |                              /    \
   Artifact swap   Artifact score         Cycle swap      Boss
                     |                                     |
          Artifact resin expectation                 Boss patterns
                                                           |
                                         Spiral Abyss · Stygian Onslaught
                                                           |
                                                 Imaginarium Theater
```

## Nodes

Status: ✅ done · 🔶 partly done · ❌ not started.

| Ring | Node | Depends on | Status | What it is |
|---|---|---|---|---|
| 0 | DPS | — | ✅ | Team DPS from the player's builds with a fixed rotation, versus view, calculated in the browser |
| 1 | faster DPS | DPS | 🔶 | Shorter time to a score: 10–27 s in the browser on 4 CPUs today, most of it simulation (§4.7). Outer nodes multiply the number of simulations per page |
| 1 | Tier & Wiki | DPS | 🔶 | Team and character tiers from scores. `/teams` lists the library; there are no tiers yet |
| 1 | Artifacts | DPS | ❌ | Each artifact kept piece by piece, shown as totals by default, checked for duplicates (`docs/artifacts.md`) |
| 1 | Cycle | DPS | 🔶 | The rotation and its timeline are shown (§4.5); nothing about it can be changed yet |
| 1 | Pull a character | DPS | ❌ | The score with a character the player does not own, on an assumed build |
| 2 | Arena | DPS (strongly), Tier & Wiki | ❌ | Players' scores ranked against each other. Results are never sent to the server today (§4.7), so this needs a design change |
| 2 | Artifact score | Artifacts | ❌ | CV extended to every stat, with weights simulated on the library builds and a penalty for a wrong main stat (`docs/artifacts.md`) |
| 2 | Artifact swap | Artifacts | ❌ | Swap an artifact and see the new score |
| 2 | Cycle swap | Cycle | ❌ | Change the rotation and see the new score |
| 2 | Boss | Cycle | 🔶 | Score against a particular boss. Boss presets for level and resistances exist (§3.3) |
| 2 | Pull planning | Pull a character | ❌ | Which characters to aim for |
| 2 | Primogem priority | Pull a character | ❌ | Where primogems raise DPS the most |
| 2 | Character building guide | Pull a character, DPS | ❌ | A building order for the player's own account |
| 3 | Artifact resin expectation | Artifact score | ❌ | The expected DPS gain from farming a domain, per resin |
| 3 | Resin per DPS gained | Character building guide | ❌ | Upgrades ranked by DPS gained per resin spent |
| 3 | Breakthrough guide | Character building guide, Primogem priority | ❌ | Ascension and talent priorities |
| 3 | Weapon guide | Character building guide | ❌ | Weapon choice and priority |
| 3 | Primogem calculator | Primogem priority | ❌ | Primogem income and savings |
| 3 | Boss patterns | Boss | ❌ | Invulnerability, movement, shields and phases |
| 4 | Spiral Abyss · Stygian Onslaught | Boss patterns | ❌ | Scores for the current season's content. Stygian Onslaught, a single boss on a timer, fits the single-target score best |
| 4 | AI planner | the primogem branch and the building branch | ❌ | A plan from the numbers above: what to pull and where to spend resin |
| 5 | Imaginarium Theater | Spiral Abyss · Stygian Onslaught | ❌ | Character restrictions and many targets |

## Next

Every ring-1 node can start now. faster DPS lies under every outer node, so it should go first or alongside the others.
