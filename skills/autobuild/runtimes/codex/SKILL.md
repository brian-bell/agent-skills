---
name: autobuild
description: Claude-runner build pipeline (implementation, review-loop, autoreview, PR creation) driven by a bundled helper that launches one Claude CLI process per phase. Codex-native autobuild is unsupported; use only when the user explicitly asks to run the Claude autobuild helper anyway.
---

# Autobuild

Autobuild drives an implementation task through a fixed pipeline —
`implementation → review-loop → autoreview → pr-creation` — by launching one
Claude CLI process per phase via the bundled helper at
`<skill-dir>/scripts/autobuild`. It is an explicitly Claude-runner skill.

Codex-native autobuild is unsupported until a future `codex exec` driver
exists. Do not silently drive the Claude helper from this runtime: when the
user asks for autobuild, report that this skill is Claude-runner-only and stop,
unless the user explicitly asks to run the Claude helper anyway.

## Running The Claude Helper On Explicit Request

Only when the user explicitly opts in: follow `<skill-dir>/README.md` — the
authoritative reference for prerequisites (an authenticated `claude` CLI, the
`review-loop` and `autoreview` skills installed in the Claude skill root, and
Codex auth for the `autoreview` phase), invocation, the phase contract, and
safety rules. `<skill-dir>/scripts/autobuild --help` has the full flag
reference. Run from a work branch, never `main`/`master`/the base branch; the
helper refuses protected branches because phases create commits. Report the
helper's outcome verbatim, including the final phase status lines.
