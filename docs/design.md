# Gscouter Design Document

> Status: Draft (prototype in progress)
> Last updated: 2026-10-28 · Game version baseline: 7.1

## 1. Overview

Gscouter is a Genshin Impact team DPS calculator that settles **"whose team is better built?"**
A user submits the builds of their party members, and Gscouter computes the team's **theoretical DPS** under fixed conditions with a fixed rotation, so it can be compared against other teams.

### Goals

- Treat team choice (meta), constellations, weapon refinements, levels, talents, and artifacts all as part of "skill", and express them as a single score (DPS).
- Compute every team under identical conditions so that even different teams can be compared fairly.
  - Example: "my Nahida team vs. your Neuvillette team"

### Non-goals

These set the scope of the MVP. Changing rotations, build recommendations and real content are planned beyond it, in `docs/roadmap.md`.

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
| 100 iterations (site default until 2026-10-28) | ±0.5% | about 1 s |
| **50 iterations (site default)** | ±0.7% | about 0.5 s |

The site scores with 50 iterations since 2026-10-28, down from 100, to halve the time to a score; time is roughly linear in the iterations. Measured natively on 6 library teams, 31 seeds per count: the standard deviation of a score across seeds was 0.04–0.64% of its mean at 50 (worst seen 1.2%, apart from filler-dependent outliers) against 0.03–0.51% at 100, and with the site's seed every team chose the same filler at both counts. Shared links are calculated again with the new count, so their numbers move slightly; with the site's seed, 50 against 100: Hu Tao / Yelan / Xingqiu / Zhongli −0.53%, Hu Tao / Furina (no ER) −0.04%, an Arlecchino team −0.04%, a Skirk freeze team −0.04%, Raiden National +0.13%, a hyperbloom team +0.29%. Fillers are ranked on 10 iterations (20 until the same day, §9) and the best run again on 50; warmup, duration and seeds are unchanged.

History: a first library batch of 64 rotations written by agents without sources scored about a third of community numbers and was removed. Rotations now come from community configs, or, for a team the database has no entry for, from a published community guide under the rules in §5.4. A rotation is never written from our own idea of a team.

### 3.2 Common simulation conditions

| Item | Value |
|---|---|
| Enemy level | 100 |
| Enemy resistance | 10% for all elements + physical |
| Number of targets | 1 |
| Enemy HP | Infinite (time-based end); the one-pass check gives the enemy `hp=999999999` so the run ends with the rotation |
| Fight duration | 450 s with the rotation looped, first 150 s excluded; plus one pass from full energy (§3.1) |
| Iterations | Site: 50 (±0.7%, §3.1); fillers are ranked on 10 (§9). CLI: 1000 (±0.2%) |
| Energy | 1 clear particle every 8–12 s (`energy every interval=480,720 amount=1`). Provisional, see §9 |
| Swap delay | 12 frames |

As a gcsim config, roughly:

```
options iteration=50 duration=450 swap_delay=12;
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
  - Fetches the characters in the profile showcase (up to 12) by UID: level, constellations, talents, weapon, refinement, artifact main stats and substats.
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
  - The result page shows both: the base level, and for a talent a bonus raises, the level in the fight as gcsim works it out (`score.Talents`), e.g. `13 (10+3)`. That covers the constellation bonus and party passives such as Tartaglia's (+1 normal attack) and Skirk's (+1 skill in a Hydro/Cryo party).
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
- **Per-character contribution**: DPS and share, with each character's build: level, constellations, weapon, talents, artifact sets and total stats as on the character screen (docs/artifacts.md §2)
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
- **Cost**: the build is about 62 MB, 10 MB gzipped, served precompressed with a versioned URL so browsers keep it. Each worker's Go heap stays around 10 MB. On the server, the same pages take under a second each, one at a time.
- **Warming up on the front page**: once the visitor clicks or types in the UID input (not on page load or focus: the input has autofocus), or right away on a loaded showcase, where they pick a team, the front page downloads the build, compiles it and starts the Go runtime once in a worker, then drops it (not on Save-Data connections). A front page left alone downloads nothing, and a returning visitor downloads nothing either: the build's versioned URL is served `immutable`, and headless Chromium read it from its disk cache on the result page, after a browser restart and on the front page again. The download lands in the HTTP cache, and Chrome keeps the code it compiled for the build in its code cache, which the result page's compile reads: its workers then start in 0.25 s instead of 2.6 s, and its first simulation runs faster. A link opened directly gains nothing, and private windows keep no code cache.
- **Speed** (2026-10-28): the page records each stage as a performance measure named `gscouter:*` (nothing is logged). Headless Chromium on 4 CPUs, a fresh profile each time, median of 3; "after" comes through the front page, "before" opens the result directly (the front page did nothing for it). Scores and the whole result were identical.

  | Stage | Hu Tao / Yelan / Xingqiu / Zhongli, before → after | Arlecchino / Citlali / Skirk / Bennett (19 simulations), before → after |
  |---|---|---|
  | Download and compile, local / 50 Mbit/s | 0.5 / 3.4 s → 0.6 / 0.6 s | 0.5 / 3.4 s → 0.6 / 0.6 s |
  | Starting the workers | 2.6 s → 0.24 s | 2.7 s → 0.28 s |
  | Score simulation (100 iterations) | 9.0 s → 8.0 s | 6.5 s → 5.4 s |
  | Filler search (16 runs of 20 iterations) | — | 12.4 s → 12.5 s |
  | Best filler again, one pass (100 iterations each) | 1.3 s → 1.4 s (one pass) | 4.1 s → 4.2 s |
  | Two timelines, then rendering | 0.2 s + 0.4 s | 0.14 s + 0.3 s |
  | Total, local | 14.1 s → 10.8 s | 26.7 s → 23.6 s |
  | Total, 50 Mbit/s | 16.9 s → 10.1 s | 30.1 s → 23.3 s |

  - The calculation is bound by the CPUs. The first simulation runs about twice as slow as the same work later (Arlecchino: 6.5 s as written, 3.4 s for the best filler at the same 100 iterations), because the browser starts on quickly compiled code and optimizes what runs; the warm-up's code cache covers part of that.
  - The filler search runs one simulation after another (each follows from the last, §9), but each is already split among all helpers (7 chunks on 4), which are busy about 90% of the time. Splitting into single iterations, as tight as packing gets, took the same 12.2 s: the 4 CPUs are full, so running the search's simulations side by side would not make it faster, and it was not done.
  - The table was measured at 100 iterations per score. With 50, the site default since 2026-10-28 (§3.1), a result page opened directly (no warm-up, a fresh profile, two runs each) took 11.4–11.8 s instead of 14.7 s for Hu Tao and 24.1–24.4 s instead of 27.7–29.6 s for Arlecchino, with the same score in the browser and on the server. That is 3–5 s, not half: download, compile, worker start and the filler search (20 iterations per run) do not depend on the iterations. The two changes together, through the front page, are not measured yet.
  - Filler search at 10 iterations per run and up to 24 runs (2026-10-28, §9), the low-ER Arlecchino / Citlali / Skirk / Bennett showcase link opened directly, two runs each: before (16 runs, ranked on 20) the page made 18 filler runs (search and brief) in 8.4–8.8 s and took 14.5–16.0 s in all; after, 26 filler runs, since the cap had stopped its search, in 7.0–7.5 s, and 12.9–13.6 s in all. The score was 70,370 both ways, in the browser and on the server (`server=1`). At the old cap of 16 runs, 10 iterations alone brought its search from 8.0–9.1 s to 4.9–5.1 s (5 iterations: 2.8–2.9 s) and the page from 13.4–15.1 s to 10.2–11.1 s, same score. A filler run costs about 0.08 s plus 0.02 s per iteration.
  - Not done either: showing the score before the timelines (at most the 0.4–0.7 s after the last simulation), the garbage collector at twice the default heap growth (3% faster for 11 MB more per worker), and Go's newer WebAssembly instructions (`GOWASM=satconv,signext`: no change).
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

Validation speed. A full run (`cmd/validate -iter 20 -j 4`, 4 CPUs) simulates per team one pass under the source's conditions, one under the common conditions, one probe fight and the looped score, which `steady.Score` reruns with fillers up to 24 times (16 until 2026-10-28, §9). Profiled on 100 random teams (2026-10-27, cap 16): the looped score took 87% of the time (its first run 22%, the filler runs 64%; 11 of the 100 teams ran all 16), the two passes 5% each, the probe 3%; half the teams need one looped run. What makes a run faster without changing any result to the last bit:
- each check's process runs the garbage collector a tenth as often (GOGC 1000, a 1 GiB soft limit; at most about 260 MB per process): 40% less time. A fight allocates fast and keeps little alive;
- gcsim collects only the stats the score needs (`score.Run`): damage, failed actions, and shields, which is not only an observer: on every hit on the player it queues a task, which reorders the frame's other tasks, so without it Dehya / Furina / Albedo / Xilonen scored 1.6 DPS lower under its source's conditions (they have hurt lines). About 5% less;
- the probe is the looped score's first iteration (same config, seed and warmup; gcsim does not read the iteration count): 3% less;
- results are cached (`score.Cache`, `validate -cache`, by default under the user's cache directory; `-cache ""` turns it off): keyed by config and options, in a directory per gcsim module with its replace directive, Go version and format name (`cacheFormat`, to change with `summarize` or the collectors). A rerun on the same gcsim simulates only the teams that changed. The library takes about 210 MB (28,000 entries) per gcsim version.

The whole library took 41 minutes (148 CPU minutes) instead of 67 (233), every report identical to the last bit (`cmd/compare`, 4,433 teams); a rerun with the cache took 2 minutes. `-shard k/n` splits a run across machines. A cached run is not identical in one way: a team near the 2-minute limit of a check, which is measured in wall time and depends on the machine's load, finishes once enough of its simulations are cached. Ningguang / Xingqiu / Bennett / Chiori timed out on both cold runs and passed with the cache. With the search's cap at 24 runs (2026-10-28, §9), a cold run took 37 minutes for the 4,398 teams recorded at 20 iterations and 1 minute for the 53 recorded at 100; the same team timed out again and passed with the cache.

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

**Fork**: the `gscouter` branch of [cenoda/gcsim](https://github.com/cenoda/gcsim/tree/gscouter), upstream `main` plus one group of commits per added character (from an open upstream PR, otherwise ours) and a few weapons, used through a `replace` directive in `go.mod`. Both tiers run on it, so it must leave every verified score unchanged to the last bit, which a regression run over the whole library checks on every fork update. How it is kept, the regression runs and what is on it: `docs/fork.md`.

**Rotations**
- Rotations come from a community gcsim config when one exists (§3.1, §5.2). When the community database has no entry for a team, its rotation is written from a published community guide under the rules below, and the team is experimental. The reason the entry is missing does not matter: the database has none for characters upstream lacks, and it can also lag behind upstream (in October 2026 it had none for Iansan, Illuga, Odette or Alyosha, all in upstream gcsim). A written rotation lasts only until the database has an entry for the team: `cmd/harvest` then replaces it.
- Rules in this document describe how a team gets a source, not which characters may have one. A new character, or a team the tooling does not fit yet, is a reason to extend these rules and the tooling, not to leave the character unscored.
- The removed agent-written batch scored a third of community numbers (§3.1), so a written rotation needs:
  - a published community rotation guide (KQM or similar) for the team or its archetype, listed in `sources`;
  - a cross-check against a public calculation or showcase DPS when one exists, recorded in `team.yaml`;
  - the looped full check (§5.3), as for any team;
  - review by a person before `status: supported`.
- Tooling for written rotations: `library.Team.Written` (an experimental team without a `reference`). `cmd/validate` skips the reference comparison for them ("reference check skipped") and runs the full looped check as for any team; it warns when `cross_check` is missing, and when a recorded cross-check DPS is more than 15% from the one pass. `team.yaml` records the cross-check as `cross_check: {dps, source, note}`, the note saying what was searched when nothing was found; a library test requires `sources` and a `cross_check` note on every written team. CI validates them like any changed team. `cmd/harvest` keeps a written team (it is in no database group while the database has no entry for its members); once the database has an entry for its members, it tries the entries without `-force` and removes the written team only when one passes, in whatever member order the entry names; if none passes, the written team stays and the team is listed in `harvest-failed.txt`. Before, `-force` removed the directory before trying the entries, which would have lost a written team whose entries all failed. Tested with a fake database cache: a failing entry left the team in place, with and without `-force`; a passing one replaced it.

**Written teams**: each written team's guide, builds, cross-check and known problems are recorded with its character: `docs/characters/first-batches.md` (Nefer, Linnea, Zibai, Iansan), and `lohen.md`, `sandrone.md`, `vesna.md` and `vodyanitsa.md` in `docs/characters/`.

**Display**
- `team.yaml` gets `tier: experimental` (`library.TierExperimental`; no tier means verified). Loading checks it: a team needs the tier when a member is in `library.ForkOnly` or its build uses a weapon in `library.ForkOnlyWeapons`, and may have it otherwise only when its rotation is written (`library.Team.Written`, no `reference`), so a team with a fork-only character cannot pass as verified, nor a team from a community config carry the badge. Teams with a fork-only member or weapon also record `gcsim_fork`, the fork commit they were written for: `gcsim_commit` names the upstream base, which a fix on the fork leaves unchanged, so without it a score change from a fork fix could not be told apart. `cmd/import` and `cmd/harvest` set both fields for a team with a fork-only member or weapon. `/teams` and the team header of result pages show an "Experimental" badge, in every site language, and the result adds a note that the score may change, saying why: a member is not in gcsim yet (`warn.experimental`), a weapon is not (`warn.experimental_weapon`; also on a verified team when the player's build uses one), the rotation was written from a community guide (`warn.written`), or both. A versus page renders that header for each side, so the badge shows on the experimental side only. Link-preview cards do not show it yet.

## 6. Initial Library

### 6.1 Test teams (top priority)

| Team | Archetype | gcsim support | Status |
|---|---|---|---|
| Skirk / Yelan / Furina / Escoffier | Cryo hypercarry | ✅ All 4 supported | First implementation target |
| Vesna / Vodyanitsa / Cryo Traveler / Faruzan | Anemo (Cryo Swirl → Stellar Swirl) | ✅ All 4 (Vesna and Vodyanitsa on the fork) | `draft`, written from the KQM Vodyanitsa quick guide (`docs/characters/vesna.md` §2) |

### 6.2 Library from the community database

The first batch held the 64 teams with the most entries in the community database. In October 2026 `cmd/harvest -n 10000` took every usable team (§5.3):

- The database had 6,020 entries; 4,428 teams (sets of four characters) had a usable entry. 123 entries were skipped: 72 without four characters, 47 with an unconditional `while 1`, 4 without an `active` setup.
- Every team passed the reference check, most with their first entry: the median gap to the database's DPS was 0.3%, the largest 7%. The harvest took 2 minutes on 4 CPUs.
- The full validation (`cmd/validate -iter 20 -j 4`, 28 minutes) then failed 156 teams: 124 whose looped rotation stalled (117 Skirk teams whose opening burst cannot be charged again in the loop), 27 whose looped rotation did not advance (a loop counter set once in the setup) and 5 whose looped config gcsim rejects. 293 more scored over 20% below their one pass, mostly one-cycle entries that open with every burst charged.
- With the skipped-burst filler (§9) and the selection by kept score (§5.3), those 449 teams were harvested again in 12 minutes. 417 came back: every Skirk team, and for example Hu Tao / Yelan / Zhongli / Furina now keeps 94% of its one pass (it lost 42%) and Tartaglia / Bennett / Xiangling / Kazuha 100% (it lost 82%). 43 of them still keep less than 80%: their team has no entry that loops better.
- 32 teams have no usable entry and are listed in `teams/harvest-failed.txt`: 24 never advance when looped, 5 stall, 3 are rejected by gcsim.
- `cmd/validate -promote -iter 20 -j 4` (50 minutes) then set 4,340 teams to `supported` and recorded each result (§5.1). 55 stay `draft`: 5 are more than 5% from their source, 41 keep less than 80% of their one pass when looped and 9 more than 130% (Ningguang / Xingqiu / Bennett / Chiori keeps 5,137%: its one pass under the common conditions barely deals damage). Ningguang / Yelan / Xingqiu / Chiori was removed: its check does not finish within two minutes. A second run changed no team's status.
- The library holds 4,396 teams.

### 6.3 Characters gcsim lacks

Characters in the game that upstream gcsim lacks, with their state upstream and on the fork, and the order they are added in: `docs/fork.md` §6. Each one's implementation and teams: `docs/characters/<name>.md`.

## 7. Accuracy and Limitations

- With the same rotation, gcsim generally matches the actual game within a few percent.
- Error mostly comes from:
  - A rotation that is not optimal for the team
  - Implementation errors for new characters
- **New character lag**: until gcsim supports a character, teams using it are `blocked`.
  - Vesna and Vodyanitsa are still not in upstream gcsim more than a month after release; Gscouter scores them on its fork (§5.4, §6.3).
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
  - **brief**, tried only on the best run once the search is done: for the characters whose energy that run handles (all of them, then each alone), skills as in **party skills**, at most 2 attacks, then the burst is passed if still not ready. A player swaps out rather than stand on a support until its burst charges: a low-ER Ineffa filling with party skills attacked up to 10 times a cycle on field (Ayato / Columbina / Lauma / Ineffa, all ER substats removed: 28 attacks in 2 minutes), where she is played with two skills to a burst; brief, she attacks 6 times and the team scores 97.7k instead of 94.4k. Tried as a fourth way in the search, it crowded the search's own runs out of the 16-run budget of the time and lowered 58 library teams (Hu Tao / Yelan / Chevreuse / Furina −19%); capping the attacks of every filler instead lowered 357 (Mavuika / Escoffier / Kaeya / Bennett −40%: a main DPS should attack until its burst is ready). Tried after the search it can only raise a score: on the library builds (2026-10-28) 117 teams rose (mean +1.5%, Bennett / Furina / Jean / Mona +45%) and none fell. A near tie still goes to the higher score, so a support can still stand on field when that scores higher by noise (Varesa / Furina / Ineffa / Xilonen, ER removed: Ineffa's 22 attacks win by 0.05%).
  - In every variant a character waiting for its skill's cooldown attacks meanwhile (at most 30 attacks).
  - A filler uses each member's skill as the rotation first writes it: its parameters (hold or tap) and the plunges that follow it (Kazuha, Xianyun). Burst-reset skills such as Kazuha C1 come from gcsim.
  - The right way differs per character and depends on the others: once Hu Tao passes, the rotation is shorter and Furina runs short of energy, a wait the run as written never showed. So every run that leaves characters standing still is followed by runs that handle them: all of them each way, and, when there are several, each one alone each way. Follow-ups of the better runs go first.
  - Filling every burst hurt teams that wait on purpose (Skirk / Furina / Mona / Escoffier went from −2% to −79%). That is why only characters that stood still are filled, and why the unfilled run stays a candidate.
  - With fillers, 46 of 64 teams stay within 5% of one pass, 16 lose 5–20%, and 2 lose more:
    - Kazuha C6 plunge −27%, down from −61% without fillers;
    - Arlecchino / Furina / Kazuha / Bennett −21%. The site shows a known-issue note on Arlecchino teams: their damage is calculated too low, by about 20%, and the cause is under investigation.
  - Scoring a team that stood still costs up to 24 simulations in the search (`steady.maxRuns`; brief's runs come on top) instead of one. On the site the rotation as written runs with the full iterations; the filled variants are ranked on 10 (`web.exploreIterations`), and only the best is run again with the full iterations (kept if it still beats the rotation as written). `cmd/validate` runs every variant with its own iterations. An Arlecchino / Citlali / Skirk / Bennett build went from 900 iterations to 440 with the same result (100 iterations, ranked on 20, 16 runs).
  - The cap and the ranking iterations (2026-10-28; until then 16 runs, ranked on 20). Measured natively with the site's pipeline (seed 1, 50 iterations, warmup 150 s, duration 450 s) on 80 random supported library teams, each with its library build and with its ER substats removed; 116 of the 160 cases search. "Best known" is an exhaustive search (every run at 50 iterations, cap 64).
    - 16 runs, ranked on 20: 290 simulated iterations per searching case on average; 12 of 116 more than 1% below the best known (Yanfei / Durin / Fischl / Nicole without ER −12.4%, Durin / Xingqiu / Nicole / Bennett −11.2%, Rosaria / Yelan / Kazuha / Bennett −10.4%, Chasca / Mavuika / Durin / Nicole −9.6%, all without ER). The cap stopped the search before it tried the best filler (search order); the ranking iterations were not the cause.
    - 16 runs, ranked on 10: 188 iterations (−35%), still 12 missed; 15 of 116 chose another filler, −0.87% to +2.67%.
    - 24 runs, ranked on 10 (chosen): 211 iterations (−27%), 6 missed; against 16 / 20, 24 of 116 chose another filler, −0.87% to +14.2%. Over 5 seed bases, 43 of 580 runs more than 1% below the best known, against 64 at 16 / 20.
    - Ranked on 5: one team −8.5%, rejected.
    - The library rerun with the cap at 24 (`cmd/validate -promote`, each team at its recorded iterations; `cmd/validate` ranks at full iterations, so only the cap matters there): 162 of 4,451 recorded scores rose (61 by more than 1%, 11 by more than 5%; Bennett / Furina / Jean / Mona +71%, Wanderer / Faruzan / Tartaglia / Bennett +32%, Gaming / Layla / Rosaria / Bennett +29%, Mavuika / Iansan / Citlali / Bennett +17%), none fell. Two teams change status: Gaming / Layla / Rosaria / Bennett now keeps 81% of its one pass (62% before) and meets the criteria, and Kaveh / Baizhu / Xingqiu / Kazuha keeps 137% (124% before), over the 130% limit.
    - In the browser, see §4.7 Speed.
- [ ] Whether to support additional rulesets such as an AoE league or a short-fight league (out of scope for now)
- [ ] The background animation stands still on a desktop computer (reported 2026-10-28, cause not confirmed)
  - Not reproduced: in Chromium at 1440×900 and 390×844 every gear, ring and star in the background (`internal/web/templates/sky.html`) animates.
  - The one way found to stop it is the OS's reduce-motion setting: under `prefers-reduced-motion: reduce` the page stops all of the background's animation by design. Windows turns this on when Settings → Accessibility → Visual effects → Animation effects is off, and macOS when Reduce motion is on. Desktops often have it on and phones rarely do, which fits "still on the computer, moving on the phone".
  - Next: check that setting on the computer where the background stood still. If it is on, this is intended behavior. If it is off, record the browser and OS and look for another cause.
  - The behavior stays as is until then. One option considered: under reduce motion, keep only the slow in-place turning of the gears and rings and stop the chains (the one part repainted each frame), the drifting light and rings, and the twinkling stars. Its performance cost is negligible, but it would override the visitor's explicit choice, and the site has no switch to turn the background off.
- [x] How to share results: links that carry the builds, copied as short links (`/s/<id>`), with link-preview cards (§4.8)

## 10. Next Steps

1. ~~gcsim execution prototype~~ Done: `cmd/gscouter` scores a build file against a library team (see README).
2. ~~Enka UID → gcsim config converter~~ Done (`internal/enka`). Artifact substats come from Enka's rounded display values; crit rate differs from Enka's final stats by under 0.1%p.
3. ~~Website with versus view~~ Done (`cmd/server`, `internal/web`), deployed at https://gscouter.altsky.info (§4.6).
4. ~~Library format, import and validation~~ Done: `cmd/import` adds a team from a gcsim share link; `cmd/validate` checks that each team reproduces its source's DPS (§5.3). CI runs it. The `add-team` skill describes the workflow.
5. ~~First library batch~~ Done: `cmd/harvest` took the 64 most simulated teams from the community gcsim database (§5.3), then every usable team, 4,397 after validation (§6.2). An earlier batch of 64 agent-written rotations was removed: written without sources, they scored a third of the community numbers.
6. ~~Display names~~ Done: in-game character and weapon names in every game language, instead of gcsim keys.
7. Experimental tier (§5.4): ~~the gcsim fork, with Kachina, Linnea and Nefer from upstream PRs~~ (done, verified scores unchanged) and `tier` in `team.yaml` with the badge (done); ~~rotations for Nefer and Linnea~~ (three draft teams, awaiting review, `docs/characters/first-batches.md`); next Kachina's ("Cappuccino"), then the remaining characters in §6.3 in the order given there.
