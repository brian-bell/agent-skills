---
name: improve-docs
description: Improve existing project documentation by trimming AGENTS.md and README.md to their essential roles, consolidating duplication, moving durable technical detail into dedicated docs, and removing dead prose. Use when the user asks to simplify, streamline, consolidate, declutter, or improve project documentation after or alongside an accuracy refresh.
---

# Improve Docs

Make accurate project documentation smaller, clearer, and easier to navigate. Give each document one job and keep detail only where it remains useful.

## Hard Rules

- Only edit documentation files: `AGENTS.md`, `README.md`, files under `docs/`, and clearly documentation-only Markdown files the user names. You may also create or repair the `CLAUDE.md` symlink to point at `AGENTS.md`.
- Preserve unique instructions, safety constraints, working commands, and information needed to use or maintain the project.
- Do not discard useful information merely because it is too detailed for `AGENTS.md` or `README.md`; consolidate it into an appropriate dedicated document.
- Do not move trimmed documentation into source comments, code, configuration, tests, or generated files.
- Do not create a new document just to hold miscellaneous leftovers. New documentation must have a clear audience and durable purpose.
- Treat source code and checked-in configuration as the source of truth. Remove or correct prose that contradicts them.

## Workflow

### 1. Establish an accurate baseline

Run the *docs* skill first when it is available so the documentation reflects the current codebase. Otherwise, inspect the source and configuration needed to verify every passage you may edit. Do not streamline inaccurate documentation without correcting it.

### 2. Assign each document a role

Use these defaults unless the repository clearly defines different roles:

- `AGENTS.md`: the minimum project context, commands, conventions, and constraints an AI coding agent needs to work safely.
- `README.md`: practical user-facing orientation, installation, and first-use guidance.
- `docs/`: durable detail such as architecture, operations, APIs, maintenance procedures, and deeper guides.

Identify passages that are duplicated, misplaced, stale, overly detailed for their location, or no longer useful.

### 3. Trim and consolidate

- Reduce `AGENTS.md` and `README.md` to the bare necessities for their audiences.
- Replace repeated detail with a short explanation and a link to its canonical document.
- Merge overlapping passages instead of preserving multiple slightly different versions.
- Move genuinely useful technical notes into the most relevant existing dedicated document. Create a focused document only when no suitable home exists.
- Delete stale claims, generic filler, editing narration, redundant examples, empty sections, and prose that adds no actionable or explanatory value.

### 4. Check the result

Read every edited document as a whole and confirm:

- `AGENTS.md` and `README.md` remain sufficient for their intended audiences.
- Moved content has one discoverable canonical home and links still resolve.
- No unique instruction, constraint, or working command was lost.
- Terminology and claims agree across documents and with the codebase.
- The prose reads cleanly, without traces of accretive editing or dead prose.

Use non-mutating checks where helpful, and inspect the final diff to ensure only documentation changed.

### 5. Summarize

Report which documents changed, what was removed or relocated, any new canonical locations, verification performed, and remaining uncertainty.
