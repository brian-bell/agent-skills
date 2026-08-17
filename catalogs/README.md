# Supplemental Skill Catalogs

The repository root provides the canonical common catalog. The catalogs in
this directory contain only additional skills, with no names that overlap the
root catalog.

```text
catalogs/
└── experimental/
    ├── claude-code/skills/<name>/
    └── codex/skills/<name>/
```

## Installation

Install the common catalog first:

```bash
npx skills add https://github.com/brian-bell/agent-skills \
  -g -a codex -a claude-code --skill '*' -y
```

Then add any supplemental catalogs you want.

### Experimental Codex additions

```bash
npx skills add https://github.com/brian-bell/agent-skills/tree/main/catalogs/experimental/codex \
  -g -a codex --copy --skill '*' -y
```

### Experimental Claude Code additions

```bash
npx skills add https://github.com/brian-bell/agent-skills/tree/main/catalogs/experimental/claude-code \
  -g -a claude-code --copy --skill '*' -y
```

The experimental catalogs have the same names but runtime-specific
instructions. `--copy` keeps their installed editions independent.

## Inventory

Both experimental catalogs contain:

- `feature-review`
- `go-review`
- `product-manager`
- `review-gate`

Together, the root common catalog and experimental catalogs install the full
skill inventory into each agent.

## Refreshing installations

Refresh by rerunning the explicit catalog commands. Do not use a blanket
`npx skills update` workflow for this repository: global update metadata is
keyed by skill name, not skill name and runtime, so the same-named Codex and
Claude Code supplemental editions cannot retain independent update sources.
