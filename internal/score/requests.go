package score

import (
	"context"
	"errors"
	"fmt"
	"maps"

	"github.com/genshinsim/gcsim/pkg/core"
	"github.com/genshinsim/gcsim/pkg/core/action"
	"github.com/genshinsim/gcsim/pkg/gcs/ast"
	"github.com/genshinsim/gcsim/pkg/gcs/eval"
	"github.com/genshinsim/gcsim/pkg/gcs/parser"
	"github.com/genshinsim/gcsim/pkg/stats"
)

// Marker is a delay a rotation can write (delay(987654)) to mark a point
// in its run, for Requests and Play. Delays of Marker or more are not
// played.
const Marker = 987654

// Request is an action a rotation asked for, in the order it asked:
// waits and delays included, whether or not the character could act yet.
type Request struct {
	Frame  int    // when the rotation asked, not when the action ran
	Char   string // gcsim key; empty for a wait, delay or mark
	Action string // gcsim action name, "wait", "delay" or "mark"
	Params map[string]int
}

// Requests runs cfg once and returns what its rotation asked for: the
// script's own actions, which the result only has once they run, and
// without their waits. A marker is a request "mark" with the marker's
// value less Marker in Params["f"].
func Requests(cfg string, seed int64) ([]Request, error) {
	var out []Request
	_, err := Play(context.Background(), cfg, seed, nil, func(c *core.Core, e *action.Eval) {
		r := Request{Frame: c.F, Action: e.Action.String(), Params: maps.Clone(e.Param)}
		switch {
		case IsMarker(e):
			r.Action, r.Params = "mark", map[string]int{"f": e.Param["f"] - Marker}
		case e.Action != action.ActionWait && e.Action != action.ActionDelay:
			r.Char = e.Char.String()
		}
		out = append(out, r)
	})
	return out, err
}

// IsMarker reports whether e is a marker, which Play does not play.
func IsMarker(e *action.Eval) bool {
	return e.Action == action.ActionDelay && e.Param["f"] >= Marker
}

// Play runs cfg once, calling setup with the fight's core before the
// fight is set up and ask with each action the rotation asks for, before
// the fight plays it. Markers are passed to ask and not played. The
// fight ends early when ctx is done, with ctx's error.
func Play(ctx context.Context, cfg string, seed int64, setup func(*core.Core), ask func(*core.Core, *action.Eval)) (*stats.Result, error) {
	file := ast.NewFile()
	simcfg, gcsl, err := parser.New(file, cfg).Parse()
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if len(simcfg.Errors) > 0 {
		return nil, fmt.Errorf("invalid config: %w", errors.Join(simcfg.Errors...))
	}
	res, err := simulateWith(ctx, file, simcfg, gcsl, seed, setup, func(ev *eval.Eval, c *core.Core) action.Evaluator {
		return &requester{Eval: ev, record: func(e *action.Eval) bool {
			ask(c, e)
			return !IsMarker(e)
		}}
	})
	return &res, err
}

// requester passes the program's actions to the fight, recording each
// first; record returns false for one the fight must not play.
type requester struct {
	*eval.Eval
	record func(*action.Eval) bool
}

func (r *requester) NextAction() (*action.Eval, error) {
	for {
		e, err := r.Eval.NextAction()
		if err != nil || e == nil || r.record(e) {
			return e, err
		}
		r.Eval.Continue()
	}
}

func (r *requester) RunSync(exec func(*action.Eval) bool) error {
	return r.Eval.RunSync(func(e *action.Eval) bool {
		if !r.record(e) {
			return true
		}
		return exec(e)
	})
}
