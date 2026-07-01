# autobuild

A helper-owned multi-agent build pipeline. `scripts/autobuild` drives an
implementation task through a fixed sequence of phases —
`implementation → review-loop → autoreview → pr-creation` — launching one Claude
CLI agent per phase over stdin, gating the git worktree between phases,
committing after implementation and review, and advancing only when a phase
reports completion. The orchestrator owns control flow; each agent does work
inside a phase and reports a single status line back.

Stdlib only — it runs anywhere `python3`, `git`, and the `claude` CLI exist.
Codex-native autobuild is unsupported until a future `codex exec` driver exists;
Codex should not silently run this Claude helper unless the user explicitly asks
for that.

## Dependencies

The `review-loop` and `autoreview` phases use the skills of the same name, which
must be installed for Claude. Install them from
<https://github.com/brian-bell/agent-skills> into the same Claude skill root as
this skill. The helper
verifies this at kickoff and refuses to start if either is missing (bypass with
`--skip-skill-check`).

The `autoreview` skill runs Codex by default, so the `autoreview` phase launches
Codex underneath. Codex must be installed and authenticated for that phase; the
helper forwards `CODEX_`/`OPENAI_` env vars so the nested process keeps its auth.

## Install

This skill is portable. Symlink it into your agent's skills directory, e.g.:

```bash
ln -s "$PWD/skills/autobuild" <skill-root>/autobuild
```

## Usage

```bash
# Run every phase through Claude:
scripts/autobuild --repo /path/to/repo --task "Implement the change" --base origin/main

# Tune the model/effort:
scripts/autobuild --task "Implement the change" --model opus --effort high

# Inspect the prompts without launching anything:
scripts/autobuild --task demo --base origin/main --dry-run
```

Every phase defaults to `--permission-mode bypassPermissions`, because headless
`claude -p` can't approve tool use and would otherwise auto-deny the Bash calls a
phase needs. Adjust with `--claude-permission-mode MODE` (uniform),
`--phase-permission PHASE=MODE` (one phase), or `--claude-allowed-tools TOOLS`.

Run from a work branch (not `main`/`master`/the base branch). See `SKILL.md` for
the full phase contract, flags, and safety rules.

## Tests

```bash
python3 scripts/autobuild_test.py -v   # hermetic; fake engine + temp git repos, no model calls
```
