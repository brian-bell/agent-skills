# Findings Schema — Workflow Mode Addendum

The role briefs and their checklists live in [roles/](roles/). This file is Claude-only: use it when the review runs in **workflow mode**, so each role's `agent()` call returns schema-validated JSON instead of markdown, and the verification stage has something structured to argue with.

Findings marked `needs_verification: true` feed the adversarial pass.

## Shape

Every role returns:

```json
{
  "role": "structure | error | style | security",
  "findings": [
    {
      "file": "relative/path.go",
      "line": 42,
      "category": "short category label",
      "severity": "critical | high | medium | low",
      "claim": "one-sentence statement of the defect",
      "detail": "why it is wrong, with the surrounding context",
      "suggested_fix": "concrete recommendation",
      "needs_verification": true
    }
  ],
  "checked_clean": ["categories inspected that produced nothing"]
}
```

`checked_clean` is not decoration. Without it an empty `findings` array is ambiguous between "inspected and clean" and "never got there", and the consolidation step cannot tell the difference.

## Severity mapping

Roles use their own scales; normalize on return so the consolidator compares like with like:

| Role scale | Schema `severity` |
|---|---|
| security: critical / high / medium / low | same |
| error: bug-risk / robustness / minor | critical or high / medium / low |
| structure: high / medium / low | high / medium / low |
| style: (unscaled) | low, unless it hides a real defect |

## What needs verification

Set `needs_verification: true` when the finding depends on something the role inferred rather than read:

- concurrency claims resting on assumed call patterns rather than an observed goroutine launch,
- "unused" or "dead" claims, which need a repo-wide search to stand up,
- anything about behavior across a package boundary the role did not open,
- severity that hinges on whether input is user-controlled.

Set it `false` for what is visible in the cited lines — a missing `defer Close()`, a `%v` that should be `%w`.

The verification stage tries to **refute** these. A finding whose refuter cannot reproduce the reasoning is downgraded, not silently dropped: it comes back at `low` with the doubt recorded, because a wrong severity is a smaller failure than a disappeared finding.
