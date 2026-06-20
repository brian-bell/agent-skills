---
name: autobuild
description: Run an implementation task through a local multi-agent build pipeline using a bundled Python helper. Use when the user wants Codex or Claude launched once per ordered phase, or asks for the implementation, review-loop, autoreview, and PR-creation sequence with commits after implementation and review.
---

# Autobuild

Use this skill when the current session should orchestrate a task through a
repeatable multi-agent build pipeline. The bundled helper runs one agent process
per phase, waits for it to report back, checks the git worktree, then launches
the next phase. The helper — not the agent — owns control flow; each agent does
work *inside* a phase and reports a single status line.

The phase order is fixed:

1. `implementation`
2. `review-loop`
3. `autoreview`
4. `pr-creation`

The helper supports Codex and Claude. Codex is the default engine and runs
through `codex exec`; Claude runs through `claude -p`. The engine can be chosen
per phase. Implementation and Review Loop must create commits before the helper
advances.

## Dependencies

The `review-loop` and `autoreview` phases delegate to the skills of the same
name, so those must be installed for whichever engine runs each phase. Install
them from <https://github.com/brian-bell/agent-skills> into your engine's skills
directory (`~/.claude/skills` or `~/.codex/skills`). The helper checks for them
at kickoff and refuses to start if either is missing, naming what to install.
Pass `--skip-skill-check` to bypass the check for non-standard install locations.

## Run

```bash
~/.codex/skills/autobuild/scripts/autobuild \
  --repo /path/to/target-repo \
  --task "Implement the requested change" \
  --base origin/main
```

Use Claude for everything:

```bash
~/.codex/skills/autobuild/scripts/autobuild \
  --engine claude \
  --repo /path/to/target-repo \
  --task-file /path/to/plan.md \
  --base origin/main
```

Pick the engine per phase (autoreview-style), and tune model/effort per engine:

```bash
~/.codex/skills/autobuild/scripts/autobuild \
  --task "Implement the requested change" \
  --phase-engine review-loop=claude \
  --model codex=gpt-5.1 --effort codex=high
```

Other flags: `--max-retries N` (extra attempts after the first for a
`blocked`/`needs_attention` phase; default `1`), `--report-dir` (keep per-phase
stdout/stderr reports outside the checkout), `--codex-bin` / `--claude-bin` for
alternate binaries, and `--dry-run` to inspect the per-phase prompts without
launching anything.

Run from a work branch, not `main`, `master`, the configured base branch, or a
detached `HEAD`; the helper refuses protected branches because implementation and
review-loop phases create commits.

## Phase Contract

Each phase receives the task, repository path, base ref, phase ID, and these
instructions:

- Work only on the current phase; the helper launches the next phase.
- End with `AUTOBUILD_REPORT: <phase>: <completed|blocked|needs_attention> - <short summary>`.
- Leave `git status --porcelain --untracked-files=all` clean.
- Commit changes before reporting completion. This is enforced for
  `implementation` and `review-loop`.

The helper stops and reports the failing phase if a phase exits non-zero, leaves
a dirty worktree, changes branch, rewinds the branch tip to a non-descendant
commit, or (for a required-commit phase) does not advance `HEAD`. A phase that
reports `blocked`/`needs_attention` is re-launched with the prior summary fed
back, up to `--max-retries` times, then the pipeline halts.

## Phase Guidance

Implementation should use the repository's normal TDD process, run the focused
tests, run broader checks appropriate to the change, and commit the
implementation.

Review Loop should use the `review-loop` skill, apply accepted feedback, rerun
relevant tests, and commit the review result. If the review loop finds no file
changes, create an empty checkpoint commit and report that explicitly.

Autoreview should use the `autoreview` skill against the branch/base, verify
findings against the actual code, rerun checks after accepted fixes, and leave
the worktree clean before reporting.

PR Creation should create or update the pull request using the repository's
normal PR process, include the implementation/review/autoreview results in the
PR summary or comment, and leave the worktree clean.

## Helper

The helper lives at:

```bash
~/.codex/skills/autobuild/scripts/autobuild --help
```

It sends phase prompts over stdin, sets `AUTOBUILD_PHASE` for each launched
process, and writes optional per-phase reports when `--report-dir` is supplied.
Child agent environments forward only an allowlist of standard shell, Git,
provider, and credential variables, so parent launch metadata is not passed
through accidentally. Report directories are written `0700` and report files
`0600`, because prompts and agent output can contain sensitive task context.

Run the test suite (hermetic, fake engines, no model calls) with:

```bash
python3 ~/.codex/skills/autobuild/scripts/autobuild_test.py -v
```
