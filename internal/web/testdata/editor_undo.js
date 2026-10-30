// A Node harness for the chart editor's history (TestEditorUndo): it runs
// the editor page's script in a vm context, with a stand-in for the DOM
// and a worker whose recordings the harness finishes by hand, then plays
// edits, resets and undos and checks the chart after each. The chart is
// read through the score button, which encodes the chart as it stands.
// It prints {"errors": [...]}, empty when every check holds.
//
//	node editor_undo.js page.js
"use strict";
const fs = require("fs");
const vm = require("vm");

const pageSrc = fs.readFileSync(process.argv[2], "utf8");
const errors = [];
const flush = () => new Promise((r) => setImmediate(r));

// A stand-in element: enough of the DOM for the editor to run.
function element(id) {
  const listeners = {};
  return {
    id, style: {setProperty() {}}, dataset: {}, options: [], value: "", checked: false, hidden: false, disabled: false,
    textContent: "", innerHTML: "", className: "", scrollLeft: 0, clientWidth: 800, scrollWidth: 800,
    classList: {add() {}, remove() {}, toggle() {}, contains: () => false},
    setAttribute() {}, getAttribute: () => null, appendChild() {}, insertBefore() {}, remove() {}, focus() {},
    querySelector: () => null, querySelectorAll: () => [], closest: () => null,
    getBoundingClientRect: () => ({top: 0, height: 0}),
    addEventListener(type, f) { (listeners[type] = listeners[type] || []).push(f); },
    dispatch(type, e) { (listeners[type] || []).forEach((f) => f.call(this, e)); },
  };
}

function run(scenario) {
  const els = {}, win = element("window");
  const $ = (id) => els[id] || (els[id] = element(id));
  const records = [], encoded = [], toasts = [];
  let toast = null;
  // The engine: a recording waits for the harness; encode hands back
  // the chart it is given, so the score button shows the chart.
  const engine = {
    record: () => new Promise((resolve, reject) => records.push({resolve, reject})),
    preview: () => Promise.reject(new Error("no preview here")),
    encode: (text) => { encoded.push(text); return Promise.resolve(text); },
  };
  class Worker {
    postMessage(m) {
      setImmediate(() => {
        if (m.type === "init") return this.onmessage({data: {type: "ready"}});
        engine[m.fn](...m.args).then((result) => this.onmessage({data: {type: "call", id: m.id, result}}),
          (err) => this.onmessage({data: {type: "call", id: m.id, error: err.message}}));
      });
    }
  }
  const page = {
    Worker, Promise, JSON, String, Math, Date, Array, Object, URLSearchParams, Error, RegExp,
    WebAssembly: {compileStreaming: () => Promise.resolve({})},
    fetch: () => Promise.resolve({}),
    matchMedia: () => ({matches: false}),
    setTimeout: () => 0, clearTimeout() {},
    localStorage: {getItem: () => null, setItem() {}, removeItem() {}},
    location: {search: "", pathname: "/edit", href: ""},
    history: {replaceState() {}},
    addEventListener: win.addEventListener,
    document: {
      documentElement: element("html"), body: element("body"),
      getElementById: $,
      querySelectorAll: () => [],
      querySelector: (q) => (q === ".ed-toast" ? toast : null),
      createElement: () => {
        const t = element("toast");
        Object.defineProperty(t, "textContent", {set(v) { toasts.push(v); }, get() { return ""; }});
        return t;
      },
    },
  };
  page.document.body.appendChild = (t) => { toast = t; };
  page.window = page;
  vm.createContext(page);
  vm.runInContext(pageSrc, page, {filename: "page.js"});

  const key = (code, extra) => {
    const e = Object.assign({code, key: "", target: element("t"), preventDefault() {}}, extra);
    win.dispatch("keydown", e);
    if (!extra) win.dispatch("keyup", e);
  };
  const t = {
    records, toasts,
    async settle() { for (let i = 0; i < 10; i++) await flush(); },
    async chart() {
      $("ed-score").onclick();
      await t.settle();
      return encoded[encoded.length - 1];
    },
    async expect(what, want) {
      const got = await t.chart();
      if (got !== want) errors.push(`${scenario.name}: ${what}: chart ${JSON.stringify(got)}, want ${JSON.stringify(want)}`);
    },
    check(what, ok) { if (!ok) errors.push(`${scenario.name}: ${what}`); },
    attack() { key("KeyZ"); },
    undo() { key("KeyZ", {ctrlKey: true}); },
    reset() { $("ed-reset").onclick(); },
    pickup(on) { $("ed-pickup").checked = on; $("ed-pickup").dispatch("change", {}); },
    pickupShown: () => $("ed-pickup").checked,
  };
  return scenario.run(t);
}

const scenarios = [
  {name: "reset then undo", async run(t) {
    await t.settle();
    t.records.shift().resolve("1E 0N");
    await t.settle();
    t.attack();
    await t.expect("after an edit", "1E 0N 0N");
    t.reset();
    await t.settle();
    t.records.shift().resolve("2Q");
    await t.settle();
    await t.expect("after the reset", "2Q");
    t.undo();
    await t.expect("undoing the reset", "1E 0N 0N");
    t.undo();
    await t.expect("undoing the edit", "1E 0N");
    t.undo();
    await t.expect("with nothing to undo", "1E 0N");
    t.check("no word of nothing to undo", t.toasts.length === 1);
  }},
  {name: "blank then first note", async run(t) {
    await t.settle();
    t.records.shift().reject(new Error("no recording"));
    await t.settle();
    await t.expect("a failed recording", "");
    t.attack();
    await t.expect("the first note", "0N");
    t.undo();
    await t.expect("undoing the first note", "");
    t.check("undo found nothing to undo", t.toasts.length === 0);
  }},
  {name: "undo shows pickup", async run(t) {
    await t.settle();
    t.records.shift().resolve("0N");
    await t.settle();
    t.pickup(true);
    await t.expect("pickup on", "0N ; pickup");
    t.undo();
    await t.expect("undoing pickup", "0N");
    t.check("the pickup switch still on after undo", !t.pickupShown());
  }},
  {name: "failed reset", async run(t) {
    await t.settle();
    t.records.shift().resolve("1E");
    await t.settle();
    t.reset();
    await t.settle();
    t.records.shift().reject(new Error("no recording"));
    await t.settle();
    await t.expect("after the failed reset", "");
    t.undo();
    await t.expect("undoing the failed reset", "1E");
  }},
  {name: "superseded reset", async run(t) {
    await t.settle();
    t.records.shift().resolve("1E");
    await t.settle();
    t.reset();
    await t.settle();
    t.reset();
    await t.settle();
    const [first, second] = t.records.splice(0);
    second.resolve("3E");
    await t.settle();
    first.resolve("2Q");
    await t.settle();
    await t.expect("the later reset wins", "3E");
    t.undo();
    await t.expect("undoing the reset", "1E");
    t.undo();
    t.check("the double reset left two history entries", t.toasts.length === 1);
  }},
  {name: "undo while recording", async run(t) {
    await t.settle();
    t.records.shift().resolve("1E");
    await t.settle();
    t.attack();
    t.reset();
    await t.settle();
    t.undo();
    t.records.shift().resolve("2Q");
    await t.settle();
    await t.expect("the reset taken back", "1E 1N");
    t.undo();
    await t.expect("then the edit", "1E");
  }},
];

(async () => {
  for (const s of scenarios) {
    try { await run(s); } catch (e) { errors.push(`${s.name}: ${e.stack}`); }
  }
  console.log(JSON.stringify({errors}));
})();
