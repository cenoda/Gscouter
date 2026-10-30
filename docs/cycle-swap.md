# Cycle swap

> Status: In progress (§11) · Last updated: 2026-10-30

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

- A custom rotation's score measures the rotation as much as the build. It compares with any score; the page names the rotation each side used (§7).
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

## 6. Direction

_2026-10-29._ Decided with the owner: phone and desktop are both first-class. From ADOFAI only the principle is borrowed (time follows from the sequence); its turning tiles have no counterpart, and the chart is drawn on lanes like the timeline (§1).

**Input is playing the game.** Players already know the controls, so the editor uses them:
- Desktop: the game's PC keys. `1`–`4` swap, `E` skill (held `E`: hold skill), `Q` burst, click attack (held click: charge), `Shift` dash, `Space` jump, `Space` while airborne then click: plunge. Each key appends a note at the playhead.
- Phone: the game's mobile HUD. Attack, skill and burst at the bottom right, dash and jump beside them, party portraits on the right. Tap appends; long-press is the hold form, as in the game.
- Both: the chart sits above the controls and scrolls under a fixed playhead (osu!). Tapping a note selects it; insert, delete and "repeat block" act on the selection.

**Parameters.** The library uses 31 parameter names, but a few forms dominate: `skill[hold=1]` alone in 2,074 places (Zhongli 370, Chevreuse 427 under both names, Navia 206), then Chasca `aim[bullets=4]` 138, Varesa `high_plunge[collision=1]` 125, Klee `attack[travel=6]` 93, Itto `burst[prestack=1]` 91. So:
- The hold forms (`hold`, `short_hold`) come from the input (held key, long-press), never from a menu.
- Each character's note starts with the form the library uses most for it (Chasca's aim is `bullets=4`, Klee's skill `bounce=3, mine=8`), counted from `teams/*/rotation.gcsl` at build time.
- The other parameters sit on the selected note as a few chips, from gcsim's action list (`character.dm.json`, §5), only the ones whose values change damage or timing. A parameter no chip covers is not editable in the chart.

**Time grid** (open). The notes are ordered, but the lanes are drawn at real time from the simulation, and that axis is where cooldowns live:
- Each lane's background shows the skill cooldown and the burst's energy filling, from the seeded run, with a mark where each becomes ready. Energy depends on random particles, so this is one run's picture, which the page says.
- A note sits at the earliest time gcsim lets it act. Dragging it right inserts a wait before it (snapped to 0.1 s, compiled as `wait`); it cannot be dragged left of the earliest time, and the lane shows why (the cooldown or energy bar it ran into).
- Waits are relative to the note before, so changing an earlier note moves the later ones with it, as ADOFAI's tiles do.
- Proposed (2026-10-29): nothing more. No note is pinned to an absolute time. A chart runs on other players' builds (shared links, sends), where Energy Recharge, constellations and frames differ: a note pinned at 12.0 s would wait or misfire on another build, while "after the previous note, plus this wait" plays the same intent on every build. The grid is a ruler in seconds (lines every 0.5 s) for reading cooldowns and waits, and 0.1 s is the snap for waits only.
- The chart shows the first cycle for editing and the next two as faded previews, since energy shortfalls appear only once the chart repeats.
- Mockup: https://claude.ai/artifact/VYZmP1tBcoY3zfAe7neuwk (mock timings, no gcsim). Open there: the desktop key for attack (left click is taken by the page; `Z` in the mockup).

## 7. Saving, comparing, sending

_2026-10-29._ Decided with the owner; not implemented.

- **Saved in the browser.** Charts are kept in `localStorage` per team (the sorted four gcsim keys), like showcase builds (design §4.8), with a name and the date. They are never stored on the server, apart from a chart that is sent (below). The result link carries the chart (§4), so a shared chart opens anywhere, and opening someone else's offers "keep this chart". `localStorage` can come back empty (private windows, cleared data), so the link stays the chart's real home.
- **Comparable.** A custom rotation's score is a score: versus pages compare it with anything, standard or custom, on either side. The page still names the rotation each side used ("custom: <chart name>"), as it names an alternative today (design §4.5), so a reader can tell a build lead from a rotation lead. This replaces §4's "does not compare".
- **Send to the developer, one click.** When a chart outscores the team's rotation on the player's build, the result offers "send this rotation". One click sends the result link (builds and chart) and nothing else.
  - The server never trusts a result (design §4.7, "Trust"). The link is enough: whoever reviews it calculates it again with the same seeds and gets the same numbers.
  - The server keeps sent links in SQLite next to the short links (`internal/links`), rate-limited like them (20 a minute per address). A team that gets many sends shows up first. A message to the Discord server through its bot (`cenoda/Gscbot`) is an option for being told.
  - A sent link holds the UID, nickname and builds, as a short link does. The button says so, and `/about` gets a line on it (design §4.5: removed on request through Discord).
  - **Into the library.** A sent chart becomes an alternative (design §5.2) only after review. It runs through `cmd/validate` like any alternative, with no source DPS, so it is checked as a variant is (`docs/cycle.md` §7.2): kept only when it beats the team's rotation on the build it was sent with. A tool (`cmd/submissions`, say) lists sends by team, scores each against the library rotation on its build and the library build, and writes `alt/<id>/` with the gain in `notes`. This needs a rule change in design §5.4: today a rotation comes only from a community config or a published guide, never "our own idea". A player's chart, with its measured gain on a real build, would be a third kind of source. It answers `docs/cycle.md` §8 (4,375 teams without an alternative) from the players who have the constellations.

## 8. Teams the library doesn't have

_2026-10-29._ Decided with the owner: a team with no library entry can be scored with a custom rotation.

- **Today**: an unmatched team is "unsupported". It is only logged (`slog.Info("unsupported team", ...)`, `internal/web/server.go`), and the front page's picker submits only four characters that form a library team (design §4.5). Both change: the picker accepts any four characters gcsim can simulate (upstream or the fork, design §5.4), and the unsupported page offers the editor.
- **What the page still needs gcsim to have**: every member implemented, and the build mapped (design §4.2). A character gcsim lacks stays unsupported, with or without a chart.
- **Where the chart starts.** With no library rotation to record (§3), there are three starting points:
  - a blank chart;
  - a library team sharing three members, its run recorded with the fourth member's notes removed, as a template the player edits (not a rotation for the team: `docs/cycle.md` §8 says moving a rotation to another party is writing a new one);
  - a chart someone shared for those four.
- **Score**: as for any rotation (looped, fillers). There is no one-pass reference and no validation against a source, so the page says the rotation is the player's. The library's character params (Hu Tao's `start_hp=1`, design §5.2) come with library teams only. A chart sets none, so the editor needs a way to set the few that matter.
- **Path into the library**: such a chart, sent (§7), is how a team the community database lacks gets its first rotation. It enters as an experimental team (design §5.4) after review, which turns the unsupported log into a queue of actual rotations.

## 9. Decided since

- **Review of a sent chart** (2026-10-29): we verify the gain ourselves, by recalculating the sent link; nothing is automated, so no rule for automatic acceptance is needed.
- **Whose build** (2026-10-29): a gain on the sender's build is enough to keep the chart as an alternative. An alternative only competes, so it cannot lower another build's score (`docs/cycle.md` §8).
- **Character params** (2026-10-29, counted over `teams/*/team.yaml` and `alt/*/team.yaml`): not only Hu Tao. 248 teams set params: Skirk `start_serpents_subtlety` 146, `start_hp` / `start_hp%` on many characters (Hu Tao 42, Neuvillette 19, Furina 14, Kuki 12, ...), `start_energy` (Furina, Escoffier, Vodyanitsa, Shenhe), the Traveler's `base_atk_buff` and `skirk_story_buff` 21, Yelan `breakthrough` 4. Most are starting states, which the scoring window (150–450 s) mostly forgets (Hu Tao's `start_hp=1` has no effect on the looped score, design §3.1); `base_atk_buff` lasts the whole fight. Proposal: a chart for a library team takes the team's params; a chart for another team takes, per character, the value the library uses most. To check first: which params change the looped score at all.

- **Params in the editor** (2026-10-29, owner: adjustable, without adding to the UI). gcsim reads few: `start_hp`, `start_hp%` and `start_energy` for every character (`pkg/core/core.go`), and about ten of its own for 18 characters (`internal/characters`: `hexerei` 9 uses, `revelation` 4, Skirk's `start_serpents_subtlety`, Yelan's `breakthrough`, the Traveler's story buffs, Mavuika's and Skirk's `start_energy`, Fischl's `oz_travel`, ...). So:
  - They live behind the character's portrait at the start of its lane: tapping or clicking it opens a small sheet with that character's params, one control each (a slider for HP and energy, a switch or a few values for the rest). Nothing else on the editor changes.
  - The sheet opens filled with the defaults above (the team's params, else the library's most used value). A dot on the portrait marks a character whose params differ from them, and the sheet has "reset".
  - The params travel with the chart (link, browser save, send), so a sent chart is recalculated with them.

## 10. Open questions

- The time grid (§6).
- Which character params change the looped score (§9).

## 11. Implementation

Steps, in order: (1) the chart format and recording, (2) the editor page with a preview from gcsim, (3) scoring a chart on result and versus pages, (4) saving in the browser and the params sheet, (5) sending a chart, (6) teams the library doesn't have.

### 11.1 Chart format and recording

_2026-10-29._ Done (`internal/chart`).

- **Text form** (`Chart.String`, `Parse`): notes in order, separated by spaces. `2E(hold=1)` is slot 2's skill with a param, `3Q?` slot 3's burst played only when ready, `~30` waits 30 frames before the next note, `[0N 0C 0D]3` repeats a block, and `; p0 start_hp=1` sets slot 0's character params. Slots are the members' canonical names sorted, as `library.Key` sorts them, so a chart does not depend on the order a player picked them in. Action codes: N attack, C charge, E skill, Q burst, A aim, H high plunge, L low plunge, D dash, J jump, W walk. Swaps are implicit, as in gcsl.
- **In a link** (`Encode`, `Decode`): the text form, deflated, in base64url. Parsing checks every note: at most 400 notes, waits up to 10 s, repeats of 2–20 that don't nest or overlap, at most 4 params per note and 6 per character, param names `[a-z][a-z0-9_%]*`, and at least one note that always plays or one wait (`chart.ErrIdle`, 2026-10-30). A chart someone wrote cannot inject gcsl.
- **Into gcsl** (`Chart.Gcsl`): `active` the first note's member, then one statement per note. A ready note becomes `if .x.burst.ready { x burst; }`, a wait `wait(N);`, a repeat `for let rK = 0; ...`. There are no `while` loops of its own; `config.Loop` repeats everything after `active`. Looped, a chart must pass time each pass, or gcsim runs it on one frame forever, where no cancel is seen: until 2026-10-30 a chart of only "only when ready" notes (`0Q?`) did once none was ready, and one request held the server's only scoring slot for good. Parsing now refuses such a chart, and the editor says why. An action that takes no time (`walk[f=0]`) can do the same, which parsing cannot tell from the params, so every fight stops with `score.ErrStuck` after 10,000 actions on one frame (`maxStillActions`).
- **Recording a rotation** (`chart.Record`): the library rotation runs with the player's build, with each pass in a block of its own between markers (`delay(987654)`, which `score.Requests` records and does not play). The chart is what the second pass asked for: the actions the script requested in order, with their params, waits and delays. It is not what the fight executed (`score.Trace`), which loses waits and doesn't know params a wait swallowed. Runs of a repeated block (up to 16 notes, up to 20 times) become repeats. A rotation whose pass never ends (an action priority list, `for { switch ... }`, 3 of 60) gives the stretch it asked for from 30 to 60 s.
- **How close a recorded chart plays the rotation.** 60 random supported teams, library build. Each was scored as the site scores, looped with fillers (`steady.Score`, 10 iterations), once with the library rotation and once with its recorded chart:
  - 44 of 59 within 2% of the library rotation, median +0.0%, from −22.8% to +11.6%.
  - The losses had three causes (2026-10-29, §11.7): crystallize shard pickups, which are script calls rather than actions and were not recorded (8 teams, −3% to −17%; fixed by §11.7); rotations whose pass never ends, recorded as a 30 s stretch (3 teams, −14% to −23%); and the rest, likely conditions decided once in the second pass (3 teams, −2% to −11%, not checked). `is_even` alternation is not a cause: it happens within a pass, which the chart keeps whole.
  - Marking every burst "only when ready" did worse (median −4.5%, 22 within 2%), so recorded bursts are plain notes and fillers handle the waits.
  - One team failed: Navia / Gorou / Furina / Xianyun, a high plunge recorded after Xianyun's airborne window, which gcsim rejects from the ground. Guarding plunges with `if .airborne` was tried and dropped: gcsl reads `.airborne` when it asks for the action, before the previous action has made the character airborne, and 23 teams lost (within 2% fell from 44 to 37). The editor shows such a failure at its note.

### 11.2 The editor

_2026-10-29._ Done (`/edit`, `internal/web/edit.go`, `templates/edit.html`, `chart.Preview`).

- `/edit?a=<selection>&r=<chart>` opens the editor for a result page's team. Like result pages it redirects to the link with the builds. Without `r` it starts from the chart this browser kept for the team (`localStorage`, key `gscouter.chart:` and the members by slot), else from the library rotation recorded with the player's build (`chart.Record`, run in the browser). The result page has an "Edit the rotation" button.
- The browser build (`cmd/wasm`) gives the editor `record`, `preview` and `encode`; the editor runs one worker (`static/worker.js`, message `call`).
- **Preview** (`chart.Preview`): one gcsim run (seed 1, the player's build) of the chart looped three times, up to 240 s. Before each note the compiled chart has a marker (`delay(987654 + 1 + note)`), and one opens each pass. `score.Play` records the markers and does not play them, so each played note is known with when the chart reached it (`asked`) and when gcsim ran it (`start`). A note "only when ready" that was skipped has `start` −1. Every 15 frames the preview samples each member's burst energy and skill cooldown; field time and waits come from the result (`timeline.Build`). When gcsim stops the fight, the error names the note it was at. It takes about 10 ms natively.
- **Preview DPS**: the run's DPS from the second pass on. When a member waits for burst energy or its skill's cooldown, the chart also runs with the fillers the score tries first (`docs/fillers.md` §2: own skill, party skills, pass, each with skill cooldowns filled by attacks), and the DPS is the best of those runs and the chart as written; the lanes keep the chart as written, waits and all. Until 2026-10-29 the DPS was the chart as written: a chart asking for a burst without "only when ready" stood still for the rest of the 240 s, and its preview fell far below its score (a player's Flins / Ineffa / Aino / Sucrose chart: 34.7k previewed, 72.8k scored; with the library build and Flins' burst three times a pass, 5.1k previewed, 54.9k now, 55.4k scored). The fillers cost up to three more runs.
- **Drawing** follows the mockup (§6): lanes at 46 px/s, pass 1 editable and passes 2–3 faded, waits gcsim imposed hatched in the warning color, waits the chart asks for hatched gray, cooldown bars with a ◆ where the skill is ready, the energy line in each lane's bottom band, repeat brackets, and the playhead. A phone held sideways gets shorter lanes and the controls beside the chart.
- **Input**: the controls and the game's PC keys (`Z` for attack, §6). A long press or held key gives the held form. Drag a note right to add a wait before it (0.1 s steps). Selection allows tap or hold, "only when ready", insert before, a range with a repeat count, and delete. Undo, and the chart in the address bar (`r`) after each change.
- **Settings** (⚙, under the controls): the keys and the party order, kept in this browser for every chart (`gscouter.keys`; `gscouter.order:` and the members by slot). Players dash with different keys, so every control and the four swaps can be rebound, to any key (its physical position, `KeyboardEvent.code`, so a Korean IME does not turn `E` into `ㄷ`) or to the right, middle or side mouse buttons; a key already in use trades places. The party order starts as the player picked the team and is changed by dragging a member, in the settings or on the controls; the number keys swap to a position and the lanes are drawn in that order. Chart slots do not change (§11.1), so a chart reads the same in any order.
- Not yet: the versus page's own editor links (a chart reaches a versus page only through its link, `ra`/`rb`).

### 11.3 Scoring a chart

_2026-10-29._ Done.

- A result page with `r` (a versus page with `ra` and `rb`) scores that side's chart. The picker form already uses `c`, hence `r`. The chart is part of the selection (`selection.Chart`), so link previews, short links (`shortTarget` keeps a valid chart), the enemy form and the link without artifacts keep it.
- `compute` scores the team's rotations as before, then the chart with the same pipeline (looped, fillers, `steady.Best` with the chart as its only rotation, with the team's character params unless the chart sets its own). The page shows the chart's score, with a note giving the library rotations' score on the same build and the chart's difference. A browser that opened the library score first has those simulations kept, so only the chart's are new.
- A chart gcsim cannot play gives a problem in place of the score, with gcsim's error.

### 11.4 Saving and params

_2026-10-29._ Done.

- **Saved charts**: a row under the editor's title keeps named charts per team in `localStorage` (`gscouter.charts:` and the members by slot; up to 20, newest first) and loads or deletes them. The chart being edited is also kept as a draft (§11.2). A phone held sideways hides the row.
- **Params sheet**: each lane's portrait opens its character's settings: starting HP and starting energy for everyone, Yelan's `breakthrough`, Skirk's `start_serpents_subtlety`, the Traveler's story buffs, and any param the library team sets. A dot marks a portrait whose params the chart sets. `Chart.WithParams` merges them over the team's own (the chart's value wins per key), so changing one param keeps the others, such as Hu Tao's `start_hp=1`.

### 11.5 Sending a chart

_2026-10-29._ Done (`POST /send`, `internal/web/send.go`, `cmd/sends`).

- A result scored with a chart that beats the library rotations on that build (`teamResult.Beats`) shows "Send this rotation to the developer". It posts the page's link; the server checks it as a short link (`shortTarget`, which must keep a valid `r`), keeps it in the short-link database's `sends` table (once per link, at most 5 a minute per visitor), and answers 204. The button says what the link holds, and `/about` lists it with the other things the server keeps.
- `go run ./cmd/sends -links <db> -site <url>` lists them, newest first, as links that recalculate the chart against the library when opened. Review is by hand (§9): a gain that holds on the sender's build is enough to add the chart as an alternative.
- Not built yet: turning a sent chart into `teams/<id>/alt/<id>/` (a written alternative, design §5.4's third kind of source).

### 11.6 Teams outside the library

_2026-10-29._ Done.

- **Picker**: four characters that are not a library team enable the button, which opens the editor instead of the result (`index.pick_custom`). A versus pick still needs a library team.
- **Result page**: an unsupported team's message says it can be scored with a written rotation, with a button to the editor (`teamResult.Writable`). A result with `r` scores a team the library doesn't have with the chart alone (`customTeam`: members, no rotation), with a note that nothing compares.
- **Editor**: the first chart is a library team sharing three members, recorded with its own build and moved to the player's slots, the missing member's notes dropped (`Chart.Remap`, `record`'s fourth argument), with a note naming that team; with none, an empty chart. Which team: the first by ID among supported teams sharing three members; not measured for how good a start it makes.
- A character gcsim lacks still fails: its preview or score shows gcsim's error.

### 11.7 Picking up crystallize shards

_2026-10-29._ Done (`Chart.Pickup`).

- **Why**: 511 library rotations (11%) call `pick_up_crystallize`, a script function rather than an action, so recording dropped it. A shard's shield triggers Archaic Petra's +35% elemental DMG for the party and the shards behind Navia's skill. Removing only the pickups from the library rotation (library build, fillers, 10 iterations) lost exactly what the recorded chart lost: Arlecchino / Kaeya / Yun Jin / Xilonen −12.4% (chart −12.4%), Wriothesley / Xilonen / Bennett / Skirk −12.1% (−12.1%), Navia / Rosaria / Shenhe / Xilonen −17.3% (−17.4%).
- **How**: a chart with `; pickup` sets up the hook Navia rotations already use: every action, once it ends, picks up a shard of any element if one is on the ground (`pick_up_crystallize("any")` in an `execute_action` wrapper). It lives in the setup, so `config.Loop` runs it once, and the preview's markers skip it. `chart.Record` turns it on when the library rotation picks shards up.
- **Measured** on the 8 recorded teams with pickups, against the library rotation: Arlecchino / Kaeya / Yun Jin / Xilonen +0.9%, Wriothesley / Xilonen / Bennett / Skirk +0.0%, Navia / Rosaria / Shenhe / Xilonen +0.1%, Yoimiya / Yelan / Xilonen / Chiori −1.3%, Navia / Zhongli / Xingqiu / Bennett −1.8%, Zhongli / Bennett / Mavuika / Ganyu −4.5%, Neuvillette / Furina / Zhongli / Sayu −5.1%, Arlecchino / Rosaria / Albedo / Zhongli −5.3% (before: −3.2% to −17.4%). Picking the element the rotation names instead of any element gave the same scores on all 8. What remains on the last three is not explained yet.
- **Default**: on when the team's library rotation picks shards up, and for a team the library doesn't have; off otherwise, since turning it on there gives the chart a bonus the library score lacks, and a "better" chart sent to the developer could owe its gain to the shards. The editor shows the switch under the title with an explanation ("What does this do?") of the reaction, the default, and whether this team's library rotation picks shards up. A result scored with it says so in its note. Hidden in a phone held sideways, apart from the switch.
