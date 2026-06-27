---
name: tdd-with-review
description: Compose $tdd, $commit, $docs, $review-loop, $autoreview into one implementation workflow. Use when the user wants a feature or bug fix implemented test-first, documented, and iterated through reviewer quality loops.
---

# TDD With Review

Use this workflow when the user wants implementation work to move from test-first development through documentation updates, review-loop quality gates, and into the normal shipping flow.

Load and follow these skills in order:

1. `$tdd` for the implementation.
2. `$review-loop` for critique and revision after the implementation is green and docs are current.
3. `$docs` to update project documentation from the implemented source of truth.
4. `$autoreview` for squashing bugs before submitting a PR.

IMPORTANT: Use `$commit` between each phase so a human reviewer can observe the progression of work.

## Workflow

Before starting, check the repo state and protect unrelated work. Honor user overrides for test scope, review-loop settings, stopping before ship, or dry-run behavior.

Run `$commit` after each phase when that phase reaches its own checkpoint:

- After `$tdd`: relevant tests pass, or the report documents why test-first execution was blocked.
- After `$review-loop`: the loop has completed according to its stop conditions, with accepted findings resolved or explicitly deferred by the user.
- After `$docs`: affected documentation is updated, or the report documents why no documentation changes were needed.
- After `$autoreview`: the review is clean, and blocking findings or failing checks are resolved unless the user explicitly accepts the risk.

## `$review-loop` Rules

- Quality gate 8/10
- Minimum loops 2
- Max loops 10

The user may override any or all of these values.

## Final Report

Report TDD evidence, review-loop result, documentation updates, commits, verification, and any unrelated dirty files left untouched.

## Guardrails

- Do not duplicate or reinterpret the component skill workflows.
- Do not skip `$tdd`, `$review-loop`, `$docs`, `$autoreview` unless the user explicitly narrows the workflow.
- Do not skip the `$commit` between each phase unless the user says so explicitly.
- Do not stage, commit, or revert unrelated work. Use stash as needed.
