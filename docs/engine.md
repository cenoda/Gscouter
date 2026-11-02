# gscouter-engine in Gscouter

> Status: implemented, no team approved yet · Last updated: 2026-11-02

How Gscouter scores teams on gscouter-engine, a Rust port of gcsim (https://github.com/cenoda/gscouter-engine), Phase 4 of its roadmap (`docs/roadmap.md` §3 there). The engine scores within 1% of gcsim, not to the bit, so a team moves to it only when the owner approves it. Everything else stays on gcsim.

## 1. Which runs go to the engine

A team's simulations run on the engine when all of these hold, and on gcsim otherwise:

- it is a library team (not one scored only with a player's chart);
- the page scores it with the library's rotations: a selection with a chart (`docs/cycle-swap.md`) runs on gcsim, the library score beside it too, so the comparison is of one engine;
- it is verified in the pinned release's manifest (§3), whose module is the one built into the server;
- its library files (`teams/<id>/`: `team.yaml`, `rotation.gcsl`, `build.txt`, and those of its alternatives) are the ones the release verified, at the manifest's Gscouter commit (§3);
- it is on the owner's approved list (§2).

The server decides it when it prepares a team (`teamResult.Engine`), and the job carries the decision to the browser, so both route alike. For such a team, every score simulation runs on the engine: the score, the filler and alternative ranking runs (`steady.Best`) and the one pass. Timelines, character details and the chart editor stay on gcsim, in Go.

A config the engine refuses (one it lacks something for, such as a weapon, or a fight that fails on it) runs on gcsim, all of its iterations, so a run is never part one engine and part the other: on the server `score.RunIterations` runs gcsim instead; in the browser a helper reports the refusal and the page runs all of that run's chunks again on gcsim. Iteration `i` uses seed `1 + i` wherever it runs, and a chunk's result does not depend on how the run is split, so the browser and the server give the same result either way. The manifest's list of page configs the engine cannot run is not read: the engine refuses them itself.

## 2. The approved list

`internal/engine/approved.txt`: one library team id per line, with the date of the owner's answer (`hutao-xingqiu-zhongli-yelan 2026-11-02`); a line without a valid date fails to load, and `#` starts a comment. It starts empty, and only the owner adds teams, one by one: a team's score moves, within 1% of gcsim, when it moves to the engine, and its filler can flip where two candidates are within 1% (gscouter-engine's `docs/roadmap.md` §4).

An approved team that the pinned release does not verify, or whose library files changed since (§3), stays on gcsim; the server logs it at start, and `go test ./internal/engine` fails naming it, so a release that drops a team, or a library change to an approved team, is noticed before it ships.

With the list empty, Gscouter behaves as before the engine: on 2026-11-02 the `server=1` pages of Hu Tao / Xingqiu / Zhongli / Yelan, Lohen / Durin / Nicole / Citlali and Skirk / Furina / Citlali / Escoffier (the test showcases of `internal/web`) were byte-identical between `main` and this change, and so were their bodies calculated in headless Chromium. A job's JSON carries `Engine` only when set.

## 3. The release: pinning and shipping

A release is the artifact `engine-release` of a manual run of gscouter-engine's `compare` workflow with `release` set to `true` on its `main`: the WebAssembly module and `manifest.json`, the teams verified on it, naming it by its SHA-256.

- **Pinned in the repository**: the module and the manifest are committed as `internal/engine/release/gse.wasm` and `manifest.json` (about 1.5 MB per release), with `teams.txt`, the hash of each verified team's library files (`engine.TeamHash`) at the manifest's `gscouter_commit`, read from git by the pinning step. Artifacts of workflow runs expire and need a GitHub token to download, and the browser and the server must run the same bytes, so the build takes them from the repository rather than fetching them.
- **Built into the server**: `go:embed`, as the team library is. Nothing changes in the Dockerfile, the deployment or `-static`: the image carries the engine because the server binary does. The browser build (`cmd/wasm`) does not embed it.
- **Served to browsers** by the server at `/static/gse.wasm?v=<first 16 hex digits of its SHA-256>`, compressed once in memory, `immutable` like the browser build. A page downloads it only when one of its teams runs on the engine, and compiles it beside the browser build.
- **Checked**: the server loads the release only when the module's SHA-256 is the manifest's; otherwise it logs why and runs every team on gcsim. Without the module, the engine is off. At start it hashes each approved team's embedded library files, a few small files per team, and a team whose hash is not `teams.txt`'s runs on gcsim: a verification holds for the rotations and builds it ran, and a change to them (a new rotation, a revalidation) waits for a release that verifies the new files.
- **Updating**: download the run's artifact (`gh run download <run id> -R cenoda/gscouter-engine -n engine-release`), run `go run ./cmd/pinengine -from <artifact dir>` from the repository's root (it needs the manifest's Gscouter commit in the clone's history, and lists the teams changed since it), run `go test ./internal/engine ./internal/web`, and commit the three files together. A new module changes the scores of the teams on it, so it is the owner's call like an approval.
- **Trying a release** before pinning it: `server -engine-dir <dir>` reads `approved.txt`, `gse.wasm`, `manifest.json` and `teams.txt` from a directory instead. `-engine=false` runs every team on gcsim.

**Caches** key a run on the engine by its module's hash, and gcsim's keys are as before: the server's scores in memory (`engine <sha256>` before the config), `score.Cache` (an `engine <sha256>` part, `Options.Engine`), and the simulations a tab keeps (`gse <version>` before the key). Link-preview cards are cached in memory by URL; a release or an approval reaches the site only by a deploy, which starts the server afresh.

## 4. The server: wazero

The server runs the same module on wazero (pure Go, no cgo), as the browser's helper workers do: each run is split into one share per CPU, each on its own instance of the module, and each share calls `gse_run_slice` five iterations at a time, stopping between calls when the request is cancelled. The module is compiled once, about 0.75–1 s, in the background when the server starts with teams on the engine.

Measured 2026-11-02 on this session's machine (4 CPUs), gscouter-engine's `configs/team-looped.txt`, 50 iterations: wazero 0.60 s on one thread and 0.36 s on four, gcsim 1.04 s and 0.42 s, the same module in Node 22 0.40 s on one. wazero's `WithCloseOnContextDone`, which can stop a call under way, made the engine 3.4 times slower (2.23 s), so cancellation waits for the calls under way instead.

## 5. Open questions

- **Charts on approved teams** run on gcsim (§1), kept so for now (2026-11-02). The engine runs them when it has what they use, but no release verifies players' charts.
- Decided 2026-11-02: a team whose library files changed since the manifest's Gscouter commit runs on gcsim (§3).
