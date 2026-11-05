"use strict";
(function () {
  const el = id => document.getElementById("lab-" + id);
  const number = value => Number.isFinite(value) ? value.toLocaleString(undefined, {maximumFractionDigits: 1}) : "—";
  let characters = [], worker = null, run = 0, running = false, loadingProfile = false, profileRun = 0, lastReport = null;
  let watchdog, clock, started = 0, deadline = 0, finishing = false, lastEvent = "", gcslURL = null;

  function error(message) { el("error").textContent = message; el("error").hidden = !message; }
  function selected() { return Array.from(el("characters").querySelectorAll("input:checked"), input => characters[Number(input.value)]); }
  function updateSelection() {
    const chars = selected();
    el("selection").textContent = chars.length + "/4 선택 · " + chars.map(c => c.name + " C" + c.constellation).join(" / ");
    el("start").disabled = running || loadingProfile || chars.length !== 4;
    const checks = el("characters").querySelectorAll("input");
    checks.forEach(input => { input.disabled = running || loadingProfile || (chars.length >= 4 && !input.checked); });
  }
  function controls(active) {
    running = active;
    for (const id of ["load", "uid", "budget", "seed", "duration"]) el(id).disabled = active;
    el("stop").disabled = !active;
    el("finish").disabled = !active || finishing;
    updateSelection();
  }
  function cleanup() {
    clearTimeout(watchdog); clearInterval(clock);
    if (worker) worker.terminate();
    worker = null;
    controls(false);
  }
  function fail(message) { cleanup(); error(message); el("status").textContent = "실행이 끝나지 않았습니다. 독립 검증 결과를 생성하지 못했습니다."; }
  function watch(milliseconds) {
    clearTimeout(watchdog);
    watchdog = setTimeout(() => fail("계산 응답 시간이 초과되어 작업을 중지했습니다. 현재 후보를 이 기기에서 평가하지 못했습니다."), milliseconds);
  }
  async function json(url, options) {
    const response = await fetch(url, options);
    const value = await response.json();
    if (!response.ok) throw new Error(value.error || "요청을 처리하지 못했습니다.");
    return value;
  }

  el("profile-form").addEventListener("submit", async event => {
    event.preventDefault();
    if (running || loadingProfile) return;
    const currentProfile = ++profileRun;
    loadingProfile = true; updateSelection();
    el("load").disabled = true; error("");
    el("profile-status").textContent = "Enka에서 공개 쇼케이스를 불러오는 중…";
    try {
      const profile = await json("/lab/profile?uid=" + encodeURIComponent(el("uid").value.trim()));
      if (currentProfile !== profileRun || running) return;
      characters = profile.characters;
      el("characters").replaceChildren();
      characters.forEach((c, index) => {
        const label = document.createElement("label"), input = document.createElement("input"), span = document.createElement("span");
        input.type = "checkbox"; input.value = String(index); input.addEventListener("change", updateSelection);
        if (c.face) { const img = document.createElement("img"); img.className = "face"; img.alt = ""; img.src = c.face; label.appendChild(img); }
        const name = document.createElement("strong"); name.textContent = c.name + " C" + c.constellation;
        const weapon = document.createElement("small"); weapon.textContent = c.weapon; weapon.style.display = "block";
        span.append(name, weapon); label.prepend(input); label.appendChild(span); el("characters").appendChild(label);
      });
      el("profile-status").textContent = profile.nickname + " · " + characters.length + "명" + (profile.skipped ? " · 가져오지 못한 캐릭터 " + profile.skipped + "명" : "");
      updateSelection();
    } catch (err) { el("profile-status").textContent = err.message; }
    finally { if (currentProfile === profileRun) { loadingProfile = false; el("load").disabled = running; updateSelection(); } }
  });

  function requestFinish(reason) {
    if (!worker || finishing) return;
    finishing = true;
    el("finish").disabled = true;
    el("status").textContent = reason + " 진행 중인 비교를 정리한 뒤 독립 검증합니다.";
    worker.postMessage({type: "finish"});
  }

  el("start").addEventListener("click", async () => {
    const chars = selected();
    if (chars.length !== 4 || running || loadingProfile) return;
    const seed = Number(el("seed").value);
    if (!Number.isInteger(seed) || seed < 1 || seed > 1000000) { error("시드는 1~1,000,000 사이의 정수로 입력해 주세요."); return; }
    const currentRun = ++run;
    finishing = false; lastReport = null; lastEvent = "";
    error(""); controls(true); el("results").hidden = true; el("events").textContent = "";
    for (const id of ["phase", "best"]) el(id).textContent = "—";
    el("evaluations").textContent = "0"; el("decisions").textContent = "0 / 0 / 0"; el("failures").textContent = "0";
    el("status").textContent = "파티와 로테이션을 준비하는 중…";
    el("current-gcsl").textContent = "";
    el("copy").textContent = "GCSL 복사";
    try {
      const input = await json("/lab/input", {method: "POST", headers: {"Content-Type": "application/json"}, body: JSON.stringify({builds: chars.map(c => c.build)})});
      if (currentRun !== run || !running) return;
      if (input.warnings.length) error(input.warnings.join(" "));
      el("status").textContent = input.team_name + " · 추론 모듈을 불러오는 중…";
      worker = new Worker("/lab/assets/lab-worker.js");
      started = performance.now(); deadline = started + Number(el("duration").value) * 1000;
      clock = setInterval(() => {
        el("elapsed").textContent = number((performance.now() - started) / 1000) + "초";
        if (performance.now() >= deadline && !finishing) requestFinish("실행 시간에 도달했습니다.");
      }, 250);
      worker.onmessage = event => {
        if (currentRun !== run) return;
        const message = event.data;
        if (message.type === "loading") { watch(60000); return; }
        if (message.type === "busy") { watch(10000); return; }
        clearTimeout(watchdog);
        if (message.type === "error") { fail(message.error); return; }
        if (message.type !== "progress") return;
        const r = message.report;
        if (r.engine !== input.engine) { fail("추론 엔진 버전이 일치하지 않습니다."); return; }
        lastReport = r;
        renderReport(r);
        if (r.complete) { cleanup(); renderResult(r); }
      };
      worker.onerror = event => { event.preventDefault(); if (currentRun === run) fail("브라우저 추론 작업에서 오류가 발생했습니다. " + (event.message || "")); };
      watch(60000);
      worker.postMessage({type: "start", input, seed, max_evaluations: Number(el("budget").value)});
    } catch (err) { if (currentRun === run) fail(err.message); }
  });

  const phases = {initial: "시작 로테이션 평가", decision: "후보 비교", promotion: "최고 후보 재검증", audit: "독립 시드로 최종 검증", complete: "완료"};
  function renderReport(r) {
    el("phase").textContent = phases[r.phase] || r.phase;
    el("evaluations").textContent = number(r.evaluations) + " (탐색 " + number(r.search_evaluations) + "/" + number(r.max_evaluations) + ", 검증 " + number(r.audit_evaluations) + ")";
    el("decisions").textContent = [r.decisions, r.accepted, r.promotions].map(number).join(" / ");
    el("failures").textContent = number(r.failed_evaluations);
    el("best").textContent = number(r.best_dps);
    if (!finishing) el("status").textContent = phases[r.phase] || r.phase;
    if (r.phase === "audit") { finishing = true; el("finish").disabled = true; el("status").textContent = "탐색 종료 · 독립 검증 중"; }
    if (r.last_event) {
      const key = JSON.stringify(r.last_event) + ":" + r.decisions + ":" + r.promotions;
      if (key !== lastEvent) {
        lastEvent = key;
        const e = r.last_event;
        const line = "평가 " + r.evaluations + " · " + e.kind + (e.operator ? " · " + e.operator : "") + (typeof e.delta === "number" ? " · Δ " + number(e.delta) + " DPS" + (typeof e.se === "number" ? " ± " + number(e.se) + " SE" : "") : "") + (e.error ? " · " + e.error : "");
        const lines = el("events").textContent.split("\n").filter(Boolean);
        lines.push(line); el("events").textContent = lines.slice(-120).join("\n");
      }
    }
    if (r.gcsl) el("current-gcsl").textContent = r.gcsl;
  }
  function renderResult(r) {
    if (!r.baseline || !r.selected || !r.improvement) { fail(r.audit_error || "최종 검증 결과가 없습니다."); return; }
    el("results").hidden = false;
    el("status").textContent = "독립 검증 완료";
    el("result-summary").textContent = (r.team ? r.team.name + " · " : "") + "탐색 " + number(r.search_evaluations) + "회 + 검증 " + number(r.audit_evaluations) + "회 · 시드 " + r.seed + (r.stop_reason === "requested" ? " · 요청한 시점에 탐색 종료" : " · 평가 예산 소진");
    el("baseline").textContent = number(r.baseline.mean) + " ± " + number(r.baseline.se) + " (표준오차, " + r.baseline.n + "회)";
    el("selected").textContent = number(r.selected.mean) + " ± " + number(r.selected.se) + " (표준오차, " + r.selected.n + "회)";
    el("improvement").textContent = number(r.improvement.mean) + " ± " + number(r.improvement.se) + " DPS (표준오차)";
    el("gcsl").textContent = r.gcsl;
    if (gcslURL) URL.revokeObjectURL(gcslURL);
    gcslURL = URL.createObjectURL(new Blob([r.gcsl], {type: "text/plain;charset=utf-8"}));
    el("download").href = gcslURL;
  }
  el("finish").addEventListener("click", () => requestFinish("탐색 종료를 요청했습니다."));
  el("stop").addEventListener("click", () => {
    ++run; cleanup();
    el("status").textContent = "즉시 중지했습니다. 현재 실행의 독립 검증 결과는 없습니다.";
  });
  el("copy").addEventListener("click", async () => {
    try { await navigator.clipboard.writeText(lastReport.gcsl); el("copy").textContent = "복사됨"; }
    catch (_) { error("복사하지 못했습니다. GCSL을 다운로드해 주세요."); }
  });
  el("report-download").addEventListener("click", () => {
    if (!lastReport || !lastReport.complete) return;
    const url = URL.createObjectURL(new Blob([JSON.stringify(lastReport,null,2)],{type:"application/json"}));
    const a = document.createElement("a"); a.href = url; a.download = "gscouter-inference-report.json"; a.click(); setTimeout(() => URL.revokeObjectURL(url),1000);
  });
  json("/lab/assets/manifest.json").then(m => {
    el("version").textContent = "gse " + m.engine.commit + " · gso " + (m.optimizer.commit || "browser pilot");
    const s = m.scoring;
    if (s) el("conditions").textContent = "전투 " + s.duration + "초 · 처음 " + s.warmup + "초 제외 · 적 레벨 " + s.enemy_level + " · 내성 " + (s.resistance*100) + "% · 시드별 동일 조건 비교";
  }).catch(err => { el("version").textContent = "버전 정보를 읽지 못했습니다: " + err.message; });
}());
