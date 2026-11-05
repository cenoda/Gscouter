#!/usr/bin/env node
// Exercise the shipped ABI and real WASM fights without a server or JS packages.
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {readFile} from 'node:fs/promises';
import path from 'node:path';

const assets = path.resolve(process.argv[2] || 'lab-static');
const optimizer = path.resolve(process.argv[3] || '../gscouter-optimizer-browser');
const manifest = JSON.parse(await readFile(path.join(assets, 'manifest.json'), 'utf8'));
const binary = await readFile(path.join(assets, 'gso.wasm'));
const dictionaryBytes = await readFile(path.join(assets, 'dictionary.json'));
const hash = b => createHash('sha256').update(b).digest('hex');
assert.equal(hash(binary), manifest.wasm_sha256);
assert.equal(hash(dictionaryBytes), manifest.dictionary_sha256);
const dictionary = JSON.parse(dictionaryBytes);
const fixture = path.join(optimizer, 'reports/heldout-recovery-2026-11-05/inputs/bennett-fischl-lisa-xingqiu--z89bwQG6KGHg');
const build = JSON.parse(await readFile(path.join(fixture, 'manifest.json'), 'utf8')).fixed_build;
const rotation = await readFile(path.join(fixture, 'L.gcsl'), 'utf8');
const {instance} = await WebAssembly.instantiate(binary, {});
const wasm = instance.exports;
const encoder = new TextEncoder(), decoder = new TextDecoder();

function call(request) {
  const bytes = encoder.encode(JSON.stringify(request));
  const ptr = wasm.gso_alloc(bytes.byteLength) >>> 0;
  new Uint8Array(wasm.memory.buffer, ptr, bytes.byteLength).set(bytes);
  let result;
  try { result = wasm.gso_call(ptr, bytes.byteLength) >>> 0; }
  finally { wasm.gso_free(ptr, bytes.byteLength); }
  try {
    const header = new DataView(wasm.memory.buffer, result, 8);
    const status = header.getUint32(0, true), len = header.getUint32(4, true);
    const value = JSON.parse(decoder.decode(new Uint8Array(wasm.memory.buffer, result + 8, len)));
    if (status !== 0) throw new Error(value.error || JSON.stringify(value));
    return value;
  } finally { wasm.gso_free_result(result); }
}

const input = {build, rotation, dictionary, seed: 11, max_evaluations: 256, audit_samples: 16, scoring: manifest.scoring};
let report = call({op: 'start', input});
assert.equal(report.engine, manifest.engine.commit);
assert.equal(report.complete, false);
let previous = 0, steps = 0;
const phases = new Set();
const started = performance.now();
while (!report.complete) {
  report = call({op: 'step'});
  assert.equal(report.engine, manifest.engine.commit);
  assert.ok(report.evaluations >= previous);
  assert.ok(report.evaluations - previous <= 1, 'a step must run at most one fight');
  previous = report.evaluations;
  phases.add(report.phase);
  assert.ok(++steps < 2000, 'incremental search must complete');
}
assert.equal(report.audit_error, null);
assert.equal(report.baseline.n, 16);
assert.equal(report.selected.n, 16);
assert.ok(report.baseline.mean > 0 && report.selected.mean > 0);
assert.ok(report.decisions > 0, 'must actually search, not only score the baseline');
assert.ok(report.gcsl.includes('bennett') && report.gcsl.includes('options'));
assert.ok(report.search_evaluations <= input.max_evaluations);
assert.ok(report.audit_evaluations <= 2 * input.audit_samples);
assert.deepEqual(call({op: 'status'}), report);
assert.deepEqual(call({op: 'step'}), report, 'completed session must not evaluate more fights');

// A user-requested finish retains a valid result and runs the explicit audit.
call({op: 'start', input});
let stopped = call({op: 'finish'});
while (!stopped.complete) stopped = call({op: 'step'});
assert.equal(stopped.stop_reason, 'requested');
assert.equal(stopped.search_evaluations, 0);
assert.equal(stopped.baseline.mean, stopped.selected.mean);
assert.equal(stopped.improvement.mean, 0);
console.log(JSON.stringify({
  engine: report.engine, phases: [...phases], steps,
  evaluations: report.evaluations, decisions: report.decisions,
  accepted: report.accepted, promotions: report.promotions,
  baseline: report.baseline, selected: report.selected,
  elapsed_ms: Math.round(performance.now() - started), finish_audit: 'passed',
}, null, 2));
