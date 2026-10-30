// A Node harness for the editor's draft and link (TestEditDraftEmpty): it
// fetches the editor from the test server, runs its script in a vm
// context with stand-ins for the browser and the engine's worker, deletes
// every note, and loads the page again from the address the script left,
// with the same localStorage. It prints what happened as JSON.
//
//	node edit_draft.js <server URL> <path> <ms>
//
// The notes are deleted <ms> after the page opens: from 600 on, the link
// has started to catch up. The worker records "0E 1Q 2E" and holds each
// encode until the harness lets it answer, so an answer can arrive after
// the chart has changed.
"use strict";
const vm = require("vm");
const zlib = require("zlib");

const [base, path, wait] = process.argv.slice(2);
const store = new Map();
const out = {loads: []};
const tick = () => new Promise((r) => setImmediate(r));

// encode is chart.Encode for a chart already in its text form.
const encode = (text) => zlib.deflateRawSync(Buffer.from(text), {level: 9}).toString("base64url");

function element() {
  const on = {};
  const el = {
    on, style: {setProperty() {}}, dataset: {}, classList: {add() {}, remove() {}, toggle() {}},
    options: {length: 1}, value: "", checked: false, textContent: "", innerHTML: "", hidden: false,
    scrollLeft: 0, scrollWidth: 0, clientWidth: 600,
    addEventListener(type, f) { (on[type] = on[type] || []).push(f); },
    setAttribute() {}, appendChild() {}, insertBefore() {}, remove() {}, focus() {},
    querySelector: () => element(), querySelectorAll: () => [], closest: () => null,
  };
  return el;
}

async function open(url) {
  const res = await fetch(base + url);
  const body = await res.text();
  const i = body.indexOf("var START = ");
  if (i < 0) throw new Error("no editor script: " + body.slice(0, 200));
  const from = body.lastIndexOf("<script>", i) + "<script>".length;
  const src = body.slice(from, body.indexOf("</script>", i));
  const u = new URL(res.url);

  const load = {records: 0, encodes: 0, search: ""};
  const timers = [];
  let now = 0, nextTimer = 1;
  const held = [];
  const ids = {};
  const on = {};
  const location = {pathname: u.pathname, search: u.search};
  class Worker {
    postMessage(m) {
      const reply = (r) => setImmediate(() => this.onmessage({data: r}));
      if (m.type === "init") return reply({type: "ready"});
      const [fn, args] = [m.fn, m.args];
      if (fn === "record") { load.records++; return reply({type: "call", id: m.id, result: "0E 1Q 2E"}); }
      if (fn === "encode") { load.encodes++; return held.push(() => reply({type: "call", id: m.id, result: encode(args[0])})); }
      reply({type: "call", id: m.id, error: "no preview here"});
    }
  }
  const page = {
    WebAssembly: {compileStreaming: () => Promise.resolve({})},
    Worker, URLSearchParams,
    fetch: () => Promise.resolve({}),
    localStorage: {
      getItem: (k) => (store.has(k) ? store.get(k) : null),
      setItem: (k, v) => { store.set(k, String(v)); },
      removeItem: (k) => { store.delete(k); },
    },
    location,
    history: {replaceState(_, __, to) { const v = new URL(to, base); location.pathname = v.pathname; location.search = v.search; }},
    setTimeout: (f, ms) => { const id = nextTimer++; timers.push({id, at: now + (ms || 0), f}); return id; },
    clearTimeout: (id) => { const j = timers.findIndex((t) => t.id === id); if (j >= 0) timers.splice(j, 1); },
    addEventListener(type, f) { (on[type] = on[type] || []).push(f); },
    document: {
      documentElement: element(), body: element(),
      getElementById: (id) => ids[id] || (ids[id] = element()),
      querySelector: () => null, querySelectorAll: () => [], createElement: () => element(),
    },
  };
  page.window = page;
  vm.createContext(page);
  vm.runInContext(src, page, {filename: "edit.js"});

  const settle = async () => { for (let k = 0; k < 20; k++) await tick(); };
  const advance = async (ms) => {
    now += ms;
    for (;;) {
      timers.sort((a, b) => a.at - b.at);
      if (!timers.length || timers[0].at > now) break;
      timers.shift().f();
      await settle();
    }
  };
  const fire = (target, type, e) => (target[type] || []).forEach((f) => f(e));
  const pick = (i, shiftKey) => {
    const note = {dataset: {i: String(i)}};
    fire(ids["ed-svg"].on, "pointerdown", {target: {closest: () => note}, clientX: 0, pointerId: 1});
    fire(on, "pointerup", {pointerId: 1, shiftKey});
  };
  await settle();
  return {
    load, held, settle, advance,
    chart: () => { for (const [k, v] of store) if (k.startsWith("gscouter.chart:")) return v; return null; },
    url: () => location.pathname + location.search,
    deleteAll: async (n) => {
      pick(0, false);
      if (n > 1) pick(n - 1, true);
      fire(on, "keydown", {code: "Delete", key: "Delete", target: {closest: () => null}, preventDefault() {}});
      await settle();
    },
  };
}

(async () => {
  // The chart as it opens, then after the link has caught up.
  const first = await open(path);
  const opened = first.chart();
  await first.advance(+wait);
  const inFlight = first.held.length;
  // Every note goes while any encode is still out; then it answers.
  await first.deleteAll(opened.split(/\s+/).filter(Boolean).length);
  first.held.splice(0).forEach((f) => f());
  await first.settle();
  await first.advance(5000);
  first.held.splice(0).forEach((f) => f());
  await first.settle();
  out.loads.push({opened, inFlight, records: first.load.records, chart: first.chart(), url: first.url()});

  const again = await open(first.url());
  await again.advance(5000);
  again.held.splice(0).forEach((f) => f());
  await again.settle();
  out.loads.push({opened: again.chart(), records: again.load.records, chart: again.chart(), url: again.url()});
  console.log(JSON.stringify(out));
})().catch((e) => { console.error(e.stack || String(e)); process.exit(1); });
