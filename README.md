# skills

Central repo for personal AI skills.

The repo root is a small launchpad. `AGENTS.md` is the source of truth for agent context, and `CLAUDE.md` is a symlink to it for Claude compatibility. The material is split by purpose:

- `skills/` contains first-party portable skills that are symlinked into both Codex/agents and Claude Code.
- `third-party/` contains portable skills sourced from elsewhere, installed the same way.
- `agent-teams/` contains Claude-only team skills and reviewer agents.
- `scripts/` contains repository maintenance scripts.

## First-Party Skills

- `autobuild` - Drive a task through implementation, review-loop, autoreview, and PR phases, one agent per phase.
- `chrome-reading-list` - Export Chrome Reading List data to CSV/JSON.
- `commit` - Create clean local-only git commits without pushing.
- `docs` - Update `AGENTS.md`, keep `CLAUDE.md` symlinked to it, and refresh `README.md` from source truth.
- `merge-prs-review-loop` - Review and merge PR batches with conflict-aware review-loop gates.
- `planned-implementation-agent` - Plan, review, and delegate implementation work with TDD and review-loop gates.
- `product-manager` - Product and market analysis workflow.
- `ship` - Commit, push, and open/reuse a PR.
- `skill-parity-audit` - Compare skill roots for missing, drifted, and broken skills.
- `tdd` - Test-driven development with red/green/refactor loops.
- `tdd-with-review` - Implement with TDD, review-loop, autoreview, and commit checkpoints.
- `work-prs` - Process open non-draft PRs with complete checks, fix failures/blockers, and push targeted fixes.

## Third-Party Skills

Sourced from other projects; see [`third-party/ATTRIBUTION.md`](third-party/ATTRIBUTION.md) for upstream credit.

- `autoreview` - Run structured code review as a closeout check on local or PR branches.
- `grill-me` - Stress-test a plan or design through one-question-at-a-time interview.
- `improve-codebase-architecture` - Find module-deepening opportunities.
- `prd-to-issues` - Break a PRD into vertical-slice GitHub issues.
- `prd-to-plan` - Turn a PRD into a phased tracer-bullet implementation plan.
- `review-loop` - Iterative worker/reviewer quality loop.
- `write-a-prd` - Interview, design, and draft a PRD as a GitHub issue.

## Claude-Native Skills

- `agent-teams/go-review-team/` - Claude `/go-review` skill plus Go reviewer agents.
- `agent-teams/feature-review-team/` - Claude `/feature-review` skill plus acceptance reviewer agents.

## Installation

Run the interactive installer:

```bash
~/dev/skills/install.sh
```

`install.sh` launches a small terminal UI (`scripts/skills-tui.sh`) that lists
every skill discovered on disk with its current state and lets you install or
uninstall with the spacebar:

- `↑/↓` (or `j/k`) move, `space` toggles, `a` selects all, `n` selects none.
- `enter` applies the pending changes, `q` quits.
- Rows are labelled `installed`, `not installed`, `~ partial` (linked in one
  root only), or `⬆ upgrade available` (the target differs from the repo — a
  copy or a symlink pointing elsewhere). Applying relinks foreign symlinks in
  place (non-destructive); replacing a real directory requires `--force`.

The installer discovers skills directly from the filesystem, so new skills are
picked up automatically. It:

- Symlinks first-party and third-party portable skills into `~/.agents/skills`.
- Symlinks first-party and third-party portable skills into `~/.claude/skills`.
- Symlinks Claude-native team directories and their reviewer agents into Claude.
- Uninstalls only repo-owned symlinks; real directories and foreign symlinks are
  left untouched.

For non-interactive use: `install.sh --all`, `install.sh --none`, or
`install.sh --force` (force-install everything, overwriting foreign symlinks
**and** real directories at the targets — the only destructive path). The older
`scripts/install-skills.sh` still works but is deprecated.

## Directory Structure

```text
skills/
├── README.md
├── AGENTS.md
├── CLAUDE.md                     # symlink to AGENTS.md
├── install.sh                    # launches the install/uninstall TUI
├── skills/                       # first-party portable skills
│   ├── commit/
│   ├── chrome-reading-list/
│   └── ...
├── third-party/                  # third-party portable skills
│   ├── autoreview/
│   ├── grill-me/
│   └── ...
├── agent-teams/                  # Claude-native team skills + agents
│   ├── go-review-team/
│   └── feature-review-team/
└── scripts/
    ├── skills-tui.sh             # install/uninstall TUI
    └── install-skills.sh         # deprecated non-interactive installer
```
