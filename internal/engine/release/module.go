//go:build !js

// Package release holds the pinned gscouter-engine release's module
// (docs/engine.md §3), for the parse API (internal/gse). The browser build
// does not embed it.
package release

import _ "embed"

// Module is the pinned release's WebAssembly module, gse.wasm.
//
//go:embed gse.wasm
var Module []byte
