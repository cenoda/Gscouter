// A worker running the browser build of Gscouter (cmd/wasm) for a result
// page (the "client" template). The lead calculates the page and asks
// the page for simulations; helpers run a share of their iterations, on
// gcsim or, for a team that runs on it, on gscouter-engine's module
// (internal/engine). The chart editor (edit.html) runs one worker and
// calls it directly.
importScripts("/static/wasm_exec.js" + self.location.search);

var ready, waiting = {}, next = 0, gse = null, gseModule = null;

// engineSlice runs gse_run_slice (gscouter-engine's wasm/lib.rs): the
// config goes in through gse_alloc, and the result comes back as a u32
// status (0 ok), a u32 length and that many bytes of UTF-8, the same
// JSON as gscouter.runSlice. It returns {result} or {error}.
function engineSlice(cfg, warmup, seed, first, count) {
  var x = gse.exports, bytes = new TextEncoder().encode(cfg);
  var p = x.gse_alloc(bytes.length) >>> 0;
  new Uint8Array(x.memory.buffer, p, bytes.length).set(bytes);
  var r = x.gse_run_slice(p, bytes.length, warmup, BigInt(seed), first, count) >>> 0;
  var head = new DataView(x.memory.buffer, r, 8);
  var status = head.getUint32(0, true), len = head.getUint32(4, true);
  var text = new TextDecoder().decode(new Uint8Array(x.memory.buffer, r + 8, len));
  x.gse_free_result(r);
  x.gse_free(p, bytes.length);
  return status === 0 ? {result: text} : {error: text};
}

function report(err) {
  postMessage({type: "error", error: String(err && err.message || err)});
}

self.onmessage = function (e) {
  var m = e.data;
  if (m.type === "init") {
    var go = new Go();
    ready = WebAssembly.instantiate(m.module, go.importObject).then(function (inst) { go.run(inst); });
    // The engine's module, for a page with a team on it. It imports
    // nothing.
    if (m.engine) {
      gseModule = m.engine;
      ready = ready.then(function () {
        return WebAssembly.instantiate(m.engine, {}).then(function (inst) { gse = inst; });
      });
    }
    ready.then(function () { postMessage({type: "ready"}); }, report);
    return;
  }
  ready.then(function () {
    switch (m.type) {
    case "slice":
      // A candidate gcsim rejects fails its run, not the page: the
      // error goes back with the run, for the lead's search to go on.
      // One the engine refuses goes back as engineError, for the page
      // to run all of it on gcsim.
      var head = {type: "slice", run: m.run, gen: m.gen, index: m.index, count: m.count};
      if (m.engine) {
        var e;
        try {
          e = gse ? engineSlice(m.cfg, m.warmup, m.seed, m.first, m.count) : {error: "engine not loaded"};
        } catch (err) {
          // A trap (a panic in the engine aborts it): the instance's
          // memory may be left half-written, so it is dropped and a new
          // one made for the next slices; this run goes to gcsim.
          e = {error: String(err && err.message || err)};
          gse = null;
          ready = ready.then(function () {
            return WebAssembly.instantiate(gseModule, {}).then(function (inst) { gse = inst; });
          });
        }
        if (e.result !== undefined) head.result = e.result;
        else head.engineError = e.error;
        postMessage(head);
        break;
      }
      var out = gscouter.runSlice(m.cfg, m.warmup, m.seed, m.first, m.count);
      if (typeof out === "string") head.result = out;
      else head.error = String(out.error);
      postMessage(head);
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
// With engine set, the team runs on gscouter-engine.
self.gscouterRun = function (cfg, warmup, seed, iterations, engine) {
  return new Promise(function (resolve, reject) {
    var id = next++;
    waiting[id] = {resolve: resolve, reject: reject};
    postMessage({type: "run", id: id, cfg: cfg, warmup: warmup, seed: seed, iterations: iterations, engine: !!engine});
  });
};
