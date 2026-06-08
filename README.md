# skills

Central repo for personal AI skills and the `skill-importer` tooling used to inspect and manage them.

The repo root is a small launchpad. `AGENTS.md` is the source of truth for agent context, and `CLAUDE.md` is a symlink to it for Claude compatibility. The actual material is split by purpose:

- `catalog/portable/` contains portable skills that can be symlinked into both Codex/agents and Claude Code.
- `catalog/claude-native/` contains Claude-only team skills and reviewer agents.
- `tools/skill-importer/` contains the Rust crate for JSON automation commands and the keyboard-first TUI.
- `scripts/` contains repository maintenance scripts.
- `plans/` contains implementation plans.
- `.github/workflows/autoreview-ship.yml` is a reusable GitHub Actions workflow
  that other repositories can call to run `$autoreview` before `$ship`.

## Portable Skills

- `autoreview` - Run structured code review as a closeout check on local or PR branches.
- `commit` - Create clean local-only git commits without pushing.
- `chrome-reading-list` - Export Chrome Reading List data to CSV/JSON.
- `docs` - Update `AGENTS.md`, keep `CLAUDE.md` symlinked to it, and refresh `README.md`/`docs/` from source truth.
- `grill-me` - Stress-test a plan or design through one-question-at-a-time interview.
- `improve-codebase-architecture` - Find module-deepening opportunities.
- `merge-prs-review-loop` - Review and merge PR batches with conflict-aware review-loop gates.
- `planned-implementation-agent` - Plan, review, and delegate implementation work with TDD and review-loop gates.
- `prd-to-issues` - Break a PRD into vertical-slice GitHub issues.
- `prd-to-plan` - Turn a PRD into a phased tracer-bullet implementation plan.
- `product-manager` - Product and market analysis workflow.
- `review-loop` - Iterative worker/reviewer quality loop.
- `ship` - Commit, push, and open/reuse a PR.
- `skill-parity-audit` - Compare skill roots for missing, drifted, and broken skills.
- `tdd` - Test-driven development with red/green/refactor loops.
- `tdd-with-review` - Implement with TDD, run review-loop, then ship.
- `work-prs` - Process open non-draft PRs with complete checks, fix failures/blockers, and push targeted fixes.
- `write-a-prd` - Interview, design, and draft a PRD as a GitHub issue.

## Skill Importer

`skill-importer` is a Rust crate in `tools/skill-importer/` for inspecting and managing skill roots across canonical local skills, imported skills, Claude Code skills, and Codex skills.

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

Development commands run from the repo root through the Cargo workspace:

```bash
cargo fmt --check
cargo test
cargo clippy --all-targets -- -D warnings
```

The importer is not installed by `install.sh` yet.

## Reusable GitHub Workflow

Other repositories can call this repo's shared autoreview-gated ship workflow.
The reusable workflow runs `$autoreview` as an explicit gate, then invokes
`openai/codex-action` for `$ship` only after the review gate passes:

```yaml
jobs:
  autoreview_ship:
    uses: brian-bell/skills/.github/workflows/autoreview-ship.yml@main
    permissions:
      contents: write
      pull-requests: write
      issues: write
      actions: read
    secrets: inherit
```

See `docs/autoreview-ship-workflow.md` for the full consumer workflow,
required `OPENAI_API_KEY` secret, inputs, and safety notes.

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

When launched inside this repo, default canonical discovery uses `catalog/portable/`. Outside this repo, the fallback canonical root remains the current directory. Use root overrides for tests and manual experiments when you do not want to touch real skill directories.

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

- `catalog/claude-native/go-review-team/` - Claude `/go-review` skill plus Go reviewer agents.
- `catalog/claude-native/feature-review-team/` - Claude `/feature-review` skill plus acceptance reviewer agents.

## Installation

Run:

```bash
~/dev/skills/install.sh
```

The root `install.sh` delegates to `scripts/install-skills.sh`. The installer:

- Symlinks portable catalog skills into `~/.agents/skills`.
- Symlinks portable catalog skills into `~/.claude/skills`.
- Symlinks Claude-native team directories into Claude.

## Directory Structure

```text
skills/
├── README.md
├── AGENTS.md
├── CLAUDE.md                     # symlink to AGENTS.md
├── Cargo.toml                    # workspace manifest
├── Cargo.lock
├── Makefile
├── install.sh                    # compatibility wrapper
├── catalog/
│   ├── portable/
│   │   ├── commit/
│   │   ├── chrome-reading-list/
│   │   └── ...
│   └── claude-native/
│       ├── go-review-team/
│       └── feature-review-team/
├── tools/
│   └── skill-importer/
│       ├── Cargo.toml
│       ├── src/
│       └── tests/
├── scripts/
│   └── install-skills.sh
└── plans/
```
