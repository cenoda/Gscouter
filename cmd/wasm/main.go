//go:build js && wasm

// Command wasm is the browser build of Gscouter: result pages run it in
// Web Workers to calculate their teams instead of the server
// (internal/web/client.go, internal/web/static/worker.js). Build it with
//
//	GOOS=js GOARCH=wasm go build -o gscouter.wasm ./cmd/wasm
//
// and serve it with the server's -static flag.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"syscall/js"
	"time"

	"github.com/cenoda/gscouter/internal/score"
	"github.com/cenoda/gscouter/internal/timeline"
	"github.com/cenoda/gscouter/internal/web"
)

func main() {
	js.Global().Set("gscouter", js.ValueOf(map[string]any{
		"runSlice": js.FuncOf(runSlice),
		"runJob":   js.FuncOf(runJob),
	}))
	select {}
}

// runSlice(cfg, warmup, seed, first, count) runs iterations first to
// first+count-1 of cfg and returns them as JSON, or {error}.
func runSlice(_ js.Value, args []js.Value) any {
	its, err := score.RunIterations(context.Background(), args[0].String(), score.Options{
		Warmup:     args[1].Float(),
		Seed:       int64(args[2].Int()),
		First:      args[3].Int(),
		Iterations: args[4].Int(),
		Workers:    1,
	})
	if err == nil {
		var b []byte
		if b, err = json.Marshal(its); err == nil {
			return string(b)
		}
	}
	return map[string]any{"error": err.Error()}
}

// runJob(job JSON) calculates a web.Job and resolves to {body, title}.
func runJob(_ js.Value, args []js.Value) any {
	src := args[0].String()
	return promise(func() (any, error) {
		var job web.Job
		if err := json.Unmarshal([]byte(src), &job); err != nil {
			return nil, err
		}
		body, title, err := web.RunJob(context.Background(), job, workerSim{})
		if err != nil {
			return nil, err
		}
		return map[string]any{"body": body, "title": title}, nil
	})
}

// workerSim shares each score's iterations out among the page's helper
// workers (self.gscouterRun), and runs timelines itself.
type workerSim struct{}

func (workerSim) Score(_ context.Context, cfg string, warmup float64, iterations int) (*score.Result, error) {
	v, err := await(js.Global().Call("gscouterRun", cfg, warmup, 1, iterations))
	if err != nil {
		return nil, err
	}
	var its []score.Iteration
	if err := json.Unmarshal([]byte(v.String()), &its); err != nil {
		return nil, err
	}
	if len(its) != iterations {
		return nil, errors.New("missing iterations")
	}
	return score.Aggregate(its, warmup), nil
}

func (workerSim) Timeline(_ context.Context, cfg string, from, to float64) (*timeline.Timeline, error) {
	start := time.Now()
	defer func() { reportTime("timeline", start) }()
	res, err := score.Once(cfg, 1)
	if err != nil {
		return nil, err
	}
	return timeline.Build(res, int(from*60), int(to*60)), nil
}

// reportTime tells the page how long a stage took since start.
func reportTime(name string, start time.Time) {
	js.Global().Call("gscouterTime", name, float64(time.Since(start).Microseconds())/1000)
}

// promise runs f in a goroutine, which may wait on JavaScript, and
// returns a Promise of its result.
func promise(f func() (any, error)) js.Value {
	var executor js.Func
	executor = js.FuncOf(func(_ js.Value, args []js.Value) any {
		resolve, reject := args[0], args[1]
		go func() {
			defer executor.Release()
			v, err := f()
			if err != nil {
				reject.Invoke(js.Global().Get("Error").New(err.Error()))
				return
			}
			resolve.Invoke(v)
		}()
		return nil
	})
	return js.Global().Get("Promise").New(executor)
}

// await waits for a Promise. It must not run on the event loop's
// goroutine (a js.Func), or the Promise can never settle.
func await(p js.Value) (js.Value, error) {
	values, errs := make(chan js.Value, 1), make(chan error, 1)
	then := js.FuncOf(func(_ js.Value, args []js.Value) any { values <- args[0]; return nil })
	catch := js.FuncOf(func(_ js.Value, args []js.Value) any {
		errs <- errors.New(args[0].Call("toString").String())
		return nil
	})
	defer then.Release()
	defer catch.Release()
	p.Call("then", then, catch)
	select {
	case v := <-values:
		return v, nil
	case err := <-errs:
		return js.Undefined(), err
	}
}
