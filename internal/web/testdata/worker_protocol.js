// A Node harness for the browser calculation's worker protocol
// (TestWorkerProtocol): it runs a result page's client script and the
// real static/worker.js in vm contexts, with stand-ins for the browser
// and for cmd/wasm. The lead asks for a candidate the engine rejects,
// then for one it accepts, as a search does; the harness prints what
// happened as JSON.
//
//	node worker_protocol.js worker.js page.js
"use strict";
const fs = require("fs");
const vm = require("vm");

const [workerSrc, pageSrc] = process.argv.slice(2).map((p) => fs.readFileSync(p, "utf8"));
const out = {fallback: null, body: null, lead: [], helper: [], saved: [], errors: []};
const later = (f) => setImmediate(f);
const clone = (m) => structuredClone(m);

// cmd/wasm, as each worker sees it once instantiated.
function engine(self, lead) {
  self.gscouter = {
    runSlice(cfg, warmup, seed, first, count) {
      out.helper.push({cfg, first, count});
      if (cfg === "rejected") return {error: "candidate rotation rejected"};
      const its = [];
      for (let i = 0; i < count; i++) its.push({i: first + i});
      return JSON.stringify(its);
    },
    // The search: one candidate fails, the next must still be scored.
    async runJob() {
      if (!lead) throw new Error("job on a helper");
      for (const cfg of ["rejected", "accepted"]) {
        try {
          const its = JSON.parse(await self.gscouterRun(cfg, 0, 1, 10));
          out.lead.push({cfg, iterations: its.length, distinct: new Set(its.map((x) => x.i)).size});
        } catch (e) {
          out.lead.push({cfg, error: e.message});
        }
      }
      return {body: "scored", title: "t"};
    },
  };
}

let spawned = 0;
class Worker {
  constructor() {
    const lead = spawned++ === 0, page = this;
    const self = {
      location: {search: ""},
      importScripts() {},
      postMessage: (m) => later(() => page.onmessage && page.onmessage({data: clone(m)})),
      WebAssembly: {instantiate: () => Promise.resolve({})},
      Go: class {
        constructor() { this.importObject = {}; }
        run() { engine(self, lead); }
      },
      Error, Promise, JSON, String, setTimeout,
    };
    self.self = self;
    vm.createContext(self);
    vm.runInContext(workerSrc, self, {filename: "worker.js"});
    this.self = self;
  }
  postMessage(m) { later(() => this.self.onmessage({data: clone(m)})); }
  terminate() { this.self.onmessage = () => {}; }
}

const store = new Map();
const sessionStorage = {
  getItem: (k) => (store.has(k) ? store.get(k) : null),
  setItem: (k, v) => { store.set(k, String(v)); out.saved.push(k.slice(k.indexOf("\n") + 1)); },
  removeItem: (k) => store.delete(k),
  key: (i) => [...store.keys()][i],
  get length() { return store.size; },
};
const page = {
  WebAssembly: {compileStreaming: () => Promise.resolve({})},
  Worker, Promise, sessionStorage,
  fetch: () => Promise.resolve({ok: true}),
  navigator: {hardwareConcurrency: 2},
  performance: {now: () => 0, measure() {}},
  location: {replace: (u) => { out.fallback = u; }},
  document: {
    querySelectorAll: () => [],
    getElementById: () => ({set innerHTML(v) { out.body = v; }}),
  },
  console: {error: (...a) => out.errors.push(a.map(String).join(" "))},
};
page.window = page;
vm.createContext(page);
vm.runInContext(pageSrc, page, {filename: "page.js"});

setTimeout(() => { console.log(JSON.stringify(out)); }, 500);
