# Cycle swap

> Status: Research (no code yet) · Last updated: 2026-10-29

Roadmap node Cycle swap (`docs/roadmap.md`): a player writes or changes a team's rotation and sees the new score. The owner's premise: nobody will type gcsim script, so the editor is built like a **rhythm-game chart editor**: one lane per character, actions as notes on a time axis. This document records what the code and the library allow, what other tools do, and the open decisions.

## 1. What the site has today

_2026-10-29._

- **The timeline is already a chart.** `internal/timeline` reduces one gcsim run (seed 1) to per-character lanes: actions with their start frame (`stats.ActionEvent`: frame and action name only, no parameters), time on field, waits (`FailedActions` with a reason: `insufficient_energy`, skill cooldown, stamina), and damage per second. `internal/web/timeline.go` draws it as SVG at 24 px/s, a lane per party slot, skills as circles (E), bursts as squares (Q), attacks as ticks, waits hatched (`templates/timeline.html`). An editor can reuse this drawing: the read-only view is the editor without input.
- **Everything runs in the browser.** The WebAssembly build (`cmd/wasm`, design §4.7) runs gcsim in workers. A filler run of the 450 s looped fight costs about 0.08 s plus 0.02 s per iteration (`docs/speed.md` §1); one pass of a 20–30 s rotation at one iteration is a small fraction of that. So the editor can re-simulate on every edit (debounced) and show where the notes really land, and the final score with fillers takes the 11–14 s of a result page today.
- **The score pipeline takes rotation text.** `config.Assemble(conditions, build, rotation)` → `steady.Best` (looping, fillers, alternatives) → `score.Aggregate`. A custom rotation is one more `steady.Rotation`; nothing downstream needs to know where it came from. Caching is keyed by the assembled config (design §4.4, §4.7), so a custom rotation caches like any other.
- **gcsim accepts any evaluator.** `simulation.New(cfg, eval action.Evaluator, core)` takes the evaluator that hands out the next action (`internal/score/score.go`, `simulate`). Today it is gcsim's gcsl interpreter. A chart can either be compiled to gcsl text (sharable, shown as the script, works with `config.Loop`, `config.Fill` and every check) or played by an evaluator of our own. Compiling to gcsl is the cheaper path; a wrapping evaluator is still useful to **record** what a gcsl rotation did, with parameters, which `ActionEvent` drops (§3).

## 2. Why a chart and gcsim don't line up

- **Time is not placed, it follows.** In gcsim an action starts when the previous one can be cancelled into it: `action.Info.Frames(next)` depends on the *next* action (attack → dash cancels earlier than attack → skill), hitlag pauses frames, and normal attacks carry combo state. A note's length is therefore known only after simulating it with its neighbour. A rhythm editor puts notes at free times; this editor puts notes in **order**, and the simulation decides where they land. Gaps are explicit `wait`/`delay` notes.
- **One lane at a time.** Only the character on field acts; placing a note on another lane is a swap (12-frame swap delay, design §3.2). Off-field damage (Xingqiu's rain, Yelan's dice) is not a note but shows as the lane's damage and buff spans.
- **A note can be refused.** A burst without energy, a skill on cooldown: gcsim waits (a hatched wait today). The editor must show this at the note, not as a silent stretch.
- **The library is not linear.** Of the 4,570 library rotations (`teams/*/rotation.gcsl`), only 11 have no control flow. 4,508 use `for` (a fixed number of cycles), 3,122 `if`, 1,414 `while`, 793 `fn`; 2,516 use action parameters (`skill[hold=1]` in 2,074 places). The conditions, by count of occurrences (characters folded):

| Condition | Uses | Chart equivalent |
|---|---|---|
| `if .X.burst.ready` / `.X.skill.ready` | 2,383 | an **optional note**: played only when ready |
| `if .X.burst.ready && .X.energy == .X.energymax` | 344 | the same |
| `while !.X.mods.favonius-cd` | 331 | a weapon-proc wait (now guarded, `docs/cycle.md` §5.1) |
| `if .airborne` | 294 | part of a plunge pattern |
| `if is_even(i)` | 247 | a note played **every other cycle** |
| `while !.X.burst.ready { X attack }` | 176 | "attack until ready": a **hold note** that ends on a condition |
| `while .X.status.clorinde-night-watch`, `while .X.nightsoul.state`, `while .X.status.xianyun-airborne-buff` | 353 | a **hold note** that lasts while a state does |

  So a chart covers the library only with three note kinds beyond the plain one: optional (if ready), per-cycle (every N cycles), and hold (repeat a pattern while a condition holds). Everything else (`fn`, `prev_char_id`, `action_id` checks, counters) stays script-only.

## 3. Starting from the library rotation

A blank chart is a poor start: the player wants "the team's rotation, but with Furina's C6 attacks". Two ways to fill the chart from it:

1. **Record the run.** Run the library rotation once (one pass, seed 1) with an evaluator that wraps gcsim's and records every `action.Eval` (character, action, parameters) with its frame. The first cycle of the looped run (after the setup) becomes the chart, unrolled: every `if` already decided, every `while` already counted. Works for every library rotation. Loses intent: an optional burst that happened to be ready becomes a plain note, so a slower build waits where the script would have skipped it.
2. **Translate the script.** Map the gcsl AST to notes where the table in §2 has an equivalent and refuse the rest. Keeps intent but covers only part of the library; how much is not measured yet.

Recording first, marking the notes that came from an `if` (the wrapping evaluator knows which branch issued them) as optional, gets most of (2) at the cost of (1).

## 4. Fairness and sharing

- A custom rotation's score measures the rotation as much as the build. As with a custom enemy (design §3.3), the page says the score uses a custom rotation and does not compare with standard scores, and the versus page needs a rule (both sides custom, or none).
- The rotation travels in the result link, like builds (design §4.8): a compact encoding of the notes (character slot, action, parameters, note kind), deflated. A 60-note chart is a few hundred bytes; the short link (`/s/<id>`) carries it as it carries builds.
- Safety: a chart compiles to statements with no unbounded loop except hold notes, each bounded (as `config.WaitLoops` bounds waits), so a shared chart cannot hang a visitor's browser the way some database rotations hung `cmd/validate` (design §5.3).
- The score with fillers applies as to any rotation, so a chart that overspends energy is filled or passes bursts rather than scoring 0; the editor should say when fillers changed the chart's play.

## 5. Prior art

_2026-10-29._ A web search plus a reading of the gcsim source. Little turned up for most rhythm editors (Quaver, ArrowVortex, Arcaea, Phigros, DJMAX, Muse Dash): no usable documentation was found.

**Rotation tools**
- **gcsim.app**: rotations are text only (`ui/packages/editor` is a code editor with diagnostics). Its sample viewer (`ui/packages/components/src/SampleLog/timeline/`) is read-only and close to our timeline. It has one lane per character, events as chips at a frame, status spans, and on-field stints. gcsim's docs list each character's legal actions and parameters as JSON (`ui/packages/docs/src/components/Actions/character.dm.json`, e.g. `{ability:'charge', note:'Need to use attack right before charge.'}`). That list could feed the editor's action palette (§6, parameters).
- **ClickySim** (Chrome extension on Genshin Optimizer, about 2 users): buttons (Normal, Charged, Skill, Burst, Dash, Jump) append to a sequential list. It runs gcsim WASM and exports a config. This is the only click-to-build gcsim UI found, and no user feedback on it was found.
- **RuneScape Wiki Rotation Builder** (https://tools.runescape.wiki/rs-rot/rotation_builder_guide): the closest analogue. It uses a bar of fixed ticks:
  - Clicking an ability puts it in the next free slot, and right-clicking removes it.
  - Dragging places an ability on a chosen tick.
  - Its tools are Stall, Null and Insert (Insert shifts later ticks), each on one key.
  - Cooldown-ready ticks get an icon, and buffs show as bars under the timeline. Clicking a tick shows its damage.
- **fribbels hsr-optimizer** "Rotation Combo": a grid with the ability sequence as columns and buffs or conditions as rows, and you drag-paint which buff is on for which action. It has no time axis. The pattern fits buff lanes.
- **FFXIV, WoW**: xivgear and SimC/Raidbots take rotations as code or text. Mitigation planners (LoPo) place skills at absolute times and show recast bars. XIV Rotation Hero has drag-drop and "repeatable steps". No planner that snaps to GCD slots was found.

**Chart editors**
- **osu!**: the beat-snap divisor, a fixed playhead with the timeline scrolling under it, number keys for tools (select, note, hold), and click-hold to place a hold note and drag its length.
- **Beat Saber / ChroMapper**: notes are always placed at the playhead. The wheel steps by the snap, and Ctrl+wheel changes the snap.
- **ADOFAI**: the one *sequential* chart editor. Tiles are typed in order, and each tile's angle sets the time to the next. Time follows from the sequence, as in gcsim (§2), which makes it the structural model.
- Pain point in osu! forums: there is no way to repeat a pattern across similar sections, so mappers use outside tools. Repeating a block should be built in, and the loop (§2, `for`) already is one.

**What gcsim gives an editor** (source): an action's length is a per-character cancel table indexed by the next action. It is adjusted by state and stretched by hitlag. A not-ready action is not rejected: the loop re-checks it every frame and logs the failure (`swap_cd`, `skill_cd`, `burst_cd`, `insufficient_energy`, `insufficient_stamina`, `dash_cd`). So there are no static note lengths, and energy depends on random particles. The editor must simulate: one seeded run per edit, notes drawn from the action frames, waits from the failures (as `internal/timeline` does now). Cooldown bars can come from gcsim's sample events (`pkg/simulator/sample.go`).

**Conclusion for the design**
- Notes go in order, ADOFAI-style, and the simulation decides where they land.
- Placement is the next free slot, RuneScape-style: tap an action, it is appended at the playhead. Insert and delete shift later notes.
- One key per action. Hold and optional are note modifiers.
- A block repeats as a first-class operation.
- A refused note shows as the wait in front of it.

## 6. Open questions

- Phone first or desktop first? The site's traffic comes from community posts read on phones; a lane editor needs at least tap-to-place and drag-to-reorder.
- Whether time snaps at all: notes in order with real landing times (§2), or a grid that the simulation then corrects.
- Whether custom charts can be saved per team in the browser (`localStorage`, as showcase builds are) and shared as library alternatives once reviewed.
- How a chart chooses parameters (`skill[hold=1]`, `aim[weakspot=1]`, plunge collision): per-character note variants from gcsim's action parameters.
