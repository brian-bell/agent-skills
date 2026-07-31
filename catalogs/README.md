# Supplemental Skill Catalogs

The repository root provides the canonical common catalog. The catalogs in
this directory contain only additional skills, with no names that overlap the
root catalog.

```text
catalogs/
├── first-party/
│   ├── claude-code/skills/<name>/
│   └── codex/skills/<name>/
└── third-party/
    ├── ATTRIBUTION.md
    └── skills/<name>/
```

## Installation

Install the common catalog first:

```bash
npx skills add https://github.com/brian-bell/agent-skills \
  -g -a codex -a claude-code --skill '*' -y
```

Then add any supplemental catalogs you want.

### Codex first-party additions

```bash
npx skills add https://github.com/brian-bell/agent-skills/tree/main/catalogs/first-party/codex \
  -g -a codex --copy --skill '*' -y
```

### Claude Code first-party additions

```bash
npx skills add https://github.com/brian-bell/agent-skills/tree/main/catalogs/first-party/claude-code \
  -g -a claude-code --copy --skill '*' -y
```

The first-party catalogs have the same names but runtime-specific
instructions. `--copy` keeps their installed editions independent.

### Portable third-party additions

```bash
npx skills add https://github.com/brian-bell/agent-skills/tree/main/catalogs/third-party \
  -g -a codex -a claude-code --skill '*' -y
```

The third-party catalog has one portable implementation of each skill, so one
command installs it for both agents without `--copy`.

## Inventory

Both supplemental first-party catalogs contain:

- `feature-review`
- `go-review`
- `product-manager`

The supplemental third-party catalog contains:

- `grill-me`
- `prd-to-issues`
- `prd-to-plan`
- `write-a-prd`

Together, the root common catalog and supplemental catalogs install the full
skill inventory into each agent.

## Refreshing installations

Refresh by rerunning the explicit catalog commands. Do not use a blanket
`npx skills update` workflow for this repository: global update metadata is
keyed by skill name, not skill name and runtime, so the same-named Codex and
Claude Code supplemental editions cannot retain independent update sources.

## Adopting a third-party skill

Third-party adoption is a manual filesystem workflow:

1. Review the upstream skill, license, and provenance.
2. Copy the complete skill directory into
   `catalogs/third-party/skills/<name>/`, retaining every file and executable
   mode.
3. Confirm `SKILL.md` is directly inside that directory, its frontmatter
   `name` matches the directory, and the name does not collide with the root or
   either first-party catalog.
4. Keep instructions portable across agent installation roots and ensure all
   referenced assets resolve within the skill directory.
5. Add an `ATTRIBUTION.md` inside the skill and update
   `catalogs/third-party/ATTRIBUTION.md`.

Do not assemble, prune, or fork a third-party skill by runtime. Existing tests,
evaluation tools, references, assets, walkthroughs, build utilities,
`.skillignore`, and vendored dependencies remain part of the adopted skill.

When promoting a third-party skill to the common catalog, move its complete
directory unchanged to root `skills/`, move its provenance entry to root
`ATTRIBUTION.md`, and remove its supplemental attribution row. The root package
then becomes the canonical curated copy.
