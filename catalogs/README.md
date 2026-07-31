# Skill Catalogs

The repository root provides a curated eight-skill common catalog. This
`catalogs/` directory retains the broader 10/10/11 inventories for users who
want the full Codex, Claude Code, and portable third-party collections. Each
skill is complete beneath its own directory and can be installed directly with
[`npx skills`](https://github.com/vercel-labs/skills).

```text
catalogs/
├── first-party/
│   ├── claude-code/skills/<name>/
│   └── codex/skills/<name>/
└── third-party/
    ├── ATTRIBUTION.md
    └── skills/<name>/
```

## Catalogs and installation

### Common catalog

Source: `https://github.com/brian-bell/agent-skills`

```bash
npx skills add https://github.com/brian-bell/agent-skills
```

For a non-interactive global install into both agents:

```bash
npx skills add https://github.com/brian-bell/agent-skills \
  -g -a codex -a claude-code --skill '*' -y
```

The common catalog contains `autofix`, `autoreview`, `batch-grill-me`, `docs`,
`review-loop`, `ship`, `tdd`, and `tdd-with-review`.

Choose either this common profile or the full profile below. Installing both
into the same global inventory creates overlapping names, ambiguous update
sources, and unclear ownership of the installed copy.

### Full profile

The following direct catalog URLs remain supported.

### First-party Codex

Source:
`https://github.com/brian-bell/agent-skills/tree/main/catalogs/first-party/codex`

```bash
npx skills add https://github.com/brian-bell/agent-skills/tree/main/catalogs/first-party/codex \
  -g -a codex --copy --skill '*' -y
```

### First-party Claude Code

Source:
`https://github.com/brian-bell/agent-skills/tree/main/catalogs/first-party/claude-code`

```bash
npx skills add https://github.com/brian-bell/agent-skills/tree/main/catalogs/first-party/claude-code \
  -g -a claude-code --copy --skill '*' -y
```

First-party installs use `--copy` because the Claude Code and Codex editions
have the same skill names but different runtime instructions. Independent
copies prevent one runtime's edition from becoming the canonical source for
the other.

### Portable third-party

Source:
`https://github.com/brian-bell/agent-skills/tree/main/catalogs/third-party`

```bash
npx skills add https://github.com/brian-bell/agent-skills/tree/main/catalogs/third-party \
  -g -a codex -a claude-code --skill '*' -y
```

The third-party catalog has one portable implementation of each skill. The
single command installs that shared implementation for both agents, so it does
not use `--copy`.

## Inventory

Both first-party catalogs contain these ten skills:

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

The portable third-party catalog contains these eleven skills:

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

First-party and third-party names must remain unique within an agent's
installed inventory. Claude Code and Codex first-party editions may share
names because they are installed independently. Each agent receives 21 unique
skills when the three full-profile commands are run.

## Refreshing installations

Refresh by rerunning the explicit command or commands for the selected profile.
Do not use a blanket `npx skills update` workflow for this repository: global
update metadata is keyed by skill name, not by skill name and runtime, so
same-named editions cannot retain independent update sources. Explicit catalog
commands keep the source and target unambiguous.

## Adopting a third-party skill

Third-party adoption is a manual filesystem workflow:

1. Review the upstream skill, license, and provenance.
2. Copy the complete skill directory into
   `catalogs/third-party/skills/<name>/`, retaining every file and executable
   mode.
3. Confirm `SKILL.md` is directly inside that directory, its frontmatter
   `name` matches the directory, and the name does not collide with either
   first-party catalog.
4. Keep instructions portable across agent installation roots and ensure all
   referenced assets resolve within the skill directory.
5. Add an `ATTRIBUTION.md` inside the skill and update
   `catalogs/third-party/ATTRIBUTION.md`.

Do not assemble, prune, or fork a third-party skill by runtime. Existing tests,
evaluation tools, references, assets, walkthroughs, build utilities,
`.skillignore`, and vendored dependencies remain part of the adopted skill.

When a third-party skill is promoted to the root common catalog, its complete
directory is copied unchanged. The full third-party package remains canonical;
the root copy must retain identical content, paths, metadata, and executable
modes. Make future changes in `catalogs/third-party/skills/<name>/` first, then
copy the updated package unchanged into the root catalog.
