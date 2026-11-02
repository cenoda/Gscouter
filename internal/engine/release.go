//go:build !js

package engine

import (
	"embed"
	"errors"
	"io/fs"
	"os"
	"path"
)

// The approved list and the pinned release (release/README.md), built
// into the server.
//
//go:embed approved.txt release
var files embed.FS

// Release is the engine of the pinned release with the approved teams.
// It is nil, with no error, when no release is pinned, and nil with the
// error when the files are inconsistent: either way every team runs on
// gcsim.
func Release() (e *Engine, skipped []string, err error) {
	return load(files, "release")
}

// ReleaseDir is Release from dir, which holds approved.txt, gse.wasm and
// manifest.json, for trying a release before pinning it.
func ReleaseDir(dir string) (e *Engine, skipped []string, err error) {
	return load(os.DirFS(dir), ".")
}

func load(fsys fs.FS, release string) (e *Engine, skipped []string, err error) {
	approved, err := fs.ReadFile(fsys, "approved.txt")
	if err != nil {
		return nil, nil, err
	}
	list, err := ParseApproved(string(approved))
	if err != nil {
		return nil, nil, err
	}
	module, err := fs.ReadFile(fsys, path.Join(release, "gse.wasm"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	manifest, err := fs.ReadFile(fsys, path.Join(release, "manifest.json"))
	if err != nil {
		return nil, nil, err
	}
	return New(module, manifest, list)
}
