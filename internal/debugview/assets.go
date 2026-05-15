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
      --text: #20242c;
      --muted: #647084;
      --accent: #0f766e;
      --accent-soft: #d9f2ef;
      --warn: #a15c00;
      --warn-soft: #fff2d6;
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
      min-height: 100vh;
      display: grid;
      grid-template-rows: auto 1fr;
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
    section {
      min-width: 0;
      min-height: 0;
      display: grid;
      grid-template-rows: auto 1fr;
      background: var(--panel);
    }
    section + section { border-left: 1px solid var(--line); }
    .pane-title {
      height: 40px;
      border-bottom: 1px solid var(--line);
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 0 12px;
      color: var(--muted);
      font-size: 12px;
      text-transform: uppercase;
      letter-spacing: 0;
      white-space: nowrap;
    }
    .scroll {
      min-height: 0;
      overflow: auto;
    }
    .source-line, .inst {
      display: grid;
      min-height: 26px;
      border-bottom: 1px solid #eef1f5;
      cursor: pointer;
    }
    .source-line {
      grid-template-columns: 64px minmax(0, 1fr) 52px;
    }
    .inst {
      grid-template-columns: 64px 86px minmax(0, 1fr);
    }
    .line-no, .inst-no, .inst-loc, .count {
      color: var(--muted);
      font: 12px/26px ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
      padding: 0 8px;
      border-right: 1px solid #eef1f5;
      text-align: right;
      user-select: none;
      white-space: nowrap;
    }
    .count {
      border-right: 0;
      border-left: 1px solid #eef1f5;
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
    .inst code {
      overflow: visible;
      text-overflow: clip;
    }
    .inst-kind {
      color: var(--muted);
      border-right: 1px solid #eef1f5;
      padding: 0 8px;
      font: 12px/26px ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
      white-space: nowrap;
    }
    .source-line.active, .inst.active {
      background: var(--accent-soft);
    }
    .source-line.related, .inst.related {
      background: #effaf8;
    }
    .source-line.todo, .inst.todo {
      background: var(--warn-soft);
    }
    .source-line.hidden, .inst.hidden { display: none; }
    .empty {
      padding: 24px;
      color: var(--muted);
    }
    @media (max-width: 860px) {
      header { align-items: stretch; flex-direction: column; }
      .toolbar { margin-left: 0; width: 100%; }
      input[type="search"] { width: 100%; flex: 1; }
      main { grid-template-columns: 1fr; grid-template-rows: minmax(280px, 48vh) 1fr; }
      section + section { border-left: 0; border-top: 1px solid var(--line); }
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
  </div>
  <script>
    const state = { data: null, activeLine: 0 };
    const sourceEl = document.getElementById("source");
    const instEl = document.getElementById("instructions");
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

    function render(data) {
      document.getElementById("file-name").textContent = data.sourceName;
      document.getElementById("inst-total").textContent = data.summary.totalInstructions;
      document.getElementById("inst-located").textContent = data.summary.locatedInstructions;
      document.getElementById("inst-todo").textContent = data.summary.todoInstructions;
      document.getElementById("source-count").textContent = data.sourceLines.length + " lines";
      sourceEl.innerHTML = data.sourceLines.map(renderSourceLine).join("");
      instEl.innerHTML = data.instructions.map(renderInstruction).join("");
      sourceEl.querySelectorAll(".source-line").forEach((row) => {
        row.addEventListener("click", () => setActiveLine(Number(row.dataset.line)));
      });
      instEl.querySelectorAll(".inst").forEach((row) => {
        row.addEventListener("click", () => setActiveLine(Number(row.dataset.line || "0")));
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
      return '<div class="inst ' + (inst.kind === "todo" ? "todo" : "") + '" data-line="' + (inst.line || "") + '">' +
        '<div class="inst-no">' + (inst.index + 1) + '</div>' +
        '<div class="inst-kind">' + escapeText(inst.kind) + '</div>' +
        '<code>' + escapeText(inst.text) + '</code>' +
        '</div>';
    }

    function setActiveLine(line) {
      state.activeLine = state.activeLine === line ? 0 : line;
      applyFilters();
      if (state.activeLine > 0) {
        const first = instEl.querySelector('.inst[data-line="' + state.activeLine + '"]:not(.hidden)');
        if (first) first.scrollIntoView({ block: "nearest" });
      }
    }

    function applyFilters() {
      if (!state.data) return;
      const query = queryEl.value.toLowerCase();
      const filter = filterEl.value;
      sourceEl.querySelectorAll(".source-line").forEach((row) => {
        const line = Number(row.dataset.line);
        const related = state.activeLine > 0 && line === state.activeLine;
        row.classList.toggle("active", related);
        row.classList.toggle("related", related);
        row.classList.toggle("hidden", query && !row.textContent.toLowerCase().includes(query));
      });
      instEl.querySelectorAll(".inst").forEach((row) => {
        const line = Number(row.dataset.line || "0");
        const text = row.textContent.toLowerCase();
        const inst = state.data.instructions[Number(row.firstChild.textContent) - 1];
        const filterOut = (filter === "located" && !line) || (filter === "todo" && inst.kind !== "todo");
        const queryOut = query && !text.includes(query);
        const activeOut = state.activeLine > 0 && line !== state.activeLine;
        row.classList.toggle("active", state.activeLine > 0 && line === state.activeLine);
        row.classList.toggle("related", state.activeLine > 0 && line === state.activeLine);
        row.classList.toggle("hidden", filterOut || queryOut || activeOut);
      });
    }

    function escapeText(text) {
      return text.replace(/[&<>"']/g, (ch) => ({
        "&": "&amp;",
        "<": "&lt;",
        ">": "&gt;",
        '"': "&quot;",
        "'": "&#39;"
      }[ch]));
    }
  </script>
</body>
</html>
`
