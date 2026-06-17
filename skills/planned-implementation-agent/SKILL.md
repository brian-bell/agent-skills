---
name: planned-implementation-agent
description: Plan and execute implementation tasks through a reviewed plan that explicitly incorporates the $tdd and $review-loop skills, quality gates, and a worker subagent. Use when the user asks to implement a task by first writing a detailed implementation plan, wants the plan itself reviewed or iterated, or wants a subagent dispatched to carry out an approved implementation plan.
---

# Planned Implementation Agent

Use this workflow to turn a task into a reviewed implementation plan, then delegate execution to a worker subagent. The plan must explicitly compose the `$tdd` skill for red/green/refactor execution and the `$review-loop` skill for plan and implementation critique loops. The main agent owns planning quality, subagent scope, verification, and the final report.

## Defaults

- Plan review loops: minimum `1`, using the `$review-loop` skill.
- Implementation review loops: minimum `2`, using the `$review-loop` skill.
- Quality gate: `8/10`
- TDD expectation: follow the `$tdd` skill by writing or updating failing tests before behavior changes whenever the codebase supports tests.
- Handoff style: bounded worker subagent with a concrete plan, acceptance criteria, and verification commands.

The plan review minimum intentionally overrides `$review-loop`'s default minimum of `2` loops for plan review only; implementation review keeps the stricter minimum of `2`.

Honor user overrides for quality gate, test strategy, delegation boundaries, or whether the main agent should execute instead of a subagent.
While this workflow is active, loop-count overrides may make the process stricter, but must not reduce plan review below `1` loop or implementation review below `2` loops unless the user explicitly cancels or opts out of this skill's workflow.

## 1. Establish Scope

1. Read project instructions and the files needed to understand the request.
2. Discover existing test, format, lint, typecheck, and build commands from repo docs, manifests, and CI.
3. Check `git status --short` and protect unrelated user work.
4. Identify:
   - User-visible behavior or API changes.
   - Files and modules likely to change.
   - Existing tests that should guide the work.
   - Risks, migrations, compatibility boundaries, and rollback concerns.

Ask only for missing requirements that cannot be inferred safely from the repo or the user's request.

## 2. Write The Implementation Plan

Create a detailed plan before editing code, tests, docs, config, or other repository files, unless the user only asked for the plan. The plan may live in the conversation unless the user asks for a file.

The plan must include:

- Goal and non-goals.
- Current system observations with file references.
- Proposed implementation steps in dependency order.
- A `$tdd` section with the first failing tests or test updates to write, expected red state, implementation path, and refactor pass.
- A `$review-loop` section requiring at least `2` implementation critique/revision loops, review criteria, quality gate, and how findings will be addressed.
- Verification commands and expected evidence.
- Subagent handoff package: exact scope, constraints, files to inspect, plan steps to execute, acceptance criteria, and reporting format.
- Risks and explicit stop conditions.

Keep the plan concrete enough that another agent can execute it without re-discovering the entire problem.

## 3. Review-Loop The Plan

Before dispatching implementation, run at least one review-loop on the plan itself.

For each plan review loop:

1. Give the reviewer the user request, relevant repo observations, and the draft plan.
2. Ask for a score, blocking gaps, missing tests, scope risks, sequencing problems, and clearer acceptance criteria.
3. Revise the plan for actionable findings.
4. Continue until the required minimum loops are complete and the plan meets the quality gate.
5. If the plan remains below the quality gate after `4` loops, or has unresolved blocking findings, stop and ask the user whether to revise scope, accept the risk, or cancel the workflow.

Do not dispatch the worker subagent until the plan has completed at least one review loop and meets the quality gate. If the user wants to proceed below the gate, treat that as opting out of this workflow and state that clearly.

## 4. Dispatch The Worker Subagent

Use the available multi-agent/subagent tooling. If subagent tools are not visible, search for them with `tool_search` using a query such as `subagent` or `multi-agent`. If no safe subagent mechanism is available, ask whether the main agent should execute the reviewed plan instead, and do not claim delegation happened. Main-agent execution must still honor the same TDD, verification, and minimum `2` implementation review-loop requirements.

Give the worker:

- The reviewed implementation plan.
- The `$tdd` requirement: create or update tests first, observe the expected failure, then implement and refactor.
- The `$review-loop` implementation requirement: run at least `2` critique/revision loops and meet the quality gate, default `8/10`, unless the user sets a stricter gate.
- The exact verification commands to run.
- Guardrails to avoid reverting unrelated work or widening scope.
- Boundaries: allowed files or modules, forbidden scope expansion, credential-dependent commands to avoid, and production/user configuration that must not be touched.
- Stop-and-report triggers: unclear requirements, failing checks that cannot be resolved locally, missing credentials, unsafe migrations or data access, merge conflicts, or required edits outside the assigned scope.
- A request to report changed files, tests added, review-loop score/findings, verification results, and unresolved risks.

The worker may implement and iterate, but the main agent remains responsible for final validation.

## 5. Inspect And Integrate

After the worker returns:

1. Read the worker's summary, diff, and verification output.
2. Inspect the actual changes directly; do not rely only on the worker report.
3. Run or re-run the most relevant checks in the main context, or explicitly report why each relevant check was skipped.
4. Address any remaining review findings, failed checks, or integration issues.
5. If non-trivial code or test fixes are needed after the worker returns, run another implementation review-loop before finalizing. For mechanical changes, report why another loop was not needed.

## 6. Report

Summarize:

- Plan review-loop score and key revisions.
- Worker subagent outcome.
- TDD evidence: failing tests introduced or updated, implementation, and passing result.
- Implementation review-loop score and any fixes made.
- Verification commands and results.
- Changed files and any residual risks.

## Guardrails

- Never skip the plan review loop unless the user explicitly cancels this skill's workflow.
- Never give a worker vague authority to redesign unrelated parts of the repo.
- Never claim TDD was used without naming the tests and red/green evidence.
- Never hide failed checks, unresolved review findings, or skipped verification.
- Do not let tests or smoke runs touch real user configuration or production services unless explicitly configured.
