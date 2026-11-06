"use strict";
(function () {
  const el = id => document.getElementById("lab-" + id);
  const number = (val, maxDigits = 1) => Number.isFinite(val) ? val.toLocaleString(undefined, {maximumFractionDigits: maxDigits}) : "—";
  const thousands = val => Number.isFinite(val) ? Math.round(val).toLocaleString() : "—";

  let characters = [], worker = null, run = 0, running = false, loadingProfile = false, profileRun = 0, lastReport = null;
  let watchdog, clock, started = 0, deadline = 0, finishing = false, lastEventKey = "", gcslURL = null;
  let baselineGcsl = "", currentBestGcsl = "", baselineDps = null;
  let viewingBaseline = false;
  let history = []; // array of { eval, bestDps, candidateDps, eventKind, delta, se, operator }
  let chartInitialized = false;

  function error(message) {
    const errEl = el("error");
    if (!errEl) return;
    errEl.textContent = message;
    errEl.hidden = !message;
  }

  function selected() {
    return Array.from(el("characters").querySelectorAll("input:checked"), input => characters[Number(input.value)]);
  }

  /* Recent UIDs Management (mirrors main page index.html) */
  const RECENT_KEY = "gscouter.recentUIDs";
  function getRecentUIDs() {
    try { return JSON.parse(localStorage.getItem(RECENT_KEY) || "[]") || []; }
    catch (_) { return []; }
  }
  function saveRecentUID(uid) {
    if (!uid) return;
    try {
      let list = getRecentUIDs().filter(u => String(u) !== String(uid));
      list.unshift(uid);
      localStorage.setItem(RECENT_KEY, JSON.stringify(list.slice(0, 6)));
      renderRecentUIDs();
    } catch (_) {}
  }
  function renderRecentUIDs() {
    const box = document.getElementById("lab-recent-box");
    const listEl = document.getElementById("lab-recent-list");
    if (!box || !listEl) return;
    const list = getRecentUIDs();
    if (!list.length) { box.hidden = true; return; }
    listEl.replaceChildren();
    list.forEach(u => {
      const a = document.createElement("a");
      a.href = "#";
      a.textContent = u;
      a.addEventListener("click", e => {
        e.preventDefault();
        el("uid").value = u;
        el("profile-form").requestSubmit();
      });
      listEl.appendChild(a);
    });
    box.hidden = false;
  }

  /* Preset Selection Buttons */
  document.querySelectorAll(".lab-preset-btn").forEach(btn => {
    btn.addEventListener("click", () => {
      document.querySelectorAll(".lab-preset-btn").forEach(b => b.classList.remove("active"));
      btn.classList.add("active");
      const b = btn.getAttribute("data-budget"), d = btn.getAttribute("data-duration");
      if (b && el("budget")) el("budget").value = b;
      if (d && el("duration")) el("duration").value = d;
    });
  });

  /* Tabs Management */
  document.querySelectorAll(".lab-tab-btn").forEach(btn => {
    btn.addEventListener("click", () => {
      const tabName = btn.getAttribute("data-tab");
      document.querySelectorAll(".lab-tab-btn").forEach(b => b.classList.remove("active"));
      document.querySelectorAll(".lab-tab-pane").forEach(p => p.classList.remove("active"));
      btn.classList.add("active");
      const pane = document.getElementById("pane-" + tabName);
      if (pane) pane.classList.add("active");
      if (tabName === "chart") renderChart();
    });
  });

  /* Party Preview Bar & Character Selection */
  function updateSelection() {
    const chars = selected();
    const count = chars.length;
    const selEl = el("selection");
    if (selEl) {
      if (count === 4) {
        selEl.textContent = "파티 구성 완료 · " + chars.map(c => c.name + " (C" + c.constellation + ")").join(" / ");
      } else {
        selEl.textContent = count + "/4명 선택됨 · 네 명의 캐릭터를 선택해 주세요.";
      }
    }

    const partyBar = document.getElementById("lab-party-bar");
    const partyFaces = document.getElementById("lab-party-faces");
    const partyNames = document.getElementById("lab-party-names");
    const partyValidity = document.getElementById("lab-party-validity");

    if (partyBar && partyFaces && partyNames) {
      if (count > 0) {
        partyBar.hidden = false;
        partyFaces.replaceChildren();
        partyNames.replaceChildren();
        chars.forEach((c, idx) => {
          if (c.face) {
            const img = document.createElement("img");
            img.className = "face";
            img.src = c.face;
            img.alt = c.name;
            partyFaces.appendChild(img);
          }
          const m = document.createElement("span");
          m.className = "lab-party-member";
          const sw = document.createElement("i");
          sw.className = "sw s" + (idx % 4);
          m.append(sw, document.createTextNode(c.name + " C" + c.constellation));
          partyNames.appendChild(m);
        });
        if (partyValidity) {
          if (count === 4) {
            partyValidity.textContent = "4명 선택 완료";
            partyValidity.style.borderColor = "var(--gold)";
            partyValidity.style.color = "var(--gold)";
            partyValidity.style.background = "var(--gold-soft)";
          } else {
            partyValidity.textContent = count + " / 4명";
            partyValidity.style.borderColor = "var(--line)";
            partyValidity.style.color = "var(--muted)";
            partyValidity.style.background = "var(--bg)";
          }
        }
      } else {
        partyBar.hidden = true;
      }
    }

    el("start").disabled = running || loadingProfile || count !== 4;
    const checks = el("characters").querySelectorAll("input");
    checks.forEach(input => {
      input.disabled = running || loadingProfile || (count >= 4 && !input.checked);
    });
  }

  const clearBtn = document.getElementById("lab-party-clear");
  if (clearBtn) {
    clearBtn.addEventListener("click", () => {
      if (running || loadingProfile) return;
      el("characters").querySelectorAll("input:checked").forEach(input => { input.checked = false; });
      updateSelection();
    });
  }

  function controls(active) {
    running = active;
    for (const id of ["load", "uid", "budget", "seed", "duration"]) {
      if (el(id)) el(id).disabled = active;
    }
    el("stop").disabled = !active;
    el("finish").disabled = !active || finishing;
    updateSelection();
  }

  function cleanup() {
    clearTimeout(watchdog);
    clearInterval(clock);
    if (worker) worker.terminate();
    worker = null;
    controls(false);
  }

  function fail(message) {
    cleanup();
    error(message);
    el("status").textContent = "실행이 끝나지 않았습니다. 독립 검증 결과를 생성하지 못했습니다.";
    const badge = document.getElementById("lab-phase-badge");
    if (badge) { badge.textContent = "오류"; badge.style.background = "var(--bad)"; }
  }

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

  /* Profile Load (Enka Showcase) */
  el("profile-form").addEventListener("submit", async event => {
    event.preventDefault();
    if (running || loadingProfile) return;
    const rawUid = el("uid").value.trim();
    if (!rawUid) return;
    const currentProfile = ++profileRun;
    loadingProfile = true;
    updateSelection();
    el("load").disabled = true;
    error("");
    el("profile-status").textContent = "Enka.Network에서 공개 쇼케이스 정보를 불러오는 중…";

    try {
      const profile = await json("/lab/profile?uid=" + encodeURIComponent(rawUid));
      if (currentProfile !== profileRun || running) return;
      characters = profile.characters || [];
      saveRecentUID(rawUid);
      el("characters").replaceChildren();

      characters.forEach((c, index) => {
        const label = document.createElement("label");
        const input = document.createElement("input");
        const span = document.createElement("span");
        input.type = "checkbox";
        input.value = String(index);
        input.addEventListener("change", updateSelection);

        if (c.face) {
          const img = document.createElement("img");
          img.className = "face";
          img.alt = c.name;
          img.src = c.face;
          label.appendChild(img);
        }
        const name = document.createElement("strong");
        name.className = "name";
        name.textContent = c.name + " C" + c.constellation;
        const weapon = document.createElement("small");
        weapon.textContent = c.weapon;
        weapon.style.display = "block";
        weapon.className = "muted";
        span.append(name, weapon);
        label.prepend(input);
        label.appendChild(span);
        el("characters").appendChild(label);
      });

      el("profile-status").textContent = profile.nickname + " · " + characters.length + "명 쇼케이스 등록" +
        (profile.skipped ? " (세부 스펙 미공개 캐릭터 " + profile.skipped + "명 제외)" : "");
      updateSelection();
    } catch (err) {
      el("profile-status").textContent = err.message;
    } finally {
      if (currentProfile === profileRun) {
        loadingProfile = false;
        el("load").disabled = running;
        updateSelection();
      }
    }
  });

  function requestFinish(reason) {
    if (!worker || finishing) return;
    finishing = true;
    el("finish").disabled = true;
    el("status").textContent = reason + " 진행 중인 비교를 정리한 뒤 독립 검증합니다.";
    const badge = document.getElementById("lab-phase-badge");
    if (badge) badge.textContent = "검증 준비";
    worker.postMessage({type: "finish"});
  }

  /* GCSL Parser & Visualizer */
  function cleanGCSL(raw) {
    if (!raw) return "";
    // Find where character definitions / build options end and rotation begins
    const lines = raw.split("\n");
    let startIndex = 0;
    for (let i = 0; i < lines.length; i++) {
      const trimmed = lines[i].trim();
      if (trimmed.startsWith("active ") || trimmed.startsWith("while ") || trimmed.startsWith("for let ")) {
        startIndex = i;
        break;
      }
    }
    return lines.slice(startIndex).join("\n").trim();
  }

  function parseRotationToStints(gcslText, party) {
    if (!gcslText || !party || !party.length) return [];
    const rotationCode = cleanGCSL(gcslText);
    const statements = rotationCode.split(";");
    const stints = [];
    let currentStint = null;

    function getCharMatch(identifier) {
      if (!identifier) return null;
      const lower = identifier.toLowerCase().trim();
      for (let i = 0; i < party.length; i++) {
        const c = party[i];
        const key = (c.key || "").toLowerCase();
        const name = (c.name || "").toLowerCase();
        if (key === lower || name === lower || key.startsWith(lower) || lower.startsWith(key)) {
          return { ...c, slot: i };
        }
      }
      return null;
    }

    statements.forEach(stmt => {
      const clean = stmt.replace(/\/\/.*/g, "").replace(/#.*/g, "").trim();
      if (!clean) return;

      // Handle active switch: active arle
      const activeMatch = clean.match(/^active\s+([a-zA-Z0-9_]+)/i);
      if (activeMatch) {
        const c = getCharMatch(activeMatch[1]);
        if (c) {
          currentStint = { char: c, actions: [{ kind: "util", label: "액티브 지정", raw: clean }] };
          stints.push(currentStint);
        }
        return;
      }

      // Handle loop or wait
      const waitMatch = clean.match(/^(?:wait|delay)\s*\(\s*(\d+)\s*\)/i);
      if (waitMatch) {
        if (currentStint) {
          currentStint.actions.push({ kind: "wait", label: "대기 " + waitMatch[1] + "f", raw: clean });
        } else {
          stints.push({ char: { name: "시스템", slot: 0 }, actions: [{ kind: "wait", label: "대기 " + waitMatch[1] + "f", raw: clean }] });
        }
        return;
      }

      // Check character action line: <char> <actions>
      // e.g. "xilo skill, attack:2" or "citlali burst, skill"
      const actionLineMatch = clean.match(/^([a-zA-Z0-9_]+)\s+([a-zA-Z0-9_:,\[\]\(\)\s\.\+\-\*\/]+)$/);
      if (actionLineMatch) {
        const charId = actionLineMatch[1];
        const actionsStr = actionLineMatch[2];
        const c = getCharMatch(charId);

        if (c) {
          // If different character or no current stint, start a new stint
          if (!currentStint || currentStint.char.name !== c.name) {
            currentStint = { char: c, actions: [] };
            stints.push(currentStint);
          }

          const actionTokens = actionsStr.split(",").map(s => s.trim()).filter(Boolean);
          actionTokens.forEach(act => {
            if (/^skill/i.test(act)) {
              currentStint.actions.push({ kind: "skill", label: "스킬", mk: "E", raw: act });
            } else if (/^burst/i.test(act)) {
              currentStint.actions.push({ kind: "burst", label: "원소폭발", mk: "Q", raw: act });
            } else if (/^attack(?::(\d+))?/i.test(act)) {
              const m = act.match(/^attack(?::(\d+))?/i);
              const count = m && m[1] ? m[1] : "1";
              currentStint.actions.push({ kind: "attack", label: "N" + count + " 평타", raw: act });
            } else if (/^charge/i.test(act)) {
              currentStint.actions.push({ kind: "charge", label: "C 강공격", raw: act });
            } else if (/^(?:low_|high_)?plunge/i.test(act)) {
              currentStint.actions.push({ kind: "attack", label: "낙하공격", raw: act });
            } else if (/^dash/i.test(act)) {
              currentStint.actions.push({ kind: "util", label: "대시", raw: act });
            } else if (/^swap/i.test(act)) {
              currentStint.actions.push({ kind: "util", label: "교체", raw: act });
            } else if (/^walk/i.test(act)) {
              currentStint.actions.push({ kind: "util", label: "이동", raw: act });
            } else if (/^jump/i.test(act)) {
              currentStint.actions.push({ kind: "util", label: "점프", raw: act });
            } else if (/^(?:wait|delay)\s*\(\s*(\d+)\s*\)/i.test(act)) {
              const dm = act.match(/^(?:wait|delay)\s*\(\s*(\d+)\s*\)/i);
              currentStint.actions.push({ kind: "wait", label: "대기 " + (dm ? dm[1] : "") + "f", raw: act });
            } else {
              currentStint.actions.push({ kind: "util", label: act, raw: act });
            }
          });
        }
      }
    });

    return stints;
  }

  function renderVisualRotation(gcslText, isBaseline = false) {
    const listEl = document.getElementById("lab-visual-rotation");
    const tagEl = document.getElementById("lab-rotation-tag");
    const sumEl = document.getElementById("lab-rotation-summary");
    if (!listEl) return;

    if (tagEl) {
      if (isBaseline) {
        tagEl.textContent = "기존 라이브러리 로테이션 (기준)";
        tagEl.style.background = "var(--accent-soft)";
        tagEl.style.borderColor = "var(--line)";
        tagEl.style.color = "var(--text)";
      } else {
        tagEl.textContent = "현재 최고 후보 로테이션";
        tagEl.style.background = "var(--gold-soft)";
        tagEl.style.borderColor = "var(--gold)";
        tagEl.style.color = "var(--gold)";
      }
    }

    const chars = selected();
    const stints = parseRotationToStints(gcslText, chars);
    listEl.replaceChildren();

    if (!stints.length) {
      const empty = document.createElement("li");
      empty.className = "muted";
      empty.style.padding = "24px";
      empty.style.textAlign = "center";
      empty.textContent = "표시할 로테이션 액션이 없습니다.";
      listEl.appendChild(empty);
      if (sumEl) sumEl.textContent = "";
      return;
    }

    if (sumEl) {
      const actionCount = stints.reduce((sum, s) => sum + s.actions.length, 0);
      sumEl.textContent = "총 " + stints.length + "개 스틴트 · " + actionCount + "개 액션";
    }

    stints.forEach((stint, idx) => {
      const row = document.createElement("li");
      row.className = "lab-stint-row";

      const idxEl = document.createElement("span");
      idxEl.className = "lab-stint-idx";
      idxEl.textContent = "#" + (idx + 1);

      const whoEl = document.createElement("div");
      whoEl.className = "lab-stint-who";
      const sw = document.createElement("i");
      sw.className = "sw s" + (stint.char.slot !== undefined ? stint.char.slot : (idx % 4));
      whoEl.appendChild(sw);

      if (stint.char.face) {
        const faceImg = document.createElement("img");
        faceImg.className = "face";
        faceImg.src = stint.char.face;
        faceImg.alt = stint.char.name || "";
        whoEl.appendChild(faceImg);
      }
      const nameB = document.createElement("b");
      nameB.textContent = stint.char.name || "캐릭터";
      whoEl.appendChild(nameB);

      const actWrap = document.createElement("div");
      actWrap.className = "lab-stint-actions";

      stint.actions.forEach(action => {
        const chip = document.createElement("span");
        chip.className = "lab-action-chip act-" + (action.kind || "util");
        if (action.mk) {
          const mk = document.createElement("span");
          mk.className = "chip-mk";
          mk.textContent = action.mk;
          chip.appendChild(mk);
        }
        chip.appendChild(document.createTextNode(action.label));
        chip.title = action.raw || action.label;
        actWrap.appendChild(chip);
      });

      row.append(idxEl, whoEl, actWrap);
      listEl.appendChild(row);
    });
  }

  const viewToggleBtn = document.getElementById("lab-view-toggle");
  if (viewToggleBtn) {
    viewToggleBtn.addEventListener("click", () => {
      viewlingBaselineToggle();
    });
  }
  function viewlingBaselineToggle() {
    if (!baselineGcsl && !currentBestGcsl) return;
    viewingBaseline = !viewingBaseline;
    if (viewToggleBtn) {
      viewToggleBtn.textContent = viewingBaseline ? "최적 후보 로테이션 보기" : "기존 로테이션 보기";
    }
    renderVisualRotation(viewingBaseline ? baselineGcsl : currentBestGcsl, viewingBaseline);
  }

  /* Real-time SVG DPS Progression Chart */
  function renderChart() {
    const svg = document.getElementById("lab-chart");
    if (!svg) return;

    if (!history.length && !baselineDps) {
      svg.innerHTML = '<text x="340" y="115" text-anchor="middle" fill="var(--muted)" font-size="13">추론이 시작되면 실시간 DPS 수렴 그래프가 그려집니다.</text>';
      return;
    }

    const svgW = 680, svgH = 220;
    const padL = 75, padR = 30, padT = 25, padB = 35;
    const chartW = svgW - padL - padR;
    const chartH = svgH - padT - padB;

    let maxEval = Number(el("budget") ? el("budget").value : 2048) || 2048;
    if (history.length) {
      const lastEval = history[history.length - 1].eval;
      if (lastEval > maxEval) maxEval = lastEval;
    }

    // Determine Y range (DPS)
    let dpsValues = [];
    if (baselineDps) dpsValues.push(baselineDps);
    history.forEach(h => {
      if (h.bestDps) dpsValues.push(h.bestDps);
      if (h.candidateDps) dpsValues.push(h.candidateDps);
    });
    if (!dpsValues.length) dpsValues = [100000];

    let minDPS = Math.min(...dpsValues);
    let maxDPS = Math.max(...dpsValues);
    if (minDPS === maxDPS) {
      minDPS = Math.max(0, minDPS - 15000);
      maxDPS = maxDPS + 15000;
    }
    const dpsSpan = maxDPS - minDPS;
    const yPad = dpsSpan * 0.12;
    const yMin = Math.max(0, minDPS - yPad);
    const yMax = maxDPS + yPad;
    const yRange = yMax - yMin;

    const scaleX = e => padL + (Math.max(0, Math.min(e, maxEval)) / maxEval) * chartW;
    const scaleY = d => padT + chartH - ((d - yMin) / yRange) * chartH;

    let parts = [];

    // Horizontal grid lines (4 lines)
    for (let i = 0; i <= 3; i++) {
      const frac = i / 3;
      const dpsVal = yMin + frac * yRange;
      const y = scaleY(dpsVal);
      parts.push(`<line class="grid" x1="${padL}" y1="${y}" x2="${padL + chartW}" y2="${y}"/>`);
      parts.push(`<text class="tick" x="${padL - 8}" y="${y + 4}" text-anchor="end">${thousands(dpsVal)}</text>`);
    }

    // Vertical grid lines (4 intervals)
    for (let i = 0; i <= 4; i++) {
      const evalVal = Math.round((i / 4) * maxEval);
      const x = scaleX(evalVal);
      parts.push(`<line class="grid" x1="${x}" y1="${padT}" x2="${x}" y2="${padT + chartH}"/>`);
      parts.push(`<text class="tick" x="${x}" y="${padT + chartH + 18}" text-anchor="middle">${thousands(evalVal)}</text>`);
    }

    // Baseline reference line
    if (baselineDps) {
      const baseY = scaleY(baselineDps);
      parts.push(`<line class="line-base" x1="${padL}" y1="${baseY}" x2="${padL + chartW}" y2="${baseY}"/>`);
      parts.push(`<text x="${padL + 8}" y="${baseY - 6}" fill="var(--muted)" font-size="10">기준: ${thousands(baselineDps)} DPS</text>`);
    }

    // Best DPS area and line
    if (history.length) {
      let linePoints = [];
      let areaPoints = [`${scaleX(history[0].eval)},${padT + chartH}`];

      history.forEach((pt, i) => {
        const x = scaleX(pt.eval);
        const y = scaleY(pt.bestDps);
        linePoints.push(`${x},${y}`);
        areaPoints.push(`${x},${y}`);
      });
      const lastX = scaleX(history[history.length - 1].eval);
      areaPoints.push(`${lastX},${padT + chartH}`);

      parts.push(`<polygon class="area-best" points="${areaPoints.join(' ')}"/>`);
      parts.push(`<polyline class="line-best" points="${linePoints.join(' ')}"/>`);

      // Scatter dots for events
      history.forEach(pt => {
        if (pt.eventKind === "promoted") {
          const cx = scaleX(pt.eval);
          const cy = scaleY(pt.bestDps);
          parts.push(`<circle class="dot-accepted" cx="${cx}" cy="${cy}" r="4"><title>평가 ${pt.eval}회 · 갱신: ${thousands(pt.bestDps)} DPS</title></circle>`);
        } else if (pt.eventKind === "rejected" && pt.candidateDps) {
          const cx = scaleX(pt.eval);
          const cy = scaleY(pt.candidateDps);
          parts.push(`<circle class="dot-rejected" cx="${cx}" cy="${cy}" r="2"><title>평가 ${pt.eval}회 · 기각: ${thousands(pt.candidateDps)} DPS</title></circle>`);
        }
      });

      // Latest peak pulse marker
      const latest = history[history.length - 1];
      const lx = scaleX(latest.eval);
      const ly = scaleY(latest.bestDps);
      parts.push(`<circle class="dot-current" cx="${lx}" cy="${ly}" r="5"/>`);
      parts.push(`<rect x="${Math.min(lx + 8, svgW - 120)}" y="${ly - 16}" width="105" height="20" rx="3" fill="var(--card)" stroke="var(--gold)" stroke-width="1"/>`);
      parts.push(`<text x="${Math.min(lx + 14, svgW - 114)}" y="${ly - 3}" fill="var(--text)" font-size="11" font-weight="700">${thousands(latest.bestDps)} DPS</text>`);
    }

    svg.innerHTML = parts.join("");
  }

  /* Start Inference */
  el("start").addEventListener("click", async () => {
    const chars = selected();
    if (chars.length !== 4 || running || loadingProfile) return;

    const seed = Number(el("seed").value);
    if (!Number.isInteger(seed) || seed < 1 || seed > 1000000) {
      error("시드는 1~1,000,000 사이의 정수로 입력해 주세요.");
      return;
    }

    const currentRun = ++run;
    finishing = false;
    lastReport = null;
    lastEventKey = "";
    history = [];
    baselineDps = null;
    baselineGcsl = "";
    currentBestGcsl = "";
    viewingBaseline = false;

    error("");
    controls(true);
    el("results").hidden = true;
    el("events").textContent = "";

    const tbody = document.getElementById("lab-events-tbody");
    if (tbody) tbody.replaceChildren();

    const badge = document.getElementById("lab-phase-badge");
    if (badge) { badge.textContent = "초기화"; badge.style.background = "var(--accent)"; }
    el("phase").textContent = "시작 로테이션 준비";
    el("best").textContent = "—";
    el("evaluations").textContent = "0";
    el("decisions").textContent = "0 / 0 / 0";
    el("failures").textContent = "0";
    el("status").textContent = "파티 및 라이브러리 로테이션을 검증하는 중…";
    el("current-gcsl").textContent = "";
    el("copy").textContent = "GCSL 복사";

    const diffEl = document.getElementById("lab-best-diff");
    if (diffEl) { diffEl.textContent = "기준 로테이션 측정 대기"; diffEl.className = "lab-kpi-sub"; }
    const acceptEl = document.getElementById("lab-accept-rate");
    if (acceptEl) acceptEl.textContent = "채택률 0.0%";
    const progFill = document.getElementById("lab-progress-fill");
    if (progFill) progFill.style.width = "0%";
    const progText = document.getElementById("lab-progress-text");
    if (progText) progText.textContent = "0 / " + el("budget").value + "회 (0%)";

    renderChart();

    try {
      const input = await json("/lab/input", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ builds: chars.map(c => c.build) })
      });
      if (currentRun !== run || !running) return;

      if (input.warnings && input.warnings.length) {
        error(input.warnings.join(" "));
      }

      baselineGcsl = input.rotation || "";
      currentBestGcsl = baselineGcsl;
      renderVisualRotation(baselineGcsl, false);

      el("status").textContent = input.team_name + " · 브라우저 WASM 모듈을 준비하는 중…";
      worker = new Worker("/lab/assets/lab-worker.js");
      started = performance.now();
      const durationSec = Number(el("duration").value) || 180;
      deadline = started + durationSec * 1000;

      clock = setInterval(() => {
        const elapsedSec = (performance.now() - started) / 1000;
        el("elapsed").textContent = number(elapsedSec, 0) + "초";
        const remainSec = Math.max(0, (deadline - performance.now()) / 1000);
        const remainEl = document.getElementById("lab-time-remain");
        if (remainEl) {
          remainEl.textContent = "남은 시간 약 " + Math.floor(remainSec / 60) + "분 " + Math.floor(remainSec % 60) + "초";
        }
        if (performance.now() >= deadline && !finishing) {
          requestFinish("설정한 실행 제한 시간에 도달했습니다.");
        }
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
        if (r.engine !== input.engine) {
          fail("추론 엔진 버전이 일치하지 않습니다. 페이지를 새로고침해 주세요.");
          return;
        }
        lastReport = r;
        renderReport(r);
        if (r.complete) {
          cleanup();
          renderResult(r);
        }
      };

      worker.onerror = event => {
        event.preventDefault();
        if (currentRun === run) fail("브라우저 추론 작업에서 오류가 발생했습니다: " + (event.message || ""));
      };

      const pilotSelect = el("pilot");
      const pilotArm = pilotSelect ? pilotSelect.value : "syn";

      watch(60000);
      worker.postMessage({
        type: "start",
        input,
        seed,
        pilot: pilotArm,
        max_evaluations: Number(el("budget").value)
      });

    } catch (err) {
      if (currentRun === run) fail(err.message);
    }
  });

  const phases = {
    initial: "시작 로테이션 평가",
    decision: "후보 비교 및 탐색",
    promotion: "최고 후보 승격 검증",
    audit: "독립 시드 최종 검증",
    complete: "탐색 및 검증 완료"
  };

  function renderReport(r) {
    const phaseName = phases[r.phase] || r.phase;
    el("phase").textContent = phaseName;
    const badge = document.getElementById("lab-phase-badge");
    if (badge) {
      badge.textContent = phaseName;
      if (r.phase === "audit") badge.style.background = "var(--gold)";
      else if (r.phase === "complete") badge.style.background = "var(--good)";
      else badge.style.background = "var(--accent)";
    }

    el("evaluations").textContent = thousands(r.evaluations);
    el("decisions").textContent = [r.decisions, r.accepted, r.promotions].map(thousands).join(" / ");
    el("failures").textContent = thousands(r.failed_evaluations);
    if (r.best_dps) el("best").textContent = thousands(r.best_dps);

    // Baseline tracking
    if (baselineDps === null && r.best_dps && (r.phase === "initial" || r.decisions > 0)) {
      baselineDps = r.best_dps;
    }
    const diffEl = document.getElementById("lab-best-diff");
    if (diffEl && baselineDps !== null && r.best_dps) {
      const diff = r.best_dps - baselineDps;
      const pct = (diff / baselineDps) * 100;
      if (diff > 0) {
        diffEl.textContent = "+ " + thousands(diff) + " DPS (+" + number(pct, 2) + "%)";
        diffEl.className = "lab-kpi-sub good";
      } else {
        diffEl.textContent = "기준 로테이션과 동일";
        diffEl.className = "lab-kpi-sub";
      }
    }

    // Acceptance rate
    const acceptEl = document.getElementById("lab-accept-rate");
    if (acceptEl && r.decisions > 0) {
      const rate = ((r.accepted / r.decisions) * 100);
      acceptEl.textContent = "채택률 " + number(rate, 1) + "%";
    }

    // Progress bar
    const maxBudget = r.max_evaluations || 2048;
    const pctProg = Math.min(100, Math.round((r.evaluations / maxBudget) * 100));
    const progFill = document.getElementById("lab-progress-fill");
    if (progFill) progFill.style.width = pctProg + "%";
    const progText = document.getElementById("lab-progress-text");
    if (progText) {
      progText.textContent = thousands(r.evaluations) + " / " + thousands(maxBudget) + "회 (" + pctProg + "%)";
    }

    if (!finishing) el("status").textContent = phaseName;
    if (r.phase === "audit") {
      finishing = true;
      el("finish").disabled = true;
      el("status").textContent = "탐색 종료 · 32회 독립 시드 맞비교 검증 중";
    }

    // Events log & history recording
    if (r.last_event) {
      const key = JSON.stringify(r.last_event) + ":" + r.decisions + ":" + r.promotions + ":" + r.evaluations;
      if (key !== lastEventKey) {
        lastEventKey = key;
        const e = r.last_event;
        const delta = typeof e.delta === "number" ? e.delta : 0;
        const candidateDps = r.best_dps ? (r.best_dps + delta) : null;

        // Record history for graph
        if (r.best_dps) {
          history.push({
            eval: r.evaluations,
            bestDps: r.best_dps,
            candidateDps,
            eventKind: e.kind || "step",
            delta: e.delta,
            se: e.se,
            operator: e.operator
          });
          renderChart();
        }

        // Table log append
        const tbody = document.getElementById("lab-events-tbody");
        if (tbody) {
          const row = document.createElement("tr");
          let badgeClass = "rejected";
          let badgeLabel = "기각";
          if (e.kind === "promoted") { badgeClass = "promoted"; badgeLabel = "최고점 갱신"; }
          else if (e.kind === "accepted") { badgeClass = "accepted"; badgeLabel = "후보 채택"; }
          else if (e.kind === "baseline_measured") { badgeClass = "baseline"; badgeLabel = "기준 측정"; }
          else if (e.kind === "completed") { badgeClass = "promoted"; badgeLabel = "검증 완료"; }

          row.innerHTML = `
            <td>#${thousands(r.evaluations)}</td>
            <td><span class="lab-event-badge ${badgeClass}">${badgeLabel}</span></td>
            <td>${e.operator || "—"}</td>
            <td class="num" style="color:${delta > 0 ? 'var(--good)' : (delta < 0 ? 'var(--bad)' : 'inherit')}">${delta ? (delta > 0 ? '+ ' : '') + thousands(delta) + ' DPS' : '—'}</td>
            <td class="num">${typeof e.se === 'number' ? '± ' + number(e.se, 1) : '—'}</td>
          `;
          tbody.prepend(row);
          // Keep at most 100 rows in table
          if (tbody.children.length > 100) tbody.removeChild(tbody.lastChild);
        }

        // Raw pre text
        const line = "평가 " + r.evaluations + " · " + (e.kind || "") + (e.operator ? " · " + e.operator : "") +
          (typeof e.delta === "number" ? " · Δ " + number(e.delta) + " DPS" + (typeof e.se === "number" ? " ± " + number(e.se) + " SE" : "") : "") +
          (e.error ? " · " + e.error : "");
        const lines = el("events").textContent.split("\n").filter(Boolean);
        lines.push(line);
        el("events").textContent = lines.slice(-120).join("\n");
      }
    }

    // GCSL and Visual Rotation update
    if (r.gcsl && r.gcsl !== currentBestGcsl) {
      currentBestGcsl = r.gcsl;
      el("current-gcsl").textContent = r.gcsl;
      if (!viewingBaseline) {
        renderVisualRotation(currentBestGcsl, false);
      }
    }
  }

  /* Render Final Audit Verification Result */
  function renderResult(r) {
    if (!r.baseline || !r.selected || !r.improvement) {
      fail(r.audit_error || "독립 검증 결과를 생성하지 못했습니다.");
      return;
    }

    el("results").hidden = false;
    el("status").textContent = "독립 검증 완료";
    const badge = document.getElementById("lab-phase-badge");
    if (badge) { badge.textContent = "완료"; badge.style.background = "var(--good)"; }

    // Summary text
    el("result-summary").textContent = (r.team ? r.team.name + " · " : "") +
      "탐색 " + thousands(r.search_evaluations) + "회 + 검증 " + thousands(r.audit_evaluations) + "회 · 시드 " + r.seed +
      (r.stop_reason === "requested" ? " · 사용자 조기 검증 요청" : " · 설정 예산 완주");

    // Headline score & gain
    const gainEl = document.getElementById("lab-result-headline-gain");
    const dpsEl = document.getElementById("lab-result-headline-dps");
    const deltaMean = r.improvement.mean || 0;
    const baselineMean = r.baseline.mean || 1;
    const pctGain = (deltaMean / baselineMean) * 100;

    if (gainEl) {
      if (deltaMean > 0) {
        gainEl.textContent = "+ " + thousands(deltaMean) + " DPS 개선 (+" + number(pctGain, 2) + "%)";
        gainEl.style.color = "var(--good)";
      } else {
        gainEl.textContent = "기존 대비 통계적 유의미한 차이 없음";
        gainEl.style.color = "var(--muted)";
      }
    }
    if (dpsEl) dpsEl.textContent = thousands(r.selected.mean) + " DPS";

    // Detailed comparison table
    const bMeanEl = document.getElementById("lab-baseline-mean");
    const bSeEl = document.getElementById("lab-baseline-se");
    const bNEl = document.getElementById("lab-baseline-n");
    if (bMeanEl) bMeanEl.textContent = thousands(r.baseline.mean);
    if (bSeEl) bSeEl.textContent = "± " + number(r.baseline.se, 1);
    if (bNEl) bNEl.textContent = r.baseline.n + "회";

    const sMeanEl = document.getElementById("lab-selected-mean");
    const sSeEl = document.getElementById("lab-selected-se");
    const sNEl = document.getElementById("lab-selected-n");
    if (sMeanEl) sMeanEl.textContent = thousands(r.selected.mean);
    if (sSeEl) sSeEl.textContent = "± " + number(r.selected.se, 1);
    if (sNEl) sNEl.textContent = r.selected.n + "회";

    const iMeanEl = document.getElementById("lab-improvement-mean");
    const iSeEl = document.getElementById("lab-improvement-se");
    if (iMeanEl) iMeanEl.textContent = (deltaMean > 0 ? "+ " : "") + thousands(deltaMean) + " DPS";
    if (iSeEl) iSeEl.textContent = "± " + number(r.improvement.se, 1);

    // Keep legacy IDs populated for tests
    el("baseline").textContent = number(r.baseline.mean) + " ± " + number(r.baseline.se) + " (표준오차, " + r.baseline.n + "회)";
    el("selected").textContent = number(r.selected.mean) + " ± " + number(r.selected.se) + " (표준오차, " + r.selected.n + "회)";
    el("improvement").textContent = number(r.improvement.mean) + " ± " + number(r.improvement.se) + " DPS (표준오차)";
    el("gcsl").textContent = r.gcsl;

    if (gcslURL) URL.revokeObjectURL(gcslURL);
    gcslURL = URL.createObjectURL(new Blob([r.gcsl], {type: "text/plain;charset=utf-8"}));
    el("download").href = gcslURL;

    // Ensure chart is fully rendered with final state
    renderChart();
  }

  el("finish").addEventListener("click", () => requestFinish("사용자가 탐색 종료를 요청했습니다."));
  el("stop").addEventListener("click", () => {
    ++run;
    cleanup();
    el("status").textContent = "즉시 중지했습니다. 현재 실행의 독립 검증 결과는 없습니다.";
    const badge = document.getElementById("lab-phase-badge");
    if (badge) { badge.textContent = "중지됨"; badge.style.background = "var(--muted)"; }
  });

  /* Copy Buttons with visual feedback */
  function copyText(text, btn) {
    if (!text) return;
    navigator.clipboard.writeText(text).then(() => {
      const orig = btn.textContent;
      btn.textContent = "복사 완료!";
      setTimeout(() => { btn.textContent = orig; }, 2000);
    }).catch(() => {
      error("클립보드에 복사하지 못했습니다. 다운로드 버튼을 이용해 주세요.");
    });
  }

  el("copy").addEventListener("click", () => {
    if (lastReport && lastReport.gcsl) copyText(lastReport.gcsl, el("copy"));
  });

  const copyCurrentBtn = document.getElementById("lab-copy-current");
  if (copyCurrentBtn) {
    copyCurrentBtn.addEventListener("click", () => {
      if (lastReport && lastReport.gcsl) copyText(lastReport.gcsl, copyCurrentBtn);
    });
  }

  el("report-download").addEventListener("click", () => {
    if (!lastReport || !lastReport.complete) return;
    const url = URL.createObjectURL(new Blob([JSON.stringify(lastReport, null, 2)], {type: "application/json"}));
    const a = document.createElement("a");
    a.href = url;
    a.download = "gscouter-inference-report.json";
    a.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  });

  /* Manifest & Versions */
  json("/lab/assets/manifest.json").then(m => {
    el("version").textContent = "gse 엔진 " + m.engine.commit.substring(0, 10) + " · gso WASM " + ((m.optimizer && m.optimizer.commit) ? m.optimizer.commit.substring(0, 10) : "브라우저 파일럿");
    const s = m.scoring;
    if (s) {
      el("conditions").textContent = "전투 시간 " + s.duration + "초 (초반 " + s.warmup + "초 제외) · 적 레벨 " + s.enemy_level + " · 속성 내성 " + Math.round(s.resistance * 100) + "% · 시드별 1:1 맞비교 평가";
    }
  }).catch(err => {
    el("version").textContent = "버전 정보를 읽지 못했습니다: " + err.message;
  });

  /* Initial Page Setup */
  renderRecentUIDs();

  // If URL has ?uid=..., automatically populate and submit
  try {
    const params = new URLSearchParams(window.location.search);
    const qUid = params.get("uid");
    if (qUid && /^[1-9][0-9]{8,9}$/.test(qUid)) {
      el("uid").value = qUid;
      el("profile-form").requestSubmit();
    }
  } catch (_) {}
}());
