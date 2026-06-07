# Skills Repo

This repository is the central source for personal AI skills and the `skill-importer` management tool.

## Current Layout

- The repo root is a small launchpad for guides, workspace commands, and compatibility entrypoints.
- `AGENTS.md` is the source of truth for agent context; `CLAUDE.md` is a symlink to `AGENTS.md`.
- Portable skills live under `catalog/portable/<skill>`.
- Portable catalog skills are symlinked into both `~/.agents/skills` and `~/.claude/skills`.
- Claude-native team skills live under `catalog/claude-native/`.
- `tools/skill-importer/` contains the Rust `skill-importer` crate, which exposes merged skill discovery, JSON automation commands, import/enable/disable/promote/delete operations, and an additive keyboard-first terminal UI.
- `scripts/` contains repo-facing maintenance scripts.
- `plans/` contains implementation plans, including the phased `skill-importer` TUI plan.
- `.github/workflows/autoreview-ship.yml` is a reusable GitHub Actions workflow for consumer repositories that should run `$autoreview` before `$ship`.

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

## Rust Crate

The crate is named `skill-importer` and lives in `tools/skill-importer/`.

- `tools/skill-importer/src/lib.rs` exposes `discover_skills`, `DiscoveryRoots`, and the inventory model for merged skill discovery.
- Discovery reads configurable canonical, import, Claude Code, and Codex roots.
- When launched inside this repo without `--canonical-root`, the default canonical root is `catalog/portable/`.
- Missing roots are treated as empty.
- Canonical/imported skill directories are detected from `SKILL.md` frontmatter.
- Agent entries report aggregate enablement for Claude Code, Codex, both, or neither.
- Agent entry status distinguishes real directories, canonical/imported/external symlinks, broken symlinks, and missing entries.
- Regular files in agent roots are ignored.
- Imports support Markdown from stdin, local path imports, URL imports, and repository imports. Repository imports can return an interactive multi-skill selection when more than one valid skill is found.
- Enable/disable operations reuse the core symlink safety checks and return action JSON for automation.
- Promote and delete operations keep imported-skill filesystem mutations in core operation code; the TUI only dispatches operation requests.
- `tools/skill-importer/src/tui/` contains reducer-friendly app state, backend-independent key mapping, ratatui rendering, and the crossterm terminal loop.
- `skill-importer tui` is the interactive entrypoint. Bare `skill-importer` remains a usage error, and JSON commands remain scriptable.
- `tools/skill-importer/tests/discovery.rs` covers merged discovery behavior with temporary directories and symlinks.
- `tools/skill-importer/tests/tui_state.rs`, `tools/skill-importer/tests/tui_input.rs`, and `tools/skill-importer/tests/tui_render.rs` cover TUI state, key mapping, and in-memory render smoke behavior. `tools/skill-importer/src/tui/terminal.rs` has module-local tests for repository operation dispatch through the core provider boundary.

Supported command surfaces:

```bash
skill-importer list --json [--canonical-root PATH] [--imports-root PATH] [--claude-code-root PATH] [--codex-root PATH]
skill-importer import markdown --json [--source-location VALUE] [--canonical-root PATH] [--imports-root PATH] [--claude-code-root PATH] [--codex-root PATH]
skill-importer import path --json --path PATH [--canonical-root PATH] [--imports-root PATH] [--claude-code-root PATH] [--codex-root PATH]
skill-importer import url --json --url URL [--canonical-root PATH] [--imports-root PATH] [--claude-code-root PATH] [--codex-root PATH]
skill-importer enable --json --skill NAME --agent claude-code|codex [--agent claude-code|codex] [--canonical-root PATH] [--imports-root PATH] [--claude-code-root PATH] [--codex-root PATH]
skill-importer disable --json --skill NAME --agent claude-code|codex [--agent claude-code|codex] [--canonical-root PATH] [--imports-root PATH] [--claude-code-root PATH] [--codex-root PATH]
skill-importer promote --json --skill NAME [--canonical-root PATH] [--imports-root PATH] [--claude-code-root PATH] [--codex-root PATH]
skill-importer delete --json --skill NAME [--canonical-root PATH] [--imports-root PATH] [--claude-code-root PATH] [--codex-root PATH]
skill-importer tui [--canonical-root PATH] [--imports-root PATH] [--claude-code-root PATH] [--codex-root PATH]
```

Useful commands from the repo root:

```bash
cargo fmt --check
cargo test
cargo clippy --all-targets -- -D warnings
```

Consumer setup for the reusable autoreview-gated ship workflow is documented in
`docs/autoreview-ship-workflow.md`.

## Claude-Native Assets

- `catalog/claude-native/go-review-team/` contains the Claude `/go-review` launcher and reviewer agents.
- `catalog/claude-native/feature-review-team/` contains the Claude `/feature-review` launcher and acceptance reviewer agents.

Do not force Claude-native assets into portable Codex-compatible shape unless explicitly asked.

## Installation

Run:

```bash
./install.sh
```

The root installer delegates to `scripts/install-skills.sh` and symlinks repo directories into:

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
- When adding a new portable skill, update the documented skill inventories and `scripts/install-skills.sh`.
- Keep agent context in `AGENTS.md`; keep `CLAUDE.md` as a symlink for Claude compatibility.
- Prefer symlinks over copies so `~/dev/skills` remains the single source of truth.
- Treat `skill-importer` implementation phases as TDD tracer bullets.
- Keep filesystem safety behavior in core operations and test it through public core/command interfaces. The TUI reducer should emit operation requests and should not reimplement symlink, promotion, deletion, repository scanning, or import validation safety rules.
- Prefer disposable roots in tests and manual TUI smoke runs. Do not let tests or manual verification touch real `~/.claude/skills` or `~/.agents/skills` unless explicitly configured.
