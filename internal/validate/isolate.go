package validate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/cenoda/gscouter/internal/batch"
)

// childEnv carries a check request to a child process.
const childEnv = "GSCOUTER_VALIDATE_CHILD"

type childRequest struct {
	Dir       string
	Settings  Settings
	Reference bool // CheckReference only
}

// ServeChild must be called first thing in main by every command that
// uses CheckIsolated. In a process CheckIsolated started, it runs the
// requested check, writes the report to stdout as JSON and exits.
func ServeChild() {
	batch.Serve(childEnv, func(body []byte) any {
		var req childRequest
		if err := json.Unmarshal(body, &req); err != nil {
			fmt.Fprintln(os.Stderr, "bad check request:", err)
			os.Exit(2)
		}
		if req.Reference {
			return CheckReference(req.Dir, req.Settings)
		}
		return Check(req.Dir, req.Settings)
	})
}

// CheckIsolated runs Check in a child process of the running command. A
// rotation that never advances spins inside gcsim, which cannot be
// cancelled; the child's exit stops it, where in this process it would
// keep a CPU busy until the end.
func CheckIsolated(dir string, s Settings) *Report {
	return isolated(childRequest{Dir: dir, Settings: s})
}

// CheckReferenceIsolated runs CheckReference in a child process.
func CheckReferenceIsolated(dir string, s Settings) *Report {
	return isolated(childRequest{Dir: dir, Settings: s, Reference: true})
}

func isolated(req childRequest) *Report {
	// Check has its own timeout; this catches a child that outlives it.
	ctx, cancel := context.WithTimeout(context.Background(), req.Settings.timeout()+30*time.Second)
	defer cancel()
	var r Report
	if err := batch.Isolated(ctx, childEnv, req, &r); err != nil {
		return &Report{Dir: req.Dir, Failures: []string{err.Error()}}
	}
	return &r
}
