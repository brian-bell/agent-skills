# skills

Central repo for personal AI skills. `AGENTS.md` is the source of truth for
agent context, and `CLAUDE.md` is a symlink to it for Claude compatibility.

The `skill-importer` Rust crate has been split into its own standalone local
repository at `/Users/brian/dev/skill-importer`.

## Layout

- `catalog/portable/` contains portable skills that can be symlinked into both
  Codex/agents and Claude Code.
- `catalog/claude-native/` contains Claude-only team skills and reviewer agents.
- `scripts/` contains repository maintenance scripts.
- `docs/` contains consumer-facing documentation.
- `.github/workflows/ci.yml` smoke-tests skill installation.
- `.github/workflows/codex.yml` is this repo's collaborator-gated Codex
  workflow entrypoint.
- `.github/workflows/claude.yml` is the Claude Code `@claude` workflow
  entrypoint.
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

## Development

Useful commands from the repo root:

```bash
make install
make check
make test
make clean
```

`make check` smoke-tests `install.sh` with a temporary `HOME` so real
`~/.claude/skills` and `~/.agents/skills` entries are not touched.

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
├── Makefile
├── install.sh                    # compatibility wrapper
├── .github/
│   └── workflows/                # CI, Codex, Claude, and reusable ship workflows
├── docs/
│   └── autoreview-ship-workflow.md
├── catalog/
│   ├── portable/
│   │   ├── commit/
│   │   ├── chrome-reading-list/
│   │   └── ...
│   └── claude-native/
│       ├── go-review-team/
│       └── feature-review-team/
└── scripts/
    └── install-skills.sh
```
