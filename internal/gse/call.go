//go:build !js

package gse

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"

	"github.com/tetratelabs/wazero/api"
)

// ErrNoExport is the error of a call to a function the module does not
// export, such as gse_parse in a release older than the parse API.
var ErrNoExport = errors.New("engine: the module does not export")

// Call is one call of the module (wasm/lib.rs in gscouter-engine): cfg
// goes in through gse_alloc, fn is called with its pointer, its length and
// args, and the result comes back as a u32 status (0 ok), a u32 length
// and that many bytes, the body or the error.
func Call(ctx context.Context, m api.Module, fn string, cfg string, args ...uint64) ([]byte, error) {
	status, body, err := CallStatus(ctx, m, fn, cfg, args...)
	if err != nil {
		return nil, err
	}
	if status != 0 {
		return nil, errors.New(string(body))
	}
	return body, nil
}

// CallStatus is Call with the status returned beside the body, for a
// function whose error body is itself data (gse_parse).
func CallStatus(ctx context.Context, m api.Module, fn string, cfg string, args ...uint64) (uint32, []byte, error) {
	alloc, free, freeResult, f := m.ExportedFunction("gse_alloc"), m.ExportedFunction("gse_free"), m.ExportedFunction("gse_free_result"), m.ExportedFunction(fn)
	if alloc == nil || free == nil || freeResult == nil || f == nil {
		return 0, nil, fmt.Errorf("%w: %s", ErrNoExport, fn)
	}
	if len(cfg) > math.MaxUint32 {
		return 0, nil, errors.New("engine: config too long")
	}
	res, err := alloc.Call(ctx, uint64(len(cfg)))
	if err != nil {
		return 0, nil, fmt.Errorf("engine: %w", err)
	}
	ptr := uint32(res[0])
	if !m.Memory().WriteString(ptr, cfg) {
		return 0, nil, errors.New("engine: config out of the module's memory")
	}
	res, err = f.Call(ctx, append([]uint64{uint64(ptr), uint64(len(cfg))}, args...)...)
	if err != nil {
		return 0, nil, fmt.Errorf("engine: %w", err)
	}
	out := uint32(res[0])
	head, ok := m.Memory().Read(out, 8)
	if !ok {
		return 0, nil, errors.New("engine: result out of the module's memory")
	}
	status, n := binary.LittleEndian.Uint32(head), binary.LittleEndian.Uint32(head[4:])
	b, ok := m.Memory().Read(out+8, n)
	if !ok {
		return 0, nil, errors.New("engine: result out of the module's memory")
	}
	body := append([]byte(nil), b...)
	if _, err := freeResult.Call(ctx, uint64(out)); err != nil {
		return 0, nil, fmt.Errorf("engine: %w", err)
	}
	if _, err := free.Call(ctx, uint64(ptr), uint64(len(cfg))); err != nil {
		return 0, nil, fmt.Errorf("engine: %w", err)
	}
	return status, body, nil
}
