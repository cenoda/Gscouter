# Documents

> Last updated: 2026-10-29

What each document is for, and the habits that keep them readable. These are conventions, not rules: follow them unless a document has a reason not to, and change them here when they stop fitting.

## Index

| Document | What it holds |
|---|---|
| `docs/design.md` | The MVP: what the score is, how the system computes it, the team library and its tiers. Decisions and how things work now |
| `docs/roadmap.md` | Where the site goes after the MVP, as a map of nodes |
| `docs/artifacts.md` | Roadmap nodes Artifacts and Artifact score |
| `docs/pull.md` | Roadmap node Pull a character |
| `docs/speed.md` | Roadmap node faster DPS: time to a score in the browser and in validation runs, and what was tried |
| `docs/cycle.md` | Roadmap node Cycle: how the fixed rotation fits builds other than the source's (constellations, weapons), and what to change |
| `docs/fillers.md` | How the looped score fills a character's waits for energy or cooldowns, and what that changes |
| `docs/fork.md` | The gcsim fork: how it is kept, regression runs, the characters and weapons on it |
| `docs/characters/<name>.md` | One character on the fork: its implementation notes (source, frame data, known problems) and its written library teams |
| `docs/characters/first-batches.md` | The first written teams (Nefer, Linnea, Zibai, Iansan), recorded before characters had their own documents |

`README.md` at the root covers running the site and the commands. `deploy/README.md` covers deployment. `.claude/skills/*/SKILL.md` are workflows for agents.

## Where things go

- **Design is for decisions.** `docs/design.md` says what the system does and why, in the present tense. It stays short enough to read in one sitting.
- **Records go next to their subject.** Measurements, calibration, regression results, batches of teams and the cross-checks behind them are records: they grow with every change and few readers need all of them. They go in the document of their subject (a character, the fork, a roadmap node), and the design keeps the conclusion and a pointer.
- **One document per roadmap node** once work on it starts, named after the node (`artifacts.md`, `pull.md`).
- **One document per fork character** in `docs/characters/`, named by the gcsim key. A new character's teams are recorded there, not in the design.

## When to split

- Split a section into its own document when it is mostly records, or when it grows past what a reader would scroll through to reach the next section (roughly a few screens).
- Split a document when it passes about 60 KB, or when most readers come for one part of it.
- When splitting, leave behind a short summary of the decision and a pointer to the new document, and update every reference to the moved text (see below). Keep section numbers of what stays, so old references keep working.
- A table cell holds a few words or a number. Prose that outgrows a cell becomes a subsection.

## Writing

- English, plain, present tense for how things are. Say what was measured, on what, and when, so a number can be checked again.
- Date what can go stale: measurements, searches, "checked" states (`2026-10-28`). The header's "Last updated" is the date of the last real change.
- A finished question or step stays as a short line with what was decided; its reasoning moves to where the decision lives.
- Update the document in the same commit as the change it describes.

## Header

Every document starts with its title and one quoted line:

```
# Title

> Status: <state> · Last updated: <date>
```

Status says how far the document is from the code: a draft design, in progress, implemented. Leave it out when it does not apply.

## References

- Within a document: `§3.2`. To another document: `design §3.2`, or the path for any other document, `docs/fork.md §2`.
- Code comments cite the document and section their rule comes from, so a reader can find the reason: `// ... (docs/fork.md §2).`
- Moving or renumbering a section means updating what cites it: `grep -rn "§" --include=*.go --include=*.md --include=*.yml --include=*.yaml .` finds them. Leave library rotations and builds (`rotation.gcsl`, `build.txt`) as they are: their text is part of the gcsim config, so editing a comment there changes cache keys.
