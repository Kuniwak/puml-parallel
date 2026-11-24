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
  // A local state with more instances than this shows the first of them
  // and the number of the others, until clicked.
  const PIECE_LIMIT = 6;
  // The colours of the pieces of the instances, each with the colour of
  // its text; none is the colour of the whole's piece.
  const PALETTE = [
    { fill: "#e69f00", text: "#000000" }, { fill: "#56b4e9", text: "#000000" },
    { fill: "#009e73", text: "#ffffff" }, { fill: "#f0e442", text: "#000000" },
    { fill: "#cc79a7", text: "#000000" }, { fill: "#d55e00", text: "#ffffff" },
    { fill: "#7f3c8d", text: "#ffffff" }, { fill: "#8c6d31", text: "#ffffff" },
  ];
  const WHOLE_COLOUR = { fill: "#0b57d0", text: "#ffffff" };
  const MORE_COLOUR = { fill: "#ffffff", text: "#333333" };

  // ---- Words ----

  function chooseLanguage() {
    // A link names the language as ?lang=ja; the tester's choice saved on
    // the page comes next.
    const asked = new URLSearchParams(window.location.search).get("lang");
    if (asked && MESSAGES[asked]) return asked;
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
    return "choose";
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
    // colours are the colours of the instances' pieces, by map and ID.
    colours: new Map(),
    // hovered is the transition whose button the pointer is over.
    hovered: null,
    // suggesting says the page's Python is ready to suggest values.
    suggesting: false,
    // admits says, by the state and the index of a transition out of it
    // (admitsKey), whether the checker admits any values after it: "none"
    // or "some"; a transition not asked yet, or whose asking failed, is
    // not there.
    admits: new Map(),
    tab: chooseTab(),
    // familyLabels are the labels of the local states of each family, by
    // map, as the views give them.
    familyLabels: new Map(),
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
    for (const fv of read.data.families || []) app.familyLabels.set(fv.map, fv.labels || {});
    const history = await send({ command: "history" });
    app.history = history.ok ? history.data.history || [] : [];
    render();
    autoContinue();
    checkAdmits();
  }

  function admitsKey(state, index) {
    return JSON.stringify(state) + "#" + index;
  }

  // checkAdmits asks the checker, one transition at a time and after the
  // choices are shown, whether any values satisfy each transition out of
  // the current state, and shows again those it says none do.
  async function checkAdmits() {
    const v = app.view;
    if (!app.suggesting || !v || v.mode !== "command") return;
    for (const tr of v.transitions || []) {
      const key = admitsKey(v.state, tr.index);
      if (app.admits.has(key)) continue;
      await new Promise((resolve) => setTimeout(resolve, 0));
      if (app.view !== v) return;
      const resp = await send({ command: "suggest", index: tr.index });
      if (app.view !== v) return;
      if (!resp.ok || !resp.data || !Array.isArray(resp.data.suggestions)) continue;
      const admits = resp.data.suggestions.length > 0 ? "some" : "none";
      app.admits.set(key, admits);
      if (admits === "none") { renderControls(); renderDiagram(); }
    }
  }

  // admitsNone says whether the checker said no values satisfy tr from the
  // current state.
  function admitsNone(tr) {
    return app.admits.get(admitsKey(app.view.state, tr.index)) === "none";
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

  // goToStart goes back to the first entry of the path to the current one,
  // as clicking it in the history pane does.
  async function goToStart() {
    if (canGoToStart()) await jump(chainOf(app.current)[0]);
  }

  function canGoToStart() {
    const v = app.view;
    if (!v || app.current < 0) return false;
    return v.mode === "values" || v.mode === "ended" || chainOf(app.current).length > 1;
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

  // tNodes is the message id with its {name} slots filled by the nodes
  // (or texts) of args, as a span.
  function tNodes(id, args) {
    const text = (MESSAGES[language] && MESSAGES[language][id]) || MESSAGES.en[id] || id;
    const out = el("span");
    let at = 0;
    for (const m of text.matchAll(/\{(\w+)\}/g)) {
      if (!Object.prototype.hasOwnProperty.call(args, m[1])) continue;
      out.appendChild(document.createTextNode(text.slice(at, m.index)));
      const arg = args[m[1]];
      out.appendChild(arg instanceof Node ? arg : document.createTextNode(String(arg)));
      at = m.index + m[0].length;
    }
    out.appendChild(document.createTextNode(text.slice(at)));
    return out;
  }

  // valueView shows value as a reader sees it: a record as a table of its
  // keys and values, a list as a list (as a table of rows when its items
  // are records of the same keys), a string without quotes. Given before
  // (compare), the parts that differ from before are marked changed.
  function valueView(value, before, compare) {
    const differs = (a, b) => show(a) !== show(b);
    const mark = (node, a, b) => {
      if (compare && differs(a, b)) { node.classList.add("changed"); node.title = t("history.changed"); }
      return node;
    };
    if (isMap(value)) {
      const keys = Object.keys(value);
      if (keys.length === 0) return el("span", { class: "value-scalar", text: "{}" });
      const inner = compare && isMap(before);
      const table = el("table", { class: "value-table" });
      for (const k of keys) {
        const prev = inner ? before[k] : undefined;
        const cell = el("td", null, [valueView(value[k], prev, inner && k in before)]);
        if (inner && !(k in before)) mark(cell, value[k], undefined);
        table.appendChild(el("tr", null, [el("th", { text: k }), cell]));
      }
      return compare && !inner ? mark(table, value, before) : table;
    }
    if (Array.isArray(value)) {
      if (value.length === 0) return mark(el("span", { class: "value-scalar", text: "[]" }), value, before);
      const keys = value.every(isMap) ? Object.keys(value[0]) : null;
      const sameKeys = keys && keys.length > 0 && value.every((x) => show(Object.keys(x)) === show(keys));
      const inner = compare && Array.isArray(before);
      if (sameKeys) {
        const table = el("table", { class: "value-table value-rows" }, [el("tr", null, keys.map((k) => el("th", { text: k })))]);
        value.forEach((row, i) => {
          const prev = inner && isMap(before[i]) ? before[i] : undefined;
          table.appendChild(el("tr", null, keys.map((k) =>
            el("td", null, [valueView(row[k], prev ? prev[k] : undefined, inner && !!prev && k in prev)]))));
        });
        return compare && !inner ? mark(table, value, before) : table;
      }
      const list = el("ul", { class: "value-list" }, value.map((x, i) =>
        el("li", null, [valueView(x, inner ? before[i] : undefined, inner && i < before.length)])));
      return compare && !inner ? mark(list, value, before) : list;
    }
    const text = typeof value === "string" ? value : show(value);
    return mark(el("span", { class: "value-scalar", text }), value, before);
  }

  // variableView shows the value of the variable name: the instances of
  // a family by instancesView, anything else by valueView.
  function variableView(name, value, before, compare) {
    if (isInstances(name, value)) return instancesView(name, value, isMap(before) ? before : undefined, compare);
    return valueView(value, before, compare);
  }

  function isFamily(name) {
    return app.familyLabels.has(name) || Object.prototype.hasOwnProperty.call(families, name);
  }

  // isInstance says whether value is an instance of a family as the
  // engine writes it: {"at": local state, "fields": {...}}.
  function isInstance(value) {
    return isMap(value) && Object.keys(value).length === 2 && typeof value.at === "string" && isMap(value.fields);
  }

  function isInstances(name, value) {
    return isFamily(name) && isMap(value) && Object.keys(value).length > 0 && Object.values(value).every(isInstance);
  }

  // instancesView shows the instances of the family map one by one, as
  // the history shows a state: where each is, then its fields.
  function instancesView(map, value, before, compare) {
    return el("ul", { class: "instances" }, Object.keys(value).sort().map((id) =>
      instanceItem(map, id, value[id], before ? before[id] : undefined, compare)));
  }

  // instanceItem is the instance id of map, inst, in its piece's colour:
  // the label of its local state, then its fields as the history lists
  // the values of a state. Given before (compare), what differs from
  // before is marked changed; a new instance is marked whole.
  function instanceItem(map, id, inst, before, compare) {
    const known = compare && isInstance(before);
    const dot = el("span", { class: "dot" });
    dot.style.background = pieceColour(map, id).fill;
    const labels = app.familyLabels.get(map) || {};
    const at = el("span", { class: "instance-at", text: labels[inst.at] || inst.at });
    if (known && before.at !== inst.at) { at.classList.add("changed"); at.title = t("history.changed"); }
    const fields = Object.entries(inst.fields).map(([name, value]) => ({ name, value }));
    const beforeFields = known ? Object.entries(before.fields).map(([name, value]) => ({ name, value })) : null;
    const item = el("li", { class: "instance" }, [
      el("div", { class: "instance-head" }, [dot, el("span", { class: "instance-id", text: id }), at]),
      fields.length > 0 ? valuesList(fields, beforeFields) : null,
    ]);
    if (compare && !known) { item.classList.add("changed"); item.title = t("history.changed"); }
    return item;
  }

  function clause(text, noneId) {
    return text === "(none)" || text === "" ? t(noneId) : text;
  }

  // ---- The clauses shown: the prose version's (-prose), else the engine's ----

  const proses = config.prose || {};
  const NONE_IDS = { guard: "clause.noGuard", post: "clause.noPost" };
  const CLAUSE_LABELS = { guard: "controls.when", post: "controls.then" };

  // shownProse is the prose version written in the page's language, else
  // the one of no language, else the English one, else any; null when
  // there is none.
  function shownProse() {
    const any = Object.values(proses)[0];
    return proses[language] || proses[""] || proses.en || any || null;
  }

  // proseOfEdge is the clauses the prose version writes of the index-th
  // edge out of the state src, or null.
  function proseOfEdge(src, index) {
    const prose = shownProse();
    return prose ? ((prose.edges || {})[src] || [])[index] || null : null;
  }

  // proseOfEnd is the guard the prose version writes of the index-th end
  // edge out of the state src, or null.
  function proseOfEnd(src, index) {
    const prose = shownProse();
    const guard = prose ? ((prose.ends || {})[src] || [])[index] : undefined;
    return guard === undefined ? null : { guard };
  }

  // proseOfPending is the clauses the prose version writes of the edge a
  // value prompt follows, or null.
  function proseOfPending(pending) {
    const prose = shownProse();
    if (!prose) return null;
    return pending.previous ? proseOfEdge(pending.previous.state_id, pending.edge) : { post: prose.init_post };
  }

  // clauseRows are the rows of the clauses keys names (guard, post) of an
  // edge: the prose version's when it writes them, else formal's, the
  // engine's.
  // The engine's post is shown as its frame: what it empties and keeps
  // apart, one row each, then the rest. Given names (namesOf), a clause is
  // shown marked (marked); without, as written.
  function clauseRows(keys, formal, written, names) {
    const shown = (text, noneId) => {
      const plain = clause(text, noneId);
      return names && plain === text ? marked(text, names) : plain;
    };
    const listed = (vars) => names ? markedNames(vars, names) : vars.join(t("list.separator"));
    return keys.flatMap((k) => {
      const frame = k === "post" && !written ? formal.post_frame : null;
      if (!frame) return [[t(CLAUSE_LABELS[k]), shown((written || formal)[k], NONE_IDS[k])]];
      const rows = [];
      if (frame.emptied) rows.push([t("clause.emptied"), listed(frame.emptied)]);
      if (frame.unchanged) rows.push([t("clause.unchanged"), listed(frame.unchanged)]);
      if (rows.length === 0 || frame.rest !== "(none)") rows.unshift([t(CLAUSE_LABELS[k]), shown(frame.rest, NONE_IDS[k])]);
      return rows;
    });
  }

  // ---- Variables in clauses, marked as the diagram's own pictures mark them ----

  // namesOf are the names a clause may mention: the state variables and
  // the fields of the event, each the event's label names after ? or !.
  function namesOf(vars, event) {
    return { vars: new Set(vars), fields: new Set(event ? event.split(/[?!]/).slice(1) : []) };
  }

  const NAME = /(\$?)([\p{L}\p{N}_]+)(')?/gu;

  // origin is where a name a clause mentions comes from, when its notation
  // tells: after the step when primed, else the event's when the event
  // carries it, else before the step when written $x; "" when it does not.
  function origin(dollar, name, primed, names) {
    if (primed) return "after";
    if (names.fields.has(name)) return "event";
    return dollar ? "before" : "";
  }

  // marked is text with the variables and event fields it mentions in the
  // variables' colour, each followed by its origin, small and grey, and
  // one conjunct to a line.
  function marked(text, names) {
    return el("span", { class: "marked" }, text.split(" ∧ ").map((part) => {
      const line = el("span", { class: "conjunct" });
      let at = 0;
      for (const m of part.matchAll(NAME)) {
        const [whole, dollar, name, primed] = m;
        if (!dollar && !names.vars.has(name) && !names.fields.has(name)) continue;
        line.appendChild(document.createTextNode(part.slice(at, m.index)));
        line.appendChild(markedName(name, origin(dollar, name, primed, names)));
        at = m.index + whole.length;
      }
      line.appendChild(document.createTextNode(part.slice(at)));
      return line;
    }));
  }

  function markedName(name, from) {
    return el("span", {}, [el("span", { class: "var", text: name }), from ? el("span", { class: "origin", text: t("origin." + from) }) : null]);
  }

  function markedNames(vars, names) {
    const out = el("span");
    vars.forEach((v, i) => {
      if (i > 0) out.appendChild(document.createTextNode(t("list.separator")));
      out.appendChild(markedName(v, ""));
    });
    return out;
  }

  // formalClauses shows, folded, the engine's clauses in place of which the
  // prose version's are shown; null when they are not.
  function formalClauses(keys, formal, written) {
    if (!written) return null;
    return el("details", { class: "formal" }, [
      el("summary", { text: t("clause.formal") }),
      clausesList(clauseRows(keys, formal, null, null)),
    ]);
  }

  // piecesList lists where each instance is, by the colour of its piece;
  // null when there are no instances.
  function piecesList(v) {
    const items = [];
    for (const fv of v.families || []) {
      for (const [state, ids] of Object.entries(fv.instances || {})) {
        for (const id of ids) items.push({ map: fv.map, id, state });
      }
    }
    if (items.length === 0) return null;
    items.sort((a, b) => a.map.localeCompare(b.map) || a.id.localeCompare(b.id));
    const list = el("ul", { class: "pieces" });
    for (const it of items) {
      const dot = el("span", { class: "dot" });
      const colour = pieceColour(it.map, it.id);
      dot.style.background = colour.fill;
      list.appendChild(el("li", null, [dot, el("span", { text: t("controls.piece", { id: it.id, state: it.state }) })]));
    }
    return el("div", { class: "hint" }, [el("span", { text: t("controls.pieces") }), list]);
  }

  // renderLegend shows what each mark on the drawing means, by a small
  // sample of the mark.
  function renderLegend() {
    const legend = document.getElementById("legend");
    if (!config.highlighted) { legend.replaceChildren(el("span", { text: t("diagram.noHighlights") })); return; }
    const items = [["whole", "legend.current"]];
    if (Object.keys(families).length > 0) items.push(["piece", "legend.piece"]);
    items.push(["choosable", "legend.choosable"], ["visited", "legend.visited"], ["filtered", "legend.filtered"]);
    legend.replaceChildren(...items.map(([kind, id]) =>
      el("span", { class: "legend-item" }, [el("span", { class: "swatch swatch-" + kind }), el("span", { text: t(id) })])));
  }

  function render() {
    document.documentElement.lang = language;
    document.getElementById("title").textContent = t("app.title", { name: config.title });
    document.title = t("app.title", { name: config.title });
    for (const node of document.querySelectorAll("[data-msg]")) node.textContent = t(node.dataset.msg);
    renderLegend();
    document.getElementById("back").disabled = !canGoBack();
    document.getElementById("to-start").disabled = !canGoToStart();
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
      list.appendChild(el("dt", { text: v.name }));
      const dd = el("dd", null, [variableView(v.name, v.value, prev ? prev.value : undefined, !!prev)]);
      if (before && !prev) {
        dd.classList.add("changed");
        dd.title = t("history.changed");
      }
      list.appendChild(dd);
    }
    return list;
  }

  // shownBeside says whether the values of history entry i are shown
  // beside the boxes of the value form, as the values from before, and so
  // are not repeated in the history.
  function shownBeside(i) {
    const v = app.view;
    return i === app.current && app.tab === "details" && v && v.mode === "values" && !!v.pending.previous;
  }

  // chainOf is the path to the engine's history entry i, as the indices of
  // the entries along it: a jump is a clone of the entry it jumps to, so it
  // stands in that entry's place.
  function chainOf(i) {
    const entry = app.entries[i];
    if (!entry || entry.kind === "initial") return [i];
    if (entry.kind === "jump") return chainOf(entry.from).slice(0, -1).concat([i]);
    return chainOf(entry.parent).concat([i]);
  }

  // originOf is the entry, not a jump, that the entry i is a clone of.
  function originOf(i) {
    const entry = app.entries[i];
    return entry && entry.kind === "jump" ? originOf(entry.from) : i;
  }

  // renderHistory lists the path to the current entry, one step each.
  function renderHistory() {
    const list = document.getElementById("history");
    list.replaceChildren();
    if (app.history.length === 0) list.appendChild(el("li", { class: "hint", text: t("history.empty") }));
    const chain = app.current >= 0 ? chainOf(app.current) : [];
    chain.forEach((i, k) => {
      const h = app.history[i];
      if (!h) return;
      const entry = app.entries[originOf(i)] || { kind: "initial", parent: null };
      const label = entry.kind === "step"
        ? t("history.step", { event: entry.event, state: h.state.state_name })
        : t("history.initial", { state: h.state.state_name });
      const parent = k > 0 ? app.history[chain[k - 1]] : null;
      const head = el("div", null, [
        el("span", { class: "step-number", text: t("history.stepNumber", { step: k + 1 }) }),
        el("span", { text: label }),
        entry.auto ? el("span", { class: "auto", text: t("history.auto") }) : null,
        i === app.current ? el("span", { class: "here", text: t("history.current") }) : null,
      ]);
      const item = el("li", {
        class: i === app.current ? "current" : "",
        tabindex: "0",
        onclick: () => jump(i),
        onkeydown: (e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); jump(i); } },
      }, [head, shownBeside(i) ? el("p", { class: "hint", text: t("history.valuesBeside") }) : valuesList(h.state.values, parent ? parent.state.values : null)]);
      list.appendChild(item);
    });
    const current = list.querySelector("li.current");
    if (current) current.scrollIntoView({ block: "nearest" });
  }

  function clausesList(rows) {
    const list = el("dl", { class: "clauses" });
    for (const [label, value] of rows) {
      list.appendChild(el("dt", { text: label }));
      list.appendChild(typeof value === "string" ? el("dd", { text: value }) : el("dd", {}, [value]));
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
    root.appendChild(stepBar(1));
    root.appendChild(el("h2", { text: t("controls.title") }));
    root.appendChild(el("p", { class: "hint now" }, [el("span", { class: "dot dot-whole" }), el("span", { text: t("controls.now", { state: state.state_name }) })]));
    const pieces = piecesList(v);
    if (pieces) root.appendChild(pieces);
    root.appendChild(errorBox());
    const all = v.transitions || [];
    const transitions = app.tab === "details" ? all : all.filter((tr) => possible(v, tr));
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
          clausesList(clauseRows(["guard"], end, proseOfEnd(state.state_id, end.index), namesOf(stateVars(state), null))),
        ]));
        const formal = formalClauses(["guard"], end, proseOfEnd(state.state_id, end.index));
        if (formal) root.appendChild(formal);
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
    const details = app.tab === "details";
    const written = proseOfEdge(app.view.state.state_id, tr.index);
    const names = namesOf(stateVars(app.view.state), tr.event);
    const blocked = admitsNone(tr);
    const rows = details
      ? [...clauseRows(["guard", "post"], tr, written, names), [t("controls.goesTo"), tr.dst_name]]
      : [...(blocked ? clauseRows(["guard"], tr, written, names) : []), [t("controls.goesTo"), tr.dst_name]];
    const button = el("button", {
      type: "button", class: blocked ? "choice blocked" : "choice", onclick: () => choose(tr),
      onmouseenter: () => hover(tr), onmouseleave: () => hover(null),
      onfocus: () => hover(tr), onblur: () => hover(null),
    }, [
      el("div", { class: "event", text: tr.event }),
      blocked ? el("p", { class: "reason", text: t("controls.conditionFails") }) : document.createTextNode(""),
      clausesList(rows),
    ]);
    if (blocked) button.disabled = true;
    const formal = details ? formalClauses(["guard", "post"], tr, written) : null;
    return formal ? el("div", { class: "choice-item" }, [button, formal]) : button;
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
    const written = proseOfEdge(app.view.state.state_id, tr.index) || {};
    const text = [tr.event, tr.guard, tr.post, written.guard, written.post, tr.dst_name].join(" ").toLowerCase();
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
    if (byMap.size > 0) list.appendChild(el("p", { class: "hint", text: t("controls.byFamily") }));
    for (const [map, edges] of byMap) {
      const count = new Set([...edges.values()].flatMap((g) => g.transitions)).size;
      list.appendChild(groupBox("map:" + map, map, count, (body) => {
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
  // After a transition chosen it is the second of the two steps, under
  // the transition; the initial values are a step of their own.
  function renderValueForm(root, pending) {
    const group = pending.group;
    const promoted = (pending.families || []).length > 0;
    if (app.choice) {
      root.appendChild(stepBar(2));
      root.appendChild(chosenBox(app.choice.transition));
    }
    if (app.tab === "details") {
      root.appendChild(el("h2", { text: promoted ? t("values.titleNext") : t("values.title", { state: group.state_name }) }));
      renderValueDetails(root, pending);
    } else {
      root.appendChild(el("h2", { text: app.choice ? t("suggest.title", { event: app.choice.transition.event }) : t("suggest.titleInitial") }));
      renderValueChoices(root, pending);
    }
  }

  // stepBar shows the two steps of going on, choosing a transition and
  // then the values it leads to, with the step active marked.
  function stepBar(active) {
    const step = (n, key) => {
      const item = el("li", { class: n === active ? "active" : n < active ? "done" : "", text: t(key) });
      if (n === active) item.setAttribute("aria-current", "step");
      return item;
    };
    return el("ol", { class: "steps", "aria-label": t("steps.label") }, [step(1, "steps.choose"), step(2, "steps.values")]);
  }

  // chosenBox says which transition the values are the result of, with a
  // way back to choosing another.
  function chosenBox(tr) {
    return el("div", { class: "chosen" }, [
      el("span", { class: "label", text: t("chosen.label") }),
      el("span", { class: "event", text: tr.event }),
      el("span", { class: "dst", text: t("chosen.goesTo", { state: tr.dst_name }) }),
      el("button", { type: "button", id: "back-to-choose", text: t("chosen.back"), onclick: cancelChoice }),
    ]);
  }

  function renderValueChoices(root, pending) {
    const vars = pending.group.vars || [];
    root.appendChild(errorBox());
    const determined = pending.determined || [];
    if (determined.length > 0 && !app.error) {
      root.appendChild(el("p", { class: "hint", text: t("values.determined") }));
      const note = instancesNote(pending);
      if (note) root.appendChild(note);
      root.appendChild(valuesList(determined.map((d) => ({ name: varLabel(pending, d.name), value: d.value })), null));
    }
    const previous = pending.previous ? pending.previous.values || [] : [];
    const suggestions = pending.suggestions || [];
    if (app.suggesting && !pending.suggest_error && suggestions.length > 0) {
      renderSuggestions(root, vars, previous, suggestions, t(app.choice ? "suggest.hint" : "suggest.hintInitial"));
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
    const types = pending.group.types || {};
    for (const type of new Set(vars.map((name) => types[name]).filter((x) => x))) {
      const help = typeHelp(type);
      if (help) root.appendChild(help);
    }
  }

  // valueProblem says, in the tester's words, what keeps the boxes from
  // being read: the first box left empty or not holding one JSON value.
  function valueProblem(labels, texts) {
    for (let i = 0; i < texts.length; i++) {
      const text = texts[i].trim();
      if (text === "") return t("values.empty", { name: labels[i] });
      try { JSON.parse(text); } catch (e) { return t("values.notJSON", { name: labels[i] }); }
    }
    return null;
  }

  // varLabel is how the prompt names the variable name; that a name is
  // the instances of a diagram is said once, by instancesNote.
  function varLabel(pending, name) {
    return name;
  }

  // shapeHelp tells how the instances of a map are written: an example,
  // and every local state an instance may be at with the fields it holds.
  function shapeHelp(shape) {
    const first = shape.states[0];
    const fields = first ? "{" + (first.fields || []).map((f) => JSON.stringify(f.name) + ": " + shapeExample(f.type)).join(", ") + "}" : "{}";
    const example = first ? "{" + JSON.stringify(t("shape.id", { type: shape.key_type || "ID" })) + ": {\"at\": " + JSON.stringify(first.state) + ", \"fields\": " + fields + "}}" : "{}";
    const rows = shape.states.map((s) => el("li", null, [
      el("code", { text: s.state }),
      el("span", { text: s.name !== s.state ? " " + s.name : "" }),
      el("span", { class: "fields", text: (s.fields || []).length > 0
        ? t("shape.fields", { fields: s.fields.map((f) => f.type && f.type !== f.name ? f.name + " (" + f.type + ")" : f.name).join(", ") })
        : t("shape.noFields") }),
    ]));
    return el("details", { class: "shape" }, [
      el("summary", { text: t("shape.summary") }),
      el("p", { class: "hint", text: t("shape.empty") }),
      el("p", { class: "hint", text: t("shape.example") }),
      el("code", { class: "example", text: example }),
      el("p", { class: "hint", text: t("shape.states") }),
      el("ul", null, rows),
      ...[...new Set(shape.states.flatMap((s) => (s.fields || []).map((f) => f.type)))].map(typeHelp),
    ]);
  }

  // typeShape is what csdfbmc shapes (-shapes) says the values of the type
  // named type look like, or null when it says nothing worth showing: for
  // a map type K ⇸ V, that of V, each value under its key.
  function typeShape(type) {
    const types = (config.shapes && config.shapes.types) || {};
    const known = (node) => node && (node.kind !== "scalar" || (node.values || []).length > 0 || (node.examples || []).length > 0 || node.numeric);
    if (known(types[type])) return { node: types[type], key: null };
    const [key, value] = String(type).split("⇸").map((x) => x.trim());
    return value && known(types[value]) ? { node: types[value], key } : null;
  }

  // skeleton is an example value of the shape node: a record of its
  // attributes, a list of one item, its first allowed value, a constant
  // it is compared with, else
  // 0 for a number and "…" for anything else.
  function skeleton(node) {
    if (node.kind === "record") return Object.fromEntries(Object.entries(node.fields).map(([k, v]) => [k, skeleton(v)]));
    if (node.kind === "collection") return [skeleton(node.items)];
    if ((node.values || []).length > 0) return node.values[0];
    if ((node.examples || []).length > 0) return node.examples[0];
    return node.numeric ? 0 : "…";
  }

  // shapeLeaves lists the attributes of node whose values something is
  // known of, as table rows by their paths: the values they may take, else
  // the constants they are compared with, else that they are numbers.
  function shapeLeaves(node, path) {
    if (node.kind === "record") return Object.entries(node.fields).flatMap(([k, v]) => shapeLeaves(v, path.concat([k])));
    if (node.kind === "collection") return shapeLeaves(node.items, path.concat(["[…]"]));
    const text = allowedText(node);
    if (text === null) return [];
    const name = path.length > 0 ? path.join(".").replaceAll(".[…]", "[…]") : t("typeShape.value");
    return [el("tr", null, [el("th", { text: name }), el("td", { text })])];
  }

  // allowedText says which values a scalar node takes, or null when
  // nothing is known: its values, a run of consecutive integers as a
  // range; else the constants it is compared with; else that it is a number.
  function allowedText(node) {
    const plain = (x) => typeof x === "string" ? x : JSON.stringify(x);
    const values = node.values || [];
    if (values.length > 0) {
      const run = values.length > 2 && values.every((x, i) => Number.isInteger(x) && (i === 0 || x === values[i - 1] + 1));
      return run ? t("typeShape.range", { from: values[0], to: values[values.length - 1] }) : values.map(plain).join(t("list.separator"));
    }
    const examples = node.examples || [];
    if (examples.length > 0) return t("typeShape.examples", { values: examples.map(plain).join(t("list.separator")) });
    return node.numeric ? t("typeShape.number") : null;
  }

  // exampleOf is an example value of what typeShape gives.
  function exampleOf(shape) {
    const value = skeleton(shape.node);
    return shape.key ? { [t("shape.id", { type: shape.key })]: value } : value;
  }

  // shapeExample is the example JSON of a value of type, or "…".
  function shapeExample(type) {
    const shape = typeShape(type);
    return shape ? JSON.stringify(exampleOf(shape)) : "…";
  }

  // typeHelp tells what a value of type looks like, as far as the clauses
  // of the diagram tell (-shapes), or is null when they tell nothing.
  function typeHelp(type) {
    const shape = typeShape(type);
    if (!shape) return null;
    const leaves = shapeLeaves(shape.node, []);
    return el("details", { class: "shape type-shape" }, [
      el("summary", { text: t("typeShape.summary", { type }) }),
      el("p", { class: "hint", text: t(((config.shapes && config.shapes.declared) || []).includes(type) ? "typeShape.declared" : "typeShape.note") }),
      leaves.length > 0 ? el("table", { class: "value-table type-table" }, [
        el("tr", null, [el("th", { text: t("typeShape.item") }), el("th", { text: t("typeShape.allowed") })]),
        ...leaves,
      ]) : null,
      el("p", { class: "hint", text: t("typeShape.exampleLabel") }),
      el("code", { class: "example", text: JSON.stringify(exampleOf(shape)) }),
    ]);
  }

  // instancesNote says once, for a prompt with maps of instances, that
  // those names are the instances of the diagrams they are named after,
  // and how one is written.
  function stateVars(state) {
    return (state.values || []).map((v) => v.name);
  }

  function instancesNote(pending) {
    const maps = (pending.families || []).filter((m) => (pending.group.vars || []).includes(m));
    return maps.length > 0 ? el("p", { class: "hint shape", text: t("values.instancesNote", { example: maps[0] }) }) : null;
  }

  function renderValueDetails(root, pending) {
    const group = pending.group;
    const vars = group.vars || [];
    const rows = [];
    const keys = pending.previous ? ["guard", "post"] : ["post"];
    const written = proseOfPending(pending);
    const before = pending.previous ? stateVars(pending.previous) : [];
    rows.push(...clauseRows(keys, pending, written, namesOf(before.concat(vars), app.choice ? app.choice.transition.event : null)));
    root.appendChild(clausesList(rows));
    const formal = formalClauses(keys, pending, written);
    if (formal) root.appendChild(formal);
    root.appendChild(errorBox());
    const previous = pending.previous ? pending.previous.values || [] : [];
    const inputs = [];
    const form = el("form", {
      onsubmit: (e) => {
        e.preventDefault();
        const problem = valueProblem(vars.map((name) => varLabel(pending, name)), inputs.map((i) => i.value));
        if (problem) { app.error = problem; render(); return; }
        enterValues(inputs.map((i) => i.value));
      },
    });
    if (vars.length === 0) form.appendChild(el("p", { class: "hint", text: t("values.none", { state: group.state_name }) }));
    else form.appendChild(el("p", { class: "hint", text: t("values.hint") }));
    const note = instancesNote(pending);
    if (note) form.appendChild(note);
    const types = group.types || {};
    vars.forEach((name, i) => {
      const id = "value-" + i;
      const before = previous.find((p) => p.name === name);
      const fixed = determinedValue(pending, name);
      // A value the diagram fixes is filled in; any other box starts empty,
      // with the value from before beside it, so that each is decided.
      const input = el("input", { type: "text", id, autocomplete: "off", spellcheck: "false" });
      input.value = fixed !== undefined ? show(fixed) : "";
      if (fixed !== undefined) input.disabled = true;
      inputs.push(input);
      form.appendChild(el("label", { for: id }, [
        el("span", { text: varLabel(pending, name) }),
        types[name] ? el("span", { class: "type", text: t("values.type", { type: types[name] }) }) : null,
      ]));
      form.appendChild(input);
      const shape = (pending.shapes || []).find((s) => s.map === name);
      if (shape && fixed === undefined) form.appendChild(shapeHelp(shape));
      const help = !shape && fixed === undefined && types[name] ? typeHelp(types[name]) : null;
      if (help) form.appendChild(help);
      if (fixed !== undefined) {
        form.appendChild(el("p", { class: "hint fixed", text: t("values.fixed") }));
      } else if (before) {
        form.appendChild(el("div", { class: "before" }, [
          el("span", { text: t("values.before") }),
          variableView(name, before.value, undefined, false),
          el("button", { type: "button", text: t("values.keep"), onclick: () => { input.value = show(before.value); input.focus(); } }),
        ]));
      }
    });
    form.appendChild(el("div", { class: "row" }, [
      el("button", { type: "submit", class: "primary", text: t("values.continue") }),
      app.choice ? el("button", { type: "button", text: t("values.cancel"), onclick: cancelChoice }) : null,
    ]));
    root.appendChild(form);
    const first = inputs.find((i) => i.value === "") || inputs[0];
    if (first) first.focus({ preventScroll: true });
  }

  // renderSuggestions shows the values the checker admits as buttons, each
  // saying what changes from before, under the hint.
  function renderSuggestions(root, vars, previous, suggestions, hint) {
    root.appendChild(el("p", { class: "hint", text: hint }));
    const list = el("div", { class: "suggestions" });
    suggestions.forEach((s, i) => {
      const lines = [];
      const params = Object.entries(s.params || {});
      if (params.length > 0) {
        const shownParams = el("span");
        params.forEach(([k, v], j) => {
          if (j > 0) shownParams.appendChild(document.createTextNode(", "));
          shownParams.appendChild(tNodes("suggest.set", { name: k, value: valueView(v, undefined, false) }));
        });
        lines.push(tNodes("suggest.params", { params: shownParams }));
      }
      const changes = changesOf(vars, previous, s.values);
      const shown = changes.slice(0, CHANGE_LIMIT);
      if (changes.length === 0) lines.push(t("suggest.nothingChanges"));
      lines.push(...shown);
      if (changes.length > shown.length) lines.push(t("suggest.more", { n: changes.length - shown.length }));
      list.appendChild(el("button", {
        type: "button", class: "choice suggestion", id: "suggestion-" + i,
        onclick: () => enterValues(s.values.map(show)),
      }, lines.map((line) => typeof line === "string" ? el("div", { text: line }) : el("div", null, [line]))));
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
      const view = (x) => valueView(x, undefined, false);
      if (!before) { lines.push(tNodes("suggest.set", { name, value: variableView(name, after, undefined, false) })); return; }
      if (show(before.value) === show(after)) return;
      if (isInstances(name, after) || (isFamily(name) && isInstances(name, before.value))) {
        lines.push(...instanceChanges(name, before.value, after));
        return;
      }
      if (isMap(before.value) && isMap(after)) {
        const keys = new Set(Object.keys(before.value).concat(Object.keys(after)));
        for (const key of keys) {
          const entry = name + "[" + key + "]";
          if (!(key in after)) lines.push(t("suggest.removed", { name: entry }));
          else if (!(key in before.value)) lines.push(tNodes("suggest.set", { name: entry, value: view(after[key]) }));
          else if (show(before.value[key]) !== show(after[key])) {
            lines.push(tNodes("suggest.change", { name: entry, before: view(before.value[key]), after: view(after[key]) }));
          }
        }
        return;
      }
      lines.push(tNodes("suggest.change", { name, before: view(before.value), after: view(after) }));
    });
    return lines;
  }

  // instanceChanges are the lines of what after makes of the instances of
  // map held in before: each instance new or changed, as instanceItem
  // shows it, and each removed.
  function instanceChanges(map, before, after) {
    const lines = [];
    const prev = isMap(before) ? before : {};
    const next = isMap(after) ? after : {};
    for (const id of [...new Set(Object.keys(prev).concat(Object.keys(next)))].sort()) {
      if (!(id in next)) lines.push(t("suggest.removed", { name: map + "[" + id + "]" }));
      else if (show(prev[id]) !== show(next[id])) {
        lines.push(el("ul", { class: "instances" }, [instanceItem(map, id, next[id], prev[id], true)]));
      }
    }
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

  // pieceColour is the colour of the piece of the instance id of map: the
  // colours of PALETTE in the order the page first meets the instances, so
  // an instance keeps its colour for the whole session.
  function pieceColour(map, id) {
    const key = map + "\u0000" + id;
    if (!app.colours.has(key)) app.colours.set(key, PALETTE[app.colours.size % PALETTE.length]);
    return app.colours.get(key);
  }

  // drawPiece draws on overlay a round badge reading label, its left end
  // at x and its middle at y, in colour, and answers it and its width.
  function drawPiece(overlay, x, y, label, colour, cls, tip) {
    const ns = "http://www.w3.org/2000/svg";
    const g = document.createElementNS(ns, "g");
    g.setAttribute("class", cls);
    const title = document.createElementNS(ns, "title");
    title.textContent = tip;
    const rect = document.createElementNS(ns, "rect");
    const text = document.createElementNS(ns, "text");
    text.textContent = label;
    text.setAttribute("y", y);
    text.setAttribute("dominant-baseline", "central");
    text.setAttribute("text-anchor", "middle");
    text.setAttribute("fill", colour.text);
    g.append(title, rect, text);
    overlay.appendChild(g);
    const h = cls.includes("csdf-piece-whole") ? 22 : 16;
    const w = Math.max(h, text.getComputedTextLength() + h * 0.7);
    text.setAttribute("x", x + w / 2);
    rect.setAttribute("x", x);
    rect.setAttribute("y", y - h / 2);
    rect.setAttribute("width", w);
    rect.setAttribute("height", h);
    rect.setAttribute("rx", h / 2);
    rect.setAttribute("fill", colour.fill);
    return { node: g, width: w };
  }

  // renderPieces draws the pieces on the drawing: the whole's piece on the
  // box of the state the diagram is in (whole), and on the box of each local
  // state one piece per instance there, in the instance's colour, side by
  // side along the top edge of the box. A state with no box gets no piece.
  function renderPieces(v, whole) {
    const svg = document.querySelector("#diagram svg");
    if (!svg) return;
    const old = svg.querySelector("#csdf-overlay");
    if (old) old.remove();
    const overlay = document.createElementNS("http://www.w3.org/2000/svg", "g");
    overlay.id = "csdf-overlay";
    svg.appendChild(overlay);
    if (whole) {
      const bbox = whole.getBBox();
      drawPiece(overlay, bbox.x + 6, bbox.y, t("piece.whole"), WHOLE_COLOUR, "csdf-piece csdf-piece-whole", t("legend.current"));
    }
    for (const fv of v.families || []) {
      const f = families[fv.map];
      if (!f) continue;
      for (const [state, ids] of Object.entries(fv.instances || {})) {
        const box = svgElement(f.states[state]);
        if (!box || ids.length === 0) continue;
        const key = fv.map + "/" + state;
        const shown = ids.length <= PIECE_LIMIT || app.expanded.has(key) ? ids : ids.slice(0, PIECE_LIMIT - 1);
        const bbox = box.getBBox();
        let x = bbox.x + 6;
        for (const id of shown) {
          x += drawPiece(overlay, x, bbox.y, id, pieceColour(fv.map, id), "csdf-piece", t("piece.title", { id, state })).width + 3;
        }
        if (ids.length > PIECE_LIMIT) {
          const label = app.expanded.has(key) ? "−" : t("instances.more", { n: ids.length - shown.length });
          const more = drawPiece(overlay, x, bbox.y, label, MORE_COLOUR, "csdf-piece csdf-piece-more", t("instances.title", { state, ids: ids.join(", ") }));
          more.node.dataset.key = key;
        }
      }
    }
    applyDrawing();
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
        renderPieces(v, from);
      } else renderPieces(v, null);
      const links = app.choice ? linksOf(app.choice.transition) : [svgElement(highlights.start)];
      for (const link of links) if (link) link.classList.add("csdf-choosable");
      return;
    }
    const current = svgElement(highlights.states[v.state.state_id]);
    if (current) current.classList.add("csdf-current");
    renderPieces(v, current);
    if (v.mode === "command") {
      for (const tr of (v.transitions || []).filter((x) => possible(v, x) && !admitsNone(x))) {
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
    const more = e.target.closest("g.csdf-piece-more");
    if (more) {
      const key = more.dataset.key;
      if (app.expanded.has(key)) app.expanded.delete(key); else app.expanded.add(key);
      renderDiagram();
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
      if (e.target.closest("g.csdf-choosable, g.csdf-piece-more")) return;
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

  // emptyInits are the local start states of the families, where every
  // instance not created yet is: an instance can always be created.
  function emptyInits(v) {
    if (!v.families && v.mode !== "command") return [];
    return Object.keys(families).map((m) => ({ map: m, node: localInitOf(m) })).filter((o) => o.node);
  }

  // possible says whether tr can be taken as far as where the instances
  // are tells: every move of it leaves a local state holding an instance,
  // or creates one. Its guard and post are checked only when it is taken.
  function possible(v, tr) {
    return (tr.moves || []).every((m) => {
      if (m.kind === "creation") return true;
      const fv = (v.families || []).find((f) => f.map === m.map);
      const ids = fv && fv.instances ? fv.instances[m.src] || [] : [];
      return ids.length > 0;
    });
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

  // viewBoxOf is the part of the drawing to show: the block shown, else
  // all of it, widened to take in the pieces on it, which may stick out
  // past the edges of the boxes they sit on.
  function viewBoxOf() {
    const [x, y, w, h] = shownArea();
    let x0 = x, y0 = y, x1 = x + w, y1 = y + h;
    for (const piece of document.querySelectorAll("#csdf-overlay .csdf-piece")) {
      const b = piece.getBBox();
      if (b.x > x1 || b.x + b.width < x0 || b.y > y1 || b.y + b.height < y0) continue;
      x0 = Math.min(x0, b.x - PIECE_MARGIN); y0 = Math.min(y0, b.y - PIECE_MARGIN);
      x1 = Math.max(x1, b.x + b.width + PIECE_MARGIN); y1 = Math.max(y1, b.y + b.height + PIECE_MARGIN);
    }
    return [x0, y0, x1 - x0, y1 - y0];
  }

  // The room left around a piece at an edge of the view.
  const PIECE_MARGIN = 4;

  // shownArea is the part of the drawing the block shown takes, else all
  // of it.
  function shownArea() {
    const d = app.drawing;
    if (!app.block) return d.viewBox.split(/[\s,]+/).map(Number);
    let x0 = Infinity, y0 = Infinity, x1 = -Infinity, y1 = -Infinity;
    for (const map of app.block) {
      const f = families[map];
      const ids = [...Object.values(f.states || {}), ...Object.values(f.edges || {}), f.start, f.block].filter(Boolean);
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
    document.getElementById("to-start").addEventListener("click", goToStart);
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
