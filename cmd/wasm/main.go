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

	"github.com/cenoda/gscouter/internal/chart"
	"github.com/cenoda/gscouter/internal/score"
	"github.com/cenoda/gscouter/internal/timeline"
	"github.com/cenoda/gscouter/internal/web"
)

func main() {
	js.Global().Set("gscouter", js.ValueOf(map[string]any{
		"runSlice": js.FuncOf(runSlice),
		"runJob":   js.FuncOf(runJob),
		// The chart editor (internal/web/templates/edit.html).
		"record":  js.FuncOf(record),
		"preview": js.FuncOf(preview),
		"encode":  js.FuncOf(encode),
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
// workers (self.gscouterRun), and runs timelines itself. A candidate the
// engine rejects comes back as Score's error, as it does natively, for
// the search (steady.Score, steady.Best) to go on with the next one.
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

func (workerSim) Timeline(ctx context.Context, cfg string, from, to float64) (*timeline.Timeline, error) {
	start := time.Now()
	defer func() { reportTime("timeline", start) }()
	return timeline.Simulate(ctx, cfg, int(from*60), int(to*60))
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
		// An Error's message, as the native search would have it.
		if msg := args[0].Get("message"); msg.Type() == js.TypeString {
			errs <- errors.New(msg.String())
		} else {
			errs <- errors.New(args[0].Call("toString").String())
		}
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

// record(build, rotation, names, moves) writes a rotation as a chart for
// the members named names (a JSON array, by slot) and returns its text,
// or {error}. moves, a JSON array by slot or "", moves the chart to the
// player's slots (chart.Remap), for a rotation of another team.
func record(_ js.Value, args []js.Value) any {
	names, err := slots(args[2].String())
	if err == nil {
		var c *chart.Chart
		if c, err = chart.Record(args[0].String(), args[1].String(), names); err == nil {
			if len(args) > 3 && args[3].String() != "" {
				var moves [chart.Slots]int
				if err = json.Unmarshal([]byte(args[3].String()), &moves); err != nil {
					return map[string]any{"error": err.Error()}
				}
				c = c.Remap(moves)
			}
			return c.String()
		}
	}
	return map[string]any{"error": err.Error()}
}

// preview(build, chart, names, defaults) plays a chart, its text, with
// build and each slot's character params (the chart's, else defaults, a
// JSON array by slot), and returns the chart.Preview as JSON, or {error}.
func preview(_ js.Value, args []js.Value) any {
	out, err := func() (string, error) {
		c, err := chart.Parse(args[1].String())
		if err != nil {
			return "", err
		}
		names, err := slots(args[2].String())
		if err != nil {
			return "", err
		}
		defaults, err := slots(args[3].String())
		if err != nil {
			return "", err
		}
		p, err := c.Preview(c.WithParams(args[0].String(), names, defaults), names, nil)
		if err != nil {
			return "", err
		}
		b, err := json.Marshal(p)
		return string(b), err
	}()
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	return out
}

// encode(chart) is a chart's text for a link (chart.Encode), or {error}.
func encode(_ js.Value, args []js.Value) any {
	c, err := chart.Parse(args[0].String())
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	return c.Encode()
}

func slots(s string) ([chart.Slots]string, error) {
	var out [chart.Slots]string
	var list []string
	if err := json.Unmarshal([]byte(s), &list); err != nil {
		return out, err
	}
	if len(list) != chart.Slots {
		return out, errors.New("four members expected")
	}
	copy(out[:], list)
	return out, nil
}
