# Skills Repo

This repository is the central source for personal AI skills.

## Current Layout

- The repo root is a small launchpad for guides and the installer (`install.sh` builds and execs the Go TUI at `tools/skills-tui/`).
- `AGENTS.md` is the source of truth for agent context; `CLAUDE.md` is a symlink to `AGENTS.md`.
- First-party portable skills live under `skills/<skill>`. Runtime-forked
  skills use `shared/` plus `runtimes/{claude,codex,cursor}/`; legacy portable
  skills still keep a root `SKILL.md`.
- Third-party portable skills live under `third-party/<skill>`.
- Legacy portable skills are copied into `~/.skill-symlinks/skills/`, then
  symlinked into `~/.agents/skills`, `~/.claude/skills`, and
  `~/.cursor/skills`. Runtime-forked first-party skills are assembled into
  `~/.skill-symlinks/runtimes/<runtime>/skills/<name>/` and linked to the
  matching runtime root.
- Agent team packages live under `agent-teams/`; most are Claude-native, while
  packages with `agents/openai.yaml` are hybrid Claude/Codex skills.
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

- `autobuild`
- `autofix`
- `chrome-reading-list`
- `commit`
- `docs`
- `fix-pr`
- `merge-prs-review-loop`
- `plan-with-review`
- `planned-implementation-agent`
- `product-manager`
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
  reviewer agents, and Codex `$go-review` metadata/instructions.
- `agent-teams/feature-review-team/` contains the Claude `/feature-review` launcher and acceptance reviewer agents.

Do not force Claude-native assets into portable Codex-compatible shape unless explicitly asked.

## Hooks

Agent hooks live under `hooks/<hook>/`. Each is self-contained with its own
`install.sh` and is **not** wired into the TUI installer.

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
directly to the binary to operate on another checkout. The former bash
implementation, `scripts/skills-tui.sh`, is retained only as the behavioral
spec/reference for the Go port and is no longer invoked by `install.sh`.

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
Install, uninstall, and on-disk state checks all honor the same target list.
Non-interactive flags: `--all`, `--none`, `--force` (destructive: overwrites
real directories at the targets). The legacy `scripts/install-skills.sh` still
works but is deprecated.

The installer copies or assembles repo directories into `~/.skill-symlinks/`
and points installed symlinks at those staged copies:

| Repo path | Staged copy | Installed to |
|---|---|---|
| `skills/<name>/shared` + `skills/<name>/runtimes/codex` | `~/.skill-symlinks/runtimes/codex/skills/<name>` | `~/.agents/skills/<name>` |
| `skills/<name>/shared` + `skills/<name>/runtimes/claude` | `~/.skill-symlinks/runtimes/claude/skills/<name>` | `~/.claude/skills/<name>` |
| `skills/<name>/shared` + `skills/<name>/runtimes/cursor` | `~/.skill-symlinks/runtimes/cursor/skills/<name>` | `~/.cursor/skills/<name>` |
| legacy `skills/<name>` | `~/.skill-symlinks/skills/<name>` | `~/.agents/skills/<name>` |
| legacy `skills/<name>` | `~/.skill-symlinks/skills/<name>` | `~/.claude/skills/<name>` |
| legacy `skills/<name>` | `~/.skill-symlinks/skills/<name>` | `~/.cursor/skills/<name>` |
| `third-party/<name>` | `~/.skill-symlinks/skills/<name>` | `~/.agents/skills/<name>` |
| `third-party/<name>` | `~/.skill-symlinks/skills/<name>` | `~/.claude/skills/<name>` |
| `third-party/<name>` | `~/.skill-symlinks/skills/<name>` | `~/.cursor/skills/<name>` |
| `agent-teams/go-review-team` | `~/.skill-symlinks/agent-teams/go-review-team` | `~/.agents/skills/go-review` |
| `agent-teams/go-review-team` | `~/.skill-symlinks/agent-teams/go-review-team` | `~/.claude/skills/go-review` |
| `agent-teams/feature-review-team` | `~/.skill-symlinks/agent-teams/feature-review-team` | `~/.claude/skills/feature-review` |
| `agent-teams/go-review-team/*.md` | `~/.skill-symlinks/agent-teams/go-review-team/*.md` | `~/.claude/agents/go-review-team/*.md` |
| `agent-teams/feature-review-team/*.md` | `~/.skill-symlinks/agent-teams/feature-review-team/*.md` | `~/.claude/agents/feature-review-team/*.md` |

## Verification

Run focused checks directly:

```bash
scripts/test-skills-tui-go.sh
scripts/test-skills-tui.sh
scripts/test-install-skills.sh
scripts/test-forked-skills-layout.sh
scripts/test-forked-skills-install.sh
scripts/test-save-codex-session.sh
scripts/test-fix-pr.sh
python3 skills/autobuild/scripts/autobuild_test.py -v
```

The shell tests create temporary homes/repos and exercise the installer, hook,
and PR-comment helper behavior without touching the real installed skill roots.
`scripts/test-skills-tui-go.sh` runs `gofmt`, `go vet`, `go build`, and
`go test` on the Go installer module. `scripts/test-forked-skills-layout.sh`
checks runtime-forked skill shape and overlay token hygiene.
`scripts/test-forked-skills-install.sh` verifies temp-HOME runtime staging.
`scripts/test-save-codex-session.sh` requires `jq`.

## Conventions

- Keep portable skill frontmatter minimal: `name` and `description`. Optional Claude-only fields (`argument-hint`, `disallowed-tools`) are acceptable when the skill degrades gracefully on runtimes that ignore them.
- Put Codex UI metadata for legacy portable skills in `agents/openai.yaml`; for
  runtime-forked first-party skills, put it under
  `runtimes/codex/agents/openai.yaml`.
- Keep Claude-only agent frontmatter in `agent-teams/` files only.
- Agent team packages with `agents/openai.yaml` are hybrid and install into
  both Codex/agents and Claude roots; team packages without it remain
  Claude-only.
- Treat first-party portable skills as shared source for Claude Code, Codex, and
  Cursor. Runtime-forked skills should keep shared scripts/templates/reference
  docs in `shared/` and put runtime instructions in
  `runtimes/{claude,codex,cursor}/SKILL.md`.
- Unmigrated portable skills may still use adjacent `**Platform — Claude Code:**`
  and `**Platform — Codex:**` blocks when runtime-specific behavior is needed.
- In portable skill prose, write skill composition as "run the *skill-name* skill" instead of using Codex-only `$skill` chaining. Keep `$skill` syntax only in Codex `agents/openai.yaml` prompts or literal user-invocation examples.
- Use `<skill-dir>` in portable skill instructions for bundled scripts and assets rather than hardcoding Claude or agents install roots.
- For delegation, Claude Code may use its `Agent`/subagent path. Codex may use subagents only when the user explicitly asks for delegation or parallel agent work and the current surface exposes a documented safe mechanism; otherwise run inline or ask before main-agent execution, and do not claim separate subagent delegation.
- For GitHub-touching skills, Codex should prefer an installed GitHub connector when available and use `gh` when connector coverage is insufficient; Claude Code should use `gh`/CLI unless the user provides another integration.
- When adding a new portable skill, update the documented skill inventories. The TUI installer (`tools/skills-tui/`) discovers skills from disk automatically; update the legacy `scripts/install-skills.sh` only if you still rely on it.
- Keep agent context in `AGENTS.md`; keep `CLAUDE.md` as a symlink for Claude compatibility.
- Keep this repo as the source of truth; `~/.skill-symlinks` is an install cache refreshed by the installer so installed skills survive branch changes.
