# skills

Central repo for personal AI skills.

Portable skills live as top-level directories and are symlinked into both:

- `~/.agents/skills/<skill>`
- `~/.claude/skills/<skill>`

Claude-native team skills live under `claude-native/`.

The repo also contains the in-progress Rust `skill-importer` crate. It provides merged discovery, JSON automation commands, import and enablement operations, and a keyboard-first terminal UI.

## Portable Skills

- `commit` — Create clean local-only git commits without pushing.
- `chrome-reading-list` — Export Chrome Reading List data to CSV/JSON.
- `docs` — Update `CLAUDE.md`, `README.md`, and `docs/` from source truth.
- `grill-me` — Stress-test a plan or design through one-question-at-a-time interview.
- `improve-codebase-architecture` — Find module-deepening opportunities.
- `prd-to-issues` — Break a PRD into vertical-slice GitHub issues.
- `prd-to-plan` — Turn a PRD into a phased tracer-bullet implementation plan.
- `product-manager` — Product and market analysis workflow.
- `review-loop` — Iterative worker/reviewer quality loop.
- `ship` — Commit, push, and open/reuse a PR.
- `skill-parity-audit` — Compare skill roots for missing, drifted, and broken skills.
- `tdd` — Test-driven development with red/green/refactor loops.
- `work-prs` — Process open non-draft PRs with complete checks, fix failures/blockers, and push targeted fixes.
- `write-a-prd` — Interview, design, and draft a PRD as a GitHub issue.

`rebase` is intentionally not a skill. The old `commands/` layer is retired.

## Skill Importer

`skill-importer` is a Rust crate for inspecting and managing skill roots across canonical local skills, imported skills, Claude Code skills, and Codex skills.

Current behavior:

- Uses configurable roots rather than touching real user directories in tests.
- Reads skill metadata from `SKILL.md` frontmatter.
- Merges canonical/imported and agent-root entries into one inventory.
- Reports whether each skill is enabled for Claude Code, Codex, both, or neither.
- Reports per-agent entry status for real directories, canonical/imported/external symlinks, broken symlinks, and missing entries.
- Ignores regular files in agent skill roots.
- Imports skills from Markdown, local paths, URLs, and repositories.
- Enables and disables canonical/imported skills for Claude Code and/or Codex.
- Promotes imported skills into canonical storage and deletes unpromoted imports.
- Provides an additive `skill-importer tui` entrypoint over the same core operations.

Development commands:

```bash
cargo fmt --check
cargo test
cargo clippy --all-targets -- -D warnings
```

The importer is not installed by `install.sh` yet.

### JSON Commands

Automation commands require `--json` and write stable JSON output:

```bash
skill-importer list --json
skill-importer import markdown --json < SKILL.md
skill-importer import path --json --path ./some-skill
skill-importer import url --json --url https://example.test/skill.md
skill-importer enable --json --skill my-skill --agent codex
skill-importer disable --json --skill my-skill --agent claude-code
skill-importer promote --json --skill my-imported-skill
skill-importer delete --json --skill my-unpromoted-import
```

All commands accept root overrides:

```bash
--canonical-root PATH
--imports-root PATH
--claude-code-root PATH
--codex-root PATH
```

Use root overrides for tests and manual experiments when you do not want to touch real skill directories.

### Running the TUI

Run the interactive TUI with:

```bash
skill-importer tui
```

The TUI is keyboard-first and displays the merged inventory, selected skill detail, active enablement target, keyboard hints, and operation status. It uses the same core operation boundaries as the JSON commands.

Important keys:

```bash
j/k or arrows  move selection
c             target Claude Code
x             target Codex
e             enable selected skill for active target
d             disable selected skill for active target
p             confirm promotion for selected skill
r             confirm deletion for selected import
m             import Markdown from prompt text
f             import local path from prompt text
u             import URL from prompt text
g             import repository from prompt text
enter         confirm prompt, confirmation, or repository candidate
esc           cancel prompt or repository selection
q             quit from the main screen
```

Repository imports that find more than one valid skill enter an interactive candidate selection flow before dispatching the selected import.

## Claude-Native Skills

- `claude-native/go-review-team/` — Claude `/go-review` skill plus Go reviewer agents.
- `claude-native/feature-review-team/` — Claude `/feature-review` skill plus acceptance reviewer agents.

## Installation

Run:

```bash
~/dev/skills/install.sh
```

The installer:

- Symlinks portable top-level skills into `~/.agents/skills`.
- Symlinks portable top-level skills into `~/.claude/skills`.
- Symlinks Claude-native team directories into Claude.
- Removes stale `~/.claude/commands`.
- Removes stale installed `rebase` skills.

## Directory Structure

```text
skills/
├── commit/
├── chrome-reading-list/
├── docs/
├── grill-me/
├── improve-codebase-architecture/
├── prd-to-issues/
├── prd-to-plan/
├── product-manager/              # portable/Codex-compatible
├── review-loop/
├── ship/
├── skill-parity-audit/
├── tdd/
├── work-prs/
├── write-a-prd/
├── src/                          # skill-importer Rust library
├── tests/                        # skill-importer integration tests
├── plans/
├── claude-native/
│   ├── go-review-team/
│   └── feature-review-team/
├── Cargo.toml
└── install.sh
```
