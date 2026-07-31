# Findings Schema — Workflow Mode Addendum

The role briefs and their checklists live in [roles/](roles/). This file is Claude-only: use it when the review runs in **workflow mode**, so each role's `agent()` call returns schema-validated JSON instead of markdown, and the verification stage has something structured to argue with.

Findings marked `needs_verification: true` feed the adversarial pass.

## Shape

Every role returns:

```json
{
  "role": "product | safety | quality | maintainability | documentation",
  "assessment": "the role's overall read, in prose",
  "findings": [
    {
      "file": "relative/path",
      "line": 42,
      "category": "short category label",
      "severity": "blocker | significant | minor | note",
      "claim": "one-sentence statement of the gap",
      "detail": "evidence and rationale",
      "scenario": "when it manifests, concretely",
      "suggested_fix": "concrete recommendation",
      "needs_verification": true
    }
  ],
  "checked_clean": ["checklist areas inspected that produced nothing"]
}
```

`assessment` is not optional. A feature verdict rests on judgment the finding list does not carry — "the happy path is solid but nothing handles first-run" is the kind of read that decides ACCEPT vs ACCEPT WITH CONDITIONS.

`checked_clean` is not decoration. Without it an empty `findings` array is ambiguous between "inspected and clean" and "never got there", and the verdict cannot tell the difference.

## What needs verification

Set `needs_verification: true` when the finding depends on something the role inferred rather than read:

- "undocumented" or "untested" claims, which need a repo-wide search to stand up,
- completeness claims about states (empty, error, first-run) the role did not exercise,
- product-alignment judgments resting on inferred user intent rather than a stated goal,
- any **blocker**. A blocker is the one severity that stops a merge, so it earns a second opinion by default.

Set it `false` for what is visible in the cited lines.

## Verification and the verdict

Verifiers are prompted to **refute**. A finding that survives keeps its severity; one that does not is downgraded with the doubt recorded, never silently dropped.

This matters more here than in a code review: the verdict is a single word, and a blocker that quietly evaporates flips REQUEST CHANGES to ACCEPT with no visible reason. If a blocker is downgraded, say so in the report and say why.
