# Skills Repo

This repository is the central source for personal AI skills.

## Current Layout

- Portable skills live as top-level directories named after the skill.
- Top-level portable skills are symlinked into both `~/.agents/skills` and `~/.claude/skills`.
- Claude-native team skills live under `claude-native/`.
- `src/` and `tests/` contain the Rust `skill-importer` crate, which exposes merged skill discovery, JSON automation commands, import/enable/disable/promote/delete operations, and an additive keyboard-first terminal UI.
- `plans/` contains implementation plans, including the phased `skill-importer` TUI plan.
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

## Rust Crate

The crate is named `skill-importer`.

- `src/lib.rs` exposes `discover_skills`, `DiscoveryRoots`, and the inventory model for merged skill discovery.
- Discovery reads configurable canonical, import, Claude Code, and Codex roots.
- Missing roots are treated as empty.
- Canonical/imported skill directories are detected from `SKILL.md` frontmatter.
- Agent entries report aggregate enablement for Claude Code, Codex, both, or neither.
- Agent entry status distinguishes real directories, canonical/imported/external symlinks, broken symlinks, and missing entries.
- Regular files in agent roots are ignored.
- Imports support Markdown from stdin, local path imports, URL imports, and repository imports. Repository imports can return an interactive multi-skill selection when more than one valid skill is found.
- Enable/disable operations reuse the core symlink safety checks and return action JSON for automation.
- Promote and delete operations keep imported-skill filesystem mutations in core operation code; the TUI only dispatches operation requests.
- `src/tui/` contains reducer-friendly app state, backend-independent key mapping, ratatui rendering, and the crossterm terminal loop.
- `skill-importer tui` is the interactive entrypoint. Bare `skill-importer` remains a usage error, and JSON commands remain scriptable.
- `tests/discovery.rs` covers merged discovery behavior with temporary directories and symlinks.
- `tests/tui_state.rs`, `tests/tui_input.rs`, and `tests/tui_render.rs` cover TUI state, key mapping, and in-memory render smoke behavior. `src/tui/terminal.rs` has module-local tests for repository operation dispatch through the core provider boundary.

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

Useful commands:

```bash
cargo fmt --check
cargo test
cargo clippy --all-targets -- -D warnings
```

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
- Treat `skill-importer` implementation phases as TDD tracer bullets.
- Keep filesystem safety behavior in core operations and test it through public core/command interfaces. The TUI reducer should emit operation requests and should not reimplement symlink, promotion, deletion, repository scanning, or import validation safety rules.
- Prefer disposable roots in tests and manual TUI smoke runs. Do not let tests or manual verification touch real `~/.claude/skills` or `~/.agents/skills` unless explicitly configured.
