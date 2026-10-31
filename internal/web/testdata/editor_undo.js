// A Node harness for the chart editor's history (TestEditorUndo): it runs
// the editor page's script in a vm context, with a stand-in for the DOM
// and a worker whose recordings the harness finishes by hand, then plays
// edits, resets and undos and checks the chart after each. The chart is
// read through the score button, which encodes the chart as it stands.
// It prints {"errors": [...]}, empty when every check holds.
//
// More pages, each a team with Skirk or Mavuika and that character's
// slot, run the scenarios of their params sheet (page: "skirk" or
// "mavuika"), which open the sheet and move its sliders; a scenario's
// defaults replace the team's params the page has. A scenario with a
// preview has the engine answer previews with it, and runs the page's
// timers when it asks (t.timers()).
//
//	node editor_undo.js page.js [skirk.js slot [mavuika.js slot]]
"use strict";
const fs = require("fs");
const vm = require("vm");

const pageSrc = fs.readFileSync(process.argv[2], "utf8");
const skirkSrc = process.argv[3] ? fs.readFileSync(process.argv[3], "utf8") : null;
const S = process.argv[4];
const mavuikaSrc = process.argv[5] ? fs.readFileSync(process.argv[5], "utf8") : null;
const M = process.argv[6];
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
  const records = [], encoded = [], toasts = [], timers = [];
  // localStorage, which a scenario may fill before the page runs: saved
  // charts go under the editor's saved-charts key (gscouter.charts:...),
  // and the draft is read back from gscouter.chart:....
  const store = new Map();
  const saved = scenario.saved;
  let toast = null;
  // The engine: a recording waits for the harness; encode hands back
  // the chart it is given, so the score button shows the chart.
  const engine = {
    record: () => new Promise((resolve, reject) => records.push({resolve, reject})),
    preview: () => scenario.preview ? Promise.resolve(JSON.stringify(scenario.preview)) : Promise.reject(new Error("no preview here")),
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
    setTimeout: (f) => { if (scenario.preview) timers.push(f); return 0; }, clearTimeout() {},
    localStorage: {
      getItem: (k) => {
        if (saved && !store.has(k) && k.startsWith("gscouter.charts:")) store.set(k, JSON.stringify(saved));
        return store.has(k) ? store.get(k) : null;
      },
      setItem: (k, v) => { store.set(k, String(v)); },
      removeItem: (k) => { store.delete(k); },
    },
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
  // The params sheet: its inputs are read back from the HTML openSheet
  // writes, by param key.
  const inputs = {};
  $("ed-layer").querySelectorAll = function (q) {
    if (q !== "input") return [];
    for (const k of Object.keys(inputs)) delete inputs[k];
    const re = /<input type="(\w+)" id="([^"]*)" data-k="([^"]*)"([^>]*)>/g;
    let m;
    while ((m = re.exec(this.innerHTML))) {
      const inp = element(m[2]), v = /value="([^"]*)"/.exec(m[4]), max = / max="([^"]*)"/.exec(m[4]);
      Object.assign(inp, {type: m[1], value: v ? v[1] : "", max: max ? max[1] : "", checked: / checked/.test(m[4]), closest: () => ({querySelector: () => null})});
      inp.dataset.k = m[3];
      inputs[m[3]] = inp;
    }
    return Object.values(inputs);
  };
  let src = scenario.page === "skirk" ? skirkSrc : scenario.page === "mavuika" ? mavuikaSrc : pageSrc;
  if (scenario.defaults) {
    const re = /DEFAULTS = JSON\.stringify\(\[[^\]]*\]\)/;
    if (!re.test(src)) throw new Error("no DEFAULTS in the page");
    src = src.replace(re, "DEFAULTS = JSON.stringify(" + JSON.stringify(scenario.defaults) + ")");
  }
  vm.createContext(page);
  vm.runInContext(src, page, {filename: "page.js"});

  const key = (code, extra) => {
    const e = Object.assign({code, key: "", target: element("t"), preventDefault() {}}, extra);
    win.dispatch("keydown", e);
    if (!extra) win.dispatch("keyup", e);
  };
  const t = {
    records, toasts,
    async settle() { for (let i = 0; i < 10; i++) await flush(); },
    // timers runs the page's timers set so far: the preview among them.
    async timers() { timers.splice(0).forEach((f) => f()); await t.settle(); },
    // sheet is the params sheet's HTML, as openSheet left it.
    sheet: () => $("ed-layer").innerHTML,
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
    choose(v) { $("ed-saved").value = v; $("ed-saved").onchange.call($("ed-saved")); },
    // openSheet opens slot s's params sheet and gives its inputs by key.
    openSheet(s) {
      $("ed-labels").dispatch("click", {target: {closest: () => ({dataset: {p: String(s)}})}});
      return Object.assign({}, inputs);
    },
    slide(inp, v) { inp.value = String(v); inp.dispatch("input", {}); inp.dispatch("change", {}); },
    draft() { for (const [k, v] of store) if (k.startsWith("gscouter.chart:")) return v; return null; },
    async expectDraft(what, want) {
      await t.settle();
      const got = t.draft();
      if (got !== want) errors.push(`${scenario.name}: ${what}: draft ${JSON.stringify(got)}, want ${JSON.stringify(want)}`);
    },
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
  // A reset still recording when a note is added: the edit is newer,
  // so the reset's late answer, recorded or failed, is dropped.
  {name: "edit during a reset", async run(t) {
    await t.settle();
    t.records.shift().resolve("1E");
    await t.settle();
    t.reset();
    await t.settle();
    t.attack();
    await t.expect("the edit", "1E 1N");
    t.records.shift().resolve("2Q");
    await t.settle();
    await t.expect("after the late reset", "1E 1N");
    await t.expectDraft("the draft after the late reset", "1E 1N");
    t.undo();
    await t.expect("undoing the edit", "1E");
  }},
  {name: "edit during a failed reset", async run(t) {
    await t.settle();
    t.records.shift().resolve("1E");
    await t.settle();
    t.reset();
    await t.settle();
    t.attack();
    await t.expect("the edit", "1E 1N");
    t.records.shift().reject(new Error("no recording"));
    await t.settle();
    await t.expect("after the late failure", "1E 1N");
    await t.expectDraft("the draft after the late failure", "1E 1N");
  }},
  {name: "reset without an edit", async run(t) {
    await t.settle();
    t.records.shift().resolve("1E");
    await t.settle();
    t.reset();
    await t.settle();
    t.records.shift().resolve("2Q");
    await t.settle();
    await t.expect("after the reset", "2Q");
    await t.expectDraft("the draft after the reset", "2Q");
  }},
  // The saved-chart list's placeholder (value "") is no chart; it does
  // not load the first saved one.
  {name: "saved-chart placeholder", saved: [{n: "first", t: "0N 1E", d: "2026-01-01"}], async run(t) {
    await t.settle();
    t.records.shift().resolve("2Q 3E");
    await t.settle();
    t.choose("");
    await t.expect("choosing the placeholder", "2Q 3E");
    await t.expectDraft("the draft after the placeholder", "2Q 3E");
    t.choose("0");
    await t.expect("choosing the saved chart", "0N 1E");
    t.choose("");
    await t.expect("back to the placeholder", "0N 1E");
    t.choose("7");
    await t.expect("an index past the list", "0N 1E");
    t.undo();
    await t.expect("undoing the saved chart", "2Q 3E");
  }},
];

// Skirk's sheet has one control for her starting Serpent's Subtlety,
// start_energy, which shows what gcsim starts her at: the chart's
// start_energy, else its legacy start_serpents_subtlety, else the team's
// (legacy or not), else 100. Moving it writes start_energy alone.
const legacy = "start_serpents_subtlety";
function skirkSheet(t, what, want) {
  const inputs = t.openSheet(S);
  t.check(`${what}: a legacy control is shown`, !(legacy in inputs));
  const e = inputs.start_energy;
  t.check(`${what}: no start_energy slider`, e && e.type === "range");
  if (e) t.check(`${what}: start_energy shows ${e.value}, want ${want}`, e.value === String(want));
  return inputs;
}
const blank = ["", "", "", ""];
const withDefault = (d) => blank.map((x, i) => (String(i) === S ? d : x));
scenarios.push(
  {name: "skirk: neither key", page: "skirk", defaults: blank, async run(t) {
    await t.settle();
    t.records.shift().resolve(`${S}E`);
    await t.settle();
    skirkSheet(t, "no params", 100);
  }},
  {name: "skirk: the library's legacy key", page: "skirk", defaults: withDefault(`${legacy}=0`), async run(t) {
    await t.settle();
    t.records.shift().resolve(`${S}E`);
    await t.settle();
    const inputs = skirkSheet(t, "the legacy default", 0);
    t.slide(inputs.start_energy, 60);
    await t.expect("after the slider", `${S}E ; p${S} start_energy=60`);
  }},
  {name: "skirk: the chart's start_energy over the legacy default", page: "skirk", defaults: withDefault(`${legacy}=0`), async run(t) {
    await t.settle();
    t.records.shift().resolve(`${S}E ; p${S} start_energy=40`);
    await t.settle();
    skirkSheet(t, "the chart's start_energy", 40);
  }},
  {name: "skirk: an older link with the legacy key", page: "skirk", saved: [{n: "old", t: `${S}E ; p${S} ${legacy}=20`, d: "2026-01-01"}], async run(t) {
    await t.settle();
    t.records.shift().resolve(`${S}E`);
    await t.settle();
    t.choose("0");
    await t.expect("the legacy key folded", `${S}E ; p${S} start_energy=20`);
    skirkSheet(t, "the chart's legacy key", 20);
  }},
  {name: "skirk: editing after both keys", page: "skirk", saved: [{n: "both", t: `${S}E ; p${S} start_energy=20 ${legacy}=100 start_hp%=50`, d: "2026-01-01"}], async run(t) {
    await t.settle();
    t.records.shift().resolve(`${S}E`);
    await t.settle();
    t.choose("0");
    await t.expect("start_energy kept, the legacy key dropped", `${S}E ; p${S} start_energy=20 start_hp%=50`);
    const inputs = skirkSheet(t, "both keys", 20);
    t.slide(inputs.start_energy, 70);
    await t.expect("after the slider", `${S}E ; p${S} start_hp%=50 start_energy=70`);
  }},
);

// The starting HP has one control, start_hp% (chart.HPKeys), on the
// first page, whose slot 0 is Hu Tao: it shows the HP a team's start_hp
// gives, as a share of max HP once a preview has told it max HP, and a
// move of it writes start_hp% alone. Starting energy runs to Hu Tao's
// burst cost, 60.
const lanes = (hp) => [0, 1, 2, 3].map((s) => ({hp: s === 0 ? hp : [0, 0]}));
const preview = (hp) => ({notes: [], passes: [0], end: 10, dps: 1, lanes: lanes(hp)});
const hutao = ["start_hp=10000", "", "", ""];
function hpSheet(t, what, want, text) {
  const inputs = t.openSheet(0);
  t.check(`${what}: a start_hp field is shown`, !("start_hp" in inputs));
  const hp = inputs["start_hp%"];
  t.check(`${what}: no start_hp% slider`, hp && hp.type === "range");
  if (hp) t.check(`${what}: start_hp% shows ${hp.value}, want ${want}`, hp.value === String(want));
  if (text) t.check(`${what}: the sheet does not say ${JSON.stringify(text)}`, t.sheet().includes(text));
  return inputs;
}
scenarios.push(
  {name: "hp: no HP keys", defaults: ["", "", "", ""], async run(t) {
    await t.settle();
    t.records.shift().resolve("0E");
    await t.settle();
    hpSheet(t, "no params", 100, "<b>100%</b>");
  }},
  {name: "hp: the team's start_hp before a preview", defaults: hutao, async run(t) {
    await t.settle();
    t.records.shift().resolve("0E");
    await t.settle();
    hpSheet(t, "no max HP yet", 100, "<b>10000 HP</b>");
  }},
  {name: "hp: the team's start_hp as a share", defaults: hutao, preview: preview([10000, 36662.23]), async run(t) {
    await t.settle();
    t.records.shift().resolve("0E");
    await t.settle();
    await t.timers();
    const inputs = hpSheet(t, "the team's start_hp", 27, "<b>27% (10000 HP)</b>");
    t.check("the default line does not give the team's start_hp", t.sheet().includes("Default: 27% (10000 HP)"));
    t.slide(inputs["start_hp%"], 50);
    await t.expect("after the slider", "0E ; p0 start_hp%=50");
    hpSheet(t, "after the slider", 50, "<b>50%</b>");
  }},
  {name: "hp: an older link with both keys", defaults: hutao, preview: preview([19331.12, 36662.23]), saved: [{n: "both", t: "0E ; p0 start_hp%=50 start_hp=1000", d: "2026-01-01"}], async run(t) {
    await t.settle();
    t.records.shift().resolve("0E");
    await t.settle();
    t.choose("0");
    await t.timers();
    await t.expect("the link as it was", "0E ; p0 start_hp%=50 start_hp=1000");
    const inputs = hpSheet(t, "both keys", 53, "<b>53% (50% + 1000 HP)</b>");
    t.slide(inputs["start_hp%"], 40);
    await t.expect("after the slider, start_hp dropped", "0E ; p0 start_hp%=40");
  }},
  {name: "hp: an older link with start_hp", defaults: hutao, preview: preview([5000, 36662.23]), saved: [{n: "abs", t: "0E ; p0 start_hp=5000", d: "2026-01-01"}], async run(t) {
    await t.settle();
    t.records.shift().resolve("0E");
    await t.settle();
    t.choose("0");
    await t.timers();
    await t.expect("the link as it was", "0E ; p0 start_hp=5000");
    hpSheet(t, "the link's start_hp", 14, "<b>14% (5000 HP)</b>");
  }},
  {name: "energy: up to the burst's cost", defaults: ["start_energy=80", "", "", ""], async run(t) {
    await t.settle();
    t.records.shift().resolve("0E");
    await t.settle();
    const e = t.openSheet(0).start_energy;
    t.check("no start_energy slider", e && e.type === "range");
    if (!e) return;
    t.check(`start_energy runs to ${e.max}, want 60`, e.max === "60");
    t.check(`start_energy shows ${e.value}, want 60, what gcsim cuts 80 to`, e.value === "60");
    await t.expect("the team's value kept", "0E");
    const blank = t.openSheet(1).start_energy;
    t.check(`Xingqiu's start_energy runs to ${blank && blank.max}, want 80`, blank && blank.max === "80" && blank.value === "80");
  }},
);

// Mavuika's start_energy is her Fighting Spirit: 0 to 200, 200 with no
// key (chart.StartResource).
scenarios.push(
  {name: "mavuika: no key", page: "mavuika", defaults: blank, async run(t) {
    await t.settle();
    t.records.shift().resolve(`${M}E`);
    await t.settle();
    const e = t.openSheet(M).start_energy;
    t.check("no start_energy slider", e && e.type === "range");
    if (!e) return;
    t.check(`start_energy runs to ${e.max}, want 200`, e.max === "200");
    t.check(`start_energy shows ${e.value}, want 200`, e.value === "200");
    t.check("the control is not named Fighting Spirit", t.sheet().includes("Starting Fighting Spirit"));
    t.slide(e, 150);
    await t.expect("after the slider", `${M}E ; p${M} start_energy=150`);
  }},
  {name: "mavuika: a link at 200", page: "mavuika", saved: [{n: "full", t: `${M}E ; p${M} start_energy=200`, d: "2026-01-01"}], async run(t) {
    await t.settle();
    t.records.shift().resolve(`${M}E`);
    await t.settle();
    t.choose("0");
    await t.expect("the link as it was", `${M}E ; p${M} start_energy=200`);
    const e = t.openSheet(M).start_energy;
    t.check(`start_energy shows ${e && e.value}, want 200`, e && e.value === "200");
  }},
);

(async () => {
  for (const s of scenarios) {
    if ((s.page === "skirk" && !skirkSrc) || (s.page === "mavuika" && !mavuikaSrc)) continue;
    try { await run(s); } catch (e) { errors.push(`${s.name}: ${e.stack}`); }
  }
  console.log(JSON.stringify({errors}));
})();
