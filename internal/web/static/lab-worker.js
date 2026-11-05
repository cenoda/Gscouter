/* One independent WASM instance per run. A main-thread watchdog can terminate
 * this worker even when GCSL never returns. No build or result is uploaded. */
"use strict";
let runtime, manifest, team, finishing = false, scheduled = false, done = false;
const utf8 = new TextEncoder(), decode = new TextDecoder("utf-8", {fatal: true});

function call(command) {
  const bytes = utf8.encode(JSON.stringify(command));
  const ptr = runtime.gso_alloc(bytes.length);
  let result;
  try {
    new Uint8Array(runtime.memory.buffer, ptr, bytes.length).set(bytes);
    result = runtime.gso_call(ptr, bytes.length);
    const view = new DataView(runtime.memory.buffer);
    const status = view.getUint32(result, true), length = view.getUint32(result + 4, true);
    const raw = decode.decode(new Uint8Array(runtime.memory.buffer, result + 8, length));
    const value = JSON.parse(raw);
    if (status !== 0) throw new Error(typeof value === "string" ? value : value.error || raw);
    if (value.engine !== manifest.engine.commit) throw new Error("WASM 엔진 버전이 배포 정보와 다릅니다. 페이지를 새로 불러와 주세요.");
    return value;
  } finally {
    if (result !== undefined) runtime.gso_free_result(result);
    runtime.gso_free(ptr, bytes.length);
  }
}

async function asset(url, expectedHash) {
  const response = await fetch(url, {cache: "no-store"});
  if (!response.ok) throw new Error("추론 파일을 불러오지 못했습니다: " + response.status);
  const bytes = await response.arrayBuffer();
  if (expectedHash) {
    const hash = Array.from(new Uint8Array(await crypto.subtle.digest("SHA-256", bytes)), b => b.toString(16).padStart(2,"0")).join("");
    if (hash !== expectedHash) throw new Error("추론 파일의 버전 확인에 실패했습니다. 페이지를 새로 불러와 주세요.");
  }
  return bytes;
}

function report(command) {
  self.postMessage({type: "busy", operation: command.op});
  const value = call(command);
  value.team = team;
  if (value.complete) value.provenance = manifest;
  self.postMessage({type: "progress", report: value});
  done = value.complete;
  return value;
}

function tick() {
  scheduled = false;
  if (done) return;
  try {
    if (finishing) { finishing = false; report({op: "finish"}); }
    if (!done) report({op: "step"});
    if (!done) { scheduled = true; setTimeout(tick, 0); }
  } catch (err) {
    done = true;
    self.postMessage({type: "error", error: err.message || String(err)});
  }
}

self.onmessage = async function (event) {
  const message = event.data;
  if (message.type === "finish") { finishing = true; return; }
  if (message.type !== "start" || runtime || scheduled) return;
  try {
    self.postMessage({type: "loading"});
    const info = message.input;
    team = {id: info.team_id, name: info.team_name};
    const manifestBytes = await asset(info.manifest_url);
    manifest = JSON.parse(decode.decode(manifestBytes));
    if (manifest.format !== 1 || manifest.engine.commit !== info.engine) throw new Error("추론 버전 정보가 일치하지 않습니다.");
    const [wasm, dictionaryBytes] = await Promise.all([
      asset(info.wasm_url, manifest.wasm_sha256),
      asset(info.dictionary_url, manifest.dictionary_sha256)
    ]);
    const loaded = await WebAssembly.instantiate(wasm, {});
    runtime = loaded.instance.exports;
    for (const name of ["memory", "gso_alloc", "gso_free", "gso_free_result", "gso_call"]) {
      if (!runtime[name]) throw new Error("WASM 추론 인터페이스가 없습니다: " + name);
    }
    report({op: "start", input: {
      build: info.build, rotation: info.rotation,
      dictionary: JSON.parse(decode.decode(dictionaryBytes)),
      seed: message.seed, max_evaluations: message.max_evaluations,
      audit_samples: 32, scoring: manifest.scoring
    }});
    if (!done) { scheduled = true; setTimeout(tick, 0); }
  } catch (err) {
    done = true;
    self.postMessage({type: "error", error: err.message || String(err)});
  }
};
