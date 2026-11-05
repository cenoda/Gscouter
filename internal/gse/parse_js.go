//go:build js

package gse

import "errors"

// Parse is not available in the browser build, which does not run the
// module from Go; nothing there parses setup lines.
func Parse(cfg string) (*Parsed, error) {
	return nil, errors.New("engine: Parse is not available in the browser build")
}

// Available says the browser build has no parse API.
func Available() error { return errors.New("engine: Parse is not available in the browser build") }
