---
name: plan-with-review
description: Write a TDD-based implementation plan and then critique-and-revise the plan itself through a $review-loop reviewer cycle until it hits a 9/10 quality gate. Use when the user wants a reviewed, hardened implementation plan (not the implementation) — a plan that bakes in test-first development and has been adversarially critiqued and revised before any code is written.
---

# Plan With Review

Use this workflow to produce a high-quality, TDD-based implementation **plan** and then harden it by running the `$review-loop` skill against the plan itself. The output is a reviewed plan — this skill does **not** implement, edit code, or ship. If the user wants execution after the plan is ready, hand the finished plan to a separate implementation workflow.

## Defaults

These override `$review-loop`'s defaults for this workflow:

| Setting | Value |
|---------|-------|
| Min loops | `1` |
| Max loops | `12` |
| Quality gate | `9/10` |
| Reviewer | separate subagent, equal to or more powerful than the planner |

`Min loops = 1` means at least one critique pass always runs; if that first review already scores `9/10`, the loop exits without a revision. If you want to guarantee at least one revision cycle, raise the minimum to `2`.

Honor user overrides for the plan's scope, test strategy, output location, or the loop settings — but unless the user explicitly opts out of this workflow, always run at least the `1` plan-review pass and do not relax the `9/10` quality gate.

## 1. Establish Scope

1. Read project instructions and the files needed to understand the request.
2. Discover existing test, format, lint, typecheck, and build commands from repo docs, manifests, and CI.
3. Check `git status --short` so the plan accounts for any in-progress work.
4. Identify:
   - User-visible behavior or API changes.
   - Files and modules likely to change.
   - Existing tests that should guide the work.
   - Risks, migrations, compatibility boundaries, and rollback concerns.

Ask only for requirements that cannot be safely inferred from the repo or the request.

## 2. Write the TDD-Based Plan

Write a detailed implementation plan. Do not edit code, tests, config, or other repository files — the deliverable is the plan. Put the plan in the conversation, or in a file if the user asks (default to `./plans/<slug>.md` when a file is requested).

The plan must include:

- **Goal and non-goals.**
- **Current system observations** with concrete file references.
- **Implementation steps** in dependency order.
- **A `$tdd` section** that drives the work test-first, structured as vertical tracer-bullet slices (one failing test → its implementation → next slice), per `$tdd` — **not** all tests written up front. For each slice, state the failing test to write, the expected red state, the implementation path to green, and the refactor pass. Every behavior change must be tied to a test. If the codebase cannot support tests (e.g. greenfield with no harness yet), say so and have the plan stand up the test harness as its first slice.
- **Verification commands** (the exact test/lint/typecheck/build commands discovered in step 1) and the expected evidence of success.
- **Risks, migrations, and explicit stop conditions.**

Keep the plan concrete enough that another agent could execute it without re-discovering the problem.

## 3. Review-Loop the Plan

Run the `$review-loop` skill against the plan itself, explicitly passing the overrides above (min `1`, max `12`, quality gate `9/10`) so it does not fall back to its own `2`/`4`/`8` defaults.

Each loop:

1. Spawn a **separate reviewer subagent** (fresh context — give it the user request, the relevant repo observations, and the draft plan, not your reasoning). The subagent has no memory of prior loops, so pass the **complete current plan text** (not a diff or summary) plus a running list of prior feedback on every loop.
2. Ask the reviewer to score the plan `1-10` and flag, with specifics:
   - Missing or weak TDD coverage (behavior changes with no test, vague red/green steps).
   - Blocking gaps, unstated assumptions, or scope risks.
   - Sequencing or dependency-ordering problems.
   - Unclear or unmeasurable acceptance criteria and verification.
3. Revise the plan to address Critical and Important findings, keeping the running feedback list current so resolved issues are not reintroduced.
4. Apply `$review-loop`'s stop conditions in order: continue until min loops are met, stop when the score reaches `9`, and stop at `12` loops even if the gate is unmet (note the residual gap to the user).

## 4. Final Report

Deliver the final reviewed plan plus:

- Final score and number of plan-review loops run.
- Key issues the review surfaced and how the plan now addresses them.
- Any residual gaps if the quality gate was not reached within `12` loops.
- A clear note that this skill produced a plan only — implementation is a separate step.

## Guardrails

- Do not implement, edit code, or ship; produce a reviewed plan only.
- Do not self-review in place of a reviewer subagent (anchoring bias).
- Do not drop below `1` plan-review loop or relax the `9/10` gate unless the user explicitly opts out.
- Do not duplicate or reinterpret the `$review-loop` or `$tdd` workflows — compose them.
