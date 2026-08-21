# Skills Repo

This repository is the central source for personal AI skills published as a
canonical common catalog and supplemental install-ready filesystem catalogs.

## Current Layout

- `skills/<name>/` contains the cross-agent common catalog exposed
  by the repository-root URL.
- `catalogs/experimental/claude-code/skills/<name>/` contains experimental
  Claude Code editions not present in the root catalog.
- `catalogs/experimental/codex/skills/<name>/` contains experimental Codex
  editions, including `agents/openai.yaml` where applicable.
- `catalogs/README.md` is the concise user-facing installation and maintenance
  guide.
- `AGENTS.md` is the source of truth for agent context; `CLAUDE.md` is a
  symlink to `AGENTS.md`.

The root catalog and supplemental catalog directories are complete at rest and
install without repository-owned assembly, staging, generation, or wrapper
commands. Each published skill has one repository owner: common skills live
only in root `skills/`, while supplemental skills live only in the appropriate
catalog.

## Catalog Inventories

The root common catalog contains these names:

- `autofix`
- `autoreview`
- `docs`
- `grill-me`
- `improve-codebase-architecture`
- `improve-docs`
- `last30days`
- `review-loop`
- `ship`
- `slice-issues`
- `tdd`
- `tdd-with-review`
- `tui-iterate`
- `write-spec`

Both experimental runtime catalogs contain these names:

- `feature-review`
- `go-review`
- `product-manager`
- `review-gate`

Root and experimental names must not overlap within an agent's installed
inventory. Claude Code and Codex experimental editions may share names because
they are installed independently. Update these inventories whenever a catalog
skill is added, removed, promoted, or renamed.

## Catalog Invariants

- Every catalog skill has `SKILL.md` directly at
  `skills/<name>/SKILL.md`, and its frontmatter `name` matches the directory.
- Every skill is complete beneath its own directory. All scripts, roles,
  templates, references, tests, and other named assets resolve there.
- Catalogs contain regular files and directories only; do not use repository
  symlinks to assemble a skill.
- Experimental skills are materialized runtime editions. Do not
  introduce `shared/`, `runtimes/`, runtime routers, or an assembly step.
- Codex-specific metadata belongs under `agents/openai.yaml` in the Codex
  experimental edition and, when supplied, in the portable root edition.
  Do not add it to the Claude Code edition.
- Preserve executable modes on scripts, tests, build helpers, and other
  runnable files.
- Every curated third-party skill in the root catalog carries installable
  provenance in its directory and in root `ATTRIBUTION.md`.

## Installation and Refresh

Install the curated common catalog interactively:

```bash
npx skills add https://github.com/brian-bell/agent-skills
```

Install all common skills globally for Codex and Claude Code:

```bash
npx skills add https://github.com/brian-bell/agent-skills \
  -g -a codex -a claude-code --skill '*' -y
```

The experimental catalogs have no names in common with the root catalog. Add
the experimental Codex catalog to expand the Codex inventory:

```bash
npx skills add https://github.com/brian-bell/agent-skills/tree/main/catalogs/experimental/codex \
  -g -a codex --copy --skill '*' -y
```

Add the experimental Claude Code catalog to expand the Claude Code inventory:

```bash
npx skills add https://github.com/brian-bell/agent-skills/tree/main/catalogs/experimental/claude-code \
  -g -a claude-code --copy --skill '*' -y
```

Experimental installs use `--copy` so same-named runtime editions remain
independent.

Refresh installations by rerunning the explicit catalog commands. Do not
document a blanket `npx skills update` workflow: global update metadata is
keyed by skill name rather than skill name and runtime, so same-named Codex and
Claude Code supplemental editions cannot retain independent update sources.

Repository changes must not delete or overwrite a user's existing global
installations. Workstation cutover is a separate operation after review and
merge.

## Common Catalog Ownership

The root versions of all common skills are their sole canonical
repository implementations. Start new common-skill behavior in root
`skills/<name>/`; do not add same-named copies to the supplemental catalogs.

Use the complete experimental Codex edition as the baseline when promoting
another skill. Keep GitHub access wording integration-neutral, use `gh`
when higher-level integration coverage is insufficient, avoid runtime-specific
orchestration tool names, use portable skill-composition prose, and keep Codex
UI metadata under `agents/openai.yaml`. After verification in both agents,
remove the old Codex and Claude Code editions so ownership remains unique.

## Verification

Catalog migration and maintenance use one-off inspections rather than a
repository-owned validation harness. At minimum, confirm inventory counts,
name uniqueness, the absence of catalog symlinks, local asset resolution,
executable modes, metadata placement, provenance, and clean whitespace.

Inspect CLI discovery without installing:

```bash
npx skills add . --list
npx skills add ./catalogs/experimental/codex --list
npx skills add ./catalogs/experimental/claude-code --list
```

CLI discovery must return the exact skill counts for the root and experimental
Codex and Claude Code catalogs.
Verify that root names do not overlap either experimental inventory. Also check
frontmatter names, local asset resolution, attribution, metadata placement,
executable modes, and the absence of catalog symlinks.

Keep the contributor-documentation link intact:

```bash
test -L CLAUDE.md && test "$(readlink CLAUDE.md)" = AGENTS.md
```

Do not add replacement catalog-generation, layout, parity, or installation
test infrastructure.

## Conventions

- Keep portable skill frontmatter focused: `name` and `description` are
  required. Preserve or add optional agent-specific frontmatter fields and
  conventions, such as `argument-hint`, `disable-model-invocation`, and
  `disallowed-tools`, when other supported agents safely ignore them or
  interpret them compatibly. Do not use conventions whose semantics conflict
  across agents or negatively affect cross-agent behavior.
- Keep repository-only maintenance skills under `.agents/`; do not add them to
  any published catalog.
- Put Codex UI metadata under `agents/openai.yaml` in the Codex experimental
  edition and portable root edition. Portable root skills may keep optional
  `agents/openai.yaml` metadata; other agents can ignore it.
- Multi-reviewer skills use the orchestrator-role shape rather than registered
  agent definitions. Reviewer briefs are runtime-neutral prompt sources under
  `roles/`, and the orchestrator runs inline in the main session rather than
  being delegated to a lead subagent. This lets the orchestrator check in with
  the user and read each role's full report directly. `go-review`,
  `feature-review`, `review-gate`, and `product-manager` follow this shape.
- Role briefs carry no frontmatter and must restate their own read-only
  `<HARD-GATE>`. The gate must explicitly prohibit shell and git mutation.
- Nothing installs into `~/.claude/agents/`. If registered agent definitions
  are ever reconsidered, weigh them against the inline orchestrator's ability
  to check in with the user and read full role reports directly.
- Treat the experimental catalogs as independent materialized runtime editions.
  Common first-party skills live only in the root catalog.
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
  `gh` when its coverage is insufficient. Experimental runtime editions may
  use runtime-specific integration wording.
- When adding, removing, or renaming a portable skill, update the documented
  inventories and recheck cross-catalog name uniqueness.
- Keep agent context in `AGENTS.md`; keep `CLAUDE.md` as a symlink to it.
- Keep published skills under root `skills/` or one of the explicit
  `catalogs/*/skills/` directories; do not introduce another source layout.
