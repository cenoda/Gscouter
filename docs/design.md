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

- **One Go binary** (`cmd/server`) serves the site, calls Enka, runs gcsim in-process, and caches Enka responses (for Enka's TTL) and scores (24 h) in memory. The team library is embedded in the binary. Result pages are calculated in visitors' browsers (§4.7); the server still calculates link-preview cards and pages for browsers that cannot.
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
  - Alternatives considered: short links stored on the server. In memory, every deploy (each push to `main` restarts the server) would break the links shared so far. On disk, a share is 1–2 KB, so space and CPU are no concern, but the server would gain a volume, backups, write limits against abuse and an expiry policy to look after, all for a shorter URL; chat apps show a preview card instead of the URL anyway.
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
  - After a rebase: `go mod edit -replace` to the new commit, `internal/enka/gen.sh` (Enka mapping and names), `library.ForkOnly` updated for characters added to the fork or merged upstream, `share.GcsimFork` set to the new fork commit (a test checks it against `go.mod`), the fork's own `go test ./...` (its frame and ability tests cover every character), and the regression run below.
- Each added character comes, in order of preference, from an open upstream PR (Nefer, Linnea, Kachina have one) or from our own implementation. Multipliers come from gcsim's data pipeline either way.
- Each added character records its source (PR or ours), where each action's frame data comes from (measured, taken from the PR, or estimated from a similar character) and its known gaps. The result page links them.
- Our own implementations are also sent upstream as PRs. Review there improves accuracy, and every merge ends the fork's cost for that character.
- When upstream merges a character, its fork commit is dropped and its teams move to the verified tier after revalidation. Scores may shift; results already record the gcsim version.
- **Verified scores must not move.** The server and the browser build (§4.7) run both tiers on the same gcsim, so the fork must leave every verified team's score unchanged (the regression run in §5.3, step 4 of the table). Engine changes a new character needs, such as Vodyanitsa's Wandering Vortex, are switched on only when that character is in the team, the way gcsim already enables Stellar Swirl through `Flags.Custom`.
  - Regression run: every library team through the full check (`validate.Check`, 20 iterations) on upstream and on the fork, comparing the reference pass, the common pass and the looped score, per character, at full precision. Seeds are fixed, so the difference must be exactly 0. Result for the first fork (2026-10-27, 40 minutes per run on 4 CPUs): 4,396 of 4,397 teams identical to the last bit, among them all 228 with Lauma, Columbina, Flins, Ineffa, Aino or Zibai. The other, Klee / Citlali / Xilonen / Xianyun, differed by 0.16 DPS in Xianyun's damage on the one pass under the common conditions; the upstream run was the odd one out. Rerun 83 times, upstream and the fork both always gave the fork's number. So upstream gcsim, very rarely, does not repeat an iteration exactly; the cause is not known yet.
  - Because of that, a team that differs is not yet a regression: rerun it on both builds, at least 20 times each. Only a difference that repeats counts and must be fixed; a value that appears once on one side and never again is recorded as that nondeterminism. The scratch tool for the run is not in the repository: it calls `validate.CheckIsolated` on every team and prints each `Report` as JSON, and the two outputs are compared field by field.

**Characters on the fork** (fork `91c5de4`, on upstream `2350be9`, 2026-10-27)

Nefer and Linnea have draft library teams (Rotations below); Kachina has none yet.

| Character | Source | Frame data | Known problems |
|---|---|---|---|
| Kachina | Upstream PR #2697 (danieldlc01), open since 2026-07 with requested changes | The PR cites a frame count by caramielle. Normal attacks look counted (the review asked for a recount); charged attack estimated (every cancel 49–50); skill: counted cancels; its animation ended at 733, the time of Turbo Twirly's last independent attack, and now ends at 57, the longest counted cancel (estimated). The community confirms the skill as about 12 s of damage over time, which Turbo Twirly models on its own (143, then every 118 frames, 10 of 60 points per hit); mount/dismount recast all 30 (placeholder); burst not counted | A1 gives +20% DMG to the active character on any Geo hit instead of Geo DMG to Kachina only; C1 triggers on crystallize, not shard pickup, and its shard absorption is missing; C4 is not polled; no Geo construct; Twirly loses no points while idle and loses 10 per enemy hit; swapping out while mounted can leave Twirly mounted forever; no plunge. Fixed on the fork: compile fix for current upstream, 500-frame charge cancels, Twirly's independent attacks firing every 236 frames instead of 118, the 733-frame skill animation |
| Linnea | Upstream PR #2596 (TheSast), open since 2026-04 with requested changes | Skill (first press) and burst cancels counted by caramielle; normal attack hitmarks counted, but their cancels are Gorou's; aimed shots are the generic bow template; skill recast a placeholder (10); plunge missing | The burst while Lumi is out stops Lumi's attacks and the A1 shred for the rest of its duration; C1 consumes stacks once per Lunar-Crystallize contributor instead of once; C2's extra attack does not emit Moondrift Harmony; C4 may give Linnea +50% DEF on field instead of +25%; the A4 EM buff is not refreshed on swap. Fixed on the fork: a duplicate registration that panicked at startup, frame invariants (skill animation 14 → 44, the longest cancel, as the upstream review asked; N3 animation 43 → 47, its hitmark; burst queue at the earliest cancel), C3 raising normal attacks instead of the skill, C2 skipping members after the first non-Geo/Hydro one |
| Nefer | Upstream PR #3045 (dersario), open since 2026-09, no review | The PR cites a frame count by caramielle; normal attack, charged attack, skill and burst numbers are irregular and match no other catalyst, so counted. Slither's stamina drain and movement, the Seed absorb radius (6) and the Slither entry (0 frames) are assumptions | Changes Verdant Dew generation for every team (below); no hitlag or poise; documented hitboxes differ from the code; Phantasm and charged hits are mostly 25U with no ICD; `.nefer.veilStacks` and `.nefer.phantasmCharges` are documented but do not resolve; C4 re-adds its resistance shred to every enemy every frame (slow); Slither moves the player. Fixed on the fork: N4's animation (51 → 63, its walk cancel), plunges with no frames (removed: Nefer cannot plunge), the burst's Veil bonus expiring before either hit |

Shared code: Nefer's PR changes how Verdant Dew is collected so that her P2 and C4 can speed it up. The partial dew counter becomes a float and each frame adds 1 plus the sum of registered rate mods; only Nefer registers one. Without Nefer the sum is 0 and the counter reaches 146 on the same frame as before; Lauma and Columbina teams were part of the regression run. Linnea and Kachina change nothing outside their packages; all their event subscriptions are made when they are in the party.

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

**Display**
- `team.yaml` gets `tier: experimental` (`library.TierExperimental`; no tier means verified). Loading checks it: a team needs the tier when a member is in `library.ForkOnly`, and may have it otherwise only when its rotation is written (`library.Team.Written`, no `reference`), so a team with a fork-only character cannot pass as verified, nor a team from a community config carry the badge. Teams with a fork-only member also record `gcsim_fork`, the fork commit they were written for: `gcsim_commit` names the upstream base, which a fix on the fork leaves unchanged, so without it a score change from a fork fix could not be told apart. `cmd/import` and `cmd/harvest` set both fields for a team with a fork-only member. `/teams` and the team header of result pages show an "Experimental" badge, in every site language, and the result adds a note that the score may change, saying why: a member is not in gcsim yet (`warn.experimental`), the rotation was written from a community guide (`warn.written`), or both. A versus page renders that header for each side, so the badge shows on the experimental side only. Link-preview cards do not show it yet.

## 6. Initial Library

### 6.1 Test teams (top priority)

| Team | Archetype | gcsim support | Status |
|---|---|---|---|
| Skirk / Yelan / Furina / Escoffier | Cryo hypercarry | ✅ All 4 supported | First implementation target |
| Vesna / Vodyanitsa / Cryo Traveler / Faruzan | Anemo (Cryo Swirl → Stellar Swirl) | ❌ Vesna and Vodyanitsa unsupported | `blocked` |

**Vesna team status (checked 2026-10-26)**
- As of the latest gcsim `main` commit (`2350be9`, 2026-10-23), the Cryo Traveler (`travelercryo`) and Faruzan are implemented. Vesna and Vodyanitsa are not in the character list.
- Both characters were released in 7.1 (2026-09-23). gcsim already has Stellar Swirl: the Stellar Vortex and the Radiance: Stellar Swirl state (`pkg/reactable/stellarswirl.go`), enabled by the Cryo Traveler and Odette. What is missing is the two characters and the Wandering Vortex, which Vodyanitsa's A1 turns Stellar Vortices into (it also drives her C2). Vesna changes how Stellar Swirl itself works, so she needs the most engine work.
- No upstream PR exists for either. They are planned for the experimental tier (§5.4, §6.3).
- This team also serves as the first test case for "teams the engine does not support yet": matched, but reported as `blocked`.

### 6.3 Characters gcsim lacks (checked 2026-10-26)

Enka.Network's character list compared with the characters in gcsim `2350be9`. The Manekin and Manekina (Miliastra Wonderland avatars) are left out.

| Character | Upstream state | Plan (§5.4) |
|---|---|---|
| Kachina (4★ Geo, 5.0) | Open PR #2697 (2026-07-25) | On the fork; frames partly estimated, known problems in §5.4. First team: Kachina / Furina / Qiqi / Noelle ("Cappuccino"), after Nefer's and Linnea's, since she adds little damage and few players use her |
| Nefer (5★ Dendro, 6.1) | Open PR #3045 (2026-09-20), no review yet; an earlier draft, #2587, was closed | On the fork; frames counted, known problems in §5.4. Draft teams: Nefer / Lauma / Columbina / Sucrose (179.3k one pass, 183k in game in the HoYoLAB showcase it comes from), Nefer / Linnea / Lauma / Columbina (150.1k), Nefer / Lauma / Columbina / Nahida (140.5k one pass, 165.3k looped), four with Nilou (93.9k–156.3k looped, §5.4); nine more in the second batch |
| Linnea (5★ Geo) | Open PR #2596 (2026-04-19) | On the fork; frames partly counted, known problems in §5.4. Draft teams: Zibai / Linnea / Columbina / Gorou (235.9k one pass; the community reports about 200k for C0 Zibai with Illuga), Nefer / Linnea / Lauma / Columbina; rotations from the KQM Linnea and Zibai quick guides |
| Lohen (5★ Cryo) | None | Implement |
| Sandrone (5★ Cryo) | None | Implement |
| Vodyanitsa (5★ Hydro, 7.1) | None | Implement, with the Wandering Vortex |
| Vesna (5★ Anemo, 7.1) | None | Implement last: changes Stellar Swirl |

Order: the fork and the three characters with upstream PRs first (most coverage for the least work), then Vodyanitsa, Lohen and Sandrone, then Vesna.

The fork and the three PR characters are in place (2026-10-27): the site builds on the fork, and Enka profiles and names map all three. Nefer and Linnea have draft experimental teams (§5.4): the first three, then 19 more in the second batch, six of them with Zibai. Kachina has none yet, so her teams are still scored as unsupported. Iansan, an upstream character the database has no entry for, has ten written teams.

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
  - A player fills that time, or passes the burst: Hu Tao teams often never use hers, and a rotation that waits for it loses half its damage (Hu Tao / Zhongli / Yelan / Furina, a player build with no Energy Recharge on Hu Tao: 19.8k waiting, 42.8k passing; one pass 36.1k). The score therefore runs the looped rotation as written first. If a character waits 1 s/min or more for energy or for its skill, it runs the rotation again with that wait handled and keeps the best of all runs. A character short of energy is handled one of three ways:
    - **own skill**: before each burst, the character uses its skill when ready, otherwise attacks, until the burst is ready (at most 20 actions);
    - **party skills**: as above, but when its skill is on cooldown, another member whose skill is ready swaps in and uses it, and the character's next action swaps back to catch the particles;
    - **pass**: the character bursts only when the burst is ready, and the rotation moves on otherwise (a one-frame wait keeps time passing). Some rotations wait for a burst they can never charge once looped: Skirk's opening burst is written for her full starting Serpent's Subtlety (gcsim reports her wait as `insufficient_energy`), so a loop that repeats the opener stood still for good, and the whole team with it. The other fillers fail there: their skill puts Skirk into Seven-Phase Flash, and the rotation's own next skill is then rejected by gcsim (whose `.skirk.skill.ready` still reads true in that state).
  - In every variant a character waiting for its skill's cooldown attacks meanwhile (at most 30 attacks).
  - A filler uses each member's skill as the rotation first writes it: its parameters (hold or tap) and the plunges that follow it (Kazuha, Xianyun). Burst-reset skills such as Kazuha C1 come from gcsim.
  - The right way differs per character and depends on the others: once Hu Tao passes, the rotation is shorter and Furina runs short of energy, a wait the run as written never showed. So every run that leaves characters standing still is followed by runs that handle them: all of them each way, and, when there are several, each one alone each way. Follow-ups of the better runs go first.
  - Filling every burst hurt teams that wait on purpose (Skirk / Furina / Mona / Escoffier went from −2% to −79%). That is why only characters that stood still are filled, and why the unfilled run stays a candidate.
  - With fillers, 46 of 64 teams stay within 5% of one pass, 16 lose 5–20%, and 2 lose more:
    - Kazuha C6 plunge −27%, down from −61% without fillers;
    - Arlecchino / Furina / Kazuha / Bennett −21%. The site shows a known-issue note on Arlecchino teams: their damage is calculated too low, by about 20%, and the cause is under investigation.
  - Scoring a team that stood still costs up to 16 simulations instead of one. The rotation as written runs with the full iterations; the filled variants are ranked on 20, and only the best is run again with the full iterations (kept if it still beats the rotation as written). An Arlecchino / Citlali / Skirk / Bennett build went from 900 iterations to 440 with the same result.
- [ ] Whether to support additional rulesets such as an AoE league or a short-fight league (out of scope for now)
- [ ] How to share results (versus card image, link, etc.)

## 10. Next Steps

1. ~~gcsim execution prototype~~ Done: `cmd/gscouter` scores a build file against a library team (see README).
2. ~~Enka UID → gcsim config converter~~ Done (`internal/enka`). Artifact substats come from Enka's rounded display values; crit rate differs from Enka's final stats by under 0.1%p.
3. ~~Website with versus view~~ Done (`cmd/server`, `internal/web`), deployed at https://gscouter.altsky.info (§4.6).
4. ~~Library format, import and validation~~ Done: `cmd/import` adds a team from a gcsim share link; `cmd/validate` checks that each team reproduces its source's DPS (§5.3). CI runs it. The `add-team` skill describes the workflow.
5. ~~First library batch~~ Done: `cmd/harvest` took the 64 most simulated teams from the community gcsim database (§5.3), then every usable team, 4,397 after validation (§6.2). An earlier batch of 64 agent-written rotations was removed: written without sources, they scored a third of the community numbers.
6. ~~Display names~~ Done: in-game character and weapon names in every game language, instead of gcsim keys.
7. Experimental tier (§5.4): ~~the gcsim fork, with Kachina, Linnea and Nefer from upstream PRs~~ (done, verified scores unchanged) and `tier` in `team.yaml` with the badge (done); ~~rotations for Nefer and Linnea~~ (three draft teams, awaiting review, §5.4); next Kachina's ("Cappuccino"), then the remaining characters in §6.3 in the order given there.
