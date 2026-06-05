# skills

Central repo for personal AI skills.

Portable skills live as top-level directories and are symlinked into both:

- `~/.agents/skills/<skill>`
- `~/.claude/skills/<skill>`

Claude-native team skills live under `claude-native/`.

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
├── claude-native/
│   ├── go-review-team/
│   └── feature-review-team/
└── install.sh
```
