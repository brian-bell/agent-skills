---
name: docs
description: Update project documentation from the current source of truth. Use when the user asks to refresh, audit, repair, or synchronize AGENTS.md, CLAUDE.md, README.md, docs/, or project documentation with the actual codebase.
---

# Docs

Update documentation so it accurately reflects the current codebase. Source code and checked-in configuration are the source of truth.

## Hard Rules

- Only edit documentation files: `AGENTS.md`, `README.md`, files under `docs/`, and clearly documentation-only Markdown files the user names.
- `AGENTS.md` is the source of truth for agent context.
- Do not modify source code, generated code, configs, lockfiles, tests, or build files.
- Preserve the existing tone and structure of each document where possible.
- Remove or correct anything that no longer matches the code.
- When intended behavior is unclear, read more source before editing.

## Workflow

### 1. Gather Current State

Read enough of the codebase to understand what the project actually does.

- Enumerate files with `rg --files`.
- Read the source for every entry point, module, and package the docs describe or should describe.
- Read the build and dependency manifests (such as `Makefile`, `package.json`, `go.mod`, `pyproject.toml`, or `Cargo.toml`), CI config, release config, and other relevant project configuration.
- Run:

  ```bash
  git log --oneline -20
  ```

- Read `legacy/` if it exists.
- Use the project's non-mutating test, lint, or check commands only when they help verify understanding. Do not run formatting commands that write files.

### 2. Update `AGENTS.md`

Read the existing `AGENTS.md`, or create it if missing.

Correct what is already there first. Remove outdated architecture notes, commands, package descriptions, or workflow claims.

Add content only when it is missing and an AI coding agent needs it to work safely:

- What the project is and how it is structured
- How to build, test, and run, using only commands the project actually supports
- Key packages or modules and their responsibilities
- Conventions, patterns, and operational notes that are visible in the code
- Current gotchas or constraints

Do not expand existing sections with detail an agent does not need.

### 3. Update `README.md`

Read the existing `README.md`, or create it if missing only when the project clearly needs one. Compare it against the actual code and update:

- Features and commands so they match the current CLI/API/UI
- Installation instructions so they match the build system
- Usage examples so they work with the current interface
- Requirements so they list actual dependencies
- References to docs, packages, commands, or features so they point to things that exist

Keep user-facing README content clear and practical. Do not expose internal-only implementation notes unless the README already serves that purpose.

### 4. Scan `docs/`

If `docs/` exists, read every file in it.

- Flag or fix content that contradicts the current source code.
- Update outdated instructions, API references, architecture descriptions, and command examples.
- Remove docs for features that no longer exist when they are plainly obsolete.
- Keep valid historical or design docs when they are clearly labeled as historical.

If no `docs/` directory exists, skip this step silently.

### 5. Final pass

Do a final pass on all edited docs to ensure they read cleanly and consistently. Remove any traces of accretive editing.

### 6. Summarize

After editing, report briefly:

- Which documentation files changed
- What was corrected or added
- Any docs intentionally left unchanged
- Any verification commands run and their result
- Any remaining uncertainty
