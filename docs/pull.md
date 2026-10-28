# Pull a character

> Status: Design draft; presets done · Last updated: 2026-10-28

This covers the roadmap node **Pull a character** (ring 1, `docs/roadmap.md`): a team's score with a character the player does not own. It is the base of Pull planning, Primogem priority and the Character building guide.

The site's main use stays the player's own characters (design §4.1): a UID, its showcase, and the teams it can build. Everything here is a side path from that flow and never replaces it.

## 1. Two ways to fill a slot

A team still needs four characters that form a library team (design §4.3). A slot the player cannot fill from their own characters takes one of:

- **Assumed build** (§2): a build the visitor sets up, starting from a preset.
- **Borrowed character** (§3): a real build from another player's showcase or from a shared result.

The front page's team picker gets one secondary button, "Fill with a character I don't have", after the visitor's own characters. It is not offered before a UID is loaded.

## 2. Assumed builds

### 2.1 Preset

A slot starts from a preset, so the visitor gets a score without filling anything in:

- The character's build in that library team (`build.txt`). The harvest kept the lowest constellations and refinements that reproduce the source's DPS (design §5.3), so this is a reasonable build for that rotation, and the rotation was written for it.
- A team without a `build.txt` gets the character's most common build across the library (the build most teams give it; on a tie the lower constellation, then the lower refinement). A fixed preset (C0, a 4★ weapon, standard substats) was considered and dropped: library builds are real builds that rotations were validated with, and the basic settings (§2.2) lower the constellation or change the weapon in one step.
- A character no library build has gets no preset, and its slot cannot be filled until a team with a build adds it. Vesna was the last such character until her teams got `build.txt` files (2026-10-28).
- `internal/pull` reads the presets (`NewPresets`, `Presets.For`) with gcsim's own parser, so every form a `build.txt` writes reads as gcsim reads it. Weapon, set and character params (`+params=[stacks=5]`, on about 7% of the library's builds) are dropped, as a showcase build has none: a preset runs with gcsim's defaults, like a player's build. So a preset can score a little differently from the team's validated number. The server embeds the `build.txt` files for this (6 MB, the server binary 64 → 70 MB); the browser build does not include them. On 2026-10-28 the library's 4,456 teams give 17,823 presets for 125 characters, read in about 1.2 s.

### 2.2 Basic settings

The basic form changes the few things a player decides when pulling, and nothing else:

- Constellations
- Weapon and refinement
- Talent levels
- Artifact set and the three main stats (sands, goblet, circlet)
- Substat quality, one choice: average / good / excellent, as a number of effective rolls spread by the character's weights. The weights come from the artifact weights table (`docs/artifacts.md` §5), so this follows that work.

### 2.3 Advanced settings

A toggle, off by default, opens per-stat substat totals (CRIT Rate, CRIT DMG, ATK%, ER and the rest). It replaces the quality choice while on. Values are checked against what five fully levelled pieces can roll, as packed builds are today (design §4.8).

### 2.4 Links

An assumed build is a packed build (`enka.Pack`) with a flag saying it is assumed, so result links carry it and reopen with the same numbers like any other build. The flag needs a new pack format version, so this lands after the pieces' format version (`docs/artifacts.md` §1, branch `claude/artifact-pieces`) is merged.

## 3. Borrowed characters

A player can use a character from someone else's account: a friend's built Furina, to see what the team would score with it.

- **From a UID**: the visitor enters another UID; the site looks up its showcase as it does for the visitor's own (Enka lookup and cache, design §4.1) and lists its characters that fit the team.
- **From a result link**: a result page's builds (design §4.8) can be taken into the picker, so a build someone shared can be borrowed without their UID being in the showcase anymore.
- The borrowed build is used as it is and cannot be edited. Editing turns it into an assumed build (§2), started from the borrowed build instead of the preset.
- The result names where it came from: the lender's nickname and UID, as the versus view already shows another player's. A showcase is public on Enka.Network, so this shows nothing that is not already public.
- In links, a borrowed build is a packed build with the lender's nickname and UID, under the same new pack format version as §2.4.

## 4. Showing it

- Every assumed or borrowed character carries a badge on the result page, in the versus view and on the link-preview card: "Assumed build", or "Borrowed from <nickname>". A result with one is never read as the player's own account.
- The result names the team's best score with the player's own characters, when there is one, so the gain from the pull is visible: "With Yelan: 52,000 DPS, +37% over your best team today."

## 5. Kept out of rankings

Assumed builds can be anything up to C6R5 with perfect artifacts, and borrowed ones are someone else's account. Both are fine for a what-if and wrong in a comparison:

- The Arena and any ranking (roadmap ring 2) accept only verified builds (`docs/artifacts.md` §3), all from the player's own fresh Enka fetch. A team with an assumed or borrowed character never enters one.
- The same holds for Tier & Wiki if its tiers ever take visitors' scores.

## 6. Order of work

1. ~~This design, then the presets (§2.1) as data~~ Done: `internal/pull`.
2. After `claude/artifact-pieces` is merged: the pack format flag for assumed and borrowed builds (§2.4, §3).
3. The picker, the basic and advanced forms, borrowing from a UID and from a link, the badges (§1–§4). This changes the web handlers and templates, so it is ordered with the artifacts' web step (`docs/artifacts.md` §6, step 3).
4. Substat quality (§2.2) once the artifact weights table is on `main`. Until then the basic form offers the preset's substats and the advanced settings.

## 7. Not in this design

- Which character to pull (Pull planning, ring 2), which runs §2's score for many candidates.
- Primogem costs and income (Primogem priority, Primogem calculator).
- Characters gcsim does not support: they stay unsupported here as everywhere (design §6.3).
