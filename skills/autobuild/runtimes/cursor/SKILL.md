---
name: autobuild
description: Claude-runner build pipeline (implementation, review-loop, autoreview, PR creation) driven by a bundled helper that launches one Claude CLI process per phase. Cursor-native autobuild is unsupported; use only when the user explicitly asks to run the Claude autobuild helper anyway.
---

# Autobuild

Autobuild drives an implementation task through a fixed pipeline —
`implementation → review-loop → autoreview → pr-creation` — by launching one
Claude CLI process per phase via the bundled helper at
`<skill-dir>/scripts/autobuild`. It is an explicitly Claude-runner skill.

Cursor-native autobuild is unsupported. Do not silently drive the Claude
helper from this runtime: when the user asks for autobuild, report that this
skill is Claude-runner-only and stop, unless the user explicitly asks to run
the Claude helper anyway.

## Running The Claude Helper On Explicit Request

Only when the user explicitly opts in:

- The `claude` CLI must be installed and authenticated; every phase runs
  through `claude -p`.
- The `review-loop` and `autoreview` skills must be installed in the Claude
  skill root (install from <https://github.com/brian-bell/agent-skills>). The
  helper checks at kickoff and refuses to start if either is missing; pass
  `--skip-skill-check` only for non-standard install locations.
- The `autoreview` phase launches Codex underneath, so Codex must also be
  installed and authenticated; the helper forwards `CODEX_`/`OPENAI_`
  environment variables to keep that auth.
- Run from a work branch, never `main`/`master`/the base branch; the helper
  refuses protected branches because phases create commits.

```bash
<skill-dir>/scripts/autobuild \
  --repo /path/to/target-repo \
  --task "Implement the requested change" \
  --base origin/main
```

See `<skill-dir>/scripts/autobuild --help` and `<skill-dir>/README.md` for the
full flag reference and phase contract. Report the helper's outcome verbatim,
including the final phase status lines.
