---
name: tdd-with-review
description: Compose $tdd, $docs, $review-loop, and $ship into one implementation workflow. Use when the user wants a feature or bug fix implemented test-first, documented, iterated through a reviewer quality loop, then committed, pushed, and opened or updated as a PR.
---

# TDD With Review

Use this workflow when the user wants implementation work to move from test-first development through documentation updates, review-loop quality gates, and into the normal shipping flow.

Load and follow these skills in order:

1. `$tdd` for the implementation.
2. `$docs` to update project documentation from the implemented source of truth.
3. `$review-loop` for critique and revision after the implementation is green and docs are current.
4. `$ship` for commit, push, and PR handling.

## Workflow

Before starting, check the repo state and protect unrelated work. Honor user overrides for test scope, review-loop settings, stopping before ship, or dry-run behavior.

Run `$ship` only after:

- `$tdd` has produced passing relevant tests or documented why test-first execution was blocked.
- `$docs` has updated affected documentation or documented why no documentation changes were needed.
- `$review-loop` has completed according to its stop conditions.
- Blocking review findings and failing checks are resolved, unless the user explicitly accepts the risk.

## Final Report

Report TDD evidence, documentation updates, review-loop result, verification, ship result, and any unrelated dirty files left untouched.

## Guardrails

- Do not duplicate or reinterpret the component skill workflows.
- Do not skip `$tdd`, `$docs`, `$review-loop`, or `$ship` unless the user explicitly narrows the workflow.
- Do not stage, commit, or revert unrelated work.
