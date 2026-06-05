# skills

Central repo for personal AI skills.

Portable skills live as top-level directories and are symlinked into both:

- `~/.agents/skills/<skill>`
- `~/.claude/skills/<skill>`

Claude-native team skills live under `claude-native/`.

The repo also contains the in-progress Rust `skill-importer` crate. It currently provides the Phase 1 discovery library for a future terminal UI.

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

`skill-importer` is a Rust crate for inspecting skill roots across canonical local skills, imported skills, Claude Code skills, and Codex skills.

Current behavior:

- Uses configurable roots rather than touching real user directories in tests.
- Reads skill metadata from `SKILL.md` frontmatter.
- Merges canonical/imported and agent-root entries into one inventory.
- Reports whether each skill is enabled for Claude Code, Codex, both, or neither.
- Reports per-agent entry status for real directories, canonical/imported/external symlinks, broken symlinks, and missing entries.
- Ignores regular files in agent skill roots.

Development commands:

```bash
cargo fmt --check
cargo test
cargo clippy --all-targets -- -D warnings
```

The importer is not installed by `install.sh` yet.

### Running the TUI

The TUI is planned but not runnable yet. The current crate is Phase 1 of the implementation and only exposes the discovery library used by future CLI/TUI surfaces.

For now, verify the importer core with:

```bash
cargo test
```

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
