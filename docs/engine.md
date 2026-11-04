# gscouter-engine in Gscouter

> Status: implemented, no team approved yet; the owner's rules for approving decided 2026-11-04 · Last updated: 2026-11-04

How Gscouter scores teams on gscouter-engine, a Rust port of gcsim (https://github.com/cenoda/gscouter-engine), Phase 4 of its roadmap (`docs/roadmap.md` §3 there). The engine scores within 1% of gcsim, not to the bit, so a team moves to it only when the owner approves it. Everything else stays on gcsim.

## 1. Which runs go to the engine

A team's simulations run on the engine when all of these hold, and on gcsim otherwise:

- it is a library team (not one scored only with a player's chart);
- the page scores it with the library's rotations: a selection with a chart (`docs/cycle-swap.md`) runs on gcsim, the library score beside it too, so the comparison is of one engine;
- it is verified in the pinned release's manifest (§3), whose module is the one built into the server;
- its library files (`teams/<id>/`: `team.yaml`, `rotation.gcsl`, `build.txt`, and those of its alternatives) are the ones the release verified, at the manifest's Gscouter commit (§3);
- it is on the owner's approved list, and its approval holds for this release (§2).

The server decides it when it prepares a team (`teamResult.Engine`), and the job carries the decision to the browser, so both route alike. For such a team, every score simulation runs on the engine: the score, the filler and alternative ranking runs (`steady.Best`) and the one pass, with gcsim's pick of rotation and filler by default, or the engine's own pick when `-engine-own-search` is enabled (§5). Timelines, character details and the chart editor stay on gcsim, in Go.

A config the engine refuses (one it lacks something for, such as a weapon, or a fight that fails on it) runs on gcsim, all of its iterations, so a run is never part one engine and part the other: on the server `score.RunIterations` runs gcsim instead; in the browser a helper reports the refusal and the page runs all of that run's chunks again on gcsim. Iteration `i` uses seed `1 + i` wherever it runs, and a chunk's result does not depend on how the run is split, so the browser and the server give the same result either way. The manifest's list of page configs the engine cannot run is not read: the engine refuses them itself.

The same holds for a config the check accepts but the engine fails on while it runs: an error it returns (a weapon or set it cannot apply to two characters, a hit pattern a gadget needs, a reaction on the player it lacks) or a trap. A panic in the engine aborts the module (`panic = "abort"` in a WebAssembly module traps on `unreachable`), and a trap stops that one call: the server's runner (wazero, pure Go, no cgo) gets it back as an error, and each share runs on its own instance, closed after the call; a browser's helper catches it, drops that instance (its memory may be left half-written) and makes a new one for the next slices. Either way the run goes to gcsim, and the player sees gcsim's result, never the error. Nothing links the engine natively, so no engine fault can take the server or a page down (`TestRuntimeFailureRunsOnGcsim`).

Each such fallback is logged at the warning level with the team and the error, which names what the engine lacks, to find what to port: the server's runs by `score.RunIterations` (`engine failed a config, running gcsim`), a browser's by `POST /engine-error`, which the page sends with `navigator.sendBeacon` for at most five failures a page, and which the server logs at most 60 a minute (`engine failed a config in a browser`). The player's build is not logged: the site says that nothing of an imported GOOD file is stored or logged (`about.d_good`).

## 2. The approved list

`internal/engine/approved.txt`: one library team id per line, with the date of the owner's answer (`hutao-xingqiu-zhongli-yelan 2026-11-04`), and for a team approved from group B, the release it was approved for (`release=` and the first 16 hex digits of the module's SHA-256). A line that is not of that form fails to load, and `#` starts a comment. Only the owner approves: a team's score moves, within 1% of gcsim, when it moves to the engine.

How the owner answers (decided 2026-11-04; gscouter-engine's `docs/phase4-approval.md`): a release puts each verified team in group A (the library build's and the one-pass page config's team DPS within ±0.5% of gcsim, no build of the team settled, every judged character within 1%) or group B (every other verified team), and its `approval-summary.md` is the page the owner answers.

- **Group A at once.** One yes for a release's group A is the answer for each of its teams. After it, `go run ./cmd/pinengine -approve-a <date of the answer>` adds every group A team of the pinned release not yet listed, one line each with that date, under a comment naming the release. Run it only after the answer: `approved-candidates.txt` in the release lists the same teams, with no dates.
- **Group B one by one.** The owner approves each team, and its line names the release (`release=<module>`): it holds for that release only.
- **Carried forward until a problem appears.** An approval from group A holds across later releases, with no new answer, while its team stays verified and in group A. A release in which the team is not verified, or not in group A, drops the approval (`engine.Dropped`). The team then runs on gcsim, and it comes back to the owner before it runs on the engine again, whatever a later release says. Pinning such a release (§3) takes the dropped lines off the list, with a comment saying why, and the release's summary page lists them (it reads `main`'s list).
- A team whose library files changed since the release (§3) is not dropped: it runs on gcsim until a release verifies its new files.
- **Verification is the library build** (the owner, 2026-11-04): a team is verified when its library build and the probes pass gscouter-engine's compare within 1% of gcsim, and nothing more; a release judges the library kind by default (`release_kinds=library`; the manifest's `kinds` and each team's `verified_kinds` say what it judged), and groups teams on the library build alone. There are no later runs of variants, build space or pages, and none that demote teams. Support means any build a player enters, for content the engine has: an approved team's runs go to the engine whatever the player's build, and a config the engine refuses (`gse_check`, which `gse_run_slice` runs first) runs on gcsim, all of that run's iterations (§1).

An approved team that the pinned release does not run (not verified, out of group A, approved for another release, or with changed library files) stays on gcsim; the server logs it at start, and `go test ./internal/engine` fails naming it, so a release that drops a team, or a library change to an approved team, is noticed before it ships.

With the list empty, Gscouter behaves as before the engine: on 2026-11-02 the `server=1` pages of Hu Tao / Xingqiu / Zhongli / Yelan, Lohen / Durin / Nicole / Citlali and Skirk / Furina / Citlali / Escoffier (the test showcases of `internal/web`) were byte-identical between `main` and this change, and so were their bodies calculated in headless Chromium. A job's JSON carries `Engine` only when set.

## 3. The release: pinning and shipping

A release is the artifact `engine-release` of a manual run of gscouter-engine's `compare` workflow with `release` set to `true` on its `main`: the WebAssembly module and `manifest.json`, the teams verified on it, naming it by its SHA-256.

- **Pinned in the repository**: the module and the manifest are committed as `internal/engine/release/gse.wasm` and `manifest.json` (about 1.5 MB per release), with `teams.txt`, the hash of each verified team's library files (`engine.TeamHash`) at the manifest's `gscouter_commit`, read from git by the pinning step. Artifacts of workflow runs expire and need a GitHub token to download, and the browser and the server must run the same bytes, so the build takes them from the repository rather than fetching them.
- **Built into the server**: `go:embed`, as the team library is. Nothing changes in the Dockerfile, the deployment or `-static`: the image carries the engine because the server binary does. The browser build (`cmd/wasm`) does not embed it.
- **Served to browsers** by the server at `/static/gse.wasm?v=<first 16 hex digits of its SHA-256>`, compressed once in memory, `immutable` like the browser build. A page downloads it only when one of its teams runs on the engine, and compiles it beside the browser build.
- **Checked**: the server loads the release only when the module's SHA-256 is the manifest's; otherwise it logs why and runs every team on gcsim. Without the module, the engine is off. At start it hashes each approved team's embedded library files, a few small files per team, and a team whose hash is not `teams.txt`'s runs on gcsim: a verification holds for the rotations and builds it ran, and a change to them (a new rotation, a revalidation) waits for a release that verifies the new files.
- **Updating**: download the run's artifact (`gh run download <run id> -R cenoda/gscouter-engine -n engine-release`), run `go run ./cmd/pinengine -from <artifact dir>` from the repository's root (it needs the manifest's Gscouter commit in the clone's history, lists the teams changed since it, and takes the approvals the release drops off `approved.txt`, §2), run `go test ./internal/engine ./internal/web`, and commit the files together. A new module changes the scores of the teams on it, so pinning it is the owner's call; the approvals that hold carry forward to it (§2).
- **Trying a release** before pinning it: `server -engine-dir <dir>` reads `approved.txt`, `gse.wasm`, `manifest.json` and `teams.txt` from a directory instead. `-engine=false` runs every team on gcsim.

**Caches** key a run on the engine by its module's hash, and gcsim's keys are as before: the server's scores in memory (`engine <sha256>` before the config), `score.Cache` (an `engine <sha256>` part, `Options.Engine`), and the simulations a tab keeps (`gse <version>` before the key). Link-preview cards are cached in memory by URL; a release or an approval reaches the site only by a deploy, which starts the server afresh.

## 4. The server: wazero

The server runs the same module on wazero (pure Go, no cgo), as the browser's helper workers do: each run is split into one share per CPU, each on its own instance of the module, and each share calls `gse_run_slice` five iterations at a time, stopping between calls when the request is cancelled. The module is compiled once, about 0.75–1 s, in the background when the server starts with teams on the engine.

Measured 2026-11-02 on this session's machine (4 CPUs), gscouter-engine's `configs/team-looped.txt`, 50 iterations: wazero 0.60 s on one thread and 0.36 s on four, gcsim 1.04 s and 0.42 s, the same module in Node 22 0.40 s on one. wazero's `WithCloseOnContextDone`, which can stop a call under way, made the engine 3.4 times slower (2.23 s), so cancellation waits for the calls under way instead.

## 5. Open questions

- **Charts on approved teams** run on gcsim (§1), kept so for now (2026-11-02). The engine runs them when it has what they use, but no release verifies players' charts.
- Decided 2026-11-02: a team whose library files changed since the manifest's Gscouter commit runs on gcsim (§3).
- **Fillers: the page keeps gcsim's pick by default** (owner, 2026-11-04). The search (`steady.Best`) is mostly runs ranked on 10 iterations, whose noise differs between the engines: on 331 library builds that run on both (`cmd/fillerpick`, 400 teams sampled by hash, module of main 794a974, 2026-11-04) they picked another rotation or filler for 56, 40 of them within 1% of each other, and the page score moved by −1.7% to +3.4% on those 56. So `compute` (shared by the server and the browser's lead worker) runs the search on the engine first; when it goes past the rotation as written, the engine's search stops at its second run, the search runs on gcsim, and gcsim's pick is scored on the engine with the full iterations (`keepGcsimPick`). Until 2026-11-04 the engine ran its whole search first and then threw it away, which made such pages slower than gcsim alone (`cmd/pagebench`, library builds at the site's settings on 4 CPUs: Hu Tao / Xingqiu / Zhongli / Yelan 4.19 s against gcsim's 1.99 s; stopping at the second run, 2.78 s, the same pick). A team whose search stops at the rotation as written runs on the engine alone: 159 of the 331 builds, so the engine's speed is kept for about half of them. With the rule, the page's pick differed from gcsim's on 3 of the 331 (the engine stopped at the rotation as written where gcsim searched; each pick within 1% of gcsim's on the engine), against 56 without it; ranking on more iterations, so that both engines find the true best, is a follow-up.

The server flag `-engine-own-search` (default **OFF**, `web.Settings.EngineOwnSearch`) lets an engine team run its whole filler and alternative search on the engine and use its pick. It saves the gcsim search on searching pages. `compute` makes the decision once for both the server and the browser's lead worker; the browser receives the setting in `Job.Settings`. Teams on gcsim stay on gcsim. If any engine search run falls back to gcsim, even a ranked candidate or the full confirmation, the search restarts on gcsim and keeps gcsim's pick as above. The fallback signal survives score caches and browser iteration transport.

The trade-off is a different filler or rotation for about **1 in 6 builds**, with about **1 in 20 builds differing by 1–3.5%** in page DPS. These are the measurements above: 56 of 331 picks differed, 16 beyond 1%, and the score changes ranged from −1.7% to +3.4%. The search ranks mostly on 10 iterations, so the engines' different noise can change the winner. Leave the flag off to preserve today's picks; turning it on chooses faster searching pages with that variation.

## 6. The `/updates` entry

Required by the owner (2026-11-04) when teams move to the engine: one entry with the merge the owner approves, then one for each release that moves teams. It is written here, not in `internal/web/about.go`, until the owner approves the first release's teams; the numbers come from that release. Draft:

- En: "Scores of {N} library teams are now calculated by gscouter-engine, a faster port of gcsim, in your browser and on the server. Each was compared with gcsim on its library build, its variants and the configs its result page runs, and its scores stay within 1% of gcsim's, but they move a little: within ±0.5% for most teams, up to 1% for the rest. Teams with your own rotation chart, timelines and character details are still calculated by gcsim. Result pages of these teams are faster."
- Ko: "라이브러리 파티 {N}개의 점수를 이제 gcsim을 더 빠르게 옮긴 gscouter-engine이 브라우저와 서버에서 계산합니다. 각 파티는 라이브러리 빌드, 변형 빌드, 결과 페이지가 돌리는 설정에서 gcsim과 비교했고 점수는 gcsim과 1% 안에서 같지만, 조금 움직입니다: 대부분 ±0.5% 안, 나머지는 1%까지. 직접 만든 로테이션 차트, 타임라인, 캐릭터 상세는 여전히 gcsim이 계산합니다. 이 파티들의 결과 페이지가 더 빨리 뜹니다."
- `Scores: true` (the entry's "Scores changed" mark).
- `about.scores` ("Scores are gcsim simulations…") is changed with it, in every language: "Scores are simulations by gcsim, or for some library teams by gscouter-engine, which matches gcsim within 1%, under fixed conditions…".
- "Faster" is said only once measured on the site's pages for the approved teams (`docs/speed.md`).
