# agent-skills

A source-of-truth repository for personal AI skills shared between Codex and
Claude Code.

Codex skills are officially distributed through the generated catalog and the
Vercel skills CLI:

```bash
npx skills add https://github.com/brian-bell/agent-skills/tree/main/catalog --agent codex
```

The existing Go terminal installer remains the path for Claude Code, session
hooks, and importing third-party skills into this repository.

The repository is organized by responsibility:

- `.agents/skills/` contains project-scoped maintenance skills that are
  available only in this repository and are not installed by the TUI.
- `skills/` contains first-party portable skills. Shared assets live in
  `shared/`, while runtime instructions live in
  `runtimes/{claude,codex}/`.
- `third-party/` contains attributed portable skills imported from other
  projects.
- `catalog/skills/` contains the generated flat Codex packages consumed by
  `npx skills`.
- `hooks/` contains Codex and Claude Code session hooks, each with a standalone
  installer and integration with the main TUI.
- `docs/` contains focused usage and contributor guides.
- `tools/skills-tui/` contains the self-contained Go installer module.
- `scripts/` contains repository verification and maintenance commands.

`AGENTS.md` is the source of truth for contributor and agent guidance;
`CLAUDE.md` is a symlink to it for Claude compatibility.

## My Skills

Some of my skills are compositions that may include other third-party skills. 

- `autofix` - Fix one PR comment thread, or triage a PR and auto-fix P0, P1, and P2 unresolved feedback with autoreview, ship, replies, and thread resolution.
- `chrome-reading-list` - Export Chrome Reading List data to CSV/JSON.
- `docs` - Update `AGENTS.md`, keep `CLAUDE.md` symlinked to it, and refresh `README.md` from source truth.
- `feature-review` - Read-only feature acceptance review across product, safety, quality, maintainability, and documentation; the acceptance lead runs inline and dispatches five leaf reviewer roles.
- `go-review` - Read-only Go code review across structure, error handling, style, and security; the orchestrator runs inline and dispatches four leaf reviewer roles.
- `product-manager` - Orchestrator–subagent product/market brief.
- `ship` - Commit, push, and open/reuse a PR.
- `slice-issues` - Break an issue or work item into independently-grabbable vertical-slice sub-issues.
- `tdd` - Test-driven development with red/green/refactor loops.
- `tdd-with-review` - Implement with TDD, review-loop, autoreview, and commit checkpoints.

## Project-Scoped Skills

- `skill-parity-audit` - Audit and maintain semantic parity between every first-party skill's Claude and Codex runtime forks.

## Third-Party Skills

Sourced from other projects; see [`third-party/ATTRIBUTION.md`](third-party/ATTRIBUTION.md) for upstream credit.

- `autoreview` - Run structured code review as a closeout check on local or PR branches.
- `batch-grill-me` - Interview every currently unblocked design decision in parallel, round by round.
- `grill-me` - Stress-test a plan or design through one-question-at-a-time interview.
- `improve-codebase-architecture` - Find module-deepening opportunities.
- `last30days` - Research what people actually say about a topic across Reddit, X, YouTube, Hacker News, and more from the last 30 days.
- `prd-to-issues` - Break a PRD into vertical-slice GitHub issues.
- `prd-to-plan` - Turn a PRD into a phased tracer-bullet implementation plan.
- `review-loop` - Iterative worker/reviewer quality loop.
- `teach` - Multi-session teaching workspace with missions, lessons, and learning records.
- `wizard` - Generate an interactive bash wizard that walks a human through a manual procedure.
- `write-a-prd` - Interview, design, and draft a PRD as a GitHub issue.

## Hooks

The skill TUI discovers hooks alongside skills and manages them in a separate
`hooks` section. Each hook can also be installed or removed with its own
`install.sh`:

- `hooks/save-codex-session/` archives Codex `Stop` hook transcripts and metadata to `~/.agent-sessions/codex/`.
- `hooks/save-claude-session/` archives Claude Code `SessionEnd` transcripts and metadata to `~/.agent-sessions/claude/`.

## Installation

### Codex

Run the official interactive command and select the skills to install:

```bash
npx skills add https://github.com/brian-bell/agent-skills/tree/main/catalog --agent codex
```

For an unattended global copy of every catalog skill:

```bash
npx skills add https://github.com/brian-bell/agent-skills/tree/main/catalog \
  --skill '*' --agent codex --global --yes --copy
```

Repeat `--skill` to install a selected set instead. List available or installed
skills and update global CLI-managed installs with:

```bash
npx skills add https://github.com/brian-bell/agent-skills/tree/main/catalog --list
npx skills list --global --agent codex
npx skills update --global --yes
```

The CLI does not resolve skill-composition dependencies, so selected installs
must include the companion skills named by a composed workflow. Installing all
catalog packages includes the complete set.

This generated catalog supports Codex only. Claude Code support through
`npx skills` is intentionally deferred. See
[Installing Skills](docs/installing-skills.md) for the complete command set,
scope choices, maintenance workflow, and compatibility checks.

### Claude Code, hooks, and GitHub import

Run the interactive Go installer from a checkout:

```bash
cd ~/dev/agent-skills
./install.sh
```

`install.sh` builds (requires the Go toolchain) and launches a small terminal
UI (`tools/skills-tui/`) that lists
every skill discovered on disk with its current state and lets you install or
uninstall with the spacebar. It remains the current installation path for
Claude Code and the only path that manages the repository's hooks and GitHub
import workflow.

### Importing skills from GitHub

Press `i` in the interactive TUI to scan a GitHub repository and import
selected portable skills into `third-party/`. Importing and applying
installation are separate steps. See
[Importing Skills from GitHub](docs/importing-skills-from-github.md) for the
complete workflow, accepted URLs, authentication requirements, saved history,
validation rules, and rollback guarantees.

## Directory Structure

```text
agent-skills/
├── .agents/
│   └── skills/
│       └── skill-parity-audit/   # repository-only runtime-fork audit
├── AGENTS.md
├── CLAUDE.md                     # symlink to AGENTS.md
├── README.md
├── install.sh                    # builds + launches the Go install/uninstall TUI
├── docs/                         # focused usage and contributor guides
├── tools/
│   └── skills-tui/               # Go module for the install/uninstall TUI
├── skills/                       # first-party portable skills
│   ├── ship/
│   │   ├── shared/
│   │   └── runtimes/
│   ├── chrome-reading-list/
│   └── ...
├── third-party/                  # third-party portable skills
│   ├── autoreview/
│   ├── grill-me/
│   └── ...
├── hooks/                        # standalone Codex/Claude hook installers
│   ├── save-codex-session/
│   └── save-claude-session/
└── scripts/                      # repo test + maintenance scripts
```

## Development Checks

There is no Makefile; the only Go module is `tools/skills-tui/`. Run the
focused checks directly:

```bash
# Focused GitHub import workflow and module checks
(
  cd tools/skills-tui
  env -u GOROOT go test -race ./internal/importer ./internal/tui
  env -u GOROOT go test ./...
)

# Installer regressions and documentation source-of-truth check
env -u GOROOT scripts/test-skills-tui-go.sh
env -u GOROOT scripts/test-install.sh
env -u GOROOT scripts/test-forked-skills-install.sh
test -L CLAUDE.md && test "$(readlink CLAUDE.md)" = AGENTS.md

# Generated Codex catalog, pinned CLI install, and live Codex activation
python3 -m unittest scripts.test_generate_skills_catalog
python3 scripts/generate-skills-catalog.py --check
scripts/test-skills-catalog-cli.sh
scripts/test-skills-catalog-codex.sh

# Broader repository checks
scripts/test-skill-parity-audit.py
scripts/test-forked-skills-layout.sh
scripts/test-hooks-install.sh
scripts/test-save-codex-session.sh
scripts/test-autofix.sh
scripts/test-autoreview.sh
```

The `env -u GOROOT` prefix makes each Go-backed check use the selected `go`
binary's own toolchain root instead of a possibly stale shell override.
The hook installation and Codex session tests require `jq`.
