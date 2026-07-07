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

This skill is runtime-forked: `shared/` holds the helper and this README, and
`runtimes/{claude,codex,cursor}/SKILL.md` hold per-runtime instructions
(Claude is the full workflow; Codex and Cursor are explicit-opt-in stubs).
Install with the repo installer, which assembles `shared/` plus the matching
runtime overlay into a staged copy and symlinks it into each skill root:

```bash
./install.sh
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

Run from a work branch (not `main`/`master`/the base branch); the helper refuses
protected branches because phases create commits. See `scripts/autobuild --help`
for the full flag reference.

## Phase Contract

Each phase launches one Claude agent that must:

- work only on the current phase (the helper launches the next one),
- end with `AUTOBUILD_REPORT: <phase>: <completed|blocked|needs_attention> - <short summary>`,
- leave `git status --porcelain --untracked-files=all` clean, and
- commit changes before reporting completion (enforced for `implementation` and
  `review-loop`).

The helper stops and reports the failing phase if a phase exits non-zero, leaves
a dirty worktree, changes branch, rewinds the branch tip to a non-descendant
commit, or (for a required-commit phase) does not advance `HEAD`. A phase that
reports `blocked`/`needs_attention` is re-launched with the prior summary fed
back, up to `--max-retries` times, then the pipeline halts.

## Tests

```bash
python3 scripts/autobuild_test.py -v   # hermetic; fake engine + temp git repos, no model calls
```
