# Skills Repo

This repository is the central source for personal AI skills.

## Current Layout

- The repo root is a small launchpad for guides and the installer (`install.sh` builds and execs the Go TUI at `tools/skills-tui/`).
- `AGENTS.md` is the source of truth for agent context; `CLAUDE.md` is a symlink to `AGENTS.md`.
- First-party portable skills live under `skills/<skill>`. All first-party
  skills are runtime-forked: `shared/` plus required `runtimes/{claude,codex}/`
  overlays. The `runtimes/cursor/` overlay is optional; when absent, the
  installer manages no `~/.cursor/skills/<name>` link and Cursor consumes the
  Claude skill via its `~/.claude/skills` compat scan. Legacy portable skills
  (now only third-party) keep a root `SKILL.md`.
- Third-party portable skills live under `third-party/<skill>`.
- Third-party skills are copied into `~/.skill-symlinks/skills/`, then
  symlinked into `~/.agents/skills`, `~/.claude/skills`, and
  `~/.cursor/skills`. Runtime-forked first-party skills are assembled into
  `~/.skill-symlinks/runtimes/<runtime>/skills/<name>/` and linked to the
  matching runtime root.
- Agent team packages live under `agent-teams/` in two shapes. Flat teams are
  Claude-native, or hybrid Claude/Codex when a root `agents/openai.yaml`
  exists (`go-review-team`). Runtime-forked teams (`feature-review-team`)
  carry `shared/` plus `runtimes/{claude,codex}/` — claude and codex only,
  never a cursor overlay — and are hybrid when
  `runtimes/codex/agents/openai.yaml` exists. Forked teams are assembled into
  `~/.skill-symlinks/runtimes/<runtime>/agent-teams/<team-dir>/` and linked
  from the matching roots.
- Agent hooks live under `hooks/<hook>/`, each with its own `install.sh`.
- `tools/skills-tui/` is the Go implementation of the TUI installer — a
  self-contained Go module (`agent-skills/tools/skills-tui`). `install.sh`
  builds and execs it, hard-requiring the Go toolchain.
- `scripts/` contains repo-facing maintenance scripts.
- Source is mostly Bash, Markdown, and small Python helpers; there is no
  Makefile or package manager manifest at the repo root — the only Go module
  is `tools/skills-tui/`.

## First-Party Skills

First-party portable skills under `skills/`:

- `autobuild` — Claude-runner pipeline: the Claude overlay carries the full
  workflow; the codex/cursor overlays are honest stubs that refuse native
  autobuild and only run the Claude helper on explicit user request.
- `autofix`
- `chrome-reading-list`
- `commit`
- `docs`
- `fix-pr`
- `merge-prs-review-loop`
- `plan-with-review`
- `planned-implementation-agent`
- `product-manager` — orchestrator–subagent PM brief: shared `roles/`
  (surveyor, researcher, brief-critic); Claude + Codex overlays only
  (cursor-less; Cursor loads the Claude skill via `~/.claude/skills`).
- `ship`
- `skill-parity-audit`
- `slice-issues`
- `tdd`
- `tdd-with-review`
- `work-prs`

## Third-Party Skills

Third-party portable skills under `third-party/`. See `third-party/ATTRIBUTION.md` for upstream sources.

- `autoreview`
- `grill-me`
- `improve-codebase-architecture`
- `prd-to-issues`
- `prd-to-plan`
- `review-loop`
- `wizard`
- `write-a-prd`

## Agent Team Assets

- `agent-teams/go-review-team/` contains the Claude `/go-review` launcher,
  reviewer agents, and Codex `$go-review` metadata/instructions. It remains
  the flat hybrid shape (root `SKILL.md` with Platform blocks plus root
  `agents/openai.yaml`).
- `agent-teams/feature-review-team/` is runtime-forked: the five acceptance
  reviewer checklists live in `shared/` (they double as the Claude agent
  definitions), the Claude overlay carries the `/feature-review` launcher
  plus `acceptance-lead.md`, and the Codex overlay carries a `$feature-review`
  lead workflow that fans reviewers out in parallel via the native subagent
  tools (`spawn_agent`/`wait`, with a sequential inline fallback) plus
  `agents/openai.yaml` (with `policy.allow_implicit_invocation: false`).

Agent teams ship no cursor overlay and no `~/.cursor` links: Cursor consumes
the Claude skill via its documented legacy discovery of `~/.claude/skills`
(and reads `~/.claude/agents/` as a legacy subagent location). Claude team
assets are never watered down for Cursor's benefit — Cursor deals with
Claude-native content on its own terms.

Do not force Claude-native assets into portable Codex-compatible shape unless explicitly asked.

## Hooks

Agent hooks live under `hooks/<hook>/`. Each is self-contained with its own
`install.sh` (the standalone entry point still works) and is also wired into
the TUI installer as a `hooks` section: alongside `install.sh`, each hook
carries a `hook.json` manifest that drives the installer's read-only state
detection (`script_source`, `script_target`, `settings_file`, `event`,
`command`). Paths in the manifest may start with `~/`; `command` is stored
exactly as the script writes it into the settings file (a literal
`$HOME/...` string) and is compared verbatim. The installer stages the hook
dir under `~/.skill-symlinks/hooks/<hook>/` and executes the **staged**
`install.sh`, so the hook symlink points at the staged copy and survives
branch changes; all settings-file writes stay in `install.sh` (the Go
installer never edits settings JSON). The hook scripts' `--force` deletes a
real file at the script path, so the installer passes it only for the CLI
`--force` (destroy) — a plain apply against a real file reports `blocked`.

- `hooks/save-codex-session/` - a Codex `Stop` hook that archives each local
  Codex session transcript plus metadata to `~/.agent-sessions/codex/`. Install
  with `hooks/save-codex-session/install.sh` (symlinks the script into
  `~/.codex/hooks/` and merges the hook entry into `~/.codex/hooks.json`;
  `--uninstall` reverses both). `hooks/save-codex-session/backfill.sh` imports
  existing transcripts from `~/.codex/sessions/` and
  `~/.codex/archived_sessions/`. The transcript's own
  `session_meta.payload.id` is authoritative for archive identity, so the
  archive directory name, `metadata.json`, and transcript id always agree.
  `hooks/save-codex-session/validate-archives.sh` audits the store for any
  drift between those three ids.

- `hooks/save-claude-session/` — a `SessionEnd` hook that archives each session's
  transcript plus a metadata sidecar to `~/.agent-sessions/claude/`. Install
  with `hooks/save-claude-session/install.sh` (symlinks the script into
  `~/.claude/hooks/` and merges the hook entry into `~/.claude/settings.json`;
  `--uninstall` reverses both). Hooks are Claude-only, so they install into
  `~/.claude` only. `hooks/save-claude-session/backfill.sh` imports pre-existing
  transcripts from `~/.claude/projects/` into the same store (skip-if-present by
  default; `--update`/`--force`/`--dry-run`).

## Installation

Run:

```bash
./install.sh
```

`install.sh` requires the Go toolchain: it builds the installer at
`tools/skills-tui/` (caching the binary under `tools/skills-tui/bin/` and
rebuilding when any `*.go` or `go.mod` file is newer), then execs it with
`--repo` pointing at the repo root. The `--repo <dir>` flag can also be passed
directly to the binary to operate on another checkout.

The installer is an interactive TUI that discovers
skills from the filesystem and lets you install/uninstall them with the spacebar
(`space` toggle, `a` all, `n` none, `enter` apply, `q` quit). Rows show state:
`installed`, `not installed`, `~ partial`, `will be updated` (selected
upgrade), `⬆ upgrade available` (held upgrade), or `will be removed` (selected
uninstall). Upgradeable skills default to `[x]` and can be toggled to `[-]` to
leave the current staged copy unchanged. Applying refreshes staged copies and
relinks foreign symlinks in place (non-destructive); overwriting a real
directory requires `--force`. Existing repo-pointing symlinks are treated as
upgradeable and migrate to staged symlinks when the installer is applied. When
an existing staged copy is refreshed, the previous copy is backed up under
`~/.skill-symlinks/backups/`.
Uninstall only removes installer-owned staged symlinks — real directories and
foreign symlinks are left untouched.

Set `SKILL_INSTALL_TARGETS` to limit which runtime roots the installer
manages. Default: `agents,claude,cursor`. Example: `SKILL_INSTALL_TARGETS=agents,claude ./install.sh --all`
skips Cursor links. Agent-teams install only when `claude` is included.
Install, uninstall, and on-disk state checks all honor the same target list
for portable skills and agent-teams. Hooks are **not** gated on the target
list (they live in `~/.claude`/`~/.codex` hook roots, outside the targets
model): every install mode manages them regardless of `SKILL_INSTALL_TARGETS`,
so the target list cannot be used to avoid hook settings writes — deselect
hooks in the TUI or leave them uninstalled instead.
Non-interactive flags: `--all`, `--none`, `--force` (destructive: overwrites
real directories at the targets). Note `--all` installs hooks too, which
merges hook entries into `~/.claude/settings.json` / `~/.codex/hooks.json`
(idempotently; the hook scripts back up the settings file before every edit,
and `--none` removes only our entries).

The installer copies or assembles repo directories into `~/.skill-symlinks/`
and points installed symlinks at those staged copies:

| Repo path | Staged copy | Installed to |
|---|---|---|
| `skills/<name>/shared` + `skills/<name>/runtimes/codex` | `~/.skill-symlinks/runtimes/codex/skills/<name>` | `~/.agents/skills/<name>` |
| `skills/<name>/shared` + `skills/<name>/runtimes/claude` | `~/.skill-symlinks/runtimes/claude/skills/<name>` | `~/.claude/skills/<name>` |
| `skills/<name>/shared` + `skills/<name>/runtimes/cursor` | `~/.skill-symlinks/runtimes/cursor/skills/<name>` | `~/.cursor/skills/<name>` (skipped when the cursor overlay is absent) |
| `third-party/<name>` | `~/.skill-symlinks/skills/<name>` | `~/.agents/skills/<name>` |
| `third-party/<name>` | `~/.skill-symlinks/skills/<name>` | `~/.claude/skills/<name>` |
| `third-party/<name>` | `~/.skill-symlinks/skills/<name>` | `~/.cursor/skills/<name>` |
| `agent-teams/go-review-team` | `~/.skill-symlinks/agent-teams/go-review-team` | `~/.agents/skills/go-review` |
| `agent-teams/go-review-team` | `~/.skill-symlinks/agent-teams/go-review-team` | `~/.claude/skills/go-review` |
| `agent-teams/go-review-team/*.md` | `~/.skill-symlinks/agent-teams/go-review-team/*.md` | `~/.claude/agents/go-review-team/*.md` |
| `agent-teams/feature-review-team/shared` + `.../runtimes/codex` | `~/.skill-symlinks/runtimes/codex/agent-teams/feature-review-team` | `~/.agents/skills/feature-review` |
| `agent-teams/feature-review-team/shared` + `.../runtimes/claude` | `~/.skill-symlinks/runtimes/claude/agent-teams/feature-review-team` | `~/.claude/skills/feature-review` |
| `agent-teams/feature-review-team/{shared,runtimes/claude}/*.md` | `~/.skill-symlinks/runtimes/claude/agent-teams/feature-review-team/*.md` | `~/.claude/agents/feature-review-team/*.md` |
| `hooks/save-claude-session` | `~/.skill-symlinks/hooks/save-claude-session` | `~/.claude/hooks/save-session.sh` symlink + `SessionEnd` entry in `~/.claude/settings.json` |
| `hooks/save-codex-session` | `~/.skill-symlinks/hooks/save-codex-session` | `~/.codex/hooks/save-session.sh` symlink + `Stop` entry in `~/.codex/hooks.json` |

## Verification

Run focused checks directly:

```bash
scripts/test-skills-tui-go.sh
scripts/test-install.sh
scripts/test-forked-skills-layout.sh
scripts/test-forked-skills-install.sh
scripts/test-hooks-install.sh
scripts/test-save-codex-session.sh
scripts/test-fix-pr.sh
python3 skills/autobuild/shared/scripts/autobuild_test.py -v
```

The shell tests create temporary homes/repos and exercise the installer, hook,
and PR-comment helper behavior without touching the real installed skill roots.
`scripts/test-skills-tui-go.sh` runs `gofmt`, `go vet`, `go build`, and
`go test` on the Go installer module. `scripts/test-install.sh` exercises the
`./install.sh` entry point against a temp HOME (blocked installs without
`--force`, `--force` overwrite, and `bin/` bootstrap of the cached binary).
`scripts/test-forked-skills-layout.sh`
checks runtime-forked skill shape and overlay token hygiene.
`scripts/test-forked-skills-install.sh` verifies temp-HOME runtime staging.
`scripts/test-hooks-install.sh` round-trips the session hooks through
`./install.sh --all`/`--none` against a temp HOME using the real hook install
scripts — the drift guard between `hooks/*/hook.json` and `hooks/*/install.sh`.
`scripts/test-hooks-install.sh` and `scripts/test-save-codex-session.sh`
require `jq`.

## Conventions

- Keep portable skill frontmatter minimal: `name` and `description`. Optional Claude-only fields (`argument-hint`, `disallowed-tools`) are acceptable when the skill degrades gracefully on runtimes that ignore them.
- Put Codex UI metadata for third-party portable skills in `agents/openai.yaml`;
  for runtime-forked first-party skills, put it under
  `runtimes/codex/agents/openai.yaml`.
- Keep Claude-only agent frontmatter in `agent-teams/` files only.
- Agent team packages with Codex UI metadata are hybrid and install into
  both Codex/agents and Claude roots; team packages without it remain
  Claude-only. Flat teams keep the metadata at root `agents/openai.yaml`;
  runtime-forked teams keep it at `runtimes/codex/agents/openai.yaml`.
- Runtime-forked agent teams carry `shared/` plus `runtimes/{claude,codex}/`
  only — no cursor overlay ever. Agent definitions needed by every runtime
  (reviewer checklists) go in `shared/`; Claude-only orchestrators (team
  leads) go in `runtimes/claude/`. The claude assembly's top-level `*.md`
  files (shared plus claude overlay, minus `SKILL.md`/`README.md`) are what
  gets registered under `~/.claude/agents/<team-dir>/`. Extend
  `scripts/test-forked-skills-layout.sh` and
  `scripts/test-forked-skills-install.sh` when adding one.
- Treat first-party portable skills as shared source for Claude Code, Codex, and
  Cursor. Runtime-forked skills should keep shared scripts/templates/reference
  docs in `shared/` and put runtime instructions in
  `runtimes/{claude,codex}/SKILL.md` (and `runtimes/cursor/SKILL.md` only when
  a distinct Cursor overlay is warranted). Prefer omitting a watered-down
  cursor overlay: Cursor discovers `~/.claude/skills`, so Claude-native skills
  can ship cursor-less and let Cursor consume the Claude overlay.
- Unmigrated portable skills may still use adjacent `**Platform — Claude Code:**`
  and `**Platform — Codex:**` blocks when runtime-specific behavior is needed.
- In portable skill prose, write skill composition as "run the *skill-name* skill" instead of using Codex-only `$skill` chaining. Keep `$skill` syntax only in Codex `agents/openai.yaml` prompts or literal user-invocation examples.
- Use `<skill-dir>` in portable skill instructions for bundled scripts and assets rather than hardcoding Claude or agents install roots.
- For delegation, Claude Code may use its `Agent`/subagent path. Codex native
  subagents are GA and default-on (`spawn_agent`, `wait_agent`, etc.): a skill
  the user explicitly invoked may direct Codex subagent fan-out with explicit
  spawn instructions (Codex only fans out when told), respecting default
  thread/depth limits, with a sequential inline fallback for when spawning is
  unavailable, blocked, or declined. Subagent prompts must be self-contained
  (workers start without parent conversation context). Outside skill-directed
  fan-out, Codex uses subagents only when the user explicitly asks for
  delegation or parallel agent work; otherwise run inline or ask before
  main-agent execution, and never claim separate subagent delegation that did
  not happen.
- For GitHub-touching skills, Codex should prefer an installed GitHub connector when available and use `gh` when connector coverage is insufficient; Claude Code should use `gh`/CLI unless the user provides another integration.
- When adding a new portable skill, update the documented skill inventories. The
  TUI installer (`tools/skills-tui/`) discovers skills from disk automatically.
- When adding a new hook under `hooks/<hook>/`, ship both `install.sh` (owns
  all writes, supports `--uninstall`, uses `--force` only for replacing a real
  file at the script path) and a `hook.json` manifest with all five fields;
  store the settings `command` as the literal `$HOME/...` string the script
  writes. A hooks dir missing either file is skipped by the installer with a
  warning. Extend `scripts/test-hooks-install.sh` to round-trip the new hook.
- Keep agent context in `AGENTS.md`; keep `CLAUDE.md` as a symlink for Claude compatibility.
- Keep this repo as the source of truth; `~/.skill-symlinks` is an install cache refreshed by the installer so installed skills survive branch changes.
