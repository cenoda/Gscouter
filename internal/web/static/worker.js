// A worker running the browser build of Gscouter (cmd/wasm) for a result
// page (the "client" template). The lead calculates the page and asks
// the page for simulations; helpers run a share of their iterations. The
// chart editor (edit.html) runs one worker and calls it directly.
importScripts("/static/wasm_exec.js" + self.location.search);

var ready, waiting = {}, next = 0;

function report(err) {
  postMessage({type: "error", error: String(err && err.message || err)});
}

self.onmessage = function (e) {
  var m = e.data;
  if (m.type === "init") {
    var go = new Go();
    ready = WebAssembly.instantiate(m.module, go.importObject).then(function (inst) { go.run(inst); });
    ready.then(function () { postMessage({type: "ready"}); }, report);
    return;
  }
  ready.then(function () {
    switch (m.type) {
    case "slice":
      // A candidate the engine rejects fails its run, not the page: the
      // error goes back with the run, for the lead's search to go on.
      var out = gscouter.runSlice(m.cfg, m.warmup, m.seed, m.first, m.count);
      postMessage(typeof out === "string" ?
        {type: "slice", run: m.run, index: m.index, count: m.count, result: out} :
        {type: "slice", run: m.run, index: m.index, count: m.count, error: String(out.error)});
      break;
    case "job":
      gscouter.runJob(JSON.stringify(m.job)).then(function (out) {
        postMessage({type: "done", body: out.body, title: out.title});
      }, report);
      break;
    case "call":
      // The chart editor: gscouter.record, preview or encode, which
      // return a string or {error}.
      var r = gscouter[m.fn].apply(null, m.args);
      postMessage(typeof r === "string" ? {type: "call", id: m.id, result: r} : {type: "call", id: m.id, error: r.error});
      break;
    case "runResult":
      var w = waiting[m.id];
      delete waiting[m.id];
      if (m.error !== undefined) w.reject(new Error(m.error));
      else w.resolve(m.result);
      break;
    }
  }).catch(report);
};

// gscouterTime reports how long a stage of the lead took, for the page
// to record (design §4.7).
self.gscouterTime = function (name, ms) {
  postMessage({type: "time", name: name, ms: ms});
};

// gscouterRun is how the lead's Go code asks the page for a simulation:
// it resolves to the iterations as JSON, or rejects with the error of a
// candidate the engine rejected.
self.gscouterRun = function (cfg, warmup, seed, iterations) {
  return new Promise(function (resolve, reject) {
    var id = next++;
    waiting[id] = {resolve: resolve, reject: reject};
    postMessage({type: "run", id: id, cfg: cfg, warmup: warmup, seed: seed, iterations: iterations});
  });
};
