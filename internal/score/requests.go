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
)

// Marker is a delay a rotation can write (delay(987654)) to mark a point
// in its run for Requests. It is not played.
const Marker = 987654

// Request is an action a rotation asked for, in the order it asked:
// waits and delays included, whether or not the character could act yet.
type Request struct {
	Frame  int    // when the rotation asked, not when the action ran
	Char   string // gcsim key; empty for a wait or delay
	Action string // gcsim action name, "wait", "delay" or "mark"
	Params map[string]int
}

// Requests runs cfg once and returns what its rotation asked for: the
// script's own actions, which the result only has once they run, and
// without their waits.
func Requests(cfg string, seed int64) ([]Request, error) {
	file := ast.NewFile()
	simcfg, gcsl, err := parser.New(file, cfg).Parse()
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if len(simcfg.Errors) > 0 {
		return nil, fmt.Errorf("invalid config: %w", errors.Join(simcfg.Errors...))
	}
	var out []Request
	_, err = simulateWith(context.Background(), file, simcfg, gcsl, seed, nil, func(ev *eval.Eval, c *core.Core) action.Evaluator {
		return &requester{Eval: ev, record: func(e *action.Eval) bool {
			r := Request{Frame: c.F, Action: e.Action.String(), Params: maps.Clone(e.Param)}
			switch e.Action {
			case action.ActionWait, action.ActionDelay:
				if e.Action == action.ActionDelay && e.Param["f"] == Marker {
					out = append(out, Request{Frame: c.F, Action: "mark"})
					return false
				}
			default:
				r.Char = e.Char.String()
			}
			out = append(out, r)
			return true
		}}
	})
	return out, err
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
