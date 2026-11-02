//go:build js

package engine

import "github.com/cenoda/gscouter/internal/score"

// newRunner has no runner in the browser build: the page's helper
// workers run the module (internal/web/static/worker.js).
func newRunner(*Engine) score.Engine { return nil }
