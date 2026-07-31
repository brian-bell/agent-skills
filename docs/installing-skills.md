# Installing Skills

## Codex

The generated catalog is the official installation source for Codex. Run the
interactive command and choose the skills you want:

```bash
npx skills add https://github.com/brian-bell/agent-skills/tree/main/catalog --agent codex
```

The catalog contains all 21 portable install names in this repository. Its ten
first-party packages are flat Codex assemblies: shared files are copied to the
package root, then the Codex runtime overlay is applied at that same root. The
remaining eleven packages are faithful attributed copies of `third-party/`.

### Unattended installation

Install every package globally as a copy:

```bash
npx skills add https://github.com/brian-bell/agent-skills/tree/main/catalog \
  --skill '*' \
  --agent codex \
  --global \
  --yes \
  --copy
```

Install only selected skills by repeating `--skill`:

```bash
npx skills add https://github.com/brian-bell/agent-skills/tree/main/catalog \
  --skill chrome-reading-list \
  --skill tdd \
  --agent codex \
  --global \
  --yes \
  --copy
```

Omit `--global` to use the CLI's project-scoped installation flow. Omit
`--copy` if you want the CLI's default linking behavior.

The skills CLI does not resolve composition dependencies. When selecting a
composed workflow, also select every companion skill named in its instructions;
installing all packages includes the complete dependency set. For example,
install `autofix`, `autoreview`, and `ship` together.

### Listing and updating

List the catalog before installing:

```bash
npx skills add https://github.com/brian-bell/agent-skills/tree/main/catalog --list
```

List installed global Codex skills:

```bash
npx skills list --global --agent codex
```

Interactively update CLI-managed skills, or update all global CLI-managed
skills without prompting:

```bash
npx skills update
npx skills update --global --yes
```

## Current boundary

The generated catalog supports Codex only. It contains direct root Codex
instructions and no Claude runtime router or Claude payload. Claude Code
installation through `npx skills` is intentionally deferred.

Use the repository's Go installer for Claude Code, Codex or Claude session
hooks, and the GitHub import workflow:

```bash
./install.sh
```

The Go installer remains supported and unchanged; this catalog does not migrate
or retire its staged installs under `~/.skill-symlinks/`.

## Maintaining the catalog

Never edit `catalog/` directly. Regenerate it from the first-party and
third-party source trees, then check for drift:

```bash
python3 scripts/generate-skills-catalog.py
python3 scripts/generate-skills-catalog.py --check
```

Run the focused behavior and installation checks with:

```bash
python3 -m unittest scripts.test_generate_skills_catalog
scripts/test-skills-catalog-cli.sh
```

The live smoke requires a logged-in Codex CLI. It installs `feature-review`
into a temporary skill root, verifies the exposed UI metadata through Codex
app-server, and activates the exact installed root instructions in an ephemeral
read-only turn:

```bash
scripts/test-skills-catalog-codex.sh
```

CI keeps failures separated by compatibility layer:

- Pull requests run generator behavior tests.
- Pull requests run the non-mutating generated-catalog drift check.
- Pull requests copy-install all 21 packages with pinned Node and
  `skills@1.5.21` into an isolated Codex root.
- Pushes to `main` install from the canonical GitHub catalog URL with the
  pinned CLI.
- A weekly schedule and manual dispatch install from the same remote source
  with the latest upstream skills CLI.
