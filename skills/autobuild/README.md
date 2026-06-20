# autobuild

A helper-owned multi-agent build pipeline. `scripts/autobuild` drives an
implementation task through a fixed sequence of phases —
`implementation → review-loop → autoreview → pr-creation` — launching one CLI
agent (Codex or Claude) per phase over stdin, gating the git worktree between
phases, committing after implementation and review, and advancing only when a
phase reports completion. The orchestrator owns control flow; each agent does
work inside a phase and reports a single status line back.

Stdlib only — it runs anywhere `python3`, `git`, and an agent CLI exist.

## Dependencies

The `review-loop` and `autoreview` phases use the skills of the same name, which
must be installed for whichever engine runs each phase. Install them from
<https://github.com/brian-bell/agent-skills> into your engine's skills directory
(`~/.claude/skills` or `~/.codex/skills`). The helper verifies this at kickoff
and refuses to start if either is missing (bypass with `--skip-skill-check`).

## Install

This skill is portable. Symlink it into your agent's skills directory, e.g.:

```bash
ln -s "$PWD/skills/autobuild" ~/.codex/skills/autobuild
ln -s "$PWD/skills/autobuild" ~/.claude/skills/autobuild
```

## Usage

```bash
# Default engine (codex) for every phase:
scripts/autobuild --repo /path/to/repo --task "Implement the change" --base origin/main

# Per-phase engine selection, plus model/effort tuning:
scripts/autobuild --task "Implement the change" \
  --phase-engine review-loop=claude \
  --model codex=gpt-5.1 --effort codex=high

# Inspect the prompts without launching anything:
scripts/autobuild --task demo --base origin/main --dry-run
```

Run from a work branch (not `main`/`master`/the base branch). See `SKILL.md` for
the full phase contract, flags, and safety rules.

## Tests

```bash
python3 scripts/autobuild_test.py -v   # hermetic; fake engines + temp git repos, no model calls
```
