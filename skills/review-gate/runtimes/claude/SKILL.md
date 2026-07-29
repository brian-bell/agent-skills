---
name: review-gate
description: Run a standalone evidence-backed code review gate using native Codex review, an independent structured challenger, isolated verification, and causal adjudication. Use for final local, commit, branch, or pull-request review when clean status must fail closed if any review or verification stage is incomplete.
---

# Review Gate

Run this skill inline as the closeout owner. The bundled helper requires an
authenticated Codex CLI for the native, challenger, and adjudication stages.
Do not replace a missing Codex stage with a Claude-only review and do not
delegate the whole skill.

## Hard constraints

- Treat exit `0` as clean only when the report says every required stage
  completed.
- Treat exit `1` as accepted findings or target-caused verification failure.
- Treat exit `2` as incomplete. Never summarize it as clean or silently fall
  back to another reviewer.
- Do not fix findings during a gate run. Verify the report first, fix in a
  later step, rerun focused tests, and rerun the complete gate.
- Do not push merely to review.
- Do not execute candidate code from an untrusted or fork PR unless the user
  explicitly authorizes that execution.

## Pick one exact target

Use the bundled helper at `<skill-dir>/scripts/review-gate`.

Dirty local work:

```bash
<skill-dir>/scripts/review-gate --mode local --verify "<focused command>"
```

One immutable commit:

```bash
<skill-dir>/scripts/review-gate --mode commit --commit HEAD --verify "<focused command>"
```

Complete branch delta:

```bash
<skill-dir>/scripts/review-gate --mode branch --base origin/main --verify "<focused command>"
```

Pull request:

```bash
<skill-dir>/scripts/review-gate --mode pr --pr <number-or-url> --verify "<focused command>"
```

Pass `--discover-verification` only when no focused command is already known.
Use `--verification-not-applicable "<reason>"` only when executable
verification genuinely cannot add evidence. For an explicitly authorized
untrusted PR, add `--allow-untrusted-execution`.

Never use local mode for committed branch or PR work. Branch and PR modes
reject a dirty checkout rather than silently reviewing a narrower patch.

## Read the report

Check:

1. The frozen target identities and changed paths match the intended change.
2. Native and challenger stages both completed.
3. Verification ran in the snapshot and its evidence is relevant.
4. Every accepted and rejected decision has a concrete rationale.
5. Findings in unchanged files identify a changed causal location and a
   reachable scenario.
6. The final source fingerprint still matches the frozen target.

The native output is preserved verbatim. The helper does not infer findings
with a prose parser; a separate structured adjudicator accounts for native and
challenger claims.

## Historical evaluation

Run the bundled non-gating corpus evaluator with:

```bash
<skill-dir>/scripts/review-gate evaluate --results <results.json>
```

Report native, challenger, and union recovery separately. Never turn
live-model recovery into a deterministic test requirement.

## Final response

Include the exact helper command, frozen target id, verification evidence,
accepted and rejected findings, and final `clean`, `findings`, or `incomplete`
status.
