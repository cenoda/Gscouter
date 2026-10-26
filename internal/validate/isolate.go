package validate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
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
	v := os.Getenv(childEnv)
	if v == "" {
		return
	}
	var req childRequest
	if err := json.Unmarshal([]byte(v), &req); err != nil {
		fmt.Fprintln(os.Stderr, "bad check request:", err)
		os.Exit(2)
	}
	var r *Report
	if req.Reference {
		r = CheckReference(req.Dir, req.Settings)
	} else {
		r = Check(req.Dir, req.Settings)
	}
	if err := json.NewEncoder(os.Stdout).Encode(r); err != nil {
		os.Exit(2)
	}
	// Exit at once: a stuck simulation may still be spinning.
	os.Exit(0)
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
	fail := func(msg string) *Report { return &Report{Dir: req.Dir, Failures: []string{msg}} }
	exe, err := os.Executable()
	if err != nil {
		return fail(err.Error())
	}
	body, err := json.Marshal(req)
	if err != nil {
		return fail(err.Error())
	}
	// Check has its own timeout; this catches a child that outlives it.
	ctx, cancel := context.WithTimeout(context.Background(), timeout+30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe)
	cmd.Env = append(os.Environ(), childEnv+"="+string(body))
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	runErr := cmd.Run()
	var r Report
	if err := json.Unmarshal(stdout.Bytes(), &r); err != nil {
		return fail(fmt.Sprintf("check did not finish: %v %s", runErr, strings.TrimSpace(stderr.String())))
	}
	return &r
}
