---
name: tdd
description: Route tdd to its isolated Codex or Claude Code runtime assembly.
---

# tdd Runtime Router

Identify the active host, then read and follow exactly one runtime assembly:

- Codex: `runtimes/codex/SKILL.md`
- Claude Code: `runtimes/claude/SKILL.md`

Never combine, merge, or fall back to the other runtime's instructions. If the
active host is neither Codex nor Claude Code, stop and report that this package
does not support the current runtime.
