// The spec animator. It drives one exploration session through a transport,
// an object whose send(request) answers a proto.Response as a Promise; the
// engine compiled to WebAssembly is one, a server would be another.
(function () {
  "use strict";

  const MESSAGES = window.CSDFREPL_MESSAGES;
  const config = JSON.parse(document.getElementById("csdfrepl-config").textContent);
  const highlights = config.highlights || { states: {}, links: {} };

  // ---- Words ----

  function chooseLanguage() {
    try {
      const saved = window.localStorage.getItem("csdfrepl.language");
      if (saved && MESSAGES[saved]) return saved;
    } catch (e) { /* storage may be blocked */ }
    const wanted = navigator.languages || [navigator.language || "en"];
    for (const tag of wanted) {
      const base = String(tag).toLowerCase().split("-")[0];
      if (MESSAGES[base]) return base;
    }
    return "en";
  }
  let language = chooseLanguage();

  function t(id, args) {
    const text = (MESSAGES[language] && MESSAGES[language][id]) || MESSAGES.en[id] || id;
    return text.replace(/\{(\w+)\}/g, (whole, name) =>
      args && Object.prototype.hasOwnProperty.call(args, name) ? String(args[name]) : whole);
  }

  function errorText(resp) {
    if (resp.code && MESSAGES.en["error." + resp.code]) return t("error." + resp.code, resp.args || {});
    return t("error.generic", { message: resp.error || "" });
  }

  // ---- Transports ----

  async function wasmTransport() {
    const encoded = document.getElementById("csdfrepl-engine").textContent.trim();
    const bytes = Uint8Array.from(atob(encoded), (c) => c.charCodeAt(0));
    const stream = new Blob([bytes]).stream().pipeThrough(new DecompressionStream("gzip"));
    const wasm = await new Response(stream).arrayBuffer();
    const go = new window.Go();
    const { instance } = await WebAssembly.instantiate(wasm, go.importObject);
    go.run(instance);
    const engine = window.csdfreplEngine;
    return { send: async (request) => JSON.parse(engine.handle(JSON.stringify(request))) };
  }

  const transports = { wasm: wasmTransport };

  // ---- State ----

  const app = {
    transport: null,
    session: "",
    view: null,
    history: [],
    // entries[i] says how the engine's history entry i came about, and the
    // path that leads to it: {kind, event, line, parent, path}.
    entries: [],
    current: -1,
    // choice is the transition chosen whose values are awaited.
    choice: null,
    error: null,
    scale: 1,
  };

  async function send(request) {
    request.session = app.session;
    return app.transport.send(request);
  }

  async function refresh() {
    const read = await send({ command: "read" });
    if (!read.ok) { app.error = errorText(read); render(); return; }
    app.view = read.data;
    const history = await send({ command: "history" });
    app.history = history.ok ? history.data.history || [] : [];
    render();
    autoContinue();
  }

  // A state of no variables takes its values without asking.
  function autoContinue() {
    const v = app.view;
    if (v && v.mode === "values" && v.pending && (v.pending.group.vars || []).length === 0 && !app.error) {
      enterValues([]);
    }
  }

  async function choose(transition) {
    app.error = null;
    const resp = await send({ command: "select", index: transition.index });
    if (!resp.ok) { app.error = errorText(resp); render(); return; }
    app.choice = { transition, src: app.view.state.state_id };
    await refresh();
  }

  async function enterValues(texts) {
    const values = "[" + texts.join(", ") + "]";
    const resp = await send({ command: "statevar", values });
    if (!resp.ok) { app.error = errorText(resp); render(); return; }
    app.error = null;
    const c = app.choice;
    if (!c) {
      app.entries.push({ kind: "initial", parent: null, path: { initial: values, steps: [] } });
    } else {
      const parent = app.entries[app.current];
      const step = { event: c.transition.event, src: c.src, dst: c.transition.dst, line: c.transition.line || 0, values };
      app.entries.push({
        kind: "step", event: c.transition.event, line: c.transition.line || 0, parent: app.current,
        path: { initial: parent.path.initial, steps: parent.path.steps.concat([step]) },
      });
    }
    app.current = app.entries.length - 1;
    app.choice = null;
    await refresh();
  }

  async function cancelChoice() {
    app.error = null;
    const resp = await send({ command: "back" });
    if (!resp.ok) { app.error = errorText(resp); render(); return; }
    app.choice = null;
    await refresh();
  }

  async function finish(end) {
    app.error = null;
    const resp = await send({ command: "end", index: end.index });
    if (!resp.ok) { app.error = errorText(resp); render(); return; }
    await refresh();
  }

  async function jump(index) {
    app.error = null;
    if (app.view && app.view.mode === "values") {
      const back = await send({ command: "back" });
      if (!back.ok) { app.error = errorText(back); render(); return; }
      app.choice = null;
    }
    const resp = await send({ command: "jump", index });
    if (!resp.ok) { app.error = errorText(resp); render(); return; }
    const from = app.entries[index];
    app.entries.push({ kind: "jump", from: index, parent: from.parent, path: from.path });
    app.current = app.entries.length - 1;
    await refresh();
  }

  async function goBack() {
    const mode = app.view && app.view.mode;
    if (mode === "values" || mode === "ended") { await cancelChoice(); return; }
    const entry = app.entries[app.current];
    if (entry && entry.parent !== null && entry.parent !== undefined) await jump(entry.parent);
  }

  function canGoBack() {
    const v = app.view;
    if (!v) return false;
    if (v.mode === "values" || v.mode === "ended") return app.history.length > 0;
    const entry = app.entries[app.current];
    return !!entry && entry.parent !== null && entry.parent !== undefined;
  }

  // ---- Saving the path ----

  function pathReport() {
    const entry = app.entries[app.current];
    const trace = app.history[app.current] ? app.history[app.current].trace : [];
    const steps = entry.path.steps.map((s) =>
      '{"event":' + JSON.stringify(s.event) + ',"src":' + JSON.stringify(s.src) + ',"dst":' + JSON.stringify(s.dst) +
      ',"line":' + s.line + ',"values":' + s.values + "}");
    return '{"paths":[{"trace":' + JSON.stringify(trace) + ',"replay":{"initial":' + entry.path.initial +
      ',"steps":[' + steps.join(",") + "]}}]}\n";
  }

  function savedName() {
    const base = (config.path || "diagram").split(/[\\/]/).pop().replace(/\.[^.]*$/, "");
    return base + ".path.json";
  }

  function save() {
    if (app.current < 0) return;
    const blob = new Blob([pathReport()], { type: "application/json" });
    const a = document.createElement("a");
    a.href = URL.createObjectURL(blob);
    a.download = savedName();
    document.body.appendChild(a);
    a.click();
    a.remove();
    setTimeout(() => URL.revokeObjectURL(a.href), 1000);
  }

  // ---- Rendering ----

  function el(tag, props, children) {
    const node = document.createElement(tag);
    if (props) {
      for (const [k, v] of Object.entries(props)) {
        if (k === "class") node.className = v;
        else if (k === "text") node.textContent = v;
        else if (k.startsWith("on")) node.addEventListener(k.slice(2), v);
        else node.setAttribute(k, v);
      }
    }
    for (const child of children || []) if (child) node.appendChild(child);
    return node;
  }

  function show(value) {
    return JSON.stringify(value);
  }

  function clause(text, noneId) {
    return text === "(none)" || text === "" ? t(noneId) : text;
  }

  function render() {
    document.documentElement.lang = language;
    document.getElementById("title").textContent = t("app.title", { name: config.title });
    document.title = t("app.title", { name: config.title });
    for (const node of document.querySelectorAll("[data-msg]")) node.textContent = t(node.dataset.msg);
    document.getElementById("legend").textContent = config.highlighted ? t("diagram.legend") : t("diagram.noHighlights");
    document.getElementById("back").disabled = !canGoBack();
    document.getElementById("save").disabled = app.current < 0;
    document.getElementById("save").title = t("history.saveHint", { file: savedName(), diagram: (config.path || "").split(/[\\/]/).pop() });
    renderHistory();
    renderControls();
    renderDiagram();
  }

  function valuesList(values, before) {
    if (!values || values.length === 0) return el("p", { class: "hint", text: t("history.noValues") });
    const list = el("dl", { class: "values" });
    for (const v of values) {
      const prev = before ? before.find((b) => b.name === v.name) : undefined;
      const changed = before && (!prev || show(prev.value) !== show(v.value));
      list.appendChild(el("dt", { text: v.name }));
      const dd = el("dd", { text: show(v.value) });
      if (changed) {
        dd.classList.add("changed");
        dd.title = t("history.changed");
      }
      list.appendChild(dd);
    }
    return list;
  }

  function renderHistory() {
    const list = document.getElementById("history");
    list.replaceChildren();
    if (app.history.length === 0) list.appendChild(el("li", { class: "hint", text: t("history.empty") }));
    app.history.forEach((h, i) => {
      const entry = app.entries[i] || { kind: "initial", parent: null };
      let label;
      if (entry.kind === "step") label = t("history.step", { event: entry.event, state: h.state.state_name });
      else if (entry.kind === "jump") label = t("history.jump", { step: entry.from + 1, state: h.state.state_name });
      else label = t("history.initial", { state: h.state.state_name });
      const parent = entry.kind === "step" ? app.history[entry.parent] : null;
      const head = el("div", null, [
        el("span", { class: "step-number", text: t("history.stepNumber", { step: i + 1 }) }),
        el("span", { text: label }),
        i === app.current ? el("span", { class: "here", text: t("history.current") }) : null,
      ]);
      const item = el("li", {
        class: i === app.current ? "current" : "",
        tabindex: "0",
        onclick: () => jump(i),
        onkeydown: (e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); jump(i); } },
      }, [head, valuesList(h.state.values, parent ? parent.state.values : null)]);
      list.appendChild(item);
    });
    const current = list.querySelector("li.current");
    if (current) current.scrollIntoView({ block: "nearest" });
    const trace = app.history[app.current] ? app.history[app.current].trace : [];
    document.getElementById("trace").textContent = trace.length ? trace.join(" → ") : t("trace.empty");
  }

  function clausesList(rows) {
    const list = el("dl", { class: "clauses" });
    for (const [label, text] of rows) {
      list.appendChild(el("dt", { text: label }));
      list.appendChild(el("dd", { text }));
    }
    return list;
  }

  function errorBox() {
    return app.error ? el("div", { class: "error", role: "alert", text: app.error }) : null;
  }

  function renderControls() {
    const root = document.getElementById("controls");
    root.replaceChildren();
    const v = app.view;
    if (!v) return;
    if (v.mode === "values") { renderValueForm(root, v.pending); return; }
    const state = v.state;
    if (v.mode === "ended") {
      root.appendChild(el("p", { class: "notice", text: t("controls.ended", { state: state.state_name }) }));
      root.appendChild(errorBox());
      return;
    }
    root.appendChild(el("h2", { text: t("controls.title") }));
    root.appendChild(el("p", { class: "hint", text: t("controls.now", { state: state.state_name }) }));
    root.appendChild(errorBox());
    const transitions = v.transitions || [];
    const ends = v.ends || [];
    if (transitions.length === 0 && ends.length === 0) {
      root.appendChild(el("p", { class: "error", text: t("controls.deadlock", { state: state.state_name }) }));
      return;
    }
    for (const tr of transitions) {
      root.appendChild(el("button", { type: "button", class: "choice", onclick: () => choose(tr) }, [
        el("div", { class: "event", text: tr.event }),
        clausesList([
          [t("controls.when"), clause(tr.guard, "clause.noGuard")],
          [t("controls.then"), clause(tr.post, "clause.noPost")],
          [t("controls.goesTo"), tr.dst_name],
        ]),
      ]));
    }
    if (ends.length > 0) {
      root.appendChild(el("h3", { text: t("controls.ends") }));
      for (const end of ends) {
        root.appendChild(el("button", { type: "button", class: "choice", onclick: () => finish(end) }, [
          el("div", { class: "event", text: t("controls.finish") }),
          clausesList([[t("controls.when"), clause(end.guard, "clause.noGuard")]]),
        ]));
      }
    }
  }

  function renderValueForm(root, pending) {
    const group = pending.group;
    const vars = group.vars || [];
    root.appendChild(el("h2", { text: t("values.title", { state: group.state_name }) }));
    const rows = [];
    if (app.choice) rows.push([t("controls.event"), app.choice.transition.event]);
    if (pending.previous) rows.push([t("controls.when"), clause(pending.guard, "clause.noGuard")]);
    rows.push([t("controls.then"), clause(pending.post, "clause.noPost")]);
    root.appendChild(clausesList(rows));
    root.appendChild(errorBox());
    const previous = pending.previous ? pending.previous.values || [] : [];
    const inputs = [];
    const form = el("form", {
      onsubmit: (e) => { e.preventDefault(); enterValues(inputs.map((i) => i.value)); },
    });
    if (vars.length === 0) form.appendChild(el("p", { class: "hint", text: t("values.none", { state: group.state_name }) }));
    else form.appendChild(el("p", { class: "hint", text: t("values.hint") }));
    vars.forEach((name, i) => {
      const id = "value-" + i;
      const before = previous.find((p) => p.name === name);
      const input = el("input", { type: "text", id, autocomplete: "off", spellcheck: "false" });
      input.value = before ? show(before.value) : "";
      inputs.push(input);
      form.appendChild(el("label", { for: id, text: name }));
      form.appendChild(input);
    });
    const buttons = el("div", { class: "row" }, [
      el("button", { type: "submit", class: "primary", text: t("values.continue") }),
      app.history.length > 0 ? el("button", { type: "button", text: t("values.cancel"), onclick: cancelChoice }) : null,
    ]);
    form.appendChild(buttons);
    root.appendChild(form);
    const first = inputs.find((i) => i.value === "") || inputs[0];
    if (first) first.focus({ preventScroll: true });
  }

  // ---- The drawing ----

  const marks = ["csdf-current", "csdf-pending", "csdf-choosable", "csdf-visited", "csdf-visited-state"];

  function svgElement(id) {
    return id ? document.getElementById(id) : null;
  }

  function linkOf(line) {
    return line ? svgElement(highlights.links[String(line)]) : null;
  }

  function renderDiagram() {
    for (const mark of marks) {
      for (const node of document.querySelectorAll("#diagram ." + mark)) node.classList.remove(mark);
    }
    const v = app.view;
    if (!v) return;
    for (const entry of app.entries) {
      const link = entry.kind === "step" ? linkOf(entry.line) : null;
      if (link) link.classList.add("csdf-visited");
    }
    for (const h of app.history) {
      const node = svgElement(highlights.states[h.state.state_id]);
      if (node) node.classList.add("csdf-visited-state");
    }
    if (app.entries.length > 0) {
      const start = svgElement(highlights.start);
      if (start) start.classList.add("csdf-visited");
    }
    if (v.mode === "values") {
      const target = svgElement(highlights.states[v.pending.group.state_id]);
      if (target) target.classList.add("csdf-pending");
      if (v.pending.previous) {
        const from = svgElement(highlights.states[v.pending.previous.state_id]);
        if (from) from.classList.add("csdf-current");
      }
      const link = app.choice ? linkOf(app.choice.transition.line) : svgElement(highlights.start);
      if (link) link.classList.add("csdf-choosable");
      return;
    }
    const current = svgElement(highlights.states[v.state.state_id]);
    if (current) current.classList.add("csdf-current");
    if (v.mode === "command") {
      for (const tr of v.transitions || []) {
        const link = linkOf(tr.line);
        if (link) link.classList.add("csdf-choosable");
      }
    }
  }

  function onDiagramClick(e) {
    const v = app.view;
    if (!v || v.mode !== "command") return;
    const group = e.target.closest("g.csdf-choosable");
    if (!group) return;
    const tr = (v.transitions || []).find((x) => x.line && highlights.links[String(x.line)] === group.id);
    if (tr) choose(tr);
  }

  // Zoom by scaling the drawing; pan by dragging or scrolling.
  function setupDiagram() {
    const box = document.getElementById("diagram");
    const svg = box.querySelector("svg");
    box.addEventListener("click", onDiagramClick);
    if (!svg) return;
    const width = parseFloat(svg.getAttribute("width")) || svg.getBoundingClientRect().width;
    const height = parseFloat(svg.getAttribute("height")) || svg.getBoundingClientRect().height;
    const apply = () => {
      svg.style.width = width * app.scale + "px";
      svg.style.height = height * app.scale + "px";
    };
    const fit = () => {
      const w = box.clientWidth || width;
      app.scale = Math.min(1, w / width);
      apply();
    };
    document.getElementById("zoom-in").addEventListener("click", () => { app.scale *= 1.25; apply(); });
    document.getElementById("zoom-out").addEventListener("click", () => { app.scale /= 1.25; apply(); });
    document.getElementById("zoom-reset").addEventListener("click", fit);
    let drag = null;
    box.addEventListener("pointerdown", (e) => {
      if (e.target.closest("g.csdf-choosable")) return;
      drag = { x: e.clientX, y: e.clientY, left: box.scrollLeft, top: box.scrollTop };
    });
    window.addEventListener("pointermove", (e) => {
      if (!drag) return;
      box.scrollLeft = drag.left - (e.clientX - drag.x);
      box.scrollTop = drag.top - (e.clientY - drag.y);
    });
    window.addEventListener("pointerup", () => { drag = null; });
    fit();
  }

  function setupLanguages() {
    const select = document.getElementById("language");
    for (const code of Object.keys(MESSAGES)) {
      const option = el("option", { value: code, text: MESSAGES[code]["language.name"] });
      if (code === language) option.selected = true;
      select.appendChild(option);
    }
    select.addEventListener("change", () => {
      language = select.value;
      try { window.localStorage.setItem("csdfrepl.language", language); } catch (e) { /* storage may be blocked */ }
      render();
    });
  }

  function showWarnings(warnings) {
    if (!warnings || warnings.length === 0) return;
    const box = document.getElementById("warnings");
    box.appendChild(el("details", null, [
      el("summary", { text: t("app.warnings") }),
      el("pre", { text: warnings.join("\n") }),
    ]));
  }

  function fail(message) {
    const root = document.getElementById("controls");
    root.replaceChildren(el("p", { class: "error", role: "alert", text: t("app.failed", { message }) }));
  }

  async function start() {
    setupLanguages();
    render();
    document.getElementById("controls").replaceChildren(el("p", { class: "hint", text: t("app.loading") }));
    document.getElementById("back").addEventListener("click", goBack);
    document.getElementById("save").addEventListener("click", save);
    setupDiagram();
    try {
      app.transport = await transports[config.transport || "wasm"]();
      const created = await app.transport.send({
        command: "session_new", path: config.path, content: config.diagram, includes: config.includes || undefined,
      });
      if (!created.ok) { fail(created.error); return; }
      app.session = created.session;
      showWarnings(created.warnings);
      await refresh();
    } catch (e) {
      fail(String(e && e.message ? e.message : e));
    }
  }

  window.csdfrepl = { app, render };
  start();
})();
