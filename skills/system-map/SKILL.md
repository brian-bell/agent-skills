---
name: system-map
description: Build an interactive isometric system map of a code repository as a single interactive HTML page — hatched isometric boxes on a grid, a component rail, a "What it does / How it's built / Condition" panel, go-inside stage views, and an animated flow around the main loop. Use when the user asks for a system map, architecture map, isometric diagram, interactive overview, or "a site like this" for a repo.
---

# System Map

Produce a single-file interactive HTML map of a repository and hand it to the user as a rendered page. The renderer is fixed (`<skill-dir>/template.html`); the work is tracing the repo accurately and writing the data block.

## Output

One artifact: khaki paper, black ink, monospace throughout, hatched isometric boxes. Four regions:

- **Stat strip** — 5–6 repo-level numbers plus `Resume the flow`, `Trace one step`, `Reset view`.
- **Left rail** — every structure, grouped, with a letter key and a count.
- **Canvas** — isometric boxes on a grid plate, main loop as a heavy routed line with bend markers, secondary edges thin, off-loop edges dashed. Pan by drag, zoom by wheel.
- **Right panel** — tabs `What it does` and `How it's built`; the second ends with `Condition` (what is open against the structure).

Keys: `→` go inside (structures with stages), `←` come back out, `↓ ↑` walk the rail, `Space` play/pause the flow, `Esc` clear.

## Workflow

### 1. Trace the repo (read-only)

Do not write the page from memory of the README. Gather, with shell commands:

- Module map: `AGENTS.md`/`CLAUDE.md`, top-level directories, per-module file counts.
- Header comments of the load-bearing modules (the first 40 lines of each). These are the source for `what`/`how` text; quote their invariants.
- Counts for the strip: source files and lines, routes (grep the router), test files, migrations or tables, CLI commands, workflows, open issues (`gh issue list --state open --json number | jq length`).
- Enumerations that become "inside" views: pipeline stage enums, verb lists, ordered procedures documented in comments, CLI command groups.
- Open issues and in-code notes ("tracked as follow-up", "known residual") for `cond`.

Stop at 25–30 structures. Fewer, well-described boxes beat a box per file.

### 2. Decide the loop

Pick the one main flow the system exists to run (request → work → storage → read back). That is `MAIN_FLOW`; it is drawn heavy and the puck travels it. Everything else is a secondary or dashed edge. Group the rail by the loop: what feeds it, the loop itself, what it stores, what reads it, what runs beside it.

### 3. Fill the data block

Copy `<skill-dir>/template.html` to a scratch location as `<repo>-map.html` and replace everything between `DATA START` and `DATA END`. The example data (reading-lite) shows every field. Contract:

- `STATS`: `[label, value]` pairs.
- `GROUPS`: `{title, nodes:[ids]}` in rail order. Every node id must appear once.
- `N[id]`: `name`, `sub` (path shown under the key), `count` (files, or what the sub-line says), `at:[x,y]` grid position, `size:[w,d,h]`, optional `slabs` (stack count — use for logs, tables, queue+DLQ), `what:[paragraphs]`, `how:[paragraphs]`, `cond:[items]` (empty array is allowed and renders "Nothing open"), optional `inside:{title, items:[[label, text]]}`.
- `MAIN_FLOW`: ordered ids; repeat the first id at the end if the flow is a loop.
- `EDGES`: `[from, to, "main" | "" | "dash"]`. Every consecutive `MAIN_FLOW` pair needs a `"main"` edge.
- `PLATES`: `{tag, box:[x0,y0,x1,y1], dash?}` outlines under groups of boxes.
- `OVERVIEW.what` / `OVERVIEW.how`: HTML for the empty-selection panel. Keep the three-paragraph shape: what this is, why the diagram has its shape, what the hard problems actually were. Then "how to read it".

Layout rules: grid `x` runs right-down, `y` left-down; one unit ≈ 60 px. Keep ≥ 1 unit of clear space between footprints. Tall boxes (`h` ≥ 2) for indexes and models; wide flat boxes for storage; stacked slabs for anything that is a log or a set of tables. Depth sort is automatic. If a node moves off the plate, widen `PLATES`, not the grid loop.

Text rules: `what` is written for someone who uses the system; `how` names files, functions, and the exact mechanism (CAS, namespace, idempotency key). `<mark>` at most one phrase per paragraph. `<code>` for paths and identifiers. `cond` items cite issue numbers.

### 4. Look once, publish

Render the file once (a browser automation tool on the `file://` URL, one screenshot) and fix only what it shows — usually overlapping boxes or a clipped strip. Then deliver it: if the runtime can publish a hosted HTML artifact, publish it there with a one-sentence description; otherwise write it to `docs/system-map/index.html` only when the user asked for it in the repo, and otherwise leave it in the scratch location and give the path. Do not loop on screenshots.

### 5. Report

Give the link or path, list the structures and interactions in a few bullets, and say whether the repo already has a system map (check `docs/`) so the user can decide about checking it in. Do not add the file to the repo unless asked.

## Notes

- The template is single-theme by design (a committed visual world); it paints its own background and needs no dark-mode tokens.
- Fonts: IBM Plex Mono from Google Fonts with a monospace fallback. No libraries.
- The template starts at `<title>` with no `<html>`/`<body>` wrapper because hosted artifact publishers add one. Browsers tolerate the missing wrapper for local viewing; add it if the file is checked into a repo.
