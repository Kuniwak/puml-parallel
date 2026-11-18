// The spec animator. It drives one exploration session through a transport,
// an object whose send(request) answers a proto.Response as a Promise; the
// engine compiled to WebAssembly is one, a server would be another.
(function () {
  "use strict";

  const MESSAGES = window.CSDFREPL_MESSAGES;
  const config = JSON.parse(document.getElementById("csdfrepl-config").textContent);
  const highlights = config.highlights || { states: {}, links: {} };
  const families = highlights.families || {};
  // localLinks says which move each link of a local diagram draws, by the
  // link's element ID.
  const localLinks = {};
  for (const [map, f] of Object.entries(families)) {
    for (const [edge, id] of Object.entries(f.edges || {})) localLinks[id] = { map, edge: Number(edge) };
  }
  // A list longer than this starts with its groups closed.
  const OPEN_LIMIT = 20;
  // A local state with more instances than this shows their number.
  const CHIP_LIMIT = 3;

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

  // ---- Python ----

  // loadPython loads Pyodide from python.url, keeps python.files in it and
  // sets globalThis.csdfreplPython, the host the engine asks csdfbmc next
  // through (pyhost.Global).
  async function loadPython(python) {
    await new Promise((resolve, reject) => {
      const script = document.createElement("script");
      script.src = python.url + "pyodide.js";
      script.onload = resolve;
      script.onerror = () => reject(new Error("cannot load " + script.src));
      document.head.appendChild(script);
    });
    const py = await window.loadPyodide({ indexURL: python.url });
    const writeFile = (path, text) => {
      const dir = path.slice(0, path.lastIndexOf("/"));
      if (dir) py.FS.mkdirTree(dir);
      py.FS.writeFile(path, text);
    };
    for (const [path, text] of Object.entries(python.files)) writeFile(path, text);
    py.FS.mkdirTree(python.work);
    py.FS.chdir(python.work);
    const sys = py.pyimport("sys");
    sys.path.insert(0, python.lib);
    sys.path.insert(0, python.work);
    const run = py.pyimport("csdfbmc.inprocess").run;
    window.csdfreplPython = {
      checker: JSON.stringify(python.checker),
      writeFile,
      remove: (path) => py.FS.unlink(path),
      run: (args) => {
        const result = run(py.toPy(args));
        const [code, stdout, stderr] = result.toJs();
        result.destroy();
        return JSON.stringify({ code, stdout, stderr });
      },
    };
  }

  // ---- State ----

  // The tabs of the controls: choose takes everything by a click, details
  // shows the clauses and takes every value typed.
  const TABS = ["choose", "details"];

  // chooseTab is the tab last chosen, else choose when a checker suggests
  // values to click, else details, where the values are typed.
  function chooseTab() {
    try {
      const saved = window.localStorage.getItem("csdfrepl.tab");
      if (TABS.includes(saved)) return saved;
    } catch (e) { /* storage may be blocked */ }
    return config.python ? "choose" : "details";
  }

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
    // What narrows the list of transitions: the words in the filter box,
    // and a local edge clicked in the drawing, {map, edge}.
    filterText: "",
    edgeFilter: null,
    // openGroups are the keys of the groups of the list the user opened or
    // closed, by whether they are open.
    openGroups: new Map(),
    // expanded are the local states, "map/state", whose instances are all
    // shown.
    expanded: new Set(),
    // hovered is the transition whose button the pointer is over.
    hovered: null,
    // suggesting says the page's Python is ready to suggest values.
    suggesting: false,
    tab: chooseTab(),
  };

  function setTab(tab) {
    app.tab = tab;
    try { window.localStorage.setItem("csdfrepl.tab", tab); } catch (e) { /* storage may be blocked */ }
    renderControls();
  }

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

  // A state whose values the post determines, all of them, takes them
  // without asking; so does a state of no variables.
  function autoContinue() {
    const v = app.view;
    if (!v || v.mode !== "values" || !v.pending || app.error) return;
    const vars = v.pending.group.vars || [];
    const values = vars.map((name) => determinedValue(v.pending, name));
    if (values.some((value) => value === undefined)) return;
    enterValues(values.map(show), vars.length > 0);
  }

  // determinedValue is the value the post determines of name, undefined
  // when it determines none.
  function determinedValue(pending, name) {
    const d = (pending.determined || []).find((v) => v.name === name);
    return d ? d.value : undefined;
  }

  async function choose(transition) {
    app.error = null;
    const resp = await send({ command: "select", index: transition.index });
    if (!resp.ok) { app.error = errorText(resp); render(); return; }
    app.choice = { transition, src: app.view.state.state_id };
    app.edgeFilter = null;
    app.hovered = null;
    await refresh();
  }

  async function enterValues(texts, auto) {
    const values = "[" + texts.join(", ") + "]";
    const resp = await send({ command: "statevar", values });
    if (!resp.ok) { app.error = errorText(resp); render(); return; }
    app.error = null;
    const c = app.choice;
    if (!c) {
      app.entries.push({ kind: "initial", auto, parent: null, path: { initial: values, steps: [] } });
    } else {
      const parent = app.entries[app.current];
      const step = { event: c.transition.event, src: c.src, dst: c.transition.dst, line: c.transition.line || 0, values };
      app.entries.push({
        kind: "step", auto, event: c.transition.event, line: c.transition.line || 0, moves: c.transition.moves || [], parent: app.current,
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
    app.edgeFilter = null;
    app.hovered = null;
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
    document.getElementById("legend").textContent = !config.highlighted ? t("diagram.noHighlights")
      : Object.keys(families).length > 0 ? t("diagram.legend") + " " + t("diagram.legendFamilies") : t("diagram.legend");
    document.getElementById("back").disabled = !canGoBack();
    document.getElementById("save").disabled = app.current < 0;
    document.getElementById("save").title = t("history.saveHint", { file: savedName(), diagram: (config.path || "").split(/[\\/]/).pop() });
    renderHistory();
    renderControls();
    if (app.refillBlockChooser) app.refillBlockChooser();
    renderDiagram();
    centreOnStep();
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
        entry.auto ? el("span", { class: "auto", text: t("history.auto") }) : null,
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
    return app.error ? el("div", { class: "error", role: "alert", text: app.error }) : document.createTextNode("");
  }

  function renderControls() {
    const root = document.getElementById("controls");
    root.replaceChildren();
    const v = app.view;
    if (!v) return;
    root.appendChild(tabBar());
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
    if (transitions.some((tr) => (tr.moves || []).length > 0)) {
      renderGroupedTransitions(root, transitions);
    } else {
      for (const tr of transitions) root.appendChild(choiceButton(tr));
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

  function tabBar() {
    return el("div", { class: "tabs", role: "tablist" }, TABS.map((tab) => el("button", {
      type: "button", role: "tab", id: "tab-" + tab, class: tab === app.tab ? "tab selected" : "tab",
      "aria-selected": String(tab === app.tab), text: t("tabs." + tab), onclick: () => setTab(tab),
    })));
  }

  // choiceButton is a transition to click: in the details tab with its
  // clauses.
  function choiceButton(tr) {
    const rows = app.tab === "details"
      ? [[t("controls.when"), clause(tr.guard, "clause.noGuard")], [t("controls.then"), clause(tr.post, "clause.noPost")], [t("controls.goesTo"), tr.dst_name]]
      : [[t("controls.goesTo"), tr.dst_name]];
    return el("button", {
      type: "button", class: "choice", onclick: () => choose(tr),
      onmouseenter: () => hover(tr), onmouseleave: () => hover(null),
      onfocus: () => hover(tr), onblur: () => hover(null),
    }, [
      el("div", { class: "event", text: tr.event }),
      clausesList(rows),
    ]);
  }

  function hover(tr) {
    if (app.hovered === tr) return;
    app.hovered = tr;
    markHovered();
  }

  // ---- The list of a promoted diagram's transitions ----

  function takesMove(tr, filter) {
    return (tr.moves || []).some((m) => m.map === filter.map && m.local_edge === filter.edge);
  }

  function matchesText(tr, words) {
    if (words.length === 0) return true;
    const text = [tr.event, tr.guard, tr.post, tr.dst_name].join(" ").toLowerCase();
    return words.every((w) => text.includes(w));
  }

  function moveName(m) {
    return m.parallel > 0 ? t("moves.edgeParallel", { src: m.src, dst: m.dst, n: m.parallel + 1 }) : t("moves.edge", { src: m.src, dst: m.dst });
  }

  // groupTransitions groups transitions by the family of each of their
  // moves, then by its local edge; a sync is in the group of each move.
  // Those of no move, a controller's own, are in a group of their own.
  function groupTransitions(transitions) {
    const byMap = new Map();
    const own = [];
    for (const tr of transitions) {
      const moves = tr.moves || [];
      if (moves.length === 0) { own.push(tr); continue; }
      for (const m of moves) {
        if (!byMap.has(m.map)) byMap.set(m.map, new Map());
        const edges = byMap.get(m.map);
        if (!edges.has(m.local_edge)) edges.set(m.local_edge, { move: m, transitions: [] });
        const group = edges.get(m.local_edge);
        if (!group.transitions.includes(tr)) group.transitions.push(tr);
      }
    }
    return { byMap, own };
  }

  function renderGroupedTransitions(root, transitions) {
    const input = el("input", {
      type: "search", class: "filter", placeholder: t("controls.filter"), "aria-label": t("controls.filter"),
      autocomplete: "off", spellcheck: "false",
    });
    input.value = app.filterText;
    const list = el("div", { class: "groups" });
    input.addEventListener("input", () => { app.filterText = input.value; renderTransitionGroups(list, transitions); });
    if (app.tab === "details") root.appendChild(input);
    root.appendChild(list);
    renderTransitionGroups(list, transitions);
  }

  // groupBox is a group of the list, open when the filters narrow the list
  // to it (forceOpen), else as the user left it, else when what holds it
  // lists no more than OPEN_LIMIT transitions (outer).
  function groupBox(key, summary, count, fill, forceOpen, outer, onOpen) {
    const remembered = app.openGroups.get(key);
    const open = forceOpen || (remembered !== undefined ? remembered : outer <= OPEN_LIMIT);
    const body = el("div", { class: "group-body" });
    const box = el("details", { class: "group" }, [
      el("summary", null, [el("span", { text: summary }), el("span", { class: "count", text: t("controls.count", { n: count }) })]),
      body,
    ]);
    let filled = false;
    const fillOnce = () => { if (!filled) { filled = true; fill(body); } };
    if (open) { box.open = true; fillOnce(); }
    box.addEventListener("toggle", () => {
      app.openGroups.set(key, box.open);
      if (box.open) fillOnce();
      if (box.open && onOpen) onOpen();
    });
    return box;
  }

  function renderTransitionGroups(list, transitions) {
    list.replaceChildren();
    const words = app.tab !== "details" ? [] : app.filterText.toLowerCase().split(/\s+/).filter((w) => w);
    const filter = app.edgeFilter;
    const shown = transitions.filter((tr) => matchesText(tr, words) && (!filter || takesMove(tr, filter)));
    const narrowed = words.length > 0 || !!filter;
    if (filter) {
      const sample = transitions.flatMap((tr) => tr.moves || []).find((m) => m.map === filter.map && m.local_edge === filter.edge);
      list.appendChild(el("div", { class: "edge-filter" }, [
        el("span", { text: t("controls.edgeFilter", { map: filter.map, edge: sample ? moveName(sample) : String(filter.edge) }) }),
        el("button", { type: "button", text: t("controls.clearFilter"), onclick: () => { app.edgeFilter = null; renderTransitionGroups(list, transitions); renderDiagram(); } }),
      ]));
    }
    if (shown.length === 0) { list.appendChild(el("p", { class: "hint", text: t("controls.noMatch") })); return; }
    const { byMap, own } = groupTransitions(shown);
    for (const [map, edges] of byMap) {
      const count = new Set([...edges.values()].flatMap((g) => g.transitions)).size;
      list.appendChild(groupBox("map:" + map, t("controls.family", { map }), count, (body) => {
        for (const [edge, group] of edges) {
          body.appendChild(groupBox("edge:" + map + "/" + edge, moveName(group.move), group.transitions.length, (inner) => {
            for (const tr of group.transitions) inner.appendChild(choiceButton(tr));
          }, narrowed && edges.size === 1, count));
        }
      }, narrowed, transitions.length, () => showBlocks([map])));
    }
    if (own.length > 0) {
      list.appendChild(groupBox("own", t("controls.own"), own.length, (body) => {
        for (const tr of own) body.appendChild(choiceButton(tr));
      }, narrowed, transitions.length));
    }
  }

  // renderValueForm asks for the values of the pending state: in the
  // choose tab by the checker's suggestions, in the details tab typed. A
  // promoted diagram's prompt speaks of the local diagrams, not the mode.
  function renderValueForm(root, pending) {
    const group = pending.group;
    const promoted = (pending.families || []).length > 0;
    root.appendChild(el("h2", { text: promoted ? t("values.titleNext") : t("values.title", { state: group.state_name }) }));
    if (app.tab === "details") renderValueDetails(root, pending);
    else renderValueChoices(root, pending);
  }

  function cancelButton() {
    return app.history.length > 0 ? el("button", { type: "button", text: t("values.cancel"), onclick: cancelChoice }) : null;
  }

  function renderValueChoices(root, pending) {
    const vars = pending.group.vars || [];
    if (app.choice) root.appendChild(clausesList([[t("controls.event"), app.choice.transition.event]]));
    root.appendChild(errorBox());
    const determined = pending.determined || [];
    if (determined.length > 0 && !app.error) {
      root.appendChild(el("p", { class: "hint", text: t("values.determined") }));
      root.appendChild(valuesList(determined.map((d) => ({ name: varLabel(pending, d.name), value: d.value })), null));
    }
    const previous = pending.previous ? pending.previous.values || [] : [];
    const suggestions = pending.suggestions || [];
    if (app.suggesting && !pending.suggest_error && suggestions.length > 0) {
      renderSuggestions(root, vars, previous, suggestions);
    } else {
      const why = !app.suggesting ? "choose.noChecker" : pending.suggest_error ? "choose.failed" : "choose.none";
      root.appendChild(el("p", { class: "notice", text: t(why) }));
      if (pending.suggest_error) {
        root.appendChild(el("details", null, [el("summary", { text: t("suggest.why") }), el("pre", { class: "error", text: pending.suggest_error })]));
      }
      root.appendChild(el("div", { class: "row" }, [
        el("button", { type: "button", class: "primary", id: "to-details", text: t("choose.toDetails"), onclick: () => setTab("details") }),
      ]));
    }
    const cancel = cancelButton();
    if (cancel) root.appendChild(el("div", { class: "row" }, [cancel]));
  }

  // varLabel is how the prompt names the variable name: a map of a
  // family's instances by the family.
  function varLabel(pending, name) {
    return (pending.families || []).includes(name) ? t("values.instances", { map: name }) : name;
  }

  function renderValueDetails(root, pending) {
    const group = pending.group;
    const vars = group.vars || [];
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
      const fixed = determinedValue(pending, name);
      const input = el("input", { type: "text", id, autocomplete: "off", spellcheck: "false" });
      input.value = fixed !== undefined ? show(fixed) : before ? show(before.value) : "";
      inputs.push(input);
      form.appendChild(el("label", { for: id, text: varLabel(pending, name) }));
      form.appendChild(input);
    });
    form.appendChild(el("div", { class: "row" }, [
      el("button", { type: "submit", class: "primary", text: t("values.continue") }),
      cancelButton(),
    ]));
    root.appendChild(form);
    const first = inputs.find((i) => i.value === "") || inputs[0];
    if (first) first.focus({ preventScroll: true });
  }

  // renderSuggestions shows the values the checker admits as buttons, each
  // saying what changes from before.
  function renderSuggestions(root, vars, previous, suggestions) {
    root.appendChild(el("h3", { text: t("suggest.title") }));
    root.appendChild(el("p", { class: "hint", text: t("suggest.hint") }));
    const list = el("div", { class: "suggestions" });
    suggestions.forEach((s, i) => {
      const lines = [];
      const params = Object.entries(s.params || {});
      if (params.length > 0) {
        lines.push(t("suggest.params", { params: params.map(([k, v]) => k + " = " + show(v)).join(", ") }));
      }
      const changes = changesOf(vars, previous, s.values);
      const shown = changes.slice(0, CHANGE_LIMIT);
      if (changes.length === 0) lines.push(t("suggest.nothingChanges"));
      lines.push(...shown);
      if (changes.length > shown.length) lines.push(t("suggest.more", { n: changes.length - shown.length }));
      list.appendChild(el("button", {
        type: "button", class: "choice suggestion", id: "suggestion-" + i,
        onclick: () => enterValues(s.values.map(show)),
      }, lines.map((line) => el("div", { text: line }))));
    });
    root.appendChild(list);
  }

  // A suggestion lists at most this many of its changes.
  const CHANGE_LIMIT = 8;

  // changesOf says, line by line, what values make of the variables vars
  // held in previous: a map entry by entry.
  function changesOf(vars, previous, values) {
    const lines = [];
    vars.forEach((name, i) => {
      const before = previous.find((p) => p.name === name);
      const after = values[i];
      if (!before) { lines.push(t("suggest.set", { name, value: show(after) })); return; }
      if (show(before.value) === show(after)) return;
      if (isMap(before.value) && isMap(after)) {
        const keys = new Set(Object.keys(before.value).concat(Object.keys(after)));
        for (const key of keys) {
          const entry = name + "[" + key + "]";
          if (!(key in after)) lines.push(t("suggest.removed", { name: entry }));
          else if (!(key in before.value)) lines.push(t("suggest.set", { name: entry, value: show(after[key]) }));
          else if (show(before.value[key]) !== show(after[key])) {
            lines.push(t("suggest.change", { name: entry, before: show(before.value[key]), after: show(after[key]) }));
          }
        }
        return;
      }
      lines.push(t("suggest.change", { name, before: show(before.value), after: show(after) }));
    });
    return lines;
  }

  function isMap(value) {
    return value !== null && typeof value === "object" && !Array.isArray(value);
  }

  // ---- The drawing ----

  const marks = ["csdf-occupied", "csdf-current", "csdf-pending", "csdf-choosable", "csdf-visited", "csdf-visited-state", "csdf-filtered"];

  function svgElement(id) {
    return id ? document.getElementById(id) : null;
  }

  function linkOf(line) {
    return line ? svgElement(highlights.links[String(line)]) : null;
  }

  function localLinkOf(move) {
    const f = families[move.map];
    return f && f.edges ? svgElement(f.edges[String(move.local_edge)]) : null;
  }

  // linksOf are the links that draw tr: its own, or those of its moves.
  function linksOf(tr) {
    const out = [];
    const own = linkOf(tr.line);
    if (own) out.push(own);
    for (const m of tr.moves || []) {
      const link = localLinkOf(m);
      if (link) out.push(link);
    }
    return out;
  }

  function markHovered() {
    for (const node of document.querySelectorAll("#diagram .csdf-hovered")) node.classList.remove("csdf-hovered");
    if (app.hovered) for (const link of linksOf(app.hovered)) link.classList.add("csdf-hovered");
  }

  // The instances at each local state, as chips on its box.
  function renderInstances(v) {
    const svg = document.querySelector("#diagram svg");
    if (!svg) return;
    const old = svg.querySelector("#csdf-overlay");
    if (old) old.remove();
    const views = v.families || [];
    if (views.length === 0) return;
    const ns = "http://www.w3.org/2000/svg";
    const overlay = document.createElementNS(ns, "g");
    overlay.id = "csdf-overlay";
    svg.appendChild(overlay);
    for (const fv of views) {
      const f = families[fv.map];
      if (!f) continue;
      for (const [state, ids] of Object.entries(fv.instances || {})) {
        const box = svgElement(f.states[state]);
        if (!box || ids.length === 0) continue;
        const key = fv.map + "/" + state;
        const lines = ids.length <= CHIP_LIMIT ? [ids.join(", ")]
          : app.expanded.has(key) ? ids.slice() : [t("instances.many", { n: ids.length })];
        const bbox = box.getBBox();
        const chip = document.createElementNS(ns, "g");
        chip.setAttribute("class", "csdf-chip");
        chip.dataset.key = key;
        const title = document.createElementNS(ns, "title");
        title.textContent = t("instances.title", { state, ids: ids.join(", ") });
        chip.appendChild(title);
        const rect = document.createElementNS(ns, "rect");
        chip.appendChild(rect);
        lines.forEach((line, i) => {
          const text = document.createElementNS(ns, "text");
          text.setAttribute("x", bbox.x + bbox.width + 6);
          text.setAttribute("y", bbox.y + 12 + i * 14);
          text.textContent = line;
          chip.appendChild(text);
        });
        overlay.appendChild(chip);
        const inner = chip.getBBox();
        rect.setAttribute("x", inner.x - 4);
        rect.setAttribute("y", inner.y - 2);
        rect.setAttribute("width", inner.width + 8);
        rect.setAttribute("height", inner.height + 4);
        rect.setAttribute("rx", 4);
        if (ids.length > CHIP_LIMIT) chip.classList.add("csdf-chip-more");
      }
    }
  }

  function renderDiagram() {
    for (const mark of marks) {
      for (const node of document.querySelectorAll("#diagram ." + mark)) node.classList.remove(mark);
    }
    const v = app.view;
    if (!v) return;
    for (const entry of app.entries) {
      if (entry.kind !== "step") continue;
      const link = linkOf(entry.line);
      if (link) link.classList.add("csdf-visited");
      for (const m of entry.moves || []) {
        const local = localLinkOf(m);
        if (local) local.classList.add("csdf-visited");
      }
    }
    renderInstances(v);
    for (const o of [...occupiedStates(v), ...emptyInits(v)]) o.node.classList.add("csdf-occupied");
    markHovered();
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
      const links = app.choice ? linksOf(app.choice.transition) : [svgElement(highlights.start)];
      for (const link of links) if (link) link.classList.add("csdf-choosable");
      return;
    }
    const current = svgElement(highlights.states[v.state.state_id]);
    if (current) current.classList.add("csdf-current");
    if (v.mode === "command") {
      for (const tr of v.transitions || []) {
        for (const link of linksOf(tr)) link.classList.add("csdf-choosable");
      }
      const filter = app.edgeFilter;
      if (filter) {
        const link = localLinkOf({ map: filter.map, local_edge: filter.edge });
        if (link) link.classList.add("csdf-filtered");
      }
    }
  }

  function onDiagramClick(e) {
    const v = app.view;
    if (!v || v.mode !== "command") return;
    const chip = e.target.closest("g.csdf-chip-more");
    if (chip) {
      const key = chip.dataset.key;
      if (app.expanded.has(key)) app.expanded.delete(key); else app.expanded.add(key);
      renderInstances(v);
      return;
    }
    const group = e.target.closest("g.csdf-choosable");
    if (!group) return;
    const local = localLinks[group.id];
    if (local) {
      const same = app.edgeFilter && app.edgeFilter.map === local.map && app.edgeFilter.edge === local.edge;
      app.edgeFilter = same ? null : local;
      renderControls();
      renderDiagram();
      return;
    }
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
    app.drawing = { svg, viewBox: svg.getAttribute("viewBox") || `0 0 ${width} ${height}`, width, height };
    const apply = applyDrawing;
    // zoomTo keeps the point of the drawing at the middle of the pane
    // where it was, as far as the edges of the drawing allow.
    const zoomTo = (scale) => {
      const cx = (box.scrollLeft + box.clientWidth / 2) / app.scale;
      const cy = (box.scrollTop + box.clientHeight / 2) / app.scale;
      app.scale = scale;
      apply();
      box.scrollLeft = cx * scale - box.clientWidth / 2;
      box.scrollTop = cy * scale - box.clientHeight / 2;
    };
    setupBlockChooser();
    document.getElementById("zoom-in").addEventListener("click", () => zoomTo(app.scale * 1.25));
    document.getElementById("zoom-out").addEventListener("click", () => zoomTo(app.scale / 1.25));
    document.getElementById("zoom-reset").addEventListener("click", () => zoomTo(1));
    let drag = null;
    box.addEventListener("pointerdown", (e) => {
      if (e.target.closest("g.csdf-choosable, g.csdf-chip-more")) return;
      drag = { x: e.clientX, y: e.clientY, left: box.scrollLeft, top: box.scrollTop };
    });
    window.addEventListener("pointermove", (e) => {
      if (!drag) return;
      box.scrollLeft = drag.left - (e.clientX - drag.x);
      box.scrollTop = drag.top - (e.clientY - drag.y);
    });
    window.addEventListener("pointerup", () => { drag = null; });
    apply();
  }

  // focusOf is what the drawing centres on: the edges of the last step,
  // else the state being entered or the current one, unless that state is
  // larger than the pane (the mode of a promotion), then the start edge.
  function focusOf(v) {
    const last = app.entries.length > 0 ? app.entries[app.entries.length - 1] : null;
    if (app.block) {
      const inBlock = (m) => app.block.includes(m.map);
      const chosen = app.choice ? (app.choice.transition.moves || []).filter(inBlock) : [];
      const moved = last && last.kind === "step" ? (last.moves || []).filter(inBlock) : [];
      const links = [...chosen, ...moved].map(localLinkOf).filter(Boolean);
      if (links.length > 0) return links;
      const occupied = [...occupiedStates(v), ...emptyInits(v)].filter((o) => app.block.includes(o.map)).map((o) => o.node);
      if (occupied.length > 0) return occupied;
      return app.block.map(localInitOf).filter(Boolean);
    }
    if (last && last.kind === "step") {
      const links = [linkOf(last.line), ...(last.moves || []).map(localLinkOf)].filter(Boolean);
      if (links.length > 0) return links;
    }
    const box = document.getElementById("diagram");
    const id = v.mode === "values" ? v.pending.group.state_id : v.state && v.state.state_id;
    const state = svgElement(highlights.states[id]);
    if (state) {
      const r = state.getBoundingClientRect();
      if (r.width <= box.clientWidth && r.height <= box.clientHeight) return [state];
    }
    return [svgElement(highlights.start)].filter(Boolean);
  }

  // localInitOf is the box of the local start state of the family of map,
  // where an instance is before it exists: the end of the local start edge.
  function localInitOf(map) {
    const start = svgElement(families[map] && families[map].start);
    return start ? svgElement(start.getAttribute("data-entity-2")) : null;
  }

  // emptyInits are the local start states of the families with no
  // instance, where every one of them is.
  function emptyInits(v) {
    if (!v.families && v.mode !== "command") return [];
    const held = new Set((v.families || []).filter((fv) => Object.values(fv.instances || {}).some((ids) => ids.length > 0)).map((fv) => fv.map));
    return Object.keys(families).filter((m) => !held.has(m)).map((m) => ({ map: m, node: localInitOf(m) })).filter((o) => o.node);
  }

  // occupiedStates are the boxes of the local states that hold instances.
  function occupiedStates(v) {
    const out = [];
    for (const fv of v.families || []) {
      const f = families[fv.map];
      if (!f) continue;
      for (const [state, ids] of Object.entries(fv.instances || {})) {
        const node = ids.length > 0 ? svgElement(f.states[state]) : null;
        if (node) out.push({ map: fv.map, node });
      }
    }
    return out;
  }

  // centreOn scrolls the pane so that the box around nodes is in its
  // middle, as far as the edges of the drawing allow.
  function centreOn(nodes) {
    const box = document.getElementById("diagram");
    if (nodes.length === 0) { box.scrollLeft = 0; box.scrollTop = 0; return; }
    const rects = nodes.map((n) => n.getBoundingClientRect());
    const left = Math.min(...rects.map((r) => r.left));
    const right = Math.max(...rects.map((r) => r.right));
    const top = Math.min(...rects.map((r) => r.top));
    const bottom = Math.max(...rects.map((r) => r.bottom));
    const pane = box.getBoundingClientRect();
    const margin = 40;
    // A box wider (taller) than the pane shows its left (top) edge, where
    // its title is, rather than its empty middle.
    const x = right - left > box.clientWidth - 2 * margin ? left - margin : (left + right) / 2 - box.clientWidth / 2;
    const y = bottom - top > box.clientHeight - 2 * margin ? top - margin : (top + bottom) / 2 - box.clientHeight / 2;
    box.scrollLeft += x - pane.left;
    box.scrollTop += y - pane.top;
  }




  // centreOnStep centres the drawing once per step, not on every redraw.
  // On a promotion it first shows the blocks of the families the last
  // step moved, or, before any, of the first family that can move.
  function centreOnStep() {
    const v = app.view;
    if (!v) return;
    const key = app.entries.length + ":" + v.mode;
    if (app.centredAt === key) return;
    app.centredAt = key;
    showCurrent();
  }

  function blocksToShow(v) {
    const last = app.entries.length > 0 ? app.entries[app.entries.length - 1] : null;
    const moved = last && last.kind === "step" ? (last.moves || []).map((m) => m.map) : [];
    const chosen = app.choice ? (app.choice.transition.moves || []).map((m) => m.map) : [];
    const maps = [...new Set([...chosen, ...moved])].filter((m) => families[m]);
    if (maps.length > 0) return maps;
    if (app.block) return app.block;
    const occupied = [...new Set(occupiedStates(v).map((o) => o.map))].sort();
    if (occupied.length > 0) return [occupied[0]];
    const movable = (v.transitions || []).flatMap((tr) => tr.moves || []).map((m) => m.map).filter((m) => families[m]).sort();
    const first = movable[0] || Object.keys(families).sort()[0];
    return first ? [first] : [];
  }

  // showCurrent brings what the current step is about into the middle of
  // the pane, switching to its blocks on a promotion.
  function showCurrent() {
    const v = app.view;
    if (!v) return;
    if (Object.keys(families).length > 0) {
      const maps = blocksToShow(v);
      if (maps.length > 0) { showBlocks(maps); return; }
    }
    const id = v.mode === "values" ? v.pending.group.state_id : v.state && v.state.state_id;
    const state = svgElement(highlights.states[id]);
    const nodes = state ? [state] : focusOf(v);
    centreOn(nodes);
  }

  // ---- Showing one block at a time ----

  function viewBoxOf() {
    const d = app.drawing;
    if (!app.block) return d.viewBox.split(/[\s,]+/).map(Number);
    let x0 = Infinity, y0 = Infinity, x1 = -Infinity, y1 = -Infinity;
    for (const map of app.block) {
      const f = families[map];
      const ids = [...Object.values(f.states || {}), ...Object.values(f.edges || {}), f.start].filter(Boolean);
      for (const id of ids) {
        const node = svgElement(id);
        if (!node || !node.getBBox) continue;
        const b = node.getBBox();
        x0 = Math.min(x0, b.x); y0 = Math.min(y0, b.y);
        x1 = Math.max(x1, b.x + b.width); y1 = Math.max(y1, b.y + b.height);
      }
    }
    if (!isFinite(x0)) return d.viewBox.split(/[\s,]+/).map(Number);
    const pad = 60;
    return [x0 - pad, y0 - pad, x1 - x0 + 2 * pad, y1 - y0 + 2 * pad];
  }

  function viewWidth() { return viewBoxOf()[2]; }

  function applyDrawing() {
    const d = app.drawing;
    if (!d) return;
    const [x, y, w, h] = viewBoxOf();
    d.svg.setAttribute("viewBox", `${x} ${y} ${w} ${h}`);
    d.svg.style.width = w * app.scale + "px";
    d.svg.style.height = h * app.scale + "px";
  }

  // showBlocks shows only the blocks of maps (all of the drawing for
  // none) and centres on what is to be seen there.
  function showBlocks(maps) {
    const known = (maps || []).filter((m) => families[m]);
    app.block = known.length > 0 ? known : null;
    const select = document.getElementById("view-block");
    if (select) select.value = app.block ? app.block[0] : "";
    applyDrawing();
    const v = app.view;
    if (v) centreOn(focusOf(v));
  }

  function setupBlockChooser() {
    const select = document.getElementById("view-block");
    const maps = Object.keys(families).sort();
    if (!select || maps.length === 0) return;
    select.hidden = false;
    const fill = () => {
      const value = select.value;
      select.replaceChildren(el("option", { value: "", text: t("diagram.whole") }),
        ...maps.map((m) => el("option", { value: m, text: m })));
      select.value = value;
    };
    fill();
    app.refillBlockChooser = fill;
    select.addEventListener("change", () => showBlocks(select.value ? [select.value] : []));
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
    document.getElementById("show-current").addEventListener("click", showCurrent);
    setupDiagram();
    try {
      const engine = transports[config.transport || "wasm"]();
      if (config.python) {
        document.getElementById("controls").replaceChildren(el("p", { class: "hint", text: t("app.loadingChecker") }));
        try {
          const started = performance.now();
          await loadPython(config.python);
          app.suggesting = true;
          app.pythonLoadMs = performance.now() - started;
        } catch (e) {
          document.getElementById("warnings").appendChild(el("p", {
            class: "notice", role: "status", text: t("suggest.unavailable", { message: String(e && e.message ? e.message : e) }),
          }));
        }
      }
      app.transport = await engine;
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
