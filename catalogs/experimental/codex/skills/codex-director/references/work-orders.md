# Work orders

Director owns intake, assignment, and completion. Keep small questions out of
the work ledger when they can be answered directly. Record substantive work
before delegating it.

## Assignment contract

Give the worker a self-contained order containing:

```text
Director ID and reporting destination:
Work ID and assignment ID:
Project: repository, checkout, saved Codex project ID when available
Source-control host and base branch, verified when relevant:
Kind: investigate | plan | implement
Outcome:
Acceptance criteria:
Constraints and excluded work:
Authorized actions and the source of that authorization:
Relevant evidence, issue links, prior findings, and decisions:
Dependencies or prerequisites, if any:
Required result evidence:
```

Include the complete worker charter from `worker.md`. A link alone is not
enough when the worker cannot access the skill. Send decisions and clarifications
against the same work and assignment IDs.

Investigations return findings and evidence. Plans return the requested plan
and unresolved decisions. Neither authorizes tracked repository changes.
Implementation orders name the behavior to change and required validation.
Publication and merge authority must be explicit when those are requested.

## Dispatch and follow-up

Keep one active assignment per work item. Split an outcome only when the parts
have independently assessable results; record separate work items with their
own acceptance criteria. Record prerequisite work IDs in the order's context,
and dispatch dependent work only after verifying those prerequisites. Do not
split merely to keep workers busy.

Reserve an assignment in SQLite before creating a worker, and include its ID
in the initial prompt. Record the returned worker ID immediately. Follow the
recovery procedure in `state.md` if creation or recording is interrupted.

For persistent Codex tasks, inspect saved projects before choosing the project
and use a worktree for Git work unless the user requested the saved checkout.
Use the work ID in the title to aid recovery. Resolve a pending task creation
to its real task ID before sending follow-ups that require one. Do not treat a
client creation ID as a task ID.

Continue the existing worker for clarification, corrections, and authorized
implementation after investigation. Update the same work item, preserve prior
findings in its context and event history, and record the authorization before
dispatching the next stage. Do not create a new work item merely to change kind.

## Result contract

Require a final reply even for blocked, empty, or no-change outcomes. It must
identify the work and assignment, explain the result against acceptance
criteria, list evidence and validation with limitations, and name remaining
decisions or blockers. Include report paths, branch, commit, or PR references
when applicable. Store useful findings in SQLite or durable referenced files;
a temporary scratch path alone is not a durable result.

Director verifies the result before marking work done. If verification reveals
missing work within the existing order, return it to the assigned worker.
If meeting the outcome would exceed authorization, record the blocker and
bring the specific decision to the user. A worker that stops without evidence
has not completed the order.
