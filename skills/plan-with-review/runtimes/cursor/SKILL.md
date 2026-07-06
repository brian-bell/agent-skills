---
name: plan-with-review
description: Write a TDD-based implementation plan and then critique-and-revise the plan itself through a review-loop reviewer cycle until it hits a 9/10 quality gate. Use when the user wants a reviewed, hardened implementation plan (not the implementation) — a plan that bakes in test-first development and has been adversarially critiqued and revised before any code is written.
---

# Plan With Review

Use this workflow to produce a high-quality, TDD-based implementation plan and then harden it by running the *review-loop* skill against the plan itself. The output is a reviewed plan only. If the user wants execution after the plan is ready, hand the finished plan to a separate implementation workflow.

## Defaults

These override the *review-loop* skill's defaults for this workflow:

| Setting | Value |
|---------|-------|
| Min loops | `1` |
| Max loops | `12` |
| Quality gate | `9/10` |
| Reviewer | fresh-context reviewer through Cursor's native Task delegation path when available |

`Min loops = 1` means at least one critique pass always runs. If the first review already scores `9/10`, the loop exits without a revision. If the user wants to guarantee at least one revision cycle, raise the minimum to `2`.

Honor user overrides for the plan's scope, test strategy, output location, or loop settings, but unless the user explicitly opts out of this workflow, always run at least one plan-review pass and do not relax the `9/10` quality gate.

## 1. Establish Scope

1. Read project instructions and the files needed to understand the request.
2. Discover existing test, format, lint, typecheck, and build commands from repo docs, manifests, and CI.
3. Check `git status --short` so the plan accounts for any in-progress work.
4. Identify user-visible behavior or API changes, likely files and modules, existing tests, risks, migrations, compatibility boundaries, and rollback concerns.

Ask only for requirements that cannot be safely inferred from the repo or the request.

## 2. Write The TDD-Based Plan

Write a detailed implementation plan. Do not edit code, tests, config, or other repository files. Put the plan in the conversation, or in a file if the user asks. When a file is requested, default to `./plans/<slug>.md`.

The plan must include:

- Goal and non-goals.
- Current system observations with concrete file references.
- Implementation steps in dependency order.
- A TDD section that uses vertical tracer-bullet slices: one failing behavior test, the expected red state, the implementation path to green, and the refactor pass before moving to the next slice. Every behavior change must be tied to a test. If the codebase cannot support tests yet, make standing up the test harness the first slice.
- Verification commands discovered from the repo and the expected evidence of success.
- Risks, migrations, and explicit stop conditions.

Keep the plan concrete enough that another Cursor session could execute it without rediscovering the problem.

## 3. Review-Loop The Plan

Run the *review-loop* skill against the plan itself with min `1`, max `12`, and quality gate `9/10`.

Use Cursor's native Task delegation path for a fresh-context reviewer when available. Give the reviewer the user request, relevant repo observations, the complete current plan text, and the running list of prior feedback. Do not include private reasoning.

For each loop:

1. Ask the reviewer to score the plan `1-10`.
2. Ask it to flag missing or weak TDD coverage, blocking gaps, unstated assumptions, scope risks, sequencing problems, and unclear or unmeasurable acceptance criteria.
3. Revise the plan to address Critical and Important findings, keeping the running feedback list current so resolved issues are not reintroduced.
4. Continue until the minimum loop count is met and the score reaches `9`, or until `12` loops have run. If the gate is not reached by then, note the residual gap to the user.

## 4. Final Report

Deliver the final reviewed plan plus:

- Final score and number of plan-review loops run.
- Key issues the review surfaced and how the plan now addresses them.
- Any residual gaps if the quality gate was not reached within `12` loops.
- A clear note that this skill produced a plan only and that implementation is a separate step.

## Guardrails

- Do not implement, edit code, or ship; produce a reviewed plan only.
- Do not drop below one plan-review loop or relax the `9/10` gate unless the user explicitly opts out.
- Do not duplicate or reinterpret the *review-loop* or *tdd* workflows; compose them.
