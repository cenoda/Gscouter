// A Node harness for the front page's team picker (TestPickerTraveler):
// it runs the showcase script in a vm context, with a stand-in for the
// DOM holding one checkbox per character, then picks teams and checks
// what the picker allows. The Traveler's elements are one character, so
// two of them must not reach the score page or the rotation editor.
// It prints {"errors": [...]}, empty when every check holds.
//
//	node picker_traveler.js page.js '{"keys": [...], "buildable": [...]}'
"use strict";
const fs = require("fs");
const vm = require("vm");

const pageSrc = fs.readFileSync(process.argv[2], "utf8");
const {keys, buildable} = JSON.parse(process.argv[3]);
const errors = [];

function element(id, attrs) {
  const listeners = {};
  attrs = attrs || {};
  return {
    id, hidden: true, disabled: false, checked: false, textContent: "", value: "",
    getAttribute: (k) => (k in attrs ? attrs[k] : null),
    setAttribute: (k, v) => { attrs[k] = String(v); },
    removeAttribute: (k) => { delete attrs[k]; },
    hasAttribute: (k) => k in attrs,
    appendChild() {}, remove() {},
    querySelector: () => null, querySelectorAll: () => [],
    addEventListener(type, f) { (listeners[type] = listeners[type] || []).push(f); },
    dispatch(type) { (listeners[type] || []).forEach((f) => f.call(this, {})); },
  };
}

const boxes = keys.map((k) => {
  const b = element("c-" + k, {"data-build": "token-" + k});
  b.value = k;
  return b;
});
const form = element("picker", {action: "/score", "data-buildable": JSON.stringify(buildable)});
form.querySelectorAll = (q) => {
  if (q === "input[name=c]") return boxes;
  if (q === "input[name=c]:checked") return boxes.filter((b) => b.checked);
  return [];
};
const card = element("showcase");
card.querySelectorAll = (q) => (q === "input[data-build]" ? boxes : []);
card.querySelector = (q) => {
  const m = /value='([^']*)'/.exec(q);
  return m ? boxes.find((b) => b.value === m[1]) || null : null;
};
const els = {showcase: card, picker: form, recent: element("recent")};
for (const id of ["pick-submit", "pick-count", "pick-unsupported", "pick-custom", "pick-traveler", "pick-clear"]) els[id] = element(id);

const store = new Map();
const page = {
  JSON, Array, Object, Date, Math, String, Number, RegExp, URL, URLSearchParams,
  location: {search: "", reload() {}},
  localStorage: {getItem: (k) => (store.has(k) ? store.get(k) : null), setItem: (k, v) => store.set(k, String(v))},
  setInterval: () => 0, clearInterval() {},
  console,
  document: {getElementById: (id) => els[id] || null, createElement: () => element("new")},
};
page.window = page;
vm.createContext(page);
vm.runInContext(pageSrc, page);

function pick(chars) {
  els["pick-clear"].dispatch("click");
  for (const k of chars) {
    const b = boxes.find((x) => x.value === k);
    if (!b) { errors.push("no checkbox for " + k); return null; }
    b.checked = true;
    b.dispatch("change");
  }
  return {
    submit: !els["pick-submit"].disabled,
    traveler: !els["pick-traveler"].hidden,
    unsupported: !els["pick-unsupported"].hidden,
    custom: !els["pick-custom"].hidden,
    action: form.getAttribute("action"),
  };
}

function expect(chars, want) {
  const got = pick(chars);
  if (!got) return;
  for (const k of Object.keys(want)) {
    if (got[k] !== want[k]) errors.push(chars.join(",") + ": " + k + " = " + got[k] + ", want " + want[k]);
  }
}

// Two elements of the Traveler, or both genders: told why, and nowhere to go.
expect(["hutao", "xingqiu", "aetheranemo", "aethergeo"], {submit: false, traveler: true, unsupported: false, custom: false, action: "/score"});
expect(["hutao", "xingqiu", "aetheranemo", "luminecryo"], {submit: false, traveler: true, unsupported: false, custom: false, action: "/score"});
// Told as soon as the second one is picked.
expect(["aetheranemo", "aethergeo"], {submit: false, traveler: true});
// One Traveler is a team like any other: to the editor, or scored.
expect(["hutao", "xingqiu", "yelan", "aetheranemo"], {submit: true, traveler: false});
expect(["hutao", "xingqiu", "yelan", "aethergeo"], {submit: true, traveler: false, unsupported: false});
expect(["hutao", "xingqiu", "yelan"], {submit: false, traveler: false, unsupported: false, custom: false});

process.stdout.write(JSON.stringify({errors}));
