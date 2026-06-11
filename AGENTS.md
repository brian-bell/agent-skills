# Skills Repo

This repository is the central source for personal AI skills.

The `skill-importer` Rust crate has been split into its own standalone local
repository at `/Users/brian/dev/skill-importer`.

## Current Layout

- The repo root is a small launchpad for guides, workspace commands, and
  compatibility entrypoints.
- `AGENTS.md` is the source of truth for agent context; `CLAUDE.md` is a symlink
  to `AGENTS.md`.
- Portable skills live under `catalog/portable/<skill>`.
- Portable catalog skills are symlinked into both `~/.agents/skills` and
  `~/.claude/skills`.
- Claude-native team skills live under `catalog/claude-native/`.
- `scripts/` contains repo-facing maintenance scripts.
- `docs/` contains consumer-facing documentation.
- `.github/workflows/ci.yml` smoke-tests skill installation on pull requests and
  manual dispatches.
- `.github/workflows/codex.yml` is this repo's collaborator-gated Codex
  entrypoint, backed by the reusable workflow.
- `.github/workflows/claude.yml` is the Claude Code entrypoint for `@claude`
  issue and PR triggers.
- `.github/workflows/autoreview-ship.yml` is a reusable GitHub Actions workflow
  for consumer repositories that should run `$autoreview` before `$ship`.

## Portable Skill Directories

Portable skills currently include:

- `autoreview`
- `commit`
- `chrome-reading-list`
- `docs`
- `grill-me`
- `improve-codebase-architecture`
- `merge-prs-review-loop`
- `planned-implementation-agent`
- `prd-to-issues`
- `prd-to-plan`
- `product-manager`
- `review-loop`
- `ship`
- `skill-parity-audit`
- `tdd`
- `tdd-with-review`
- `work-prs`
- `write-a-prd`

Useful commands from the repo root:

```bash
make install
make check
make test
make clean
```

`make check` smoke-tests installation with a temporary `HOME` so real
`~/.claude/skills` and `~/.agents/skills` entries are not touched.

Consumer setup for the reusable autoreview-gated ship workflow is documented in
`docs/autoreview-ship-workflow.md`.

## Claude-Native Assets

- `catalog/claude-native/go-review-team/` contains the Claude `/go-review`
  launcher and reviewer agents.
- `catalog/claude-native/feature-review-team/` contains the Claude
  `/feature-review` launcher and acceptance reviewer agents.

Do not force Claude-native assets into portable Codex-compatible shape unless
explicitly asked.

## Installation

Run:

```bash
./install.sh
```

The root installer delegates to `scripts/install-skills.sh` and symlinks repo
directories into:

| Repo path | Installed to |
|---|---|
| `catalog/portable/<name>` | `~/.agents/skills/<name>` |
| `catalog/portable/<name>` | `~/.claude/skills/<name>` |
| `catalog/claude-native/go-review-team` | `~/.claude/skills/go-review` |
| `catalog/claude-native/feature-review-team` | `~/.claude/skills/feature-review` |
| `catalog/claude-native/go-review-team/*.md` | `~/.claude/agents/go-review-team/*.md` |
| `catalog/claude-native/feature-review-team/*.md` | `~/.claude/agents/feature-review-team/*.md` |

## Conventions

- Keep portable skill frontmatter minimal: `name` and `description`.
- Put Codex UI metadata in `agents/openai.yaml`.
- Keep Claude-only agent frontmatter in `catalog/claude-native/` files only.
- When adding a new portable skill, update the documented skill inventories and
  `scripts/install-skills.sh`.
- Keep agent context in `AGENTS.md`; keep `CLAUDE.md` as a symlink for Claude
  compatibility.
- Prefer symlinks over copies so `~/dev/skills` remains the single source of
  truth.
- Use Makefile targets for routine local verification when possible.
- Prefer disposable roots in tests and manual smoke runs. Do not let tests or
  manual verification touch real `~/.claude/skills` or `~/.agents/skills`
  unless explicitly configured.
