//go:build !js

package gse

import (
	"context"
	"errors"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"

	"github.com/cenoda/gscouter/internal/engine/release"
)

// parser holds one instance of the pinned module for gse_parse, compiled
// on the first parse (about a second) and shared under a lock: a parse
// is a fraction of a millisecond.
var parser struct {
	once sync.Once
	mu   sync.Mutex
	mod  api.Module
	err  error
}

// Available is nil when the pinned module has the parse API, else why
// not (ErrNoParse for a release that predates it). Tests that parse skip
// on it until such a release is pinned.
func Available() error {
	_, err := Parse("")
	var pe *ParseError
	if errors.As(err, &pe) {
		return nil // it parses, and refuses an empty config
	}
	return err
}

// Parse parses cfg with the pinned release's module
// (internal/engine/release/gse.wasm):
// a config that does not parse gives its *ParseError, the Parsed beside
// it. A release without gse_parse gives ErrNoParse.
func Parse(cfg string) (*Parsed, error) {
	parser.once.Do(func() {
		parser.mod, parser.err = instantiate(release.Module)
	})
	if parser.err != nil {
		return nil, parser.err
	}
	parser.mu.Lock()
	defer parser.mu.Unlock()
	return parseOn(parser.mod, cfg)
}

// ParseModule is Parse on another module, for tests and tools.
func ParseModule(module []byte, cfg string) (*Parsed, error) {
	m, err := instantiate(module)
	if err != nil {
		return nil, err
	}
	defer m.Close(context.Background())
	return parseOn(m, cfg)
}

func instantiate(module []byte) (api.Module, error) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	compiled, err := rt.CompileModule(ctx, module)
	if err != nil {
		return nil, err
	}
	if _, ok := compiled.ExportedFunctions()["gse_parse"]; !ok {
		return nil, ErrNoParse
	}
	return rt.InstantiateModule(ctx, compiled, wazero.NewModuleConfig().WithName(""))
}

func parseOn(m api.Module, cfg string) (*Parsed, error) {
	status, body, err := CallStatus(context.Background(), m, "gse_parse", cfg)
	if errors.Is(err, ErrNoExport) {
		return nil, ErrNoParse
	}
	if err != nil {
		return nil, err
	}
	return decodeParse(status, body)
}
