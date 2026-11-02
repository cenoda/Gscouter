# Engine release

The gscouter-engine release Gscouter runs (docs/engine.md §3): the files of
one `engine-release` artifact, from a manual run of gscouter-engine's
`compare` workflow with `release` set to `true` on its `main`.

- `gse.wasm`: the WebAssembly module (`gse_wasm.wasm` in the artifact).
- `manifest.json`: the teams verified on that module, which names it by its
  SHA-256 (`engine.wasm_sha256`).

Both are embedded in the server (`internal/engine`). Without them the
engine is off and every team runs on gcsim. A module whose SHA-256 is not
the manifest's turns the engine off too, and the server logs why.
