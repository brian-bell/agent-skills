# agent-skills

An install-ready collection of AI skills for Codex and Claude Code.

The repository root is a curated common catalog. To browse its eight skills
and choose what to install interactively:

```bash
npx skills add https://github.com/brian-bell/agent-skills
```

To install the complete common catalog globally for both agents:

```bash
npx skills add https://github.com/brian-bell/agent-skills \
  -g -a codex -a claude-code --skill '*' -y
```

Installation is project-scoped unless `--global` or `-g` is supplied.

## Common catalog

The root `skills/` directory contains:

- `autofix` - Fix one PR comment thread, or triage a PR and auto-fix P0, P1,
  and P2 unresolved feedback.
- `autoreview` - Run a structured closeout code review.
- `batch-grill-me` - Interview every open design frontier in parallel rounds.
- `docs` - Refresh project documentation from source truth.
- `review-loop` - Iterate through worker and reviewer quality loops.
- `ship` - Commit, push, and open or reuse a pull request.
- `tdd` - Develop through red, green, and refactor loops.
- `tdd-with-review` - Combine TDD, documentation, review, and local commit
  checkpoints.

The five first-party packages use their Codex editions as the original
baseline and are portable across Codex and Claude Code. The three third-party
packages are unchanged mirrors of their canonical curated copies under
`catalogs/third-party/`. See the [root attribution index](ATTRIBUTION.md).

## Choose an installation profile

Use either the common profile above or the full profile below. Do not install
both profiles into the same global inventory: overlapping names can replace
one another's canonical installed copy, make update ownership ambiguous, and
produce unexpected refresh behavior.

### Full profile

The broader catalogs remain independently installable. Install the ten
first-party Codex editions:

```bash
npx skills add https://github.com/brian-bell/agent-skills/tree/main/catalogs/first-party/codex \
  -g -a codex --copy --skill '*' -y
```

Install the ten first-party Claude Code editions:

```bash
npx skills add https://github.com/brian-bell/agent-skills/tree/main/catalogs/first-party/claude-code \
  -g -a claude-code --copy --skill '*' -y
```

Install the eleven portable third-party skills for both agents:

```bash
npx skills add https://github.com/brian-bell/agent-skills/tree/main/catalogs/third-party \
  -g -a codex -a claude-code --skill '*' -y
```

First-party full-profile installs use `--copy` so same-named runtime editions
remain independent. The third-party catalog uses one portable implementation
for both agents.

Refresh an installation by rerunning the explicit command for its chosen
profile. Do not rely on a blanket `npx skills update`: global update metadata
is keyed by skill name rather than skill name and runtime.

## Catalogs

| Location | Inventory | Responsibility |
|---|---:|---|
| `skills/` | 8 | Curated common catalog exposed by the repository URL |
| `catalogs/first-party/codex/` | 10 | Complete Codex first-party catalog |
| `catalogs/first-party/claude-code/` | 10 | Complete Claude Code first-party catalog |
| `catalogs/third-party/` | 11 | Complete portable third-party catalog |

Every package is complete beneath its own `skills/<name>/` directory. There
is no repository-owned installer, manifest, runtime router, generator, or
assembly step.

## Maintenance

Promote a first-party skill by copying its complete Codex edition into the
root catalog, making only the changes needed for cross-agent behavior, and
testing it in both agents. The root package then owns cross-agent behavior;
applicable fixes must also reach the retained compatibility copies.

Promote a third-party skill only when its curated package is already portable.
Copy its complete directory unchanged from `catalogs/third-party/skills/`,
preserve all files and executable modes, verify exact parity, and add its
existing provenance to the root attribution index. Future upstream refreshes
start in the canonical third-party catalog and are mirrored to the root.

See [the catalog guide](catalogs/README.md) for full inventories and maintenance
details. `AGENTS.md` is the source of truth for contributor guidance;
`CLAUDE.md` is a symlink to it for Claude compatibility.
