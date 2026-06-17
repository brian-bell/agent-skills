# Skills Repo

This repository is the central source for personal AI skills.

## Current Layout

- The repo root is a small launchpad for guides, the installer, and compatibility entrypoints.
- `AGENTS.md` is the source of truth for agent context; `CLAUDE.md` is a symlink to `AGENTS.md`.
- First-party portable skills live under `skills/<skill>`.
- Third-party portable skills live under `third-party/<skill>`.
- Portable skills (first- and third-party) are symlinked into both `~/.agents/skills` and `~/.claude/skills`.
- Claude-native team skills live under `agent-teams/`.
- `scripts/` contains repo-facing maintenance scripts.

## First-Party Skills

First-party portable skills under `skills/`:

- `chrome-reading-list`
- `commit`
- `docs`
- `merge-prs-review-loop`
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

## Installation

Run:

```bash
./install.sh
```

The root installer delegates to `scripts/install-skills.sh` and symlinks repo directories into:

| Repo path | Installed to |
|---|---|
| `skills/<name>` | `~/.agents/skills/<name>` |
| `skills/<name>` | `~/.claude/skills/<name>` |
| `third-party/<name>` | `~/.agents/skills/<name>` |
| `third-party/<name>` | `~/.claude/skills/<name>` |
| `agent-teams/go-review-team` | `~/.claude/skills/go-review` |
| `agent-teams/feature-review-team` | `~/.claude/skills/feature-review` |
| `agent-teams/go-review-team/*.md` | `~/.claude/agents/go-review-team/*.md` |
| `agent-teams/feature-review-team/*.md` | `~/.claude/agents/feature-review-team/*.md` |

## Conventions

- Keep portable skill frontmatter minimal: `name` and `description`.
- Put Codex UI metadata in `agents/openai.yaml`.
- Keep Claude-only agent frontmatter in `agent-teams/` files only.
- When adding a new portable skill, update the documented skill inventories and `scripts/install-skills.sh`.
- Keep agent context in `AGENTS.md`; keep `CLAUDE.md` as a symlink for Claude compatibility.
- Prefer symlinks over copies so `~/dev/skills` remains the single source of truth.
