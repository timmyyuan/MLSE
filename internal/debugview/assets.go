package debugview

const debugHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>MLSE Debug</title>
  <style>
    :root {
      color-scheme: light;
      --bg: #f7f8fa;
      --panel: #ffffff;
      --line: #d9dee7;
      --soft-line: #eef1f5;
      --text: #20242c;
      --muted: #647084;
      --accent: #0f766e;
      --accent-soft: #d9f2ef;
      --warn: #a15c00;
      --warn-soft: #fff2d6;
      --bad: #a4343a;
      --bad-soft: #fde8ea;
      --code: #0e1726;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      background: var(--bg);
      color: var(--text);
      font: 14px/1.45 -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
    }
    .app {
      height: 100vh;
      min-height: 620px;
      display: grid;
      grid-template-rows: auto minmax(0, 1fr) minmax(210px, 30vh);
      background: var(--panel);
    }
    header {
      border-bottom: 1px solid var(--line);
      background: var(--panel);
      display: flex;
      align-items: center;
      gap: 16px;
      min-height: 56px;
      padding: 10px 16px;
    }
    h1 {
      margin: 0;
      font-size: 17px;
      font-weight: 650;
      min-width: 120px;
    }
    .meta {
      color: var(--muted);
      display: flex;
      gap: 12px;
      flex-wrap: wrap;
      align-items: center;
      min-width: 0;
    }
    .meta strong { color: var(--text); font-weight: 600; }
    .toolbar {
      margin-left: auto;
      display: flex;
      gap: 8px;
      align-items: center;
      flex-wrap: wrap;
    }
    input[type="search"], select {
      height: 34px;
      border: 1px solid var(--line);
      background: #fff;
      color: var(--text);
      border-radius: 6px;
      padding: 0 10px;
      font: inherit;
    }
    input[type="search"] { width: min(32vw, 360px); }
    main {
      min-height: 0;
      display: grid;
      grid-template-columns: minmax(320px, 46%) minmax(380px, 54%);
    }
    section, .debugger {
      min-width: 0;
      min-height: 0;
      display: grid;
      grid-template-rows: auto 1fr;
      background: var(--panel);
    }
    section + section { border-left: 1px solid var(--line); }
    .debugger {
      border-top: 1px solid var(--line);
      grid-template-rows: 40px minmax(0, 1fr);
    }
    .pane-title {
      height: 40px;
      border-bottom: 1px solid var(--line);
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 12px;
      padding: 0 12px;
      color: var(--muted);
      font-size: 12px;
      text-transform: uppercase;
      letter-spacing: 0;
      white-space: nowrap;
    }
    .tabs {
      display: flex;
      height: 100%;
      align-items: stretch;
    }
    .tab {
      border: 0;
      border-right: 1px solid var(--line);
      background: transparent;
      color: var(--muted);
      min-width: 92px;
      padding: 0 14px;
      font: inherit;
      cursor: pointer;
    }
    .tab.active {
      color: var(--text);
      background: #f1f7f6;
      box-shadow: inset 0 -2px 0 var(--accent);
    }
    .scroll {
      min-height: 0;
      overflow: auto;
    }
    .source-line, .inst, .scope-row, .trace-row, .frame-row, .event-row {
      display: grid;
      min-height: 26px;
      border-bottom: 1px solid var(--soft-line);
    }
    .source-line, .inst, .scope-row, .trace-row {
      cursor: pointer;
    }
    .source-line {
      grid-template-columns: 64px minmax(0, 1fr) 52px;
    }
    .inst {
      grid-template-columns: 64px 86px minmax(0, 1fr);
    }
    .scope-row {
      grid-template-columns: 76px 84px minmax(0, 1fr) 120px 64px;
    }
    .trace-grid {
      min-height: 0;
      display: grid;
      grid-template-columns: minmax(260px, 30%) minmax(280px, 25%) minmax(360px, 45%);
    }
    .trace-grid > div + div {
      border-left: 1px solid var(--line);
    }
    .trace-row {
      grid-template-columns: minmax(0, 1fr) 96px;
    }
    .frame-row {
      grid-template-columns: 64px 86px minmax(0, 1fr);
    }
    .event-row {
      grid-template-columns: 64px 120px 90px minmax(0, 1fr);
    }
    .line-no, .inst-no, .count, .cell, .event-no {
      color: var(--muted);
      font: 12px/26px ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
      padding: 0 8px;
      border-right: 1px solid var(--soft-line);
      user-select: none;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }
    .line-no, .inst-no, .count, .event-no {
      text-align: right;
    }
    .count {
      border-right: 0;
      border-left: 1px solid var(--soft-line);
      text-align: center;
    }
    code {
      min-width: 0;
      color: var(--code);
      font: 13px/26px ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
      white-space: pre;
      overflow: hidden;
      text-overflow: ellipsis;
      padding: 0 10px;
    }
    .inst code, .event-row code {
      overflow: visible;
      text-overflow: clip;
    }
    .badge {
      display: inline-block;
      min-width: 0;
      max-width: 100%;
      border-radius: 4px;
      padding: 2px 6px;
      background: #edf1f7;
      color: var(--muted);
      overflow: hidden;
      text-overflow: ellipsis;
      vertical-align: middle;
    }
    .badge.success, .badge.equivalent {
      background: var(--accent-soft);
      color: var(--accent);
    }
    .badge.blocked, .badge.failure, .badge.counterexample {
      background: var(--bad-soft);
      color: var(--bad);
    }
    .source-line.active, .inst.active, .scope-row.active, .trace-row.active {
      background: var(--accent-soft);
    }
    .source-line.related, .inst.related {
      background: #effaf8;
    }
    .source-line.todo, .inst.todo {
      background: var(--warn-soft);
    }
    .hidden { display: none; }
    .empty {
      padding: 24px;
      color: var(--muted);
    }
    .panel {
      min-height: 0;
      display: grid;
      grid-template-rows: 1fr;
    }
    @media (max-width: 920px) {
      .app { min-height: 780px; grid-template-rows: auto minmax(360px, 1fr) minmax(320px, 38vh); }
      header { align-items: stretch; flex-direction: column; }
      .toolbar { margin-left: 0; width: 100%; }
      input[type="search"] { width: 100%; flex: 1; }
      main { grid-template-columns: 1fr; grid-template-rows: minmax(220px, 48%) minmax(220px, 52%); }
      section + section { border-left: 0; border-top: 1px solid var(--line); }
      .trace-grid { grid-template-columns: 1fr; grid-template-rows: repeat(3, minmax(160px, 1fr)); }
      .trace-grid > div + div { border-left: 0; border-top: 1px solid var(--line); }
    }
  </style>
</head>
<body>
  <div class="app">
    <header>
      <h1>MLSE Debug</h1>
      <div class="meta">
        <span id="file-name">loading</span>
        <span><strong id="inst-total">0</strong> ops</span>
        <span><strong id="inst-located">0</strong> located</span>
        <span><strong id="inst-todo">0</strong> todos</span>
        <span><strong id="scope-total">0</strong> scopes</span>
        <span><strong id="trace-paths">0</strong> paths</span>
      </div>
      <div class="toolbar">
        <input id="query" type="search" placeholder="Search">
        <select id="filter">
          <option value="all">All ops</option>
          <option value="located">Located</option>
          <option value="todo">Todo</option>
        </select>
      </div>
    </header>
    <main>
      <section>
        <div class="pane-title"><span>Source</span><span id="source-count"></span></div>
        <div id="source" class="scroll"></div>
      </section>
      <section>
        <div class="pane-title"><span>Instructions</span><span><a href="/raw.mlir">raw MLIR</a></span></div>
        <div id="instructions" class="scroll"></div>
      </section>
    </main>
    <div class="debugger">
      <div class="pane-title">
        <div class="tabs">
          <button id="tab-scopes" class="tab active" type="button">Scopes</button>
          <button id="tab-trace" class="tab" type="button">Trace</button>
        </div>
        <span id="debug-status"></span>
      </div>
      <div id="scopes-panel" class="panel">
        <div id="scopes" class="scroll"></div>
      </div>
      <div id="trace-panel" class="panel hidden">
        <div class="trace-grid">
          <div class="scroll" id="trace-path-list"></div>
          <div class="scroll" id="trace-frames"></div>
          <div class="scroll" id="trace-events"></div>
        </div>
      </div>
    </div>
  </div>
  <script>
    const state = { data: null, activeLine: 0, activeScope: "", activePath: 0, activeTab: "scopes" };
    const sourceEl = document.getElementById("source");
    const instEl = document.getElementById("instructions");
    const scopesEl = document.getElementById("scopes");
    const tracePathEl = document.getElementById("trace-path-list");
    const traceFramesEl = document.getElementById("trace-frames");
    const traceEventsEl = document.getElementById("trace-events");
    const queryEl = document.getElementById("query");
    const filterEl = document.getElementById("filter");

    fetch("/debug.json")
      .then((response) => response.json())
      .then((data) => {
        state.data = data;
        render(data);
      })
      .catch((error) => {
        sourceEl.innerHTML = '<div class="empty">' + escapeText(String(error)) + '</div>';
      });

    queryEl.addEventListener("input", applyFilters);
    filterEl.addEventListener("change", applyFilters);
    document.getElementById("tab-scopes").addEventListener("click", () => setTab("scopes"));
    document.getElementById("tab-trace").addEventListener("click", () => setTab("trace"));

    function render(data) {
      document.getElementById("file-name").textContent = data.sourceName;
      document.getElementById("inst-total").textContent = data.summary.totalInstructions;
      document.getElementById("inst-located").textContent = data.summary.locatedInstructions;
      document.getElementById("inst-todo").textContent = data.summary.todoInstructions;
      document.getElementById("scope-total").textContent = data.summary.totalScopes || 0;
      document.getElementById("trace-paths").textContent = data.summary.tracePaths || 0;
      document.getElementById("source-count").textContent = data.sourceLines.length + " lines";
      sourceEl.innerHTML = data.sourceLines.map(renderSourceLine).join("");
      instEl.innerHTML = data.instructions.map(renderInstruction).join("");
      scopesEl.innerHTML = data.scopes && data.scopes.length ? data.scopes.map(renderScope).join("") : '<div class="empty">No scopes</div>';
      renderTrace();
      sourceEl.querySelectorAll(".source-line").forEach((row) => {
        row.addEventListener("click", () => setActiveLine(Number(row.dataset.line)));
      });
      instEl.querySelectorAll(".inst").forEach((row) => {
        row.addEventListener("click", () => setActiveLine(Number(row.dataset.line || "0")));
      });
      scopesEl.querySelectorAll(".scope-row").forEach((row) => {
        row.addEventListener("click", () => setActiveScope(row.dataset.scope));
      });
      applyFilters();
    }

    function renderSourceLine(line) {
      const todo = state.data.instructions.some((inst) => inst.line === line.number && inst.kind === "todo");
      return '<div class="source-line ' + (todo ? "todo" : "") + '" data-line="' + line.number + '">' +
        '<div class="line-no">' + line.number + '</div>' +
        '<code>' + escapeText(line.text || " ") + '</code>' +
        '<div class="count">' + (line.instructionCount || "") + '</div>' +
        '</div>';
    }

    function renderInstruction(inst) {
      return '<div class="inst ' + (inst.kind === "todo" ? "todo" : "") + '" data-index="' + inst.index + '" data-line="' + (inst.line || "") + '" data-scope="' + escapeAttr(inst.scope || "") + '">' +
        '<div class="inst-no">' + (inst.index + 1) + '</div>' +
        '<div class="cell">' + escapeText(inst.kind) + '</div>' +
        '<code>' + escapeText(inst.text) + '</code>' +
        '</div>';
    }

    function renderScope(scope) {
      const loc = scope.line ? scope.line + ":" + scope.column : "";
      const name = scope.name || scope.label;
      return '<div class="scope-row" data-scope="' + escapeAttr(scope.label) + '">' +
        '<div class="cell">' + escapeText(scope.label) + '</div>' +
        '<div class="cell">' + escapeText(scope.kind) + '</div>' +
        '<code>' + escapeText(name) + '</code>' +
        '<div class="cell">' + escapeText(loc) + '</div>' +
        '<div class="cell">' + (scope.instructionCount || 0) + ' ops</div>' +
        '</div>';
    }

    function renderTrace() {
      const trace = state.data.trace;
      if (!trace || !trace.paths || trace.paths.length === 0) {
        tracePathEl.innerHTML = '<div class="empty">No trace loaded</div>';
        traceFramesEl.innerHTML = '<div class="empty">No frames</div>';
        traceEventsEl.innerHTML = '<div class="empty">No events</div>';
        return;
      }
      if (state.activePath >= trace.paths.length) state.activePath = 0;
      tracePathEl.innerHTML = trace.paths.map((path, index) => renderTracePath(path, index)).join("");
      tracePathEl.querySelectorAll(".trace-row").forEach((row) => {
        row.addEventListener("click", () => {
          state.activePath = Number(row.dataset.index);
          renderTrace();
        });
      });
      const active = trace.paths[state.activePath];
      traceFramesEl.innerHTML = active.frames && active.frames.length ? active.frames.map(renderFrame).join("") : '<div class="empty">No frames</div>';
      traceEventsEl.innerHTML = active.events && active.events.length ? active.events.map(renderEvent).join("") : '<div class="empty">No events</div>';
    }

    function renderTracePath(path, index) {
      const name = path.case || path.id || ("path-" + index);
      const status = path.status || "";
      return '<div class="trace-row ' + (index === state.activePath ? "active" : "") + '" data-index="' + index + '">' +
        '<code>' + escapeText(name) + '</code>' +
        '<div class="cell"><span class="badge ' + escapeAttr(status) + '">' + escapeText(status || "unknown") + '</span></div>' +
        '</div>';
    }

    function renderFrame(frame) {
      return '<div class="frame-row">' +
        '<div class="cell">' + escapeText(frame.side || "") + '</div>' +
        '<div class="cell"><span class="badge ' + escapeAttr(frame.status || "") + '">' + escapeText(frame.status || "") + '</span></div>' +
        '<code>' + escapeText(frame.source || frame.function || "") + '</code>' +
        '</div>';
    }

    function renderEvent(event) {
      const detail = event.detail || event.artifact || "";
      return '<div class="event-row">' +
        '<div class="event-no">' + (event.index + 1) + '</div>' +
        '<div class="cell">' + escapeText(event.stage || event.kind || "") + '</div>' +
        '<div class="cell"><span class="badge ' + escapeAttr(event.status || "") + '">' + escapeText(event.status || "") + '</span></div>' +
        '<code>' + escapeText(detail) + '</code>' +
        '</div>';
    }

    function setActiveLine(line) {
      state.activeScope = "";
      state.activeLine = state.activeLine === line ? 0 : line;
      applyFilters();
      if (state.activeLine > 0) {
        const first = instEl.querySelector('.inst[data-line="' + state.activeLine + '"]:not(.hidden)');
        if (first) first.scrollIntoView({ block: "nearest" });
      }
    }

    function setActiveScope(scope) {
      state.activeLine = 0;
      state.activeScope = state.activeScope === scope ? "" : scope;
      applyFilters();
    }

    function setTab(tab) {
      state.activeTab = tab;
      document.getElementById("tab-scopes").classList.toggle("active", tab === "scopes");
      document.getElementById("tab-trace").classList.toggle("active", tab === "trace");
      document.getElementById("scopes-panel").classList.toggle("hidden", tab !== "scopes");
      document.getElementById("trace-panel").classList.toggle("hidden", tab !== "trace");
      document.getElementById("debug-status").textContent = tab === "scopes" ? activeScopeStatus() : activeTraceStatus();
    }

    function applyFilters() {
      if (!state.data) return;
      const query = queryEl.value.toLowerCase();
      const filter = filterEl.value;
      const activeScopeLines = new Set(state.data.instructions.filter((inst) => inst.scope === state.activeScope && inst.line).map((inst) => inst.line));
      sourceEl.querySelectorAll(".source-line").forEach((row) => {
        const line = Number(row.dataset.line);
        const relatedLine = state.activeLine > 0 && line === state.activeLine;
        const relatedScope = state.activeScope && activeScopeLines.has(line);
        row.classList.toggle("active", relatedLine || relatedScope);
        row.classList.toggle("related", relatedLine || relatedScope);
        row.classList.toggle("hidden", query && !row.textContent.toLowerCase().includes(query));
      });
      instEl.querySelectorAll(".inst").forEach((row) => {
        const line = Number(row.dataset.line || "0");
        const text = row.textContent.toLowerCase();
        const index = Number(row.dataset.index || "0");
        const inst = state.data.instructions[index];
        const filterOut = (filter === "located" && !line) || (filter === "todo" && inst.kind !== "todo");
        const queryOut = query && !text.includes(query);
        const activeLineOut = state.activeLine > 0 && line !== state.activeLine;
        const activeScopeOut = state.activeScope && row.dataset.scope !== state.activeScope;
        const active = (state.activeLine > 0 && line === state.activeLine) || (state.activeScope && row.dataset.scope === state.activeScope);
        row.classList.toggle("active", active);
        row.classList.toggle("related", active);
        row.classList.toggle("hidden", filterOut || queryOut || activeLineOut || activeScopeOut);
      });
      scopesEl.querySelectorAll(".scope-row").forEach((row) => {
        row.classList.toggle("active", state.activeScope && row.dataset.scope === state.activeScope);
      });
      document.getElementById("debug-status").textContent = state.activeTab === "scopes" ? activeScopeStatus() : activeTraceStatus();
    }

    function activeScopeStatus() {
      return state.activeScope ? state.activeScope : "";
    }

    function activeTraceStatus() {
      const trace = state.data && state.data.trace;
      if (!trace || !trace.paths || trace.paths.length === 0) return "";
      const active = trace.paths[state.activePath];
      return active ? (active.case || active.id || "") : "";
    }

    function escapeText(text) {
      return String(text || "").replace(/[&<>"']/g, (ch) => ({
        "&": "&amp;",
        "<": "&lt;",
        ">": "&gt;",
        '"': "&quot;",
        "'": "&#39;"
      }[ch]));
    }

    function escapeAttr(text) {
      return escapeText(text);
    }
  </script>
</body>
</html>
`
