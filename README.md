# agent-skills

A source-of-truth repository for install-ready AI skill catalogs used by Codex
and Claude Code.

The repository publishes three plain filesystem catalogs:

- `catalogs/first-party/codex/` contains ten Codex-specific first-party skills.
- `catalogs/first-party/claude-code/` contains the same ten first-party skills
  with Claude Code-specific instructions.
- `catalogs/third-party/` contains eleven portable third-party skills shared by
  both agents.

Every skill is complete beneath `skills/<name>/` in its catalog and can be
installed directly with [`npx skills`](https://github.com/vercel-labs/skills).
There is no repository-owned skill installer, assembly step, or generated
catalog.

`AGENTS.md` is the source of truth for contributor and agent guidance;
`CLAUDE.md` is a symlink to it for Claude compatibility.

## Installation

Install the first-party Codex catalog:

```bash
npx skills add https://github.com/brian-bell/agent-skills/tree/main/catalogs/first-party/codex \
  -g -a codex --copy --skill '*' -y
```

Install the first-party Claude Code catalog:

```bash
npx skills add https://github.com/brian-bell/agent-skills/tree/main/catalogs/first-party/claude-code \
  -g -a claude-code --copy --skill '*' -y
```

Install the portable third-party catalog for both agents:

```bash
npx skills add https://github.com/brian-bell/agent-skills/tree/main/catalogs/third-party \
  -g -a codex -a claude-code --skill '*' -y
```

First-party installs use `--copy` because their Codex and Claude Code editions
have the same names but different runtime instructions. The third-party
catalog has one portable implementation of each skill, so one command installs
it for both agents without `--copy`.

Refresh installations by rerunning these three commands. Do not rely on a
blanket `npx skills update`: its global update metadata is keyed by skill name,
not by skill name and runtime, so it cannot retain independent sources for the
same-named first-party editions.

See [the catalog guide](catalogs/README.md) for the complete installation and
maintenance interface.

## First-Party Skills

Both runtime catalogs contain:

- `autofix` - Fix one PR comment thread, or triage a PR and auto-fix P0, P1,
  and P2 unresolved feedback.
- `chrome-reading-list` - Export Chrome Reading List data to CSV or JSON.
- `docs` - Refresh project documentation from source truth.
- `feature-review` - Review feature acceptance across product, safety,
  quality, maintainability, and documentation.
- `go-review` - Review Go code across structure, error handling, style, and
  security.
- `product-manager` - Build an orchestrator-assisted product and market brief.
- `ship` - Commit, push, and open or reuse a pull request.
- `slice-issues` - Break work into independently deliverable vertical slices.
- `tdd` - Develop through red, green, and refactor loops.
- `tdd-with-review` - Combine TDD with review and commit checkpoints.

## Third-Party Skills

The portable catalog contains:

- `autoreview`
- `batch-grill-me`
- `grill-me`
- `improve-codebase-architecture`
- `last30days`
- `prd-to-issues`
- `prd-to-plan`
- `review-loop`
- `teach`
- `wizard`
- `write-a-prd`

See [the attribution index](catalogs/third-party/ATTRIBUTION.md) for upstream
credit. First- and third-party names must remain unique within each agent's
installed inventory; with all three catalogs installed, each agent receives 21
unique skills.

Adopting a third-party skill is a manual curation operation: review its license
and provenance, copy its complete directory into
`catalogs/third-party/skills/<name>/`, preserve executable modes and supporting
files, add per-skill attribution, and update the central attribution index.
Third-party skills are not pruned, assembled, or forked by runtime.

## Directory Structure

```text
agent-skills/
├── AGENTS.md
├── CLAUDE.md -> AGENTS.md
├── README.md
└── catalogs/
│   ├── README.md
│   ├── first-party/
│   │   ├── claude-code/skills/  # complete Claude Code editions
│   │   └── codex/skills/        # complete Codex editions
│   └── third-party/
│       ├── ATTRIBUTION.md
│       └── skills/              # complete portable skills
```

## Development

Catalog changes are ordinary filesystem changes. Keep each skill complete
beneath its own directory, preserve executable modes, and do not introduce
runtime overlays, routers, generators, manifests, or repository-owned install
wrappers.

Existing tests and evaluation utilities inside third-party skill directories
are part of those skills and stay with them. Run a skill's own checks when
changing that skill.

Always retain the contributor-documentation link invariant:

```bash
test -L CLAUDE.md && test "$(readlink CLAUDE.md)" = AGENTS.md
```
