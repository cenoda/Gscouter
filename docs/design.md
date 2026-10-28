# Gscouter Design Document

> Status: Draft (prototype in progress)
> Last updated: 2026-10-27 · Game version baseline: 7.1

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
| Custom enemy | A result page can set the enemy's level and per-element resistances, or start from a boss preset (§3.3); marked as not comparable with standard scores | Lets players see a team against a particular boss without giving up the standard score |
| Fight duration | Infinite cycles of the community rotation (steady state after the opener) | Shows how well a build's Energy Recharge sustains the rotation; the one-pass community number is shown alongside |
| Rotation | A fixed rotation per team: a community gcsim config when one exists; otherwise one written from a published community guide, marked "experimental" (§5.4) | Removes player control from the equation; community configs are the accepted standard, and a guide is the next best source while the database lacks a team |
| Teams not in the library | Marked "unsupported" and not computed | Early on, library quality comes first |
| Characters gcsim or the community database lacks | Scored in an "experimental" tier with a badge (§5.4): characters upstream gcsim lacks run on a Gscouter fork of gcsim, and teams the database has no entry for get a rotation written from a community guide | A public site needs every released character; upstream takes months per character, and the database often has no entry for a character that upstream has (Iansan, Illuga, Odette, Alyosha) |
| Library management | Manual authoring/approval + automated validation | Rotation quality *is* fairness |
| Simulation engine | [gcsim](https://github.com/genshinsim/gcsim) (MIT license) | Frame-level simulator, community standard, fast to add new characters |
| Build input | Enka.Network UID (primary), GOOD JSON (secondary) | Manual stat entry tends to go unused |

### Rules these decisions create (to be communicated to users)

- A single target favors **hypercarry teams**. The best team in Gscouter may differ from the team that feels strongest in multi-target content. That is a rule, not a bug.
- If insufficient Energy Recharge delays the rotation, DPS drops accordingly. Managing ER is part of building.

## 3. Score Definition

### 3.1 Score

```
score = average team DPS over 150–450 s, repeating the community rotation endlessly
```

- Rotations come from community gcsim configs (§5). Those configs run the rotation **once** from full energy, trusting that energy will be full again by the next rotation. Gscouter repeats it instead (`active` runs once, the rest loops), so a build whose Energy Recharge cannot sustain the rotation waits for energy and scores lower. That is the point of the infinite-cycle rule.
- The first 150 s are excluded: every character starts with full energy, and the deficit takes a few cycles to show.
- The single pass from full energy is still computed and shown as "one rotation". It is what community configs report, and validation checks it against the source (§5.3).
- Each iteration `i` uses seed `base + i`, so the same build always gets the same score. Randomness such as crits is averaged over iterations; the result stores the standard deviation and the standard error.
- Window slicing: gcsim records each iteration's damage in 0.5 s buckets, so one run per iteration gives the window's damage (verified equal to the difference method by a unit test).

Hu Tao / Yelan / Xingqiu / Zhongli, imported from a gcsim share, source build:

| Measure | DPS | Note |
|---|---|---|
| Source share (one rotation) | 103.2k | gcsim `4ed9d50` |
| One rotation, Gscouter | 103.8k | Validation: within 5% of the source |
| **Looped, 150–450 s (score)** | **85.5k** | Yelan waits about 3 s/min for energy |

Hu Tao's `start_hp=1` param raises the one-rotation number by about 13% and has no effect on the looped score, since her skill drains her HP anyway.

| Setting | Standard error | Time (4 cores) |
|---|---|---|
| 1000 iterations (CLI) | ±0.2% | 9.8 s |
| **100 iterations (site default)** | ±0.5% | about 1 s |

History: a first library batch of 64 rotations written by agents without sources scored about a third of community numbers and was removed. Rotations now come from community configs, or, for a team the database has no entry for, from a published community guide under the rules in §5.4. A rotation is never written from our own idea of a team.

### 3.2 Common simulation conditions

| Item | Value |
|---|---|
| Enemy level | 100 |
| Enemy resistance | 10% for all elements + physical |
| Number of targets | 1 |
| Enemy HP | Infinite (time-based end); the one-pass check gives the enemy `hp=999999999` so the run ends with the rotation |
| Fight duration | 450 s with the rotation looped, first 150 s excluded; plus one pass from full energy (§3.1) |
| Iterations | Site: 100 (±0.5%); fillers are ranked on 20 (§9). CLI: 1000 (±0.2%) |
| Energy | 1 clear particle every 8–12 s (`energy every interval=480,720 amount=1`). Provisional, see §9 |
| Swap delay | 12 frames |

As a gcsim config, roughly:

```
options iteration=100 duration=450 swap_delay=12;
target lvl=100 resist=0.1;
energy every interval=480,720 amount=1;
```

### 3.3 Custom enemy

A result page can score its teams against another enemy instead of the standard one. Its URL carries the enemy:

| Parameter | Meaning | Default | Limits |
|---|---|---|---|
| `boss` | Boss preset: starts from the boss's resistances | none | a key of `config.Bosses` |
| `lvl` | Enemy level | 100 | 1–200 |
| `res` | Resistance to everything, in percent | 10 | −100–1000 |
| `pyro`, `hydro`, `anemo`, `electro`, `dendro`, `cryo`, `geo`, `physical` | Resistance to one element, in percent, overriding `boss` and `res` | `res` | −100–1000 |

- DEF follows from the level, as it does for nearly every enemy in the game (gcsim uses `(level + 100)` in the DEF multiplier), so there is no DEF parameter. DEF shred and ignore still come from the characters.
- The config's target line becomes `target lvl=90 pyro=0.1 ... geo=0.7 physical=0.4` (each element set alone, since gcsim adds an element's value to `resist=`). Everything else in §3.2 stays. A uniform enemy keeps the `resist=` form, so the standard enemy's config, and so its cache key, is unchanged.
- The page redirects to its shortest URL: `lvl` unless 100, then either `boss` and the elements that differ from its preset (e.g. `/score?a=...&boss=azhdaha&lvl=95`), or `res` for the most common resistance unless 10% and the elements that differ from it (e.g. `/score?a=...&lvl=90&geo=70&physical=40`). The standard enemy has no parameters.
- A versus page has one enemy for both teams. Picking an opponent, picking other characters, and link-preview cards keep the enemy.
- The page says the score was calculated against a custom enemy and does not compare with standard scores. A boss whose level and resistances are the standard enemy's (e.g. Hydro Tulpa at Lv. 100) is named, and its score is said to compare. The enemy form (boss preset, level and eight resistances) sits below the result, with a link back to the standard enemy. Picking a preset fills in its resistances and keeps the level.
- The enemy is described in the game's own words only: the boss, the level and each resistance other than 10%, e.g. "Azhdaha · Lv. 95 · Geo RES 70%, Physical RES 40%".
- **Boss presets** (`internal/config/bosses.go`): 14 weekly bosses and 23 world bosses. Their resistances come from gcsim's monster data at startup, so they follow the gcsim version. A few weekly bosses' gcsim data is a later phase of the fight (e.g. La Signora's second phase, 70% Pyro). Their names in the site's languages come from Yatta's public game data (gi.yatta.moe); a name missing in a language falls back to English.
- **Wording**: game terms are never translated by hand. Boss names (Yatta) and the level and resistance labels (Enka.Network's store data, e.g. "Pyro RES", "Pyro-WDS", "불 원소 내성") are written to `internal/web/enemy.json` by `internal/web/genenemy`. The boss groups use each language wiki's own term (Fandom: "Weekly Boss" / "Normal Boss" and their language links, the Spanish wiki's "Jefes semanales" / "Jefes normales"). Sentences and buttons that no wiki has are translated, and outside English and Korean the English follows in parentheses (`glossed` in `internal/web/i18n.go`), as are the boss groups where no wiki was found (German, Italian, Indonesian, Chinese).
- Not modeled: phase changes in resistance, shields, immunity, and enemy auras. gcsim's `target type=<monster>` is not used: it switches to an HP-limited fight and monster particle drops, which would break the looped score (§3.1). Presets only borrow the resistances.

## 4. System Architecture

```
[Input]               [Normalize]       [Match]               [Compute]           [Output]
Enka UID ──┐
           ├─► build normalization ─► team matching ─┬─► config assembly ─► gcsim ×2 ─► score / report
GOOD JSON ─┘   (to gcsim keys)        (library)      │   (build + rotation)   (looped)    
                                                     └─► "unsupported" + request tally
```

### 4.1 Input collection

- **Enka.Network**
  - Fetches the characters in the profile showcase (up to 8) by UID: level, constellations, talents, weapon, refinement, artifact main stats and substats.
  - Terms of use: set a unique User-Agent, respect rate limits, keep a response cache (TTL).
  - Constraint: all 4 party members must be in the showcase with "Show Character Details" enabled. Otherwise, tell the user which characters are missing.
  - Characters that leave the showcase stay pickable from the visitor's browser (§4.8).
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
- Loop the rotation for 450 s per iteration and average the team DPS from 150 s; also run it once from full energy (§3.1).
- **Where it runs**: in the visitor's browser (§4.7), or on the server for link-preview cards and browsers that cannot.
- **Cache key** = hash of the assembled config. The config contains the build, the rotation and the conditions, and the gcsim version is fixed per deployment, so the same build is never recomputed.
- Characters are compared by canonical name: gcsim aliases resolve to one key (`kazuha` = `kaedeharakazuha`), and both Traveler genders fold into `traveler<element>`. When the player's key differs from the library's (e.g. `aethercryo` vs. `travelercryo`), the rotation is rewritten to the player's key.

### 4.5 Results

- **Score**: team DPS with the rotation looped (150–450 s), plus the one-rotation number
- **Per-character contribution**: DPS and share
- **Warnings**
  - Insufficient ER: time each character spent waiting for energy, in seconds per minute of the scoring window
  - Failed actions
- **Metadata**: rotation version, gcsim version, simulation conditions
- **Rotation timeline**: one run (seed 1) of the rotation, shown in two tabs: the one pass from full energy (up to 120 s) and the looped fight from the warmup for one pass's length (20–60 s), with fillers if the score used them. It draws damage per second stacked by character, each character's time on field, skills, bursts and attacks, and the time it stands still waiting for energy, a cooldown or stamina. Below it, every stint on field lists its actions (a repeated pattern folded into one chip) and the damage created during it, and a table gives each character's damage by ability. The rotation script is shown as well. Characters are colored by party slot. Crits come from the one run, so its numbers differ slightly from the averaged score.
- **Versus view**: two UIDs/teams side by side with score, contributions, and warnings. A difference under 2% is shown as a tie, since rotation error is larger than that.
- **Site**: the front page loads a UID and lists the library teams its showcase can build; a picker takes exactly four characters and only submits four that form one of those teams (any other four would be unsupported). While a score or versus page is calculating, an overlay says so. Result and versus pages have a copy-link button; their links carry the builds (§4.8). `/teams` lists the library (thousands of teams) and filters it by character names. Recent UIDs, and the builds of their showcase characters (§4.8), are kept in the browser only. The site is in the game's 15 text languages (`internal/web/i18n.go`, one `messages_*.go` per language): a visitor gets the language picked in the header (kept in a cookie), else the first one their browser's Accept-Language asks for, else English. Characters, weapons and teams show as the game names them in that language (`internal/enka/data/names.json`, from Enka.Network's store data), and `/teams` finds teams by those names as well as by gcsim keys. Link preview cards stay in English. Characters are shown with their face icons, HoYoverse's artwork loaded from Enka.Network (not stored in the repository), and element icons drawn for Gscouter. This relies on HoYoverse allowing non-commercial fan use of game images: the site carries no ads or donations, and its footer states that it is an unofficial, non-commercial fan project and that the images are © HoYoverse.

### 4.6 Deployment

- **One Go binary** (`cmd/server`) serves the site, calls Enka, runs gcsim in-process, and caches Enka responses (for Enka's TTL) and scores (24 h) in memory. Short links are the only thing it keeps on disk (§4.8). The team library is embedded in the binary. Result pages are calculated in visitors' browsers (§4.7); the server still calculates link-preview cards and pages for browsers that cannot.
- **Concurrency**: one simulation at a time, up to 8 waiting; beyond that the request gets a "busy" message. Peak memory per simulation is about 35 MB.
- **Hosting**: a single Azure B2ats v2 VM (2 vCPU, 1 GB) with Docker. Caddy terminates HTTPS for the owner's domain. Expected cost is roughly KRW 15–20k per month.
  - Alternatives considered: in-browser WASM on static hosting (works, about 1.5 s per score single-threaded, but Enka does not allow CORS, so a proxy is still needed); Cloud Run, Oracle Always Free, Hugging Face Spaces (ruled out by the owner's constraints or uncertain terms).
- **Delivery**: pushing to `main` runs tests and publishes `ghcr.io/cenoda/gscouter:latest`. A systemd timer on the VM re-downloads `deploy/` from `main` and pulls the image every 5 minutes, so no deploy credentials live in GitHub. See `deploy/README.md`.

### 4.7 Calculating in the browser

So that the server's cost does not grow with its visitors, result pages are calculated by the visitor's browser. The server stays for what is cheap or needs it: Enka lookups and their cache, team matching, pages, link-preview cards.

- **Split**: for `/score` and `/versus`, the server looks up the showcase and the library team (`prepare`) and sends the page with the teams as a job (`web.Job`) and a "calculating" placeholder. A WebAssembly build of the same Go code (`cmd/wasm`) runs the job in Web Workers and renders the page body with the site's own templates (`web.RunJob`).
- **Workers**: a lead worker runs the team logic (fillers, timelines) and asks the page for each simulation; the page splits its iterations into chunks for up to 6 helper workers (one per CPU). Iteration `i` uses seed `base + i` wherever it runs, and the chunks are merged in order (`score.Aggregate`), so the browser's numbers are exactly the server's.
- **Reuse**: finished simulations are kept in the tab's session storage, keyed by their config, warmup, seed and iterations. They don't depend on the language, so switching it (which reloads the page) renders the result again without calculating it again.
- **Cost**: the build is about 62 MB, 10 MB gzipped, served precompressed with a versioned URL so browsers keep it. Each worker's Go heap stays around 10 MB. In headless Chromium on 4 CPUs, Hu Tao / Yelan / Xingqiu / Zhongli took 8.7 s on a first visit (download and compile included), and an Arlecchino / Citlali / Skirk / Bennett build that runs 10 simulations took 10.8 s, 6.5 s once the build was cached. On the server, the same pages take under a second each, one at a time.
- **Fallbacks**: browsers without WebAssembly or Web Workers, a failing calculation, and `<noscript>` go to the same page with `server=1`, which the server calculates as before. Link previews do not run scripts, so the preview text names the team without a score; the card image is drawn on the server.
- **Trust**: a result is never sent back to the server. A shared link is calculated again by whoever opens it, with the same seeds, so it shows the same numbers.

### 4.8 Characters outside the showcase

The showcase holds a few characters, and changing it drops the others from Enka's answer. So that a player can still pick a team of characters they showed earlier, and a shared result still opens after the showcase changes, builds also travel outside Enka:

- **Packed builds** (`enka.Pack`): a character's level, constellations, talents, weapon, artifact sets and artifact stat totals, written compactly with gcsim keys (not indexes, so a link survives data updates) behind a format version. Stats are kept at the precision `Config` writes, so a packed build gives the same gcsim config, and the same score, as the showcase did. Anyone can write one, so reading checks every key against the game data (a key could otherwise write config lines) and every number against the game's limits.
- **Result links carry the builds**: `/score` and `/versus` redirect a selection by showcase (`UID:char1,...`) to one with the builds (`UID~packed`: nickname and builds, deflated, base64url; about 350 characters of URL for a team), which needs no Enka lookup. A link opens with the same result for as long as the site exists, after the showcase changes, Enka is down, or the server restarts; link previews work the same way. Old `UID:` links still work while the characters are in the showcase.
- **Short links**: the copy-link button copies a short link (`/s/<id>`, about 30 characters) that redirects to the long one. A 350-character link reads as spam where people post it, and nearly got the site's first community post removed as flooding.
  - `POST /s` takes the page's path and query, accepts only a result page whose selections carry builds (each checked as when the page opens), drops `server=1`, and stores it. The ID is the first 8 characters of the link's SHA-256 in base64url, so a result always gets the same link and copying it again stores nothing new; an ID taken by another link gets one more character. One visitor (by the address Caddy reports) makes at most 20 links a minute.
  - Making a short link also draws the result's link-preview card in the background. The first request for a card calculates the score on the server, about 3 s on the VM, and chat apps show a blank preview while they wait; the link is pasted a few seconds after it is copied, by when the card is cached.
  - Links are kept in SQLite (`internal/links`, pure Go) on a Docker volume of the VM's 30 GB disk. A row is about 400 bytes, so the disk holds tens of millions and links never expire.
  - The long link stays the result's own, and the short link only points to it: losing the volume breaks short links and nothing else. Without the store (`GSCOUTER_LINKS` unset, or the file can't be opened), the server runs as before and the button copies the long link.
  - Alternatives considered: making the packed format smaller (numeric game IDs instead of gcsim keys, default levels as a flag, per-stat precision). Measured on a Hu Tao team, it would take the link from about 330 to 220–250 characters: the stats, which are most of it, barely compress.
- **The browser keeps showcase builds**: the front page gives each showcase character's packed build to the browser, which keeps them in `localStorage` for the ten most recently loaded UIDs, with when each was last in the showcase. When a UID is loaded again with kept characters that are no longer in its showcase, the page asks `POST /showcase` to render the showcase card with them added and dated, so their teams are suggested and pickable. Picking such a team sends the kept builds along (`build` parameters), and the redirect above packs them into the result link. Nothing is stored on the server, and a visitor can forget the kept characters.
- **Not a ranking**: since builds come from the visitor, a result says what the given builds score, not what a player owns. Gscouter has no rankings, so this is fine; the score cache is keyed by the gcsim config, so a made-up build only reaches its own results.

## 5. Team Library

### 5.1 Structure

One directory per team.

```
teams/
  hutao-xingqiu-zhongli-yelan/
    team.yaml        # metadata, character params, source and its DPS
    rotation.gcsl    # rotation from the source config (actions only)
    build.txt        # the source config's build, used for validation
```

Example `team.yaml` (written by `cmd/harvest`; `cmd/import` writes the same fields, with the share's gcsim version):

```yaml
id: hutao-xingqiu-zhongli-yelan
name: Hutao / Xingqiu / Zhongli / Yelan
members: [hutao, xingqiu, zhongli, yelan]
status: supported        # supported | draft | blocked (§5.3, step 3)
rotation_version: 1
gcsim_commit: 2350be9    # gcsim version used for validation
# experimental teams also have (§5.4):
# tier: experimental
# gcsim_fork: 91c5de4    # commit of the gcsim fork
sources:
  - https://db.kqm.gg/db/B7cTMcwpmp7F
notes: 'Harvested from the KQM gcsim database: ...'
params:                  # if the source sets any, e.g.
  hutao: start_hp=1      # added to the player's character line
reference:
  dps: 68025             # what the source reported with its own build
  computed: "2026-10-02" # when the database last ran it
validation:              # the last check (cmd/validate -promote, §5.3)
  date: "2026-10-27"
  iterations: 20
  gap: -0.012            # one pass under the source's conditions vs reference.dps
  kept: 1.078            # looped score over one pass under the common conditions
  dps: 72426             # looped score with build.txt, as the site scores it
  draft: '...'           # on a draft team, the criterion it misses
```

### 5.2 Rotation authoring rules

- The rotation is the source config's action list. Gscouter loops it for the score and runs it once for validation, so the source must not loop forever itself.
  - When looping, the setup at the start of the rotation (`active`, `let`, `fn` and `set_*` statements) runs once, and the remaining statements repeat.
- Character params the rotation assumes (e.g. Hu Tao `start_hp=1`) are stored in `team.yaml` and added to the player's build.
- One rotation is used regardless of the user's build.
  - If an action can't be performed due to insufficient ER, rely on gcsim's wait/delay behavior and surface the delay as a warning.
- Aim for a near-optimal standard rotation for the team. Uneven rotation quality across teams is itself unfairness.

### 5.3 Addition workflow (automated)

The library is filled from the community gcsim database at db.kqm.gg, the list gcsim.app links to. Every entry is a full gcsim config, and the database reruns all of them on recent gcsim versions. In October 2026 it held 6,020 entries, all rerun between 2026-09-24 and 10-02.

`cmd/harvest` does the following:

1. **Download** every entry (`/api/db`, 100 per page). `-cache <file>` saves the download and reuses it on later runs.
2. **Filter** out only entries that cannot be used at all:
   - not four characters;
   - a rotation that never ends on its own (an unconditional `while 1`);
   - a rotation without an `active` setup, which Gscouter cannot loop.

   Entries that gcsim has not accepted, random delays, action priority lists and entries with other enemies (several targets, other levels or resistances) or other energy are usable: each is judged by whether it reproduces its own DPS (step 4), and the site scores it under the common conditions.
3. **Rank** teams (sets of four characters) by how many entries use them. This is the popularity measure: what the community simulates most, not our choice. `-min` (default 1) is the least number of entries a team needs.
4. **Pick** per team, trying the entries from the lowest total constellations and refinements up, closest to an ordinary account:
   - the **reference check** (`validate.CheckReference`): one pass of the rotation with the entry's build under the entry's own conditions, 20 iterations, within 15% of the database's DPS. The database reports DPS per target, so the reference is that times the number of targets;
   - the **full check** (`validate.Check`): the looped rotation, as the site scores it, must not stall;
   - of the first five entries that pass both (`-choose`), keep the one whose score keeps the most of its one pass under the same, common conditions (`Report.Kept`, counted up to 100%; within 2%, the lower investment wins). A one-cycle entry that opens with every burst charged scores well once but loses much of it when looped (Hu Tao / Yelan / Furina / Zhongli: a 21 s entry lost 42%); an entry whose cycles earn their own energy keeps it.

   Each check runs in a child process, since gcsim cannot cancel a rotation stuck in a loop that takes no action. All entries are tried until five pass (`-tries` limits them).
5. Teams run in parallel (`-j`, default one per CPU, each simulation on one CPU; on 4 CPUs this was as fast as one team at a time on all CPUs). Teams that fail every entry go to `teams/harvest-failed.txt` with the first entry's failure, and later runs skip them unless `-retry-failed` is set.

Each team stores the source's options, target, energy and hurt lines in `source.txt`. Only the reference check uses them; the site's score and the looped checks use the common conditions (§3.2).

| Step | Mode | Description |
|---|---|---|
| 1. Harvest | Automated | `cmd/harvest -n <teams>`; rerun to refresh. `cmd/import` adds a single gcsim share link by hand. A team the database has no entry for gets a rotation written from a community guide (§5.4); none is written without a source |
| 2. Validation | Automated | `cmd/validate`. CI validates fully the team directories a push or pull request changes (30 of them, chosen at random, when it changes more), or 10 random teams when it changes none |
| 3. Approval | Automated | `cmd/validate -promote` sets `status: supported` on teams that meet the criteria below and returns the others to `draft`; `cmd/harvest` writes new teams the same way |
| 4. Regression tests | Automated | Re-run everything on gcsim updates or game patches with `-promote`, so teams that no longer meet the criteria go back to `draft`. A looped score more than 5% from the recorded `validation.dps` is a warning, in CI too |

Validation speed. A full run (`cmd/validate -iter 20 -j 4`, 4 CPUs) simulates per team one pass under the source's conditions, one under the common conditions, one probe fight and the looped score, which `steady.Score` reruns with fillers up to 16 times. Profiled on 100 random teams (2026-10-27): the looped score took 87% of the time (its first run 22%, the filler runs 64%; 11 of the 100 teams ran all 16), the two passes 5% each, the probe 3%; half the teams need one looped run. What makes a run faster without changing any result to the last bit:
- each check's process runs the garbage collector a tenth as often (GOGC 1000, a 1 GiB soft limit; at most about 260 MB per process): 40% less time. A fight allocates fast and keeps little alive;
- gcsim collects only the stats the score needs (`score.Run`): damage, failed actions, and shields, which is not only an observer: on every hit on the player it queues a task, which reorders the frame's other tasks, so without it Dehya / Furina / Albedo / Xilonen scored 1.6 DPS lower under its source's conditions (they have hurt lines). About 5% less;
- the probe is the looped score's first iteration (same config, seed and warmup; gcsim does not read the iteration count): 3% less;
- results are cached (`score.Cache`, `validate -cache`, by default under the user's cache directory; `-cache ""` turns it off): keyed by config and options, in a directory per gcsim module with its replace directive, Go version and format name (`cacheFormat`, to change with `summarize` or the collectors). A rerun on the same gcsim simulates only the teams that changed. The library takes about 210 MB (28,000 entries) per gcsim version.

The whole library took 41 minutes (148 CPU minutes) instead of 67 (233), every report identical to the last bit (`cmd/compare`, 4,433 teams); a rerun with the cache took 2 minutes. `-shard k/n` splits a run across machines. A cached run is not identical in one way: a team near the 2-minute limit of a check, which is measured in wall time and depends on the machine's load, finishes once enough of its simulations are cached. Ningguang / Xingqiu / Bennett / Chiori timed out on both cold runs and passed with the cache.

Rejected: ranking filler candidates on fewer iterations and running only the winner fully. Which follow-ups `steady.Score` tries, in what order, and which wins all depend on each run's full result. Ranked on 5 iterations (7 minutes for the library), 748 of 4,431 teams chose another filler or none, so it would change scores.

Validation checks:
- The team reproduces the source's DPS with the source's build under the source's conditions within 15% (a warning above 5%). The tolerance allows for the source having run on another gcsim version
- The rotation finishes, and one fight of the looped rotation ends within 20 s. A loop that takes no action, such as `while rotations { ... }` with the counter set once in the setup, spins inside gcsim without time passing once the counter is spent; gcsim cannot cancel it, so `cmd/validate` checks each team in its own process and such teams are removed
- Action waits and members with almost no damage in the looped rotation are reported as warnings

Approval criteria (`validate.Report.Unpromotable`). A team is `supported` when:
- the validation passes;
- its one pass is within 5% of the source's DPS (the warning threshold above);
- its looped score keeps at least 80% of its one pass under the common conditions (`Report.Kept`). A team that keeps less is scored mostly on how its rotation loops, not on how the community plays it;
- and at most 130%. Keeping much more means the one pass under the common conditions does not play as written: the rotation depends on the source's own conditions, such as several targets, targets that die or more energy (Ganyu / Venti / Kazuha / Rosaria, written for two targets, keeps 371%).

`-promote` records each team's result in `team.yaml` (`validation`, §5.1), with the criterion a draft team misses. Other teams stay `draft`, and result pages warn that their rotation is not validated yet. Approval was manual at first, but no one reviewed 4,397 teams, so every result carried the warning although every team had passed the validation. The criteria turn what the validation already measures into the decision. `-promote` never promotes an experimental team (§5.4), which a person reviews first, but returns one to `draft` when it misses the criteria.

### 5.4 Experimental tier (in progress)

Goal: every released character can be scored, including the ones upstream gcsim lacks (§6.3). Waiting for upstream leaves new characters, the ones players most want to try, unscored for months: Ineffa took about 5 months, Lauma and Flins about 5, Durin about 6, Aino about 8, and Nefer is still missing a year after release.

| Tier | Engine | Rotation | Shown as |
|---|---|---|---|
| Verified | Upstream gcsim | A community database entry that reproduces its DPS (§5.3) | Score |
| Experimental | Gscouter's gcsim fork: upstream plus the characters upstream lacks | A community entry if one exists, otherwise written by us under the rules below. This covers any team the database has no entry for, whether or not a member is fork-only | Score with an "experimental" badge: values may change |
| Unsupported | No implementation | — | "Unsupported" |

**Fork**
- A fork of gcsim under the project's GitHub account, used through a `replace` directive in `go.mod`: the `gscouter` branch of [cenoda/gcsim](https://github.com/cenoda/gcsim/tree/gscouter). It is upstream `main` plus one group of commits per added character, so a gcsim update is a rebase of those groups.
  - A group taken from an upstream PR starts with the PR squashed into one commit under its author's name, unchanged; our fixes follow as separate commits, so they can be offered back to the PR.
  - After a rebase: `go mod edit -replace` to the new commit, `internal/enka/gen.sh` (Enka mapping and names), `library.ForkOnly` and `library.ForkOnlyWeapons` updated for characters and weapons added to the fork or merged upstream, `share.GcsimFork` set to the new fork commit (a test checks it against `go.mod`), the fork's own `go test ./...` (its frame and ability tests cover every character), and the regression run below.
- Each added character comes, in order of preference, from an open upstream PR (Nefer, Linnea, Kachina have one) or from our own implementation. Multipliers come from gcsim's data pipeline either way.
- Each added character records its source (PR or ours), where each action's frame data comes from (measured, taken from the PR, or estimated from a similar character) and its known gaps. The result page links them.
- Our own implementations are also sent upstream as PRs. Review there improves accuracy, and every merge ends the fork's cost for that character.
- When upstream merges a character, its fork commit is dropped and its teams move to the verified tier after revalidation. Scores may shift; results already record the gcsim version.
- **Verified scores must not move.** The server and the browser build (§4.7) run both tiers on the same gcsim, so the fork must leave every verified team's score unchanged (the regression run in §5.3, step 4 of the table). Engine changes a new character needs, such as Vodyanitsa's Wandering Vortex, are switched on only when that character is in the team, the way gcsim already enables Stellar Swirl through `Flags.Custom`.
  - Regression run: every library team through the full check (`validate.Check`, 20 iterations) on upstream and on the fork, comparing the reference pass, the common pass and the looped score, per character, at full precision. Seeds are fixed, so the difference must be exactly 0. Result for the first fork (2026-10-27, 40 minutes per run on 4 CPUs): 4,396 of 4,397 teams identical to the last bit, among them all 228 with Lauma, Columbina, Flins, Ineffa, Aino or Zibai. The other, Klee / Citlali / Xilonen / Xianyun, differed by 0.16 DPS in Xianyun's damage on the one pass under the common conditions; the upstream run was the odd one out. Rerun 83 times, upstream and the fork both always gave the fork's number. So upstream gcsim, very rarely, does not repeat an iteration exactly; the cause is not known yet.
  - Result for fork `af3eb82` (Lohen, 2026-10-27, 51–53 minutes per run on 4 CPUs): 4,406 of 4,428 teams identical to upstream `2350be9` to the last bit, in every reported field. The other 22 are the experimental teams with a fork-only member, which upstream cannot run; against the previous fork `91c5de4` all 22 were identical too, as were the 1,373 other teams that run reached. One team, Ningguang / Xingqiu / Bennett / Chiori, times out after 2 minutes on every build, so its check never finishes. No difference appeared, so no reruns were needed.
  - Result for fork `13a481c` (Sandrone, 2026-10-27, 40 minutes per run): every library team (4,433) through `validate.CheckIsolated` (20 iterations) on the previous fork `af3eb82` and on `13a481c`, compared field by field at full precision: all 4,433 identical, among them the 27 teams with a fork-only member and the 20 with Odette, Qiqi, Alyosha or the Cryo Traveler (the Stellar-Conduct and Stellar Swirl teams); none failed. So the shifted ICD tags, groups and character keys change nothing. Upstream `2350be9` was not rerun: the Lohen work found `2350be9` and `af3eb82` identical on 4,406 of 4,428 teams, and `13a481c` changes nothing outside Sandrone relative to `af3eb82`.
  - Result for fork `6fcfc4c` (Vodyanitsa, 2026-10-27, 79 minutes per run, the two builds in parallel on 2 CPUs each with `GOGC=1000`): every library team (4,433) through `validate.CheckIsolated` (20 iterations) on the previous fork `13a481c` and on `6fcfc4c`, compared field by field at full precision: 4,431 identical, among them the 27 teams with a fork-only member that run and the 20 with Odette, Qiqi, Alyosha, Sandrone or the Cryo Traveler, the ones the Wandering Vortex hook could touch. The other two differ only outside gcsim: Ningguang / Xingqiu / Bennett / Chiori times out on both builds (the known timeout above; only the kill message differs), and the blocked Vesna / Vodyanitsa / Cryo Traveler / Faruzan, which runs on neither, is skipped as blocked by this commit's loader and rejected by the previous one, which predates its `gcsim_fork`. No value differed, so no reruns were needed. The upstream run was not repeated: upstream `2350be9`, `af3eb82` and `13a481c` are already shown identical above.
  - Result for fork `3e75b40` (Vesna, 2026-10-27, 78 minutes per run, both builds in parallel on 2 CPUs each, `GOGC=1000`): every library team (4,433) through `validate.CheckIsolated` (20 iterations) on the previous fork `6fcfc4c` and on `3e75b40`, compared field by field at full precision: 4,432 identical, among them the 28 teams with a fork-only member and the 20 with Odette, Qiqi, Alyosha or the Cryo Traveler (the Stellar-Conduct and Stellar Swirl teams). The other is Vesna / Vodyanitsa / Cryo Traveler / Faruzan, which is `blocked` and only skipped on both builds; its skip message quotes its updated note. Ningguang / Xingqiu / Bennett / Chiori times out after 2 minutes on both, as before. No difference appeared, so no reruns were needed.
  - Result for fork `87338fd` (Vodyanitsa's Lead Vocal and Chorus on direct Stellar Swirl hits, 2026-10-27): the `regression` workflow, baseline `main` (fork `6fcfc4c`) and candidate `claude/vesna-fork` (fork `87338fd`, which includes Vesna's `3e75b40`), the candidate's teams on both sides ([run 40521378502](https://github.com/cenoda/Gscouter/actions/runs/40521378502)): 4,433 teams, 0 differ (0 DIFF, 0 REGRESSION, 0 NONDETERMINISTIC), none on one side only. No library team has Vodyanitsa except the blocked Vesna team, so the change shows only in the calibration in Vesna's row.
  - Result for fork `efcbe0e` (the two upstream artifact fixes below, 2026-10-28): the `regression` workflow, baseline `main` (fork `87338fd`) and candidate `claude/upstream-artifact-fixes-probe` (fork `fc10145`, the same tree as `efcbe0e`: the commits differ only in their messages), the candidate's teams on both sides ([run 40601948636](https://github.com/cenoda/Gscouter/actions/runs/40601948636)): 4,433 teams, 0 differ (0 DIFF, 0 REGRESSION, 0 NONDETERMINISTIC), none on one side only. As expected: no library team wears Disenchantment in Deep Shadow, and the 31 teams with Scroll of the Hero of Cinder City include none of the 20 with a Stellar character (Odette, Qiqi, Alyosha, the Cryo Traveler, Sandrone, Vesna, Vodyanitsa), so no wearer triggers a Stellar Swirl.
  - Result for fork `76e8116` (Hymn of the Maelstrom, 2026-10-28): the `regression` workflow, baseline `main` (fork `efcbe0e`) and candidate `claude/hymn-fork` (fork `76e8116`), the candidate's teams on both sides ([run 40617384146](https://github.com/cenoda/Gscouter/actions/runs/40617384146)): 4,433 teams, 0 differ (0 DIFF, 0 REGRESSION, 0 NONDETERMINISTIC), none on one side only. No library team wears Hymn, so this checks that the new weapon key, which moved 148 weapon keys up by one, changes nothing.
  - Because of that, a team that differs is not yet a regression: rerun it on both builds, at least 20 times each. Only a difference that repeats counts and must be fixed; a value that appears once on one side and never again is recorded as that nondeterminism. `cmd/compare/build.sh` builds `cmd/validate` on the fork, on upstream (go.mod without the replace directive) and on other fork commits, without changing go.mod; `validate -json` writes every report with the gcsim it was built with, and `cmd/compare` compares two runs field by field. Given several runs per side, it tells a regression (a result that repeats on one side and never appears on the other) from that nondeterminism (results seen once); with one run per side it lists the teams that differ, to rerun with `-cache ""` (the cache would repeat the first result). On an upstream build, `cmd/validate` skips teams with fork-only characters.

**Characters on the fork** (fork `76e8116`, on upstream `2350be9`, 2026-10-28)

Nefer and Linnea have draft library teams (Rotations below); Kachina, Lohen, Sandrone, Vodyanitsa and Vesna have none yet (the only team with Vodyanitsa and Vesna stays `blocked` until it has a rotation). Lohen is fork commit `af3eb82` (our implementation, the commit before Sandrone's); his `ForkOnly` entry and Enka data came with Sandrone's Gscouter commit.

| Character | Source | Frame data | Known problems |
|---|---|---|---|
| Kachina | Upstream PR #2697 (danieldlc01), open since 2026-07 with requested changes | The PR cites a frame count by caramielle. Normal attacks look counted (the review asked for a recount); charged attack estimated (every cancel 49–50); skill: counted cancels; its animation ended at 733, the time of Turbo Twirly's last independent attack, and now ends at 57, the longest counted cancel (estimated). The community confirms the skill as about 12 s of damage over time, which Turbo Twirly models on its own (143, then every 118 frames, 10 of 60 points per hit); mount/dismount recast all 30 (placeholder); burst not counted | A1 gives +20% DMG to the active character on any Geo hit instead of Geo DMG to Kachina only; C1 triggers on crystallize, not shard pickup, and its shard absorption is missing; C4 is not polled; no Geo construct; Twirly loses no points while idle and loses 10 per enemy hit; swapping out while mounted can leave Twirly mounted forever; no plunge. Fixed on the fork: compile fix for current upstream, 500-frame charge cancels, Twirly's independent attacks firing every 236 frames instead of 118, the 733-frame skill animation |
| Linnea | Upstream PR #2596 (TheSast), open since 2026-04 with requested changes | Skill (first press) and burst cancels counted by caramielle; normal attack hitmarks counted, but their cancels are Gorou's; aimed shots are the generic bow template; skill recast a placeholder (10); plunge missing | The burst while Lumi is out stops Lumi's attacks and the A1 shred for the rest of its duration; C1 consumes stacks once per Lunar-Crystallize contributor instead of once; C2's extra attack does not emit Moondrift Harmony; C4 may give Linnea +50% DEF on field instead of +25%; the A4 EM buff is not refreshed on swap. Fixed on the fork: a duplicate registration that panicked at startup, frame invariants (skill animation 14 → 44, the longest cancel, as the upstream review asked; N3 animation 43 → 47, its hitmark; burst queue at the earliest cancel), C3 raising normal attacks instead of the skill, C2 skipping members after the first non-Geo/Hydro one |
| Lohen | Ours (2026-10-27, fork `af3eb82`): multipliers from the pipeline; mechanics from the game's ability data (`ConfigAbility_Avatar_Lohen`, `ConfigAvatar_Lohen`): Will to Win multiplies the talent multiplier by 1 + 0.004 per point (snapshotted at cast), the Hexerei passive is +40% normal and charged attack DMG for 6 s at Will to Win ≥ 50% of its limit with 2+ Hexerei members, and normal and charged hits share a game ICD group of 6 Cryo applications per 5 s | Every frame is estimated: no community frame count exists (KQM quick guide, TCL). The N1C loop (N1 → charged attack → N1) is 43 frames, calibrated so that a 13 s Masterstroke fits 15–16 loops after the skill and 17 only with near frame-perfect input, as the KQM Lohen quick guide reports; other normal attacks follow Flins; skill, Etched Into Bone and Soul and burst animations are estimates. From the ability data, not estimated: hit lag, defense halt, hit counts and hitboxes, Etched's hit order and the burst's hit spacing. Each 5 frames of the N1C loop move team DPS by about 3–5%. Calibration: an Etched Into Bone and Soul crit hits 28.5k, against 28k+ on the creator server ([HoYoLAB 45375045](https://www.hoyolab.com/article/45375045), all C0 with signature weapons); Lohen / Nicole / Durin / Citlali (C0, single target, 10% RES, Nicole's and Durin's signatures, ER for every burst, KQM's average combo) scores 180.6k with Disaster and Remorse and 162.9k with Deathmatch R5, against community figures of about 190k and 160k | No plunging attacks; C2's Evilsbane Blade and the C6 Joy refill after the burst fire at a fixed frame 95 (the burst animation's end is estimated); the particle count (1 per Cryo hit, every 2 s) is inferred from the data, not checked in game; teammates' transformative reaction damage grants no Will to Win (direct Lunar and Stellar damage does), a guess where the data is unclear |
| Nefer | Upstream PR #3045 (dersario), open since 2026-09, no review | The PR cites a frame count by caramielle; normal attack, charged attack, skill and burst numbers are irregular and match no other catalyst, so counted. Slither's stamina drain and movement, the Seed absorb radius (6) and the Slither entry (0 frames) are assumptions | Changes Verdant Dew generation for every team (below); no hitlag or poise; documented hitboxes differ from the code; Phantasm and charged hits are mostly 25U with no ICD; `.nefer.veilStacks` and `.nefer.phantasmCharges` are documented but do not resolve; C4 re-adds its resistance shred to every enemy every frame (slow); Slither moves the player. Fixed on the fork: N4's animation (51 → 63, its walk cancel), plunges with no frames (removed: Nefer cannot plunge), the burst's Veil bonus expiring before either hit |
| Sandrone | Ours (2026-10-27): multipliers from the pipeline; mechanics from the game's ability and gadget data (`ConfigAbility_Avatar_MarionetteNew`, `ConfigAvatar_MarionetteNew`, `ConfigGadget_SkillObj_MarionetteNew`; MarionetteNew is her internal name). Fagio is a task loop driven by its Decoding Power, not a gadget: in game its sweeping fire, condensed beams and Power Overdrive shots are bullets Sandrone fires while her charged attack is held. Decoding Power gains 3.2 every 0.2 s and 17 per beam count (both halved at C1) and is set to 100 after the 3rd count (C1: 6th), so a charged attack from 0 fires 3 condensed beams at C0 and 6 at C1. In Radiance: Stellar-Conduct or Stellar Swirl the beams, the second Prism Shot and the burst's ray are replaced by direct Stellar-Conduct or Stellar Swirl hits with their own multipliers, which apply no element; the engine's Polestar Field and Radiance pattern (Odette's) is reused unchanged | Every frame is estimated: no community count exists (KQM quick guide, KQM library). Charged attack entry 21 and end 18 frames and skill → charged attack 22 are set so that a C E pair costs about 1 s besides Decoding mode, and 3[C E] at C0 and 2[C E] at C1 fill the 10–13 s of field time the KQM quick guide reports; normal attacks follow Dehya's N1–N3; burst animation 100 frames. From the data, not estimated: Fagio's timings (first beam at 0.85 s then every 1 s, sweeping fire every 0.3 s, Power Overdrive shots every 0.4 s, Decoding Power rates and decay, the skill's drain of 70 then 10 per 0.1 s), Prism Shot and burst hit times, ICD and gauges. Calibration against [HoYoLAB 46338256](https://www.hoyolab.com/article/46338256) (C0 Sandrone with The Unforged R1, C1 Yae Miko, C0 Escoffier, C0 Odette; Sandrone's and Odette's artifacts as posted, Yae's and Escoffier's weapons inferred from their base ATK and their stats matched to the posted panels; level 100 dummy, 10% RES, 120 s; every member's panel stats reproduced exactly): team 184.2k against 209.1k (−11.9%); Sandrone 11.07M against 13.28M (−16.7%), Odette −3.6%, Yae −16%, Escoffier +27%; strongest hit, the ray with 10 Refined Tactics stacks, 622k against 658k (−5.4%) with the burst right after the first skill, 521k when it ends her field time. The post gives no rotation (ours is KQM's 3[C E] with the burst after the first skill). Polestar Field stacks are 8–9 in most of her hits; with the multiplier held at 12 stacks (a test build) the team gives 200.2k (−4.2%), but Odette then overshoots (+8.8%) while Sandrone and Yae stay about 9% low, which points to field time per member, i.e. the unknown rotation. Gap investigation (2026-10-28, fork `efcbe0e`, same config: 184.3k), no cause found in our code: (1) Polestar Field stacks follow the game's rule as KQM's Stellar Glimmer guide states it and the data shows it (`TeamAbility_StarSuperconductor`: a hit counter capped at 12, copied to the party and reset every 4 s): only Cryo and Electro hits that apply an element count, as limited by ICD, one per attack, and direct Stellar-Conduct hits (which apply none) do not; the sim records about 230 such applications per 120 s (Escoffier's Frosty Parfait 52, Sandrone's sweeping fire 45 and Prism Shot 1 16, Yae's Sesshou Sakura 40, Odette's dance moves 51, the rest bursts), about 8 per 4 s window, so 8–9 stacks is right for this rotation. The post's strongest hit, the ray at 657,721, equals ours at 11 stacks (656k, −0.3%; ours fire at 8–9 stacks: 605k and 622k), so the per-hit formula, multipliers (checked against the game's talent tables) and buffs agree, and the game reached 11 stacks at least once. (2) Sandrone's per-hit buffs are at their caps in the sim: CRIT Rate over 100% (Disenchantment 16% and Cryo resonance 15%), Odette's Marvelous Splendor 4 stacks (+60%), Heart of the Furnace +50%, Yae's C1 +50%, Disenchantment +40%, Stellar Jubilee 14% from both Odette and Sandrone. (3) Field time does not explain it: the post's reaction tracker shows Yae triggering Stellar-Conduct 38 times (ours 42) and Odette once (ours 1), and Sandrone's field time (76 s, about 12.6 s per rotation) is already at the top of the KQM guide's 10–13 s. Rotations move all members together: KQM's order (Yae 3[E], Odette 2[E] (Q), Escoffier (Q) E, Sandrone) 181.0k (Sandrone −17.7%, Yae −19.0%, Odette −5.1%, Escoffier +25.2%); 4[C E] 176.1k (−14.0%, −25.1%, −19.8%, +17.2%); a second burst slot 186.8k (−16.4%, −13.1%, −2.9%, +29.1%). (4) Escoffier's +27% is per hit, not field time: each skill runs 21 Frosty Parfait ticks over about 22 s, so any rotation up to 22 s covers the whole 120 s (101 ticks). Her base ATK (1,087) fits only Calamity Queller among polearms (and Yae's 1,014 only Skyward Atlas among catalysts); the posted panels show 0% Cryo and Electro DMG Bonus although both weapons give 12%, so the panels apparently leave out weapon passives. The remaining gap (Sandrone and Yae about 16% low with every per-hit factor checked) is not attributable with the post's data; it needs a per-source breakdown or a video of the run. Weapon ratio against Prydwen's table (as quoted by genshin-build.com): A Teaspoon of Transcendence R1 over Blackcliff Slasher R5 is 1.18 for the team and 1.39 for Sandrone (Prydwen 1.18 and 1.39), over Tidal Shadow R5 1.16 and 1.33 (Prydwen 1.12 and 1.26). Constellations: Sandrone's damage is 1.43× at C2 and 2.61× at C6 of C0, against about +40% and "100%+" in [HoYoLAB 45845639](https://www.hoyolab.com/article/45845639) | No plunge; the hovermech's movement and sprint are not modeled (the hover lasts until the next action, at most 6 s); Prism Shots always find a target (the game needs one within 20 m); the C6 cluster beam stops when Decoding mode ends, which the data leaves open; Stellar Jubilee's base DMG bonus adds to Odette's and the Cryo Traveler's in a shared team, the engine's existing pattern. Forged by the Golden Melody, Blade of Atonement, A Thousand Blazing Suns and Verdict rank far below Prydwen's table on the calibration team; the first two need Sandrone to trigger Stellar-Conduct herself, which a Cryo-heavy team seldom lets her do (the showcase's reaction tracker shows none for her), so the gap is the team, not checked further |
| Vodyanitsa | Ours (2026-10-27): multipliers from the pipeline; mechanics from the game's ability data (`ConfigAbility_Avatar_Vodyanitsa`, `ConfigTalent_Vodyanitsa`; Vodyanitsa is also her internal name) and, for the Wandering Vortex, the Stellar Swirl abilities (`ElementAbility_StarSwirl_Ice_Gadget` in `ConfigAbility_Element`, `TeamAbility_StarSwirl_Ice` in `ConfigAbility_Team`). The Wandering Vortex is not a gadget of its own: every Stellar Vortex checks every 0.2 s whether a party member carries her Mark, which Song of Ages Past's party buff (reapplied every 1 s for 2 s) sets with A1, and then stays a Wandering Vortex until it explodes; its damage and detonation are unchanged. Turning and detonating reduce Anemo RES by 35% for 6 s (the data; GenshinBuilds and KQM say 35%, AllThingsHow and genshin-build.com 30%); while it exists and the Song is up, plus 5 s, the party carries the AlterMark, which moves Lead Vocal/Chorus (A4) and C2 from Hydro/Cryo hits to Stellar Swirl DMG (+60% CRIT DMG instead of +50%). Radiance: Stellar Swirl lasts 12 s instead of 8 s while the Mark is on. Skill, horns and burst scale with Max HP; the burst is ×(1 + 86.4%) during the Song. Lead Vocal (25, active character) and Chorus (10, the others) add flat DMG only above 40,000 Max HP (140 per 1000 to Hydro/Cryo, 260 to Stellar Swirl, at most 3,500 and 6,500), one stack per damage instance | Normal and charged attacks: Sucrose's counted frames (4-hit catalyst, same body type). Skill (tap, 45 frames, swap at 30) and burst (110 frames, hit at 95) estimated: no community count exists, and the KQM quick guide calls her field time low. From the data: first hit and Song of Ages Past at the press (`onAbilityStart`), a horn every 3 s hitting 0.2 s after it appears, heals from 1.7 s every 1.5 s, Lead Vocal granted at 66% of the tap, the burst hit at 95% of the burst state, gauges and ICD. Calibration (no source gives a full build, so ratios, on the library's community Skirk teams): team DPS of Skirk / Furina / Escoffier with C0 Vodyanitsa against C1 Shenhe, [AllThingsHow](https://allthings.how/genshin-impact-how-to-build-vodyanitsa-for-skirk-freeze-teams/) 152.2k against 135.5k (×1.123), with the builds it states (C1 Azurelight Skirk, C6 Favonius / Golden Troupe Furina at 9/10/9, Scroll Escoffier, C1 Calamity Queller / Noblesse Shenhe, Vodyanitsa with R5 Hymn of the Maelstrom and Tenacity at 69,659 HP; both bursts when ready): with Hymn (fork `76e8116`) 256.1k against 182.6k, ×1.403 (+25%). Before Hymn was on the fork, Everlasting Moonglow stood in for it (65.7k HP): 214.9k, ×1.177 (+4.8%). The gap is Hymn's ATK: at R5 with three stacks in its Freeze/Stellar Swirl state it gives Skirk +84% ATK (16% per stack, the data's cap per stack, ×3 ×1.75), and her damage rises 26%. Read with the 16% as a cap on all stacks together (a test build, not the fork), the team gives 229.2k, ×1.255 (+12%), still above the source; the source's ratio is below even Moonglow's, so it seems to count little of Hymn's ATK. One damage instance, Skirk's burst hit with C0 TTDS Vodyanitsa against C0 TTDS Mona, [HoYoLAB 46785492](https://www.hoyolab.com/article/46785492) about 160k against 130k (×1.23; final hit 190k against 160k): 152.0k against 124.1k, ×1.208 (−1.9%); final hit 174.5k against 155.8k, ×1.12 (−5.7%), because Skirk spends the 25 Lead Vocal stacks on normal attacks before her burst, so the last hits get none. The source's numbers fit only without Mona's Omen on the burst (with it Mona's hits are 171.5k), so Mona's burst is left out of that comparison. Weapons: Prydwen ranks Hymn R1 103.7% and TTDS R5 101% (Skirk team), a ratio of 1.027; in the AllThingsHow team (Moonglow's substats, 300 iterations) Hymn R1 scores 235.3k against TTDS R5's 219.3k, 1.073 (+4.5%; 1.012 with the cap on all stacks). Against Prototype Amber R5 in the same team: Hymn R1 1.098, Hymn R5 1.193, TTDS R5 1.024, Sacrificial Jade R5 1.005, Everlasting Moonglow 1.002, Jadefall's Splendor 1.003, A Thousand Floating Dreams 0.966; the creator-server analysis ([HoYoLAB 46781743](https://www.hoyolab.com/article/46781743)) calls the last three on par with Prototype Amber | No plunge; the out-of-combat swim is not modeled; the Wandering Vortex stays where the vortex was made (in game it wanders toward opponents). Lead Vocal on Stellar Swirl DMG goes to each contributor's share, the way the engine splits Stellar Swirl DMG, so one Stellar Swirl hit can spend up to four stacks; the game's count per reaction instance is not known. Direct Stellar Swirl hits (Odette's, Sandrone's, the Cryo Traveler's, Vesna's) spend one stack each since fork `87338fd`, as her data counts every `StarSwirlWindDamage` and `StarSwirlIceDamage` instance; before, only reaction contributions did. C6's "+60% Hydro/Cryo DMG" (`damagePercentageRatio`) is modeled as a DMG Bonus |
| Vesna | Ours (2026-10-27, fork `3e75b40`): multipliers from the pipeline; mechanics from the game's ability and gadget data (`ConfigAbility_Avatar_Vesna`, `Avatar_Girl_Sword_Vesna_AniEvent_Attack`, `ConfigGadget_SkillObj_Vesna`, `ConfigTalent_Vesna`; Vesna is also her internal name, avatar 10000143). The skill hits at the press and starts Armed for Action (15 s; C1 adds 5 s, `Buff_Duration_Extra` in the data, not in the text) with 60 Sword Essence (max 90). In the mode normal and charged attacks deal Anemo DMG no infusion overrides, and each attack (not each hit) summons a wind pinion that deals Anemo DMG and regenerates 5 Essence, so 12 attacks fill the 60 that, with the skill's 60 and the burst's 30, pay for the five Windborne Swords of a C0 window, as guides count it. Windborne Sword costs 30 Essence and rises from Lv. 1 to Lv. 3 within a window; Lv. 2 adds a Spirit Blade 0.5 s after its gadget appears, Lv. 3 four blades and a final hit, and the window ends after the third Lv. 3 (C1: fourth, the first free). The burst is one Spirit Blade 0.55 s after the press (the text says "multiple"; the data has one gadget with one hit) and +30 Essence. Windborne hits, pinions and non-Radiance blades share the game's `Vesna_MagicSword` ICD group (1 application per 2 hits, 2 s reset). In Radiance: Stellar Swirl, read when the blade's gadget appears or when the Lv. 3 animation starts, Spirit Blades deal Anemo DMG that is Stellar Swirl DMG (`StarSwirlWindDamage`, the engine's direct Stellar Swirl tag with Anemo element) with their own multipliers; the pinion never does (its Radiance bullet runs the same ability). A1: Disciplinary Action, one instance per Windborne Sword or burst, 20 s each, the count capped at 6 and lowered as each instance expires, as the game's global value does; blades ×(1 + 10% per stack); cleared by the skill and by leaving the field. A4: in Radiance, 6% ATK per Cryo or Anemo member and 25 EM per other member (C4 ×3). Stellar Jubilee: Odette's pattern, 0.7% base Stellar Swirl DMG per 100 ATK, max 14%. C1 +20% Stellar Swirl DMG during the window, C2 6 stacks at the skill and +40% ATK at 6, C6 Windborne Sword: Transpose (150% + a 200% blade, with a pinion in the window) for 5 s after each Lv. 3 and 20% elevation on her Stellar Swirl DMG. 1 Anemo particle per 2.5 s | Every animation length and cancel is estimated: no community frame count exists (searched KQM, the TCL, Prydwen, Icy Veins, HoYoLAB). Normal attacks follow a generic 6-hit sword (N1–N6 hitmarks 12/12/10+21/15/14/21, chains 21–52), charged attack hit at 24 (52 frames), skill 38 frames (swap at 28), Lv. 1 42, Lv. 2 44, Lv. 3 80 (hits at 26, 33, 41, 49 and 66), Transpose 64, burst 100. From the data, not estimated: the skill's hit at the press, Lv. 1 at 20% and Lv. 2 at 35% of their animations, the Lv. 3 hits at 36/47/58/69% of the animation and the final hit at its recovery, the Lv. 2 blade 0.5 s and the burst blade 0.55 s after their gadgets appear, gauges, hit lag and hitboxes. With these frames her window is about 14 s, inside the 15 s mode, for the KQM/Prydwen combo E N1C E N1C E N3C Q E N1C E N1C E. Calibration (no source gives every member's stats, so most comparisons are ratios; builds: KQM-standard substats from gcsim's optimizer, talents 9/9/9, level 100 enemy at 10% RES, every member bursting every rotation): weapons against Silver Light R5 in C0 Vesna / C0 Vodyanitsa / C6 Cryo Traveler / C0 Odette (Icy Veins' team, figures from search extracts): Beyond the Chrysalis 1.319 (Icy Veins 1.272, +3.7%), Whitelake Frostfeather 1.140 (1.201, −5.1%), Absolution 1.130 (1.121, +0.8%), Azurelight 1.022 (1.080, −5.4%), Emberwell R5 1.022 (1.082, −5.5%); against Emberwell R5 (Prydwen): Beyond the Chrysalis 1.291 (1.247), Whitelake 1.116 (1.168), Absolution 1.106 (1.055), Azurelight 1.000 (1.049), Silver Light R5 0.979 (0.978), Primordial Jade Cutter 0.911 (1.061: its 44% CRIT Rate puts this build over 100%). One damage instance, [HoYoLAB 46901646](https://www.hoyolab.com/article/46901646) (Silver Light, sheet 69% CRIT Rate and 171% CRIT DMG, C6 Cryo Traveler, C6 Faruzan; refinement and ATK not stated): burst 240k against 233k (R1) to 300k (R5) in the sim; one Lv. 3 Windborne Sword 340k against 290k–390k, depending on the Disciplinary Action stacks at the cast; Vesna's damage per rotation 1.3M against 1.25M (−4%). Team damage is lower than every source: Icy Veins 215,899 DPS against 135.1k (−37%; rotation and builds not visible); [HoYoLAB 46785086](https://www.hoyolab.com/article/46785086) (Vesna / Odette / Faruzan / Vodyanitsa, its combo E E Q E 2N1C E 2N1C E) 4.23M per 18.48 s rotation against 2.79M per 20.05 s (139.0k DPS, −34%); [HoYoLAB 46785152](https://www.hoyolab.com/article/46785152) (C0 Silver Light Vesna at 47.1%/118.1%, Qiqi, C6 Cryo Traveler, C6 Faruzan) about 2M per rotation against 1.54M. Traced: the per-hit formula matches the engine's direct-reaction formula term by term (checked on a Lv. 3 final hit) and Vesna's own share matches HoYoLAB 46901646; the sim's rotations are longer (19–24 s against about 18 s, from the teammates' frames and Vesna's estimated ones); Vodyanitsa's Lead Vocal and Chorus missed Vesna's blades until fork `87338fd` (with it: Icy Veins' team 128.5k → 135.1k, +5.1%; HoYoLAB 46785086's 115.8k → 123.4k, +6.6%); the first HoYoLAB 46785086 runs had Odette press her skill once and never recast it, so she dealt no Stellar damage (only Coda at Dawn's Tolling and the dance moves after it are Stellar) and her Heart of the Furnace 4pc, the party's +50% Stellar DMG, never triggered: with burst, skill, skill as in the Icy Veins team, 123.4k → 139.0k (2.35M → 2.79M per rotation, −45% → −34%; the set's code and data agree, checked against `Relic_7.0_StarSupport_4`); and none of these sources states its teammates' stats or constellations, so the rest of the gap cannot be attributed yet. Vodyanitsa's weapon is not the gap: HoYoLAB 46785086 gives her Thrilling Tales of Dragon Slayers (as the sim does), HoYoLAB 46785152 has no Vodyanitsa, and Icy Veins' weapon for her was not visible (the sim uses TTDS R5). With Hymn R1 in her place (fork `76e8116`, same substats) the Icy Veins team gives 145.9k (−32% instead of −37%) and HoYoLAB 46785086's 150.4k (3.02M per rotation, −29% instead of −34%); the TTDS runs are unchanged on `76e8116` (135.4k, 138.7k) | No plunge (her plunging attack is not modeled, and its pinion with it); Coursing is not modeled. Lv. 1 and the wind pinion hit the primary target only. Disciplinary Action timers are not extended by hit lag. The wind pinion travel time (12 frames) is estimated. Stellar Jubilee's base DMG bonus adds to Odette's and the Cryo Traveler's, as the engine already does for those two; whether the game takes the highest instead is not known. Until fork `87338fd`, Vodyanitsa's Lead Vocal and Chorus skipped direct Stellar Swirl hits, which her data (`ModifyDamageCountMixin` on `StarSwirlWindDamage` and `StarSwirlIceDamage`) counts; that commit, made separately from Vesna's, adds them (Vodyanitsa's row) |

Shared code: Nefer's PR changes how Verdant Dew is collected so that her P2 and C4 can speed it up. The partial dew counter becomes a float and each frame adds 1 plus the sum of registered rate mods; only Nefer registers one. Without Nefer the sum is 0 and the counter reaches 146 on the same frame as before; Lauma and Columbina teams were part of the regression run. Linnea and Kachina change nothing outside their packages; all their event subscriptions are made when they are in the party. Lohen adds one ICD group (`ICDGroupLohenNormalAttack`, from the game's `Lohen_ElementalArt`: 6 Cryo applications per 5 s), which only his attacks use, and the `Lohen` character key. The pipeline inserts them alphabetically, so 22 groups (`ICDGroupLyneyExtra` to `ICDGroupYelanBurst`) and 59 character keys (`LumineAnemo` to `Zibai`) moved up by one; the regression run found no change (below). His event subscriptions are made only when he is in the party. Sandrone adds one ICD tag and group (`ICDTagSandroneMachineGun`, `ICDGroupSandroneMachineGun`, from the game's `Avatar_Marionette_MachineGun`: one Cryo application per 1.4 s) and the `Sandrone` character key, which only she uses. The pipeline inserts them alphabetically, so 26 tags (`ICDTagShatter` to `ICDTagYelanBurst`, among them the Superconduct and Swirl reaction tags), 15 groups (`ICDGroupSigewinne` to `ICDGroupYelanBurst`) and 31 character keys (`SangonomiyaKokomi` onward) moved up by one; the regression run checks that this changes nothing. Her Polestar Field and Stellar Swirl enable flags and event subscriptions are set only when she is in the party, as Odette's. Vodyanitsa's Wandering Vortex adds one event, `OnStellarSwirlVortex`, which the Stellar Swirl code (`pkg/reactable/stellarswirl.go`) emits after each Stellar Swirl with the vortex it created or stacked, and only while `Flags.Custom[reactable.WanderingVortexKey]` is set; only her constructor sets it. Turning vortices into Wandering Vortices, their Anemo RES shred and the longer Radiance: Stellar Swirl live in her package, so without her no Stellar Swirl code path changes. The new event moves the five event values after it (`OnInitialize` to `OnSimEndedSuccessfully`) up by one; they are only slice indexes. Two Stellar Swirl configs without her (Odette, Cryo Traveler, Faruzan, Sucrose; Sandrone, Cryo Traveler, Faruzan, Escoffier; 30 iterations) give the same results as `13a481c` to the last bit. The library regression run is in the regression bullets above: no team changed. Vesna changes no Stellar Swirl code: the game's Stellar Swirl abilities (`ElementAbility_StarSwirl_Ice`, `ElementAbility_StarSwirl_Ice_Gadget`, `TeamAbility_StarSwirl_Ice`) contain nothing of hers, and her own data uses the same building blocks as Odette's (enabling Stellar Swirl, the base DMG bonus, direct Stellar Swirl hits; hers are Anemo `StarSwirlWindDamage` where Odette's and Sandrone's are Cryo `StarSwirlIceDamage`). So she sets the existing `StellarSwirlEnableKey` and uses the existing direct Stellar Swirl attack tag, and her Radiance checks, reaction bonuses and elevation are subscriptions and mods made in her package when she is in the party. The pipeline adds one ICD tag and group (`ICDTagVesnaMagicSword`, `ICDGroupVesnaMagicSword`, from the game's `Vesna_MagicSword`: one application per 2 hits, 2 s reset), which only she uses, and the `Vesna` character key; 6 tags (`ICDTagWandererA4` to `ICDTagYelanBurst`), 8 groups (`ICDGroupWandererA4` to `ICDGroupYelanBurst`) and 18 character keys (`Vodyanitsa` to `Zibai`) moved up by one; the regression run below checks that this changes nothing. Three Stellar Swirl configs without her (Odette / Cryo Traveler / Faruzan / Sucrose, Cryo Traveler / Qiqi / Faruzan / Sucrose, Vodyanitsa / Cryo Traveler / Faruzan / Sucrose; 30 seeds each) give the same per-character results as `6fcfc4c` to the last bit, and the same debug log.

Upstream fixes on the fork: two artifact bugs found while calibrating Vesna, each its own fork commit after the characters' groups, sent upstream as PRs (cenoda/gcsim branches `fix-cinder-city-stellar-swirl` and `fix-dids-actor-check`, on upstream `2350be9`, each with a unit test that fails without the fix). Drop each commit when upstream merges it.
- Scroll of the Hero of Cinder City (`d57703a`): the 4pc never triggered on Stellar Swirl, because its reaction-to-element table had no Stellar Swirl entry and the subscription was skipped. The game's `Relic_NyxTime_4` (`ConfigAbility_Relic`) lists `StarSwirlIce` with `SwirlIce` for the Anemo and Cryo buffs, so it now counts as Anemo and Cryo.
- Disenchantment in Deep Shadow (`efcbe0e`): the 4pc's 16% CRIT Rate against a Superconducted or Stellar-Conducted opponent went to every party member's special reaction contributions (Stellar Swirl, Lunar). In `Relic_6.6_Superconductor` (`ConfigAbility_Relic2`) it is part of the wearer's modifier, so now only the wearer's damage gets it.

**Weapons on the fork**: Hymn of the Maelstrom (`76e8116`, ours, 2026-10-28), Vodyanitsa's signature catalyst (weapon 14524, internal name Bludnye). Base stats, names and generated files from the pipeline; the passive from the game's `Weapon_Catalyst_Bludnye` (`ConfigAbility_Weapon_Catalyst2`), its params from EquipAffix 114524 mapped by `ConfigAffix_Catalyst`. Healing Bonus +4% (R5 8%, wearer); each heal the wearer performs (also off-field; healing received does not count) adds a stack, 10 s each, max 3, a heal at the cap refreshing all. Per stack the wearer gets +4% Max HP (R5 8%) and the active member +0.4% ATK per 1,000 of the wearer's Max HP over 40,000 (R5 0.8%), capped at 8% per stack (R5 16%, reached at 60,000), so three stacks give up to 24% (R5 48%). A Freeze or Stellar Swirl on an opponent by any party member multiplies both by 1.75 for 5 s. In a debug sample with Vodyanitsa at R1 and R5, every part fired and expired as the data says; guides agree on the numbers except a pre-release page (8% Healing Bonus, a 5% ATK cap, 8 s). No engine change; the new weapon key moved the 148 weapon keys after it (Ibis Piercer to Xiphos' Moonlight) up by one, which the regression run covers.
- Known problems: whether the game counts direct Stellar Swirl hits (Odette's, Sandrone's, Vesna's) as Stellar Swirl reactions for the 1.75 state is not checked (gcsim emits no reaction for them, so they do not trigger it); guides word the 8% ATK limit ambiguously, and the data's cap per stack is what makes the AllThingsHow comparison 25% high (Vodyanitsa's row).
- Fork-only weapons are listed in `library.ForkOnlyWeapons` (gcsim keys; now `hymnofthemaelstrom`), next to `library.ForkOnly`, and count like a fork-only character. A library team whose `build.txt` equips one (any gcsim name or shortcut) needs `tier: experimental` and `gcsim_fork`; `cmd/import` and `cmd/harvest` set both; `cmd/validate` skips it on an upstream build. A result where the player's showcase build uses one gets the experimental badge and a note that a weapon is not in gcsim yet (`warn.experimental_weapon`, every site language), whatever the team's tier; without it the result is unchanged. A showcase character with Hymn failed before ("weapon 14524 is not supported by gcsim yet"). The server embeds only `team.yaml` and `rotation.gcsl`, so the rule on library builds is checked where they are read from disk: `cmd/validate`, the importers and a library test that loads every team with its `build.txt` (no library team uses Hymn now). For the same reason a library team's own page shows the badge from its tier, but not the weapon note.

**Rotations**
- Rotations come from a community gcsim config when one exists (§3.1, §5.2). When the community database has no entry for a team, its rotation is written from a published community guide under the rules below, and the team is experimental. The reason the entry is missing does not matter: the database has none for characters upstream lacks, and it can also lag behind upstream (in October 2026 it had none for Iansan, Illuga, Odette or Alyosha, all in upstream gcsim). A written rotation lasts only until the database has an entry for the team: `cmd/harvest` then replaces it.
- Rules in this document describe how a team gets a source, not which characters may have one. A new character, or a team the tooling does not fit yet, is a reason to extend these rules and the tooling, not to leave the character unscored.
- The removed agent-written batch scored a third of community numbers (§3.1), so a written rotation needs:
  - a published community rotation guide (KQM or similar) for the team or its archetype, listed in `sources`;
  - a cross-check against a public calculation or showcase DPS when one exists, recorded in `team.yaml`;
  - the looped full check (§5.3), as for any team;
  - review by a person before `status: supported`.
- Tooling for written rotations: `library.Team.Written` (an experimental team without a `reference`). `cmd/validate` skips the reference comparison for them ("reference check skipped") and runs the full looped check as for any team; it warns when `cross_check` is missing, and when a recorded cross-check DPS is more than 15% from the one pass. `team.yaml` records the cross-check as `cross_check: {dps, source, note}`, the note saying what was searched when nothing was found; a library test requires `sources` and a `cross_check` note on every written team. CI validates them like any changed team. `cmd/harvest` keeps a written team (it is in no database group while the database has no entry for its members); once the database has an entry for its members, it tries the entries without `-force` and removes the written team only when one passes, in whatever member order the entry names; if none passes, the written team stays and the team is listed in `harvest-failed.txt`. Before, `-force` removed the directory before trying the entries, which would have lost a written team whose entries all failed. Tested with a fake database cache: a failing entry left the team in place, with and without `-force`; a passing one replaced it.

**First experimental teams** (2026-10-27, fork `91c5de4`, all `status: draft`)

Chosen from community guides and showcases, one or two per character, preferring teams with a published rotation. A first version used the KQM quick guides' second-choice teams with 4-star weapons and KQM-standard artifacts, the level of the database's lowest-investment entries, and scored 95k (Nefer / Lauma / Aino / Nahida), 127k and 146k. Community numbers for these characters are about 200k for Nefer teams and 240k for Zibai's, so the teams and builds were checked against them:

- The damage formula holds: a Phantasm Performance shade hit, traced in gcsim's debug log, gets Pale Hymn, the Moonsign base bonus, NotSU, SMS and the Dendro shred, and follows gcsim's direct Lunar-Bloom formula.
- The gap was the builds and the team. The strongest Nefer team by community account (below) with its own rotation scored 127.9k one pass with KQM-standard artifacts and Blackmarrow Lantern, and 179.3k with Dawning Frost R5 and artifacts well above KQM standard, against 183k in game. Zibai's team went from 137.6k to 235.9k with the community's conditions (C2 Columbina with her signature, Zibai's signature, strong artifacts).

So the validation builds (`build.txt`) are now at the level of each team's cross-check source, and `cross_check` compares like with like as far as the sources tell. A visitor's score uses their own build in any case.

| Team | Why | Rotation source | One pass | Looped (score) | Waits |
|---|---|---|---|---|---|
| Nefer / Lauma / Columbina / Sucrose | Called the strongest Nefer team by a C0R0 showcase (183k in game) that gives the rotation | [HoYoLAB 44026962](https://www.hoyolab.com/article/44026962); same combo as the [Nefer quick guide](https://keqingmains.com/q/nefer-quickguide/) | 179.3k | 168.7k | none above 2 s/min; Nefer's skill cooldown filled with attacks |
| Nefer / Linnea / Lauma / Columbina | The [Linnea quick guide](https://keqingmains.com/q/linnea-quickguide/)'s team with Nefer: covers both characters | Linnea quick guide | 150.1k | 166.5k | Linnea 10.6 s/min for her 18 s skill |
| Zibai / Linnea / Columbina / Gorou | The Linnea guide's first team; Gorou rather than Illuga (185% ER at C0; the guides call C6 Gorou close) | [Zibai quick guide](https://keqingmains.com/q/zibai-quickguide/) and the Linnea guide | 235.9k | 252.4k | none |

1000 iterations, common conditions; no energy waits, no failed actions, no stalls. Rotation notes:
- Nefer's charged attacks are held (`charge[hold=150]`, Slither) until Verdant Dew turns them into Phantasm Performance, as the guides say she may wait for Dew.
- Lauma holds her skill only with Dew, checked after her burst: checked before it, Columbina's charged attack can spend the Dew in between, and gcsim rejects the hold.
- Linnea's burst is never used: the guides use it only for healing, and on the fork a burst while Lumi is out stops Lumi's attacks (the known problem above). Her Ultimate Power Form (the guide's `5[E]`) is one press and three recasts in gcsim; a fifth press would wait for the cooldown.
- Zibai's Spirit Steed's Stride needs 70 Radiance: the rotation attacks until she has it (in the checks she always had it after N4).

**Cross-check** (searched 2026-10-27; recorded in each `team.yaml`):
- Nefer / Lauma / Columbina / Sucrose: 183k in game with C0R0 builds the author calls top 5% (200k with top 1% stats); 179.3k one pass here, −2.0%. The post gives no artifacts, so ours are an approximation.
- Zibai: an arca.live player reports about 200k at C0, 240k at C1 and 310k at C2, with C2 Columbina and her signature, in Zibai / Illuga / Columbina + Zhongli or Gorou (seen through search results; the page returns 403). Ours, with Linnea for Illuga, is 235.9k one pass at C0: for scale only. With the same builds, Zibai / Illuga / Linnea / Columbina scores 214.9k one pass with C0 Illuga, 226.6k at C2 and 257.7k at C6; the Gorou team is 91.5% of the C6 one (Prydwen: 85% of the best team), and KQM says C6 Gorou performs like a pre-C4 Illuga. Checked on the way: Illuga's buff is an additive bonus applied after the EM and reaction multiplier, as the wiki's direct Lunar formula has it, so it is small next to a Spirit Steed's Stride (about 3.9k of 344k); Illuga's team gives up the Ascendant Gleam bonus that a non-Nod-Krai member such as Gorou grants; and his burst counted the three Moondrifts present at cast (36 stacks).
- Nefer / Linnea / Lauma / Columbina: no public number. The KQM quick guides and TCL publish none, the community database has no entry with these characters, and Prydwen returns 403.

**Found while writing them** (reported, not fixed):
- Upstream gcsim: Illuga's burst is meant to gain 5 stacks per Geo construct created during it, but its `OnConstructSpawned` handler type-asserts `construct.Construct` while the event carries `*construct.Construct`, so it never fires. Constructs present at cast are counted, as in these teams' rotations, so their scores are unaffected.
- Fork: Nefer's Phantasm Performance checks the charged attack only for its first hit; the later hits are queued at fixed delays and land even when a dash or swap cancels it. The Nefer guide says a dash cancel loses Nefer's second hit, so dash-cancelled rotations keep it on the fork: about 1.5% of a Nefer team's damage.

**Second batch** (2026-10-27, all `status: draft`, 100 iterations, common conditions)

Written from the example teams of the KQM quick guides, so that every character in §6.3 with an implementation and every upstream character the database lacks has teams. Builds: the Nefer, Lauma, Linnea, Zibai and Columbina builds of the first teams verbatim, so teams with the same carry compare; new supports at C0 (C6 for common 4-stars) with 4-star weapons. Iansan's teams copy each carry's and support's build from a harvested library team with that character, named in its `build.txt`. No team has a public DPS; each `cross_check` note says what was searched and gives the closest library team for scale. A reviewer who plays the characters judged the Nefer and Linnea numbers normal for C0 (higher constellations would score more; a visitor's score uses their own build).

| Group | Teams | Looped (score) | Notes |
|---|---|---|---|
| Zibai / Linnea | with Illuga + Columbina, Xilonen + Columbina, Columbina + Ineffa, Illuga + Aino, Illuga + Yelan, Illuga + Xingqiu | 130.7k–224.8k | Zibai quick guide rotations; Illuga C0 at 204% ER, bursts every rotation. Without Columbina the teams lose about 40% |
| Linnea | Navia / Columbina / Furina, Itto / Gorou / Columbina, Noelle / Aino + Gorou or Geo Traveler | 44.8k–87.5k | Linnea quick guide; Linnea's burst replaced by her tapped skill in the Navia team (the fork problem above). Linnea out-damaging Itto is expected |
| Nefer | Aino / Lauma + Kuki, Zhongli, Nahida or Sucrose; Kokomi / Lauma / Nahida; Furina / Lauma + Ineffa or Kuki; Aino / Nahida / Kuki; Aino / Collei / Kuki | 85.7k–139.9k | The guide gives no per-team rotation: the first Nefer team's pattern with the guide's `E 3[CD] E CD CQ CD`. Without Columbina's Moonridge Dew, one pass from a standing start gets about one Phantasm Performance and the loop carries Dew over, so the looped score keeps 105–184% of one pass |
| Iansan (upstream, no database entry) | Mavuika / Citlali + Bennett or Xilonen; Kinich / Mavuika + Bennett or Emilie; Varesa / Furina / Xianyun; Varesa / Chevreuse / Mavuika; Clorinde / Chevreuse / Mavuika; Chasca / Bennett / Furina; Gaming / Bennett / Citlali; Flins / Aino / Sucrose | 55.2k–82.3k | Iansan quick guide order (her `ECQ` is `skill, charge, burst`), carry combos in the form of the closest harvested config. Movement reported where the guide says the carry keeps Iansan's buff: see below |

- Iansan's burst restores Nightsoul points from the distance the active character moves (0.67 points per metre, counted every second; her points drain 6 per second), which gcsim learns only from `movement(n)` calls or `movement=` action params in the rotation. No community config or guide gives distances, so the rotations report none, and her ATK buff falls to its weaker rate (below 42 points) a few seconds after her skill. Its effect is small: with every action reporting a constant movement speed (an `execute_action` wrapper calling `movement(speed × elapsed time)`), four of the teams (20 iterations) score 1–2% more at 3 m/s, 4–5% at 6 m/s and 5–6% at 9 m/s, where the gain saturates (Mavuika / Citlali / Iansan / Bennett 63.1k → 68.4k, Varesa / Chevreuse / Iansan / Mavuika 82.3k → 86.3k). So these teams understate Iansan by at most about 6%. Against the library's teams with the same carry they rank near the middle (Varesa / Chevreuse / Iansan / Mavuika 30th of 57 Varesa teams, the Clorinde team 33rd of 144); comparisons with a single harvested team mostly reflect that team's investment, since library builds range from C0 to full constellations. The rotations follow the guide's statements on movement: it says Varesa and Gaming keep Iansan's higher buff easily, Chasca while she keeps moving and Flins thanks to his combo, so those five teams wrap `execute_action` to report movement at 9 m/s since every previous action, the lowest speed tried (6, 9, 12) above which the score stops rising, so the buff stays up. The speed stands for the guide's statement, not a measurement. For Mavuika the guide has no statement; a reviewer who plays the characters says she keeps the buff nearly always at its maximum. The Kinich quick guide says Kinich easily keeps Iansan's points high, but that Bennett locks him into his circle. So the other five teams report 9 m/s only for the time after Mavuika's actions, and after Kinich's in his Emilie team (a wrapper that checks the previous action's character against `.keys.char`): Clorinde, whom the guide says does not keep the buff before Iansan's C4, and Kinich beside Bennett report none. Kinich / Mavuika / Iansan / Bennett is unchanged, since there Mavuika acts only before Iansan's burst. Looped scores: Mavuika / Iansan / Citlali / Bennett 64.0k, with Xilonen 69.0k, Kinich / Mavuika / Iansan / Emilie 76.5k, Kinich / Mavuika / Iansan / Bennett 63.0k, Clorinde 82.2k. Looped scores with movement: Varesa / Chevreuse / Iansan / Mavuika 86.3k, Varesa / Iansan / Furina / Xianyun 70.0k, Gaming 70.0k, Flins 65.9k, Chasca 57.8k. A measured speed per carry, or a community convention, would replace the assumption.

**Added on request**: Nefer / Lauma / Columbina / Nahida, with the rotation the Lauma and Columbina quick guides both give for it, `(Nefer E) > Columbina E Q > Nahida E Q > Lauma hE Q > Nefer E 3[CD] E 3[CD] (Q)`. Nefer, Lauma and Columbina as in the Sucrose team (Lauma with 4pc Gilded Dreams, since Nahida holds Deepwood), Nahida Widsith R5 and 4pc Deepwood. 140.5k one pass, 165.3k looped (1000 iterations), against 168.7k for the Sucrose team, which the community calls the strongest. No public number for this team.

**Added on request: Nilou** (all draft; a request: Nefer / Lauma / Columbina / Nilou is played, and Nilou can take Columbina's place in Nefer teams). Rotations from the KQM Nilou quick guide, which lists Nilou / Nefer / Lauma / Columbina, Nahida or Sucrose with a rotation each; Nilou plays `E N2 E`, C0 with Favonius Sword R5, since the guide says her stats matter little and she needs no ER. The Linnea team has no guide: the Linnea guide's rotation with Nilou's `E N2 E` in Columbina's place. Looped (1000 iterations), next to the Columbina version at the same Nefer build:

| Team | Looped | With Columbina instead |
|---|---|---|
| Nefer / Lauma / Columbina / Nilou | 156.3k | — |
| Nefer / Lauma / Nilou / Sucrose | 113.2k | 168.7k |
| Nefer / Lauma / Nilou / Nahida | 104.9k | 165.3k |
| Nefer / Linnea / Lauma / Nilou | 93.9k | 166.5k |

- Nefer's Seeds of Deceit take every Dendro Core, so Nilou makes no Bountiful Cores, as the guide says.
- Replacing Columbina costs 32–44%. Columbina deals about 12–15% of her teams' damage herself, and her Moonridge Dew adds to the Verdant Dew: in a 120 s fight Nefer gets 39 Phantasm Performances with Columbina and Nahida, 32 with Nilou and Nahida.
- Nilou's A1 EM buff (+100 when a member under Golden Chalice is hit by Dendro attacks) comes from Dendro Cores bursting on the party; with the cores turned into Seeds it is rare (about 1 shade hit in 8 has it). Whether the game behaves the same is not checked. Golden Chalice is off with Sucrose or Linnea in the team.
- No public number for any of them.
- Nilou's constellations (looped, 500 iterations, validation builds otherwise unchanged): in Nefer / Lauma / Columbina / Nilou, C0 156.3k, C2 177.2k, C4 175.8k, C6 177.6k, against 165.2k with Nahida in her place: Nilou overtakes Nahida from C2, whose Hydro and Dendro RES shred works only under Golden Chalice (all members Hydro or Dendro). With Nahida instead of Columbina: C0 104.9k, C4 120.8k, C6 121.8k. With Sucrose (no Golden Chalice): 113.2k, 113.6k, 114.8k. With Linnea: 93.9k, 82.0k, 83.3k, already 81k at C1, whose 18 s Tranquility Aura (12 s before) lets Linnea's Geo turn Nilou's Hydro into Lunar-Crystallize (20 reactions per fight against 1) and Nefer's damage falls from 72k to 58k. Columbina's versions stay well ahead in every team.

**Display**
- `team.yaml` gets `tier: experimental` (`library.TierExperimental`; no tier means verified). Loading checks it: a team needs the tier when a member is in `library.ForkOnly` or its build uses a weapon in `library.ForkOnlyWeapons`, and may have it otherwise only when its rotation is written (`library.Team.Written`, no `reference`), so a team with a fork-only character cannot pass as verified, nor a team from a community config carry the badge. Teams with a fork-only member or weapon also record `gcsim_fork`, the fork commit they were written for: `gcsim_commit` names the upstream base, which a fix on the fork leaves unchanged, so without it a score change from a fork fix could not be told apart. `cmd/import` and `cmd/harvest` set both fields for a team with a fork-only member or weapon. `/teams` and the team header of result pages show an "Experimental" badge, in every site language, and the result adds a note that the score may change, saying why: a member is not in gcsim yet (`warn.experimental`), a weapon is not (`warn.experimental_weapon`; also on a verified team when the player's build uses one), the rotation was written from a community guide (`warn.written`), or both. A versus page renders that header for each side, so the badge shows on the experimental side only. Link-preview cards do not show it yet.

## 6. Initial Library

### 6.1 Test teams (top priority)

| Team | Archetype | gcsim support | Status |
|---|---|---|---|
| Skirk / Yelan / Furina / Escoffier | Cryo hypercarry | ✅ All 4 supported | First implementation target |
| Vesna / Vodyanitsa / Cryo Traveler / Faruzan | Anemo (Cryo Swirl → Stellar Swirl) | ✅ All 4 (Vesna and Vodyanitsa on the fork) | `blocked`: no rotation yet |

**Vesna team status (checked 2026-10-26)**
- As of the latest gcsim `main` commit (`2350be9`, 2026-10-23), the Cryo Traveler (`travelercryo`) and Faruzan are implemented. Vesna and Vodyanitsa are not in the character list.
- Both characters were released in 7.1 (2026-09-23). gcsim already has Stellar Swirl: the Stellar Vortex and the Radiance: Stellar Swirl state (`pkg/reactable/stellarswirl.go`), enabled by the Cryo Traveler and Odette. What was missing is the two characters and the Wandering Vortex, which Vodyanitsa's A1 turns Stellar Vortices into (it also drives her C2). Vesna was expected to change how Stellar Swirl works; the game's data shows she does not (she enables it as Odette does, §5.4). Vodyanitsa and the Wandering Vortex are on the fork since `6fcfc4c`, Vesna since `3e75b40` (§5.4); the team is `tier: experimental` with `gcsim_fork` and stays `blocked` until it has a rotation: no community config or written rotation for it exists yet.
- No upstream PR exists for either. They are planned for the experimental tier (§5.4, §6.3).
- This team also serves as the first test case for "teams the engine does not support yet": matched, but reported as `blocked`.

### 6.3 Characters gcsim lacks (checked 2026-10-26)

Enka.Network's character list compared with the characters in gcsim `2350be9`. The Manekin and Manekina (Miliastra Wonderland avatars) are left out.

| Character | Upstream state | Plan (§5.4) |
|---|---|---|
| Kachina (4★ Geo, 5.0) | Open PR #2697 (2026-07-25) | On the fork; frames partly estimated, known problems in §5.4. First team: Kachina / Furina / Qiqi / Noelle ("Cappuccino"), after Nefer's and Linnea's, since she adds little damage and few players use her |
| Nefer (5★ Dendro, 6.1) | Open PR #3045 (2026-09-20), no review yet; an earlier draft, #2587, was closed | On the fork; frames counted, known problems in §5.4. Draft teams: Nefer / Lauma / Columbina / Sucrose (179.3k one pass, 183k in game in the HoYoLAB showcase it comes from), Nefer / Linnea / Lauma / Columbina (150.1k), Nefer / Lauma / Columbina / Nahida (140.5k one pass, 165.3k looped), four with Nilou (93.9k–156.3k looped, §5.4); nine more in the second batch |
| Linnea (5★ Geo) | Open PR #2596 (2026-04-19) | On the fork; frames partly counted, known problems in §5.4. Draft teams: Zibai / Linnea / Columbina / Gorou (235.9k one pass; the community reports about 200k for C0 Zibai with Illuga), Nefer / Linnea / Lauma / Columbina; rotations from the KQM Linnea and Zibai quick guides |
| Lohen (5★ Cryo, 6.6) | None | On the fork (our implementation, fork `af3eb82`); frames estimated, calibration and known problems in §5.4. No teams yet |
| Sandrone (5★ Cryo, 6.7) | None | On the fork (our implementation, fork `13a481c`); frames estimated, calibration and known problems in §5.4. No teams yet |
| Vodyanitsa (5★ Hydro, 7.1) | None | On the fork (our implementation, fork `6fcfc4c`), with the Wandering Vortex; frames partly estimated, calibration and known problems in §5.4. No playable team yet: her library team waits for Vesna |
| Vesna (5★ Anemo, 7.1) | None | On the fork (our implementation, fork `3e75b40`); no Stellar Swirl engine change needed; frames estimated, calibration and known problems in §5.4. Her only team (with Vodyanitsa) has no rotation yet |

Order: the fork and the three characters with upstream PRs first (most coverage for the least work), then Vodyanitsa, Lohen and Sandrone, then Vesna.

The fork and the three PR characters are in place (2026-10-27): the site builds on the fork, and Enka profiles and names map all three. Nefer and Linnea have draft experimental teams (§5.4): the first three, then 19 more in the second batch, six of them with Zibai. Kachina has none yet, so her teams are still scored as unsupported. Lohen, our first own implementation, is on the fork (`af3eb82`) with Enka mapping and names; he has no teams yet. Sandrone, our second own implementation after Lohen, is on the fork (`13a481c`) with Enka mapping and names; she has no teams yet. Vodyanitsa, our third, is on the fork (`6fcfc4c`) with the Wandering Vortex, Enka mapping and names. Vesna, our fourth, is on the fork (`3e75b40`) with Enka mapping and names; the only team with her and Vodyanitsa waits for a rotation. Iansan, an upstream character the database has no entry for, has ten written teams.

### 6.2 Library from the community database

The first batch held the 64 teams with the most entries in the community database. In October 2026 `cmd/harvest -n 10000` took every usable team (§5.3):

- The database had 6,020 entries; 4,428 teams (sets of four characters) had a usable entry. 123 entries were skipped: 72 without four characters, 47 with an unconditional `while 1`, 4 without an `active` setup.
- Every team passed the reference check, most with their first entry: the median gap to the database's DPS was 0.3%, the largest 7%. The harvest took 2 minutes on 4 CPUs.
- The full validation (`cmd/validate -iter 20 -j 4`, 28 minutes) then failed 156 teams: 124 whose looped rotation stalled (117 Skirk teams whose opening burst cannot be charged again in the loop), 27 whose looped rotation did not advance (a loop counter set once in the setup) and 5 whose looped config gcsim rejects. 293 more scored over 20% below their one pass, mostly one-cycle entries that open with every burst charged.
- With the skipped-burst filler (§9) and the selection by kept score (§5.3), those 449 teams were harvested again in 12 minutes. 417 came back: every Skirk team, and for example Hu Tao / Yelan / Zhongli / Furina now keeps 94% of its one pass (it lost 42%) and Tartaglia / Bennett / Xiangling / Kazuha 100% (it lost 82%). 43 of them still keep less than 80%: their team has no entry that loops better.
- 32 teams have no usable entry and are listed in `teams/harvest-failed.txt`: 24 never advance when looped, 5 stall, 3 are rejected by gcsim.
- `cmd/validate -promote -iter 20 -j 4` (50 minutes) then set 4,340 teams to `supported` and recorded each result (§5.1). 55 stay `draft`: 5 are more than 5% from their source, 41 keep less than 80% of their one pass when looped and 9 more than 130% (Ningguang / Xingqiu / Bennett / Chiori keeps 5,137%: its one pass under the common conditions barely deals damage). Ningguang / Yelan / Xingqiu / Chiori was removed: its check does not finish within two minutes. A second run changed no team's status.
- The library holds 4,396 teams.

## 7. Accuracy and Limitations

- With the same rotation, gcsim generally matches the actual game within a few percent.
- Error mostly comes from:
  - A rotation that is not optimal for the team
  - Implementation errors for new characters
- **New character lag**: until gcsim supports a character, teams using it are `blocked`.
  - Vesna and Vodyanitsa are still unsupported more than a month after release, and seven playable characters are missing in all (§6.3).
  - The experimental tier (§5.4) is meant to close this gap, at the cost of less certain numbers for those teams.
- **Fixed rotation limits**: the optimal rotation can vary with the build (e.g. high ER enables a more aggressive cycle). Initially we ignore this and use one standard rotation.

## 8. Risks

| Risk | Impact | Mitigation |
|---|---|---|
| gcsim lags on new characters | Latest meta teams unsupported | Experimental tier on a fork (§5.4), contribute upstream |
| Fork upkeep and estimated frames | Rebase work on every gcsim update; experimental scores off | One commit per character, upstream PRs, badge on experimental scores, verified scores checked unchanged |
| Enka API changes / blocking | No input | Caching, GOOD JSON fallback |
| Uneven rotation quality | Fairness compromised | Automated validation + cross-checks with public calcs + PR review |
| Game patches change numbers | Old scores not comparable | Record gcsim and rotation versions with results; consider seasons |
| Simulation cost | Server load | Build-hash cache, tune iteration count |

## 9. Open Questions

- [x] Where to run: server-side Go on one small VM (§4.6)
- [x] Fight length: community rotation looped, 150–450 s; one pass shown alongside (§3.1)
- [ ] Enemy energy particle drops
  - The prototype uses `energy every interval=480,720 amount=1`, as many community gcsim configs do.
  - This directly affects ER requirements, so the same value must apply across the whole library. Confirm before the first library batch.
- [x] What a character does when the looped rotation makes it stand still (internal/steady, config.Fill)
  - gcsim makes a character wait until its burst has the energy or its skill is off cooldown. A rotation that spends more energy than it gains lives off its starting energy, then stalls; the stall also stops particle generation (Flins / Ineffa / Aino / Sucrose: 111k one pass, 31k looped).
  - A player fills that time, or passes the burst: Hu Tao teams often never use hers, and a rotation that waits for it loses half its damage (Hu Tao / Zhongli / Yelan / Furina, a player build with no Energy Recharge on Hu Tao: 19.8k waiting, 42.8k passing; one pass 36.1k). The score therefore runs the looped rotation as written first. If a character waits 1 s/min or more for energy or for its skill, it runs the rotation again with that wait handled and keeps the best of all runs. A character short of energy is handled one of three ways, and the best run is then tried a fourth way:
    - **own skill**: before each burst, the character uses its skill when ready, otherwise attacks, until the burst is ready (at most 20 actions);
    - **party skills**: as above, but when its skill is on cooldown, another member whose skill is ready swaps in and uses it, and the character's next action swaps back to catch the particles;
    - **pass**: the character bursts only when the burst is ready, and the rotation moves on otherwise (a one-frame wait keeps time passing). Some rotations wait for a burst they can never charge once looped: Skirk's opening burst is written for her full starting Serpent's Subtlety (gcsim reports her wait as `insufficient_energy`), so a loop that repeats the opener stood still for good, and the whole team with it. The other fillers fail there: their skill puts Skirk into Seven-Phase Flash, and the rotation's own next skill is then rejected by gcsim (whose `.skirk.skill.ready` still reads true in that state).
  - **brief**, tried only on the best run once the search is done: for the characters whose energy that run handles (all of them, then each alone), skills as in **party skills**, at most 2 attacks, then the burst is passed if still not ready. A player swaps out rather than stand on a support until its burst charges: a low-ER Ineffa filling with party skills attacked up to 10 times a cycle on field (Ayato / Columbina / Lauma / Ineffa, all ER substats removed: 28 attacks in 2 minutes), where she is played with two skills to a burst; brief, she attacks 6 times and the team scores 97.7k instead of 94.4k. Tried as a fourth way in the search, it crowded the search's own runs out of the 16-run budget and lowered 58 library teams (Hu Tao / Yelan / Chevreuse / Furina −19%); capping the attacks of every filler instead lowered 357 (Mavuika / Escoffier / Kaeya / Bennett −40%: a main DPS should attack until its burst is ready). Tried after the search it can only raise a score: on the library builds (2026-10-28) 117 teams rose (mean +1.5%, Bennett / Furina / Jean / Mona +45%) and none fell. A near tie still goes to the higher score, so a support can still stand on field when that scores higher by noise (Varesa / Furina / Ineffa / Xilonen, ER removed: Ineffa's 22 attacks win by 0.05%).
  - In every variant a character waiting for its skill's cooldown attacks meanwhile (at most 30 attacks).
  - A filler uses each member's skill as the rotation first writes it: its parameters (hold or tap) and the plunges that follow it (Kazuha, Xianyun). Burst-reset skills such as Kazuha C1 come from gcsim.
  - The right way differs per character and depends on the others: once Hu Tao passes, the rotation is shorter and Furina runs short of energy, a wait the run as written never showed. So every run that leaves characters standing still is followed by runs that handle them: all of them each way, and, when there are several, each one alone each way. Follow-ups of the better runs go first.
  - Filling every burst hurt teams that wait on purpose (Skirk / Furina / Mona / Escoffier went from −2% to −79%). That is why only characters that stood still are filled, and why the unfilled run stays a candidate.
  - With fillers, 46 of 64 teams stay within 5% of one pass, 16 lose 5–20%, and 2 lose more:
    - Kazuha C6 plunge −27%, down from −61% without fillers;
    - Arlecchino / Furina / Kazuha / Bennett −21%. The site shows a known-issue note on Arlecchino teams: their damage is calculated too low, by about 20%, and the cause is under investigation.
  - Scoring a team that stood still costs up to 16 simulations instead of one. The rotation as written runs with the full iterations; the filled variants are ranked on 20, and only the best is run again with the full iterations (kept if it still beats the rotation as written). An Arlecchino / Citlali / Skirk / Bennett build went from 900 iterations to 440 with the same result.
- [ ] Whether to support additional rulesets such as an AoE league or a short-fight league (out of scope for now)
- [x] How to share results: links that carry the builds, copied as short links (`/s/<id>`), with link-preview cards (§4.8)

## 10. Next Steps

1. ~~gcsim execution prototype~~ Done: `cmd/gscouter` scores a build file against a library team (see README).
2. ~~Enka UID → gcsim config converter~~ Done (`internal/enka`). Artifact substats come from Enka's rounded display values; crit rate differs from Enka's final stats by under 0.1%p.
3. ~~Website with versus view~~ Done (`cmd/server`, `internal/web`), deployed at https://gscouter.altsky.info (§4.6).
4. ~~Library format, import and validation~~ Done: `cmd/import` adds a team from a gcsim share link; `cmd/validate` checks that each team reproduces its source's DPS (§5.3). CI runs it. The `add-team` skill describes the workflow.
5. ~~First library batch~~ Done: `cmd/harvest` took the 64 most simulated teams from the community gcsim database (§5.3), then every usable team, 4,397 after validation (§6.2). An earlier batch of 64 agent-written rotations was removed: written without sources, they scored a third of the community numbers.
6. ~~Display names~~ Done: in-game character and weapon names in every game language, instead of gcsim keys.
7. Experimental tier (§5.4): ~~the gcsim fork, with Kachina, Linnea and Nefer from upstream PRs~~ (done, verified scores unchanged) and `tier` in `team.yaml` with the badge (done); ~~rotations for Nefer and Linnea~~ (three draft teams, awaiting review, §5.4); next Kachina's ("Cappuccino"), then the remaining characters in §6.3 in the order given there.
