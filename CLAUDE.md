# Skills Repo

This repository is the central source for personal AI skills.

## Current Layout

- Portable skills live as top-level directories named after the skill.
- Top-level portable skills are symlinked into both `~/.agents/skills` and `~/.claude/skills`.
- Claude-native team skills live under `claude-native/`.
- `commands/` and `codex-skills/` are retired.
- `rebase` is intentionally not a skill.

## Portable Skill Directories

Portable skills currently include:

- `commit`
- `chrome-reading-list`
- `docs`
- `grill-me`
- `improve-codebase-architecture`
- `prd-to-issues`
- `prd-to-plan`
- `product-manager`
- `review-loop`
- `ship`
- `skill-parity-audit`
- `tdd`
- `work-prs`
- `write-a-prd`

## Claude-Native Assets

- `claude-native/go-review-team/` contains the Claude `/go-review` launcher and reviewer agents.
- `claude-native/feature-review-team/` contains the Claude `/feature-review` launcher and acceptance reviewer agents.

Do not force Claude-native assets into portable Codex-compatible shape unless explicitly asked.

## Installation

Run:

```bash
./install.sh
```

The installer symlinks repo directories into:

| Repo path | Installed to |
|---|---|
| Top-level portable skill | `~/.agents/skills/<name>` |
| Top-level portable skill | `~/.claude/skills/<name>` |
| `claude-native/go-review-team` | `~/.claude/skills/go-review` |
| `claude-native/feature-review-team` | `~/.claude/skills/feature-review` |
| `claude-native/go-review-team/*.md` | `~/.claude/agents/go-review-team/*.md` |
| `claude-native/feature-review-team/*.md` | `~/.claude/agents/feature-review-team/*.md` |

The installer also removes stale `~/.claude/commands` and stale installed `rebase` skills.

## Conventions

- Keep portable skill frontmatter minimal: `name` and `description`.
- Put Codex UI metadata in `agents/openai.yaml`.
- Keep Claude-only agent frontmatter in `claude-native/` files only.
- Prefer symlinks over copies so `~/dev/skills` remains the single source of truth.
- Do not reintroduce the old `commands/` or `codex-skills/` split.
