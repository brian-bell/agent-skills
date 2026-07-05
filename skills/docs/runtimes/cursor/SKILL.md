---
name: docs
description: Update project documentation from the current source of truth. Use when the user asks to refresh, audit, repair, or synchronize AGENTS.md, CLAUDE.md, README.md, docs/, or project documentation with the actual codebase, especially Go repositories with Makefile/go.mod/CI-driven build and test workflows.
---

# Docs

Update documentation so it accurately reflects the current codebase. Source code and checked-in configuration are the source of truth.

## Hard Rules

- Only edit documentation files: `AGENTS.md`, `README.md`, files under `docs/`, and clearly documentation-only Markdown files the user names. You may also create or repair the `CLAUDE.md` symlink to point at `AGENTS.md`.
- `AGENTS.md` is the source of truth for agent context. `CLAUDE.md` must be a symlink to `AGENTS.md` for Claude compatibility.
- If `CLAUDE.md` is a regular file, read it first and preserve any useful context in `AGENTS.md` before replacing it with the symlink. Do not discard unique instructions.
- Do not modify source code, generated code, configs, lockfiles, tests, or build files.
- Preserve the existing tone and structure of each document where possible.
- Remove or correct anything that no longer matches the code.
- When intended behavior is unclear, read more source before editing.

## Workflow

### 1. Gather Current State

Read enough of the codebase to understand what the project actually does.

- Enumerate files with `rg --files`.
- Read all Go source files, including `cmd/`, `model/`, `ui/`, `scanner/`, and any other packages that exist.
- Read `Makefile`, `go.mod`, CI config, release config, and other relevant project configuration.
- Run:

  ```bash
  git log --oneline -20
  ```

- Read `legacy/` if it exists.
- Use non-mutating commands such as `go test ./...`, `make test`, or `gofmt -l .` only when they help verify understanding. Do not run formatting commands that write files.

### 2. Update `AGENTS.md`

Read the existing `AGENTS.md`, or create it if missing. If `CLAUDE.md` exists as a regular file, read it too and fold any useful unique content into `AGENTS.md`. Update `AGENTS.md` to describe:

- What the project is and how it is structured
- How to build, test, and run, such as `make build`, `make test`, or `go run ./cmd/wt` when those commands are actually supported
- Key packages and their responsibilities
- Conventions, patterns, and operational notes that are visible in the code
- Current gotchas or constraints an AI coding agent should know

Remove outdated architecture notes, commands, package descriptions, or workflow claims.

After updating `AGENTS.md`, ensure `CLAUDE.md` is a symlink to `AGENTS.md`. If it is missing, create the symlink. If it is already the correct symlink, leave it alone.

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

### 5. Summarize

After editing, report briefly:

- Which documentation files changed
- What was corrected or added
- Any docs intentionally left unchanged
- Any verification commands run and their result
- Any remaining uncertainty
