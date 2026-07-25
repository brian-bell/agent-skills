# Skills Repo

This repository is the central source for personal AI skills.

## Current Layout

- The repo root is a small launchpad for guides and the installer (`install.sh` builds and execs the Go TUI at `tools/skills-tui/`).
- `AGENTS.md` is the source of truth for agent context; `CLAUDE.md` is a symlink to `AGENTS.md`.
- First-party portable skills live under `skills/<skill>`. All first-party
  skills are runtime-forked: `shared/` plus `runtimes/{claude,codex}/` overlays.
  They install into `~/.agents/skills` (codex) and `~/.claude/skills` (claude)
  only. Legacy portable skills (now only third-party) keep a root `SKILL.md`.
- Third-party portable skills live under `third-party/<skill>`.
- Third-party skills are copied into `~/.skill-symlinks/skills/`, then
  symlinked into `~/.agents/skills` and `~/.claude/skills`.
  Runtime-forked first-party skills are assembled into
  `~/.skill-symlinks/runtimes/<runtime>/skills/<name>/` and linked to the
  matching runtime root.
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
  workflow; the codex overlay is an honest stub that refuses native autobuild
  and only runs the Claude helper on explicit user request.
- `autofix`
- `chrome-reading-list`
- `commit`
- `docs`
- `feature-review` — inline-orchestrator feature acceptance review: shared
  `roles/` (product, safety, quality, maintainability, documentation).
- `fix-pr`
- `go-review` — inline-orchestrator Go code review: shared `roles/`
  (structure, error, style, security), no lead subagent.
- `merge-prs-review-loop`
- `plan-with-review`
- `planned-implementation-agent`
- `product-manager` — orchestrator–subagent PM brief: shared `roles/`
  (surveyor, researcher, brief-critic).
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
- `last30days`
- `prd-to-issues`
- `prd-to-plan`
- `review-loop`
- `teach`
- `wizard`
- `write-a-prd`

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

## Issue Tracking

This repo uses [beads](https://github.com/steveyegge/beads) (`bd`, issue
prefix `as`) as the issue tracker of record — not GitHub Issues. The
`.beads/` directory holds config, git hooks, and a JSONL export
(`issues.jsonl`, auto-exported for reviewable diffs); the Dolt database
itself is git-ignored and syncs via the configured Dolt remote.

- Browse work with `bd list`, `bd show <id>`, `bd graph <id>`; file new
  issues with `bd create`; close with `bd close`.
- Use dependencies (`bd dep add`), epics (`parent-child` deps), and
  `related` links instead of prose cross-references.
- All pre-migration GitHub issues (#5–#78) were imported with
  `external_ref: gh-<n>` preserved; look up an old GitHub number with
  `bd list --status all --json | grep gh-<n>`. GitHub Issues remain
  enabled for external visitors but are frozen — migrate anything new
  into beads.

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
(`space` toggle, `a` all, `n` none, `i import` from GitHub, `o` open the staging
dir in the OS file manager, `enter` apply, `q` quit). Rows show state:
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
manages. Default: `agents,claude`. Example:
`SKILL_INSTALL_TARGETS=agents ./install.sh --all` manages the Codex/agents
root. Install, uninstall, and on-disk state checks all honor the same target
list. Hooks are **not** gated on the target
list (they live in `~/.claude`/`~/.codex` hook roots, outside the targets
model): every install mode manages them regardless of `SKILL_INSTALL_TARGETS`,
so the target list cannot be used to avoid hook settings writes — deselect
hooks in the TUI or leave them uninstalled instead.
Non-interactive flags: `--all`, `--none`, `--force` (destructive: overwrites
real directories at the targets). Note `--all` installs hooks too, which
merges hook entries into `~/.claude/settings.json` / `~/.codex/hooks.json`
(idempotently; the hook scripts back up the settings file before every edit,
and `--none` removes only our entries).

### Importing third-party skills from GitHub

GitHub import is available only in the interactive TUI; it adds no CLI flag or
environment switch. Press `i import` from the main list, choose a saved URL or
**Paste a new repository URL**, and press `enter` to clone and scan. The
**select skills to import** screen starts every valid, non-conflicting skill
selected: move with arrows or `j`/`k`, toggle with `space`, use `a` for all
valid candidates or `n` for none, and press `enter` to import. Invalid
candidates remain visible but disabled with an actionable reason. `esc` backs
out and requests cancellation of an active scan or import; if publication
completes before that request takes effect, the import succeeds. `ctrl-c` exits
the TUI.

A successful import copies the selected directories into
`third-party/<name>/`, reloads the main list, selects the imported rows, and
moves the cursor to the first one. It does not refresh the staged cache or
create runtime links. **Press Enter to apply installation** with the normal
installer after reviewing the pending selections; existing row selections are
preserved across the import reload.

Saved repository history is user-local JSON at
`<user-config-dir>/agent-skills/import-repositories.json`, where
`<user-config-dir>` is Go's `os.UserConfigDir()` (on macOS this is normally
`~/Library/Application Support`). The installer stores normalized URLs plus
added/last-used timestamps atomically, tightens its dedicated `agent-skills`
directory to `0700`, and keeps the history and lock files at `0600`. A URL is
added or refreshed only after a successful scan yields at least one valid,
non-conflicting candidate. Records appear in
most-recently-used order; rescanning a saved URL moves it to the front without
creating a duplicate.

In the repository picker, put the cursor on a saved URL and press `d`, then
confirm the `y/N` prompt with `y`; `N`, `enter`, or `esc` keeps it. Deletion is
strictly a picker-history operation. It **does not delete imported skills**,
change `third-party/ATTRIBUTION.md`, remove staged copies under
`~/.skill-symlinks/`, or alter any installed runtime links.

The accepted URL scope is deliberately narrow: HTTPS
`https://github.com/<owner>/<repository>`, optionally followed by `.git`, one
trailing slash, or both. Owner/repository case is normalized to lowercase and
the suffix/slash is removed before persistence. HTTP and SSH URLs, embedded
credentials, explicit ports, escapes, queries, fragments, branch and subpath
URLs, GitHub Enterprise hosts, and other forges are rejected. Clone uses
shell-free argument passing, `--depth 1`, and `--no-tags`; it inherits the
process's existing Git authentication but sets `GIT_TERMINAL_PROMPT=0` and
`GCM_INTERACTIVE=Never`. Private repositories therefore require credentials
that already work non-interactively. Scan and import accept cancellation. The
provider attempts to remove every owned temporary checkout on every exit path;
cleanup failures are surfaced to the caller and can leave the session directory
for manual removal.

Scanning walks the checkout root and nested directories (including hidden
compatibility roots), skips `.git`, reads only real regular `SKILL.md` files,
and never executes repository content. YAML frontmatter must contain non-empty
`name` and `description` values. Unsafe install names and case-insensitive
duplicate candidate names are disabled, as are names colliding with existing
first-party skills or third-party entries. After a
directory is accepted as a valid skill root, scanning prunes that directory;
descendant `SKILL.md` files are not offered as separate candidates.

Import holds a repository-wide lock, revalidates the selected candidates and
all collisions, stages the complete selected batch under `third-party/`, and
publishes with atomic no-overwrite operations. It copies full skill trees while
excluding `.git`, preserves regular-file modes, and rejects symlinks and
special files. Cancellation is best-effort: when the worker observes it during
checkout, scan, or tree staging, no skill has been published; if publication
wins the race, the completed import is returned as success. Any validation,
copy, publication, or attribution failure stops the transaction; if skill
destinations were already published, the importer makes a best-effort rollback
before returning. Filesystem cleanup failures can still prevent complete
rollback, so they are joined to the original failure and surfaced to the
caller.

The no-overwrite guarantee applies to imported skill destinations.
`third-party/ATTRIBUTION.md` is the expected mutable repository file: its
updated content is staged with the batch and replaces the prior file last. The
repository import lock serializes importer transactions, but not arbitrary
editors, so do not edit attribution concurrently with an import.

Each result lands at `third-party/<name>/`. Its new row in
`third-party/ATTRIBUTION.md` links to
`https://github.com/<owner>/<repository>/tree/<commit>/<candidate-subpath>` (or
the commit root for a root skill), so provenance is pinned to the scanned
commit and source directory. Automatic imports do not verify licensing and
record the license as `Unknown (unverified)` for later human review.

The installer copies or assembles repo directories into `~/.skill-symlinks/`
and points installed symlinks at those staged copies:

| Repo path | Staged copy | Installed to |
|---|---|---|
| `skills/<name>/shared` + `skills/<name>/runtimes/codex` | `~/.skill-symlinks/runtimes/codex/skills/<name>` | `~/.agents/skills/<name>` |
| `skills/<name>/shared` + `skills/<name>/runtimes/claude` | `~/.skill-symlinks/runtimes/claude/skills/<name>` | `~/.claude/skills/<name>` |
| `third-party/<name>` | `~/.skill-symlinks/skills/<name>` | `~/.agents/skills/<name>` |
| `third-party/<name>` | `~/.skill-symlinks/skills/<name>` | `~/.claude/skills/<name>` |
| `hooks/save-claude-session` | `~/.skill-symlinks/hooks/save-claude-session` | `~/.claude/hooks/save-session.sh` symlink + `SessionEnd` entry in `~/.claude/settings.json` |
| `hooks/save-codex-session` | `~/.skill-symlinks/hooks/save-codex-session` | `~/.codex/hooks/save-session.sh` symlink + `Stop` entry in `~/.codex/hooks.json` |

## Verification

Run focused checks directly:

```bash
# GitHub import workflow and complete Go module
(
  cd tools/skills-tui
  env -u GOROOT go test -race ./internal/importer ./internal/tui
  env -u GOROOT go test ./...
)

# Go installer and installation regressions
env -u GOROOT scripts/test-skills-tui-go.sh
env -u GOROOT scripts/test-install.sh
env -u GOROOT scripts/test-forked-skills-install.sh

# Documentation source of truth
test -L CLAUDE.md && test "$(readlink CLAUDE.md)" = AGENTS.md

# Other focused repository checks
scripts/test-forked-skills-layout.sh
scripts/test-hooks-install.sh
scripts/test-save-codex-session.sh
scripts/test-fix-pr.sh
scripts/test-autoreview.sh
python3 skills/autobuild/shared/scripts/autobuild_test.py -v
```

The `env -u GOROOT` prefix makes Go-backed checks use the selected `go`
binary's own toolchain root rather than a stale shell override.

The shell tests create temporary homes/repos and exercise the installer, hook,
and PR-comment helper behavior without touching the real installed skill roots.
`scripts/test-skills-tui-go.sh` runs `gofmt`, `go vet`, `go build`, and
`go test` on the Go installer module. `scripts/test-install.sh` exercises the
`./install.sh` entry point against a temp HOME (blocked installs without
`--force`, `--force` overwrite, and `bin/` bootstrap of the cached binary).
`scripts/test-forked-skills-layout.sh` checks runtime-forked skill shape and
overlay token hygiene, including the inline-orchestrator contract for the
review skills (role briefs in `shared/roles/`, no lead agent definition, no
`agent-teams/`). `scripts/test-forked-skills-install.sh` verifies temp-HOME
runtime staging; it seeds a pre-migration agent-team layout first, so the
first apply exercises the upgrade path and the legacy-registration prune
rather than a clean install.
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
- Multi-reviewer skills use the orchestrator–role shape, not registered agent
  definitions: reviewer briefs are runtime-neutral prompt source in
  `shared/roles/`, and the orchestrator runs **inline in the main session**
  rather than being delegated to a lead subagent. An inline orchestrator can
  check in with the user (`AskUserQuestion`) and read each role's full report
  directly; a forked lead can do neither. `go-review`, `feature-review`, and
  `product-manager` all follow this shape.
- Role briefs carry no frontmatter and must restate their own read-only
  `<HARD-GATE>`. Prose is the whole constraint on a dispatched leaf worker,
  so the gate must name shell and git mutation explicitly — a `tools:` list
  that grants Bash never enforced read-only, it only blocked Edit/Write.
- Nothing installs into `~/.claude/agents/`. If you ever need a registered
  agent definition again, weigh it against what the inline shape buys, and
  extend `scripts/test-forked-skills-layout.sh` and
  `scripts/test-forked-skills-install.sh` for the new shape.
- Treat first-party portable skills as shared source for Claude Code and Codex.
  Runtime-forked skills keep shared scripts/templates/reference docs in
  `shared/` and put runtime instructions in `runtimes/{claude,codex}/SKILL.md`.
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
