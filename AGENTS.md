# Skills Repo

This repository is the central source for personal AI skills published as a
curated common catalog and three broader install-ready filesystem catalogs.

## Current Layout

- `skills/<name>/` contains the eight-skill cross-agent common catalog exposed
  by the repository-root URL.
- `catalogs/first-party/claude-code/skills/<name>/` contains complete
  first-party Claude Code editions.
- `catalogs/first-party/codex/skills/<name>/` contains complete first-party
  Codex editions, including `agents/openai.yaml` where applicable.
- `catalogs/third-party/skills/<name>/` contains complete portable third-party
  skills shared by both agents. `catalogs/third-party/ATTRIBUTION.md` is the
  central provenance index.
- `catalogs/README.md` is the concise user-facing installation and maintenance
  guide.
- `AGENTS.md` is the source of truth for agent context; `CLAUDE.md` is a
  symlink to `AGENTS.md`.

The root catalog and catalog directories are complete at rest and install
without repository-owned assembly, staging, generation, or wrapper commands.
For promoted first-party skills, the root edition owns cross-agent behavior.
For promoted third-party skills, the full third-party catalog remains canonical
and the root package is an unchanged distribution mirror.

## Catalog Inventories

The root common catalog contains these eight names:

- `autofix`
- `autoreview`
- `batch-grill-me`
- `docs`
- `review-loop`
- `ship`
- `tdd`
- `tdd-with-review`

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
- Codex-specific first-party metadata belongs under `agents/openai.yaml` in
  the Codex catalog edition and, when supplied, in the portable root edition.
  Do not add it to the Claude Code compatibility edition.
- Canonical third-party skills remain portable and are stored once under
  `catalogs/third-party/skills/`. Do not prune their files or fork them by
  runtime; selected root packages are exact distribution mirrors.
- Root copies of `autoreview`, `review-loop`, and `batch-grill-me` must match
  their canonical `catalogs/third-party/skills/<name>/` directories exactly,
  including paths, bytes, metadata, and executable modes. Never make a
  root-only edit to one of these packages.
- Preserve executable modes on scripts, tests, build helpers, and other
  runnable files.
- Existing third-party tests, evaluation utilities, build scripts,
  walkthroughs, references, `.skillignore`, and vendored dependencies stay
  with their skills.
- Third-party instructions must not depend on a particular `.agents`,
  `.claude`, or `.codex` installation root.
- Every canonical third-party skill carries installable provenance in its
  directory and in `catalogs/third-party/ATTRIBUTION.md`. Promoted root mirrors
  retain their per-skill attribution and are also listed in root
  `ATTRIBUTION.md`.

## Installation and Refresh

Install the curated common catalog interactively:

```bash
npx skills add https://github.com/brian-bell/agent-skills
```

Install all eight common skills globally for Codex and Claude Code:

```bash
npx skills add https://github.com/brian-bell/agent-skills \
  -g -a codex -a claude-code --skill '*' -y
```

Treat the root common catalog and the three commands below as alternative
installation profiles. Installing overlapping profiles into the same global
inventory creates ambiguous sources and refresh ownership.

For the full profile, install the first-party Codex catalog:

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

Refresh installations by rerunning the explicit commands for the selected
profile. Do not document a blanket `npx skills update` workflow: global update
metadata is keyed by skill name rather than skill name and runtime, so
same-named editions cannot retain independent update sources.

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

To promote an already portable third-party skill to the common catalog, copy
its complete canonical directory unchanged into `skills/<name>/`, add its
existing provenance to root `ATTRIBUTION.md`, and verify exact content and mode
parity. Upstream refreshes happen in the full third-party catalog first and are
then recopied unchanged.

Do not add an importer, manifest, generator, runtime overlay, or installation
wrapper for this workflow.

## Common Catalog Ownership

For `autofix`, `docs`, `ship`, `tdd`, and `tdd-with-review`, the root version is
the canonical cross-agent implementation and the existing runtime catalogs are
compatibility copies. Start new cross-agent functionality in the root package,
then reflect portable behavior fixes in every applicable compatibility copy.
Runtime-specific metadata may differ, but do not create a legacy-only behavior
fork.

Use the complete Codex edition as the baseline when promoting another
first-party skill. Keep GitHub access wording integration-neutral, use `gh`
when higher-level integration coverage is insufficient, avoid runtime-specific
orchestration tool names, use portable skill-composition prose, and keep Codex
UI metadata under `agents/openai.yaml`.

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
npx skills add . --list
npx skills add ./catalogs/first-party/codex --list
npx skills add ./catalogs/first-party/claude-code --list
npx skills add ./catalogs/third-party --list
```

Root discovery must return exactly the documented eight common skills; the
explicit sub-catalogs must retain their 10/10/11 inventories. For promoted
third-party packages, run `diff -qr` plus a mode-aware manifest comparison
against the canonical directories. Also verify root frontmatter names, local
asset resolution, attribution, metadata placement, executable modes, and the
absence of symlinks beneath `skills/`.

Keep the contributor-documentation link intact:

```bash
test -L CLAUDE.md && test "$(readlink CLAUDE.md)" = AGENTS.md
```

Run tests and evaluation utilities inside a third-party skill when changing
that skill. Do not add replacement catalog-generation, layout, parity, or
installation test infrastructure.

## Conventions

- Keep portable skill frontmatter minimal: `name` and `description`. Optional
  Claude-only fields such as `argument-hint` and `disallowed-tools` are
  acceptable when the skill degrades gracefully on runtimes that ignore them.
- Keep repository-only maintenance skills under `.agents/`; do not add them to
  any published catalog.
- Put Codex UI metadata for first-party skills under `agents/openai.yaml` in
  the Codex catalog edition and portable root edition. Portable third-party
  skills may keep optional `agents/openai.yaml` metadata; other agents can
  ignore it.
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
- Treat the two full first-party catalogs as materialized compatibility
  sources for promoted skills and independent sources for unpromoted skills.
  Synchronize applicable portable behavior changes while preserving
  runtime-specific instructions and metadata.
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
- In root portable skills, refer to an available GitHub integration and use
  `gh` when its coverage is insufficient. Runtime compatibility copies may use
  runtime-specific integration wording.
- When adding, removing, or renaming a portable skill, update the documented
  inventories and recheck cross-catalog name uniqueness.
- Keep agent context in `AGENTS.md`; keep `CLAUDE.md` as a symlink to it.
- Keep published skills under root `skills/` or one of the explicit
  `catalogs/*/skills/` directories; do not introduce another source layout.
