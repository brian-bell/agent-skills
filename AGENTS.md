# Skills Repo

This repository is the central source for personal AI skills published as
three install-ready filesystem catalogs.

## Current Layout

- `catalogs/first-party/claude-code/skills/<name>/` contains complete
  first-party Claude Code editions.
- `catalogs/first-party/codex/skills/<name>/` contains complete first-party
  Codex editions, including `agents/openai.yaml` where applicable.
- `catalogs/third-party/skills/<name>/` contains complete portable third-party
  skills shared by both agents. `catalogs/third-party/ATTRIBUTION.md` is the
  central provenance index.
- `catalogs/README.md` is the concise user-facing installation and maintenance
  guide.
- Project-scoped agent support lives under `.agents/` and is not part of the
  published catalogs.
- Session hooks live under `hooks/<hook>/` and retain standalone installers.
- `docs/` contains focused design and contributor documents.
- `scripts/` contains repository-facing maintenance helpers that are not part
  of skill installation.
- `AGENTS.md` is the source of truth for agent context; `CLAUDE.md` is a
  symlink to `AGENTS.md`.

The catalog directories are the source of truth. Skills are complete at rest
and install without repository-owned assembly, staging, generation, or wrapper
commands.

## Catalog Inventories

Both first-party runtime catalogs contain these ten names:

- `autofix`
- `chrome-reading-list`
- `docs`
- `feature-review`
- `go-review`
- `product-manager`
- `ship`
- `slice-issues`
- `tdd`
- `tdd-with-review`

The portable third-party catalog contains these eleven names:

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

First-party and third-party names must be unique within an agent's installed
inventory. Claude Code and Codex first-party editions may share names because
they are installed independently. Update these inventories whenever a catalog
skill is added, removed, or renamed.

## Catalog Invariants

- Every catalog skill has `SKILL.md` directly at
  `skills/<name>/SKILL.md`, and its frontmatter `name` matches the directory.
- Every skill is complete beneath its own directory. All scripts, roles,
  templates, references, tests, and other named assets resolve there.
- Catalogs contain regular files and directories only; do not use repository
  symlinks to assemble a skill.
- First-party skills are materialized runtime editions. Do not introduce
  `shared/`, `runtimes/`, runtime routers, or an assembly step.
- Codex-specific first-party metadata belongs only under
  `catalogs/first-party/codex/skills/<name>/agents/openai.yaml`.
- Third-party skills remain portable and are stored once. Do not prune their
  files or fork them by runtime.
- Preserve executable modes on scripts, tests, build helpers, and other
  runnable files.
- Existing third-party tests, evaluation utilities, build scripts,
  walkthroughs, references, `.skillignore`, and vendored dependencies stay
  with their skills.
- Third-party instructions must not depend on a particular `.agents`,
  `.claude`, or `.codex` installation root.
- Every third-party skill carries installable provenance in its directory and
  in `catalogs/third-party/ATTRIBUTION.md`.

## Installation and Refresh

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

First-party installs use `--copy` so same-named runtime editions remain
independent. The portable catalog uses the CLI's shared installation behavior
because one implementation serves both agents.

Refresh installations by rerunning all three explicit commands. Do not
document a blanket `npx skills update` workflow: global update metadata is
keyed by skill name rather than skill name and runtime, so same-named Codex and
Claude Code editions cannot retain independent update sources.

Repository changes must not delete or overwrite a user's existing global
installations. Workstation cutover is a separate operation after review and
merge.

## Third-Party Curation

Third-party adoption is a manual filesystem workflow:

1. Review the upstream skill, its license, and its provenance.
2. Copy the complete directory to
   `catalogs/third-party/skills/<name>/`, preserving every file and executable
   mode.
3. Confirm `SKILL.md` is directly inside that directory, its frontmatter name
   matches, and the name does not collide with either first-party catalog.
4. Keep the instructions portable and ensure referenced assets resolve within
   the skill directory.
5. Add or update the skill's `ATTRIBUTION.md` and the central
   `catalogs/third-party/ATTRIBUTION.md` index.

Do not add an importer, manifest, generator, runtime overlay, or installation
wrapper for this workflow.

## Hooks

Hooks remain outside the skill catalogs and are installed only through the
standalone `install.sh` in each hook directory.

- `hooks/save-codex-session/` is a Codex `Stop` hook that archives each local
  transcript plus metadata to `~/.agent-sessions/codex/`. Its backfill helper
  imports existing transcripts, and `validate-archives.sh` audits session-id
  consistency.
- `hooks/save-claude-session/` is a Claude Code `SessionEnd` hook that archives
  each transcript plus metadata to `~/.agent-sessions/claude/`. Its backfill
  helper imports existing transcripts.

Each hook installer owns its settings-file writes, is idempotent, supports
`--uninstall`, and uses `--force` only to replace a real file at its script
target. When adding or changing a hook, preserve its standalone installation
and uninstall behavior and update its own README.

## Issue Tracking

This repo uses [beads](https://github.com/steveyegge/beads) (`bd`, issue
prefix `as`) as the issue tracker of record — not GitHub Issues. The
`.beads/` directory holds config, git hooks, and a JSONL export
(`issues.jsonl`, auto-exported for reviewable diffs); the Dolt database itself
is git-ignored and syncs via the configured Dolt remote.

- Browse work with `bd list`, `bd show <id>`, `bd graph <id>`; file new issues
  with `bd create`; close with `bd close`.
- Use dependencies (`bd dep add`), epics (`parent-child` deps), and `related`
  links instead of prose cross-references.
- `.beads/formulas/tdd-autoreview-commit.formula.toml` is the reusable workflow
  for planning and implementing one bead with vertical-slice TDD, autoreview,
  and a final local commit.
- All pre-migration GitHub issues (#5–#78) were imported with
  `external_ref: gh-<n>` preserved; look up an old GitHub number with
  `bd list --status all --json | grep gh-<n>`. GitHub Issues remain enabled
  for external visitors but are frozen — migrate anything new into beads.

## Verification

Catalog migration and maintenance use one-off inspections rather than a
repository-owned validation harness. At minimum, confirm inventory counts,
name uniqueness, the absence of catalog symlinks, local asset resolution,
executable modes, metadata placement, provenance, and clean whitespace.

Inspect CLI discovery without installing:

```bash
npx skills add ./catalogs/first-party/codex --list
npx skills add ./catalogs/first-party/claude-code --list
npx skills add ./catalogs/third-party --list
```

Keep the contributor-documentation link intact:

```bash
test -L CLAUDE.md && test "$(readlink CLAUDE.md)" = AGENTS.md
```

Run tests and evaluation utilities inside a third-party skill when changing
that skill. Run `scripts/test-save-codex-session.sh` when changing the
standalone Codex session hook. Do not add replacement catalog-generation,
layout, parity, or installation test infrastructure.

## Conventions

- Keep portable skill frontmatter minimal: `name` and `description`. Optional
  Claude-only fields such as `argument-hint` and `disallowed-tools` are
  acceptable when the skill degrades gracefully on runtimes that ignore them.
- Keep repository-only maintenance skills under `.agents/`; do not add them to
  any published catalog.
- Put Codex UI metadata for first-party skills under the Codex catalog edition.
  Portable third-party skills may keep optional `agents/openai.yaml` metadata;
  other agents can ignore it.
- Multi-reviewer skills use the orchestrator-role shape rather than registered
  agent definitions. Reviewer briefs are runtime-neutral prompt sources under
  `roles/`, and the orchestrator runs inline in the main session rather than
  being delegated to a lead subagent. This lets the orchestrator check in with
  the user and read each role's full report directly. `go-review`,
  `feature-review`, and `product-manager` follow this shape.
- Role briefs carry no frontmatter and must restate their own read-only
  `<HARD-GATE>`. The gate must explicitly prohibit shell and git mutation.
- Nothing installs into `~/.claude/agents/`. If registered agent definitions
  are ever reconsidered, weigh them against the inline orchestrator's ability
  to check in with the user and read full role reports directly.
- Treat the two first-party catalogs as independent materialized sources. When
  a behavior change applies to both runtimes, update both editions while
  preserving their runtime-specific instructions.
- In portable skill prose, write skill composition as "run the *skill-name*
  skill" rather than using Codex-only `$skill` chaining. Keep `$skill` syntax
  only in Codex `agents/openai.yaml` prompts or literal invocation examples.
- Use `<skill-dir>` in portable instructions for bundled scripts and assets
  rather than hardcoding an agent installation root.
- For delegation, Claude Code may use its `Agent` or subagent path. A skill the
  user explicitly invokes may direct Codex subagent fan-out with explicit
  spawn instructions, respecting the runtime's thread and depth limits, with a
  sequential inline fallback when spawning is unavailable, blocked, or
  declined. Worker prompts must be self-contained because workers start
  without parent conversation context. Outside skill-directed fan-out, Codex
  uses subagents only when the user explicitly asks for delegation or parallel
  work, and must not claim delegation that did not happen.
- For GitHub-touching skills, Codex should prefer an installed GitHub connector
  and use `gh` when connector coverage is insufficient. Claude Code should use
  `gh` or another integration provided by the user.
- When adding, removing, or renaming a portable skill, update the documented
  inventories and recheck cross-catalog name uniqueness.
- Keep agent context in `AGENTS.md`; keep `CLAUDE.md` as a symlink to it.
- Keep `catalogs/` as the only source of truth for published skills.
