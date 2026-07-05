# agent-skills

Central repo for personal AI skills.

The repo root is a small launchpad. `AGENTS.md` is the source of truth for agent context, and `CLAUDE.md` is a symlink to it for Claude compatibility. The material is split by purpose:

- `skills/` contains first-party portable skills that are staged under `~/.skill-symlinks/` and symlinked into Codex/agents, Claude Code, and Cursor.
- `third-party/` contains portable skills sourced from elsewhere, installed the same way.
- `agent-teams/` contains team skills and reviewer agents; most are
  Claude-only, while packages with `agents/openai.yaml` are also installed for
  Codex/agents.
- `hooks/` contains standalone agent hooks, each with its own installer.
- `scripts/` contains repository maintenance scripts.

## My Skills

Some of my skills are compositions that may include other third-party skills. 

- `autobuild` - Drive a task through implementation, review-loop, autoreview, and PR phases, one agent per phase.
- `autofix` - Fix one PR comment thread, or triage a PR and auto-fix P0/P1 unresolved feedback with autoreview, ship, replies, and thread resolution.
- `chrome-reading-list` - Export Chrome Reading List data to CSV/JSON.
- `commit` - Create clean local-only git commits without pushing.
- `docs` - Update `AGENTS.md`, keep `CLAUDE.md` symlinked to it, and refresh `README.md` from source truth.
- `fix-pr` - Gather unresolved PR review comments, classify each as accepted, rejected, or already fixed; fix-pr asks whether to use autofix and ships reviewed fixes to the PR.
- `merge-prs-review-loop` - Review and merge PR batches with conflict-aware review-loop gates.
- `planned-implementation-agent` - Plan, review, and delegate implementation work with TDD and review-loop gates.
- `product-manager` - Product and market analysis workflow.
- `ship` - Commit, push, and open/reuse a PR.
- `skill-parity-audit` - Compare skill roots for missing, drifted, and broken skills.
- `slice-issues` - Break a GitHub issue into independently-grabbable vertical-slice sub-issues.
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
- `wizard` - Generate an interactive bash wizard that walks a human through a manual procedure.
- `write-a-prd` - Interview, design, and draft a PRD as a GitHub issue.

## Agent Team Skills (created by me)

- `agent-teams/go-review-team/` - Hybrid Claude `/go-review` and Codex
  `$go-review` skill plus Go reviewer agents/checklists.
- `agent-teams/feature-review-team/` - Claude `/feature-review` skill plus acceptance reviewer agents.

## Hooks

Hooks are installed separately from the skill TUI:

- `hooks/save-codex-session/` archives Codex `Stop` hook transcripts and metadata to `~/.agent-sessions/codex/`.
- `hooks/save-claude-session/` archives Claude Code `SessionEnd` transcripts and metadata to `~/.agent-sessions/claude/`.

## Installation

Run the interactive installer:

```bash
cd ~/dev/agent-skills
./install.sh
```

`install.sh` builds (requires the Go toolchain) and launches a small terminal
UI (`tools/skills-tui/`) that lists
every skill discovered on disk with its current state and lets you install or
uninstall with the spacebar:

- `↑/↓` (or `j/k`) move, `space` toggles, `a` selects all, `n` selects none.
- `enter` applies the pending changes, `q` quits.
- Rows are labelled `installed`, `not installed`, `~ partial` (linked in one
  root only), `will be updated` (selected upgrade), or `⬆ upgrade available`
  (held upgrade). Installed skills toggled off are labelled `will be removed`.
  Upgradeable skills default to `[x]` and can be toggled to `[-]` to leave the
  current staged copy unchanged. Applying refreshes staged copies under
  `~/.skill-symlinks/` and backs up the previous staged copy under
  `~/.skill-symlinks/backups/` before an upgrade. It also relinks foreign
  symlinks in place (non-destructive); replacing a real directory requires
  `--force`.

The installer discovers skills directly from the filesystem, so new skills are
picked up automatically. It:

- Copies first-party and third-party portable skills into `~/.skill-symlinks/skills/`.
- Symlinks those staged portable skills into `~/.agents/skills`, `~/.claude/skills`, and `~/.cursor/skills`.
- Copies team directories into `~/.skill-symlinks/agent-teams/` and symlinks
  those staged copies into Claude. Team packages with `agents/openai.yaml` are
  also linked into `~/.agents/skills`.
- Migrates older repo-pointing symlinks to staged symlinks when applied.
- Backs up previous staged copies under `~/.skill-symlinks/backups/` before refreshing them.
- Uninstalls only installer-owned staged symlinks; real directories and foreign
  symlinks are left untouched.

Set `SKILL_INSTALL_TARGETS` to limit which runtime roots are managed (default:
`agents,claude,cursor`). Example: `SKILL_INSTALL_TARGETS=cursor ./install.sh --all`
installs only into `~/.cursor/skills`. Agent-teams require `claude` in the list.

For non-interactive use: `install.sh --all`, `install.sh --none`, or
`install.sh --force` (force-install everything, overwriting foreign symlinks
**and** real directories at the targets — the only destructive path). The older
`scripts/install-skills.sh` still works but is deprecated.

## Directory Structure

```text
agent-skills/
├── AGENTS.md
├── CLAUDE.md                     # symlink to AGENTS.md
├── README.md
├── install.sh                    # builds + launches the Go install/uninstall TUI
├── tools/
│   └── skills-tui/               # Go module for the install/uninstall TUI
├── skills/                       # first-party portable skills
│   ├── commit/
│   ├── chrome-reading-list/
│   └── ...
├── third-party/                  # third-party portable skills
│   ├── autoreview/
│   ├── grill-me/
│   └── ...
├── agent-teams/                  # Claude-native and hybrid team skills + agents
│   ├── go-review-team/
│   └── feature-review-team/
├── hooks/                        # standalone Codex/Claude hook installers
│   ├── save-codex-session/
│   └── save-claude-session/
└── scripts/
    ├── skills-tui.sh             # bash TUI (reference spec; no longer invoked)
    └── install-skills.sh         # deprecated non-interactive installer
```

## Development Checks

There is no Makefile; the only Go module is `tools/skills-tui/`. Run the
focused checks directly:

```bash
scripts/test-skills-tui-go.sh
scripts/test-skills-tui.sh
scripts/test-install-skills.sh
scripts/test-save-codex-session.sh
scripts/test-fix-pr.sh
python3 skills/autobuild/scripts/autobuild_test.py -v
```
