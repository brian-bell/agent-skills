# agent-skills

An install-ready collection of AI skills for Codex and Claude Code.

The repository root is the canonical common catalog. To browse its skills and
choose what to install interactively:

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
- `chrome-reading-list` - Export Chrome's Reading List to CSV or JSON.
- `docs` - Refresh project documentation from source truth.
- `improve-codebase-architecture` - Find architectural deepening and
  refactoring opportunities.
- `last30days` - Research recent discussion and engagement across social,
  developer, and web sources.
- `review-loop` - Iterate through worker and reviewer quality loops.
- `ship` - Commit, push, and open or reuse a pull request.
- `slice-issues` - Break large work items into tracer-bullet vertical slices.
- `teach` - Teach a skill or concept through a stateful workspace.
- `tdd` - Develop through red, green, and refactor loops.
- `tdd-with-review` - Combine TDD, documentation, review, and local commit
  checkpoints.
- `wizard` - Generate an interactive Bash wizard for a manual procedure.

The common first-party packages are portable across Codex and Claude Code;
`slice-issues` carries attribution for the upstream work it adapts. The
curated third-party packages retain their provenance. See the
[root attribution index](ATTRIBUTION.md).

## Expanded installation

The catalogs under `catalogs/` contain only skills not present in the common
catalog. Install them alongside the common catalog to expand each agent's
inventory.

Add the experimental Codex skills:

```bash
npx skills add https://github.com/brian-bell/agent-skills/tree/main/catalogs/experimental/codex \
  -g -a codex --copy --skill '*' -y
```

Add the experimental Claude Code skills:

```bash
npx skills add https://github.com/brian-bell/agent-skills/tree/main/catalogs/experimental/claude-code \
  -g -a claude-code --copy --skill '*' -y
```

The common and experimental catalogs have no overlapping names. Experimental
installs use `--copy` so their same-named Codex and Claude Code editions remain
independent.

Refresh installations by rerunning the explicit catalog commands. Do not rely
on a blanket `npx skills update`: global update metadata is keyed by skill name
rather than skill name and runtime, so it cannot retain independent sources
for the same-named Codex and Claude Code supplemental editions.

## Catalogs

| Location | Responsibility |
|---|---|
| `skills/` | Canonical cross-agent common catalog |
| `catalogs/experimental/codex/` | Experimental Codex skills |
| `catalogs/experimental/claude-code/` | Experimental Claude Code skills |

Every package is complete beneath its own `skills/<name>/` directory. There
is no repository-owned installer, manifest, runtime router, generator, or
assembly step.

## Maintenance

The root catalog is the sole repository location for common skills. New
cross-agent behavior for those packages starts under `skills/`; do not add
same-named compatibility copies to the supplemental catalogs.

To promote an experimental skill, use its complete Codex edition as the baseline,
make it portable in the root catalog, verify both agents, and remove the old
Codex and Claude Code editions.

See [the catalog guide](catalogs/README.md) for supplemental inventories and
maintenance details. `AGENTS.md` is the source of truth for contributor
guidance; `CLAUDE.md` is a symlink to it for Claude compatibility.
