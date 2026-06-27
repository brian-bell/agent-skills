# Skills Repo

This repository is the central source for personal AI skills.

## Current Layout

- The repo root is a small launchpad for guides and the installer (`install.sh` → `scripts/skills-tui.sh`).
- `AGENTS.md` is the source of truth for agent context; `CLAUDE.md` is a symlink to `AGENTS.md`.
- First-party portable skills live under `skills/<skill>`.
- Third-party portable skills live under `third-party/<skill>`.
- Portable skills (first- and third-party) are copied into `~/.skill-symlinks/skills/`, then symlinked into both `~/.agents/skills` and `~/.claude/skills`.
- Claude-native team skills live under `agent-teams/`.
- Agent hooks live under `hooks/<hook>/`, each with its own `install.sh`.
- `scripts/` contains repo-facing maintenance scripts.

## First-Party Skills

First-party portable skills under `skills/`:

- `autobuild`
- `chrome-reading-list`
- `commit`
- `docs`
- `merge-prs-review-loop`
- `plan-with-review`
- `planned-implementation-agent`
- `product-manager`
- `ship`
- `skill-parity-audit`
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
- `write-a-prd`

## Claude-Native Assets

- `agent-teams/go-review-team/` contains the Claude `/go-review` launcher and reviewer agents.
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

`install.sh` launches `scripts/skills-tui.sh`, an interactive TUI that discovers
skills from the filesystem and lets you install/uninstall them with the spacebar
(`space` toggle, `a` all, `n` none, `enter` apply, `q` quit). Rows show state:
`installed`, `not installed`, `~ partial`, or `⬆ upgrade available` (the target
or staged copy differs from the repo). Applying refreshes staged copies and
relinks foreign symlinks in place (non-destructive); overwriting a real
directory requires `--force`. Existing repo-pointing symlinks are treated as
upgradeable and migrate to staged symlinks when the installer is applied. When
an existing staged copy is refreshed, the previous copy is backed up under
`~/.skill-symlinks/backups/`.
Uninstall only removes installer-owned staged symlinks — real directories and
foreign symlinks are left untouched.

Non-interactive flags: `--all`, `--none`, `--force` (destructive: overwrites
real directories at the targets). The legacy `scripts/install-skills.sh` still
works but is deprecated.

The installer copies repo directories into `~/.skill-symlinks/` and points
installed symlinks at those staged copies:

| Repo path | Staged copy | Installed to |
|---|---|---|
| `skills/<name>` | `~/.skill-symlinks/skills/<name>` | `~/.agents/skills/<name>` |
| `skills/<name>` | `~/.skill-symlinks/skills/<name>` | `~/.claude/skills/<name>` |
| `third-party/<name>` | `~/.skill-symlinks/skills/<name>` | `~/.agents/skills/<name>` |
| `third-party/<name>` | `~/.skill-symlinks/skills/<name>` | `~/.claude/skills/<name>` |
| `agent-teams/go-review-team` | `~/.skill-symlinks/agent-teams/go-review-team` | `~/.claude/skills/go-review` |
| `agent-teams/feature-review-team` | `~/.skill-symlinks/agent-teams/feature-review-team` | `~/.claude/skills/feature-review` |
| `agent-teams/go-review-team/*.md` | `~/.skill-symlinks/agent-teams/go-review-team/*.md` | `~/.claude/agents/go-review-team/*.md` |
| `agent-teams/feature-review-team/*.md` | `~/.skill-symlinks/agent-teams/feature-review-team/*.md` | `~/.claude/agents/feature-review-team/*.md` |

## Conventions

- Keep portable skill frontmatter minimal: `name` and `description`.
- Put Codex UI metadata in `agents/openai.yaml`.
- Keep Claude-only agent frontmatter in `agent-teams/` files only.
- When adding a new portable skill, update the documented skill inventories. The TUI installer (`scripts/skills-tui.sh`) discovers skills from disk automatically; update the legacy `scripts/install-skills.sh` only if you still rely on it.
- Keep agent context in `AGENTS.md`; keep `CLAUDE.md` as a symlink for Claude compatibility.
- Keep this repo as the source of truth; `~/.skill-symlinks` is an install cache refreshed by the installer so installed skills survive branch changes.
