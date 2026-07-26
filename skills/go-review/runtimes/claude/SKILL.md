---
name: go-review
description: "Run a read-only Go code review across structure, error handling, style, and security. Use when the user asks for a Go code review, a cleanup review of Go source, or invokes /go-review with an optional path and comma-separated focus list. Examples - /go-review, /go-review ./cmd/server, /go-review . security, /go-review ./pkg error,style"
argument-hint: "[path] [focus: structure|error|style|security]"
disallowed-tools: Edit, Write, NotebookEdit
---

# Go Review

Run a read-only Go code review for production source files and produce one prioritized report.

Announce at start: "I'm using the go-review skill to review this Go code."

Run this skill inline as the orchestrator. Do not fork the whole skill into a subagent — a forked orchestrator cannot check in with the user, and it forces four reviewer reports through a single return value. Dispatch only the leaf roles below.

## Roles And Dispatch

| Role | How to dispatch |
|---|---|
| Orchestrator (you) | Never delegated — owns scoping, file enumeration, consolidation, and the report |
| `structure-reviewer` | One agent; prompt from [roles/structure-reviewer.md](roles/structure-reviewer.md) |
| `error-reviewer` | One agent; prompt from [roles/error-reviewer.md](roles/error-reviewer.md) |
| `style-reviewer` | One agent; prompt from [roles/style-reviewer.md](roles/style-reviewer.md) |
| `security-reviewer` | One agent; prompt from [roles/security-reviewer.md](roles/security-reviewer.md) |

Roles are leaf workers — they must not spawn further agents.

## Hard Constraints

<HARD-GATE>
This skill is READ-ONLY. Read and search the repository. Do not change anything.

Never modify files. Do not edit, create, or delete files — not with an editor
tool, and not with shell commands.

Never mutate git state. No `git add`, `git commit`, `git push`, or any other
repository-mutating command.

Never apply a fix. The deliverable is a report presented in chat.

No exceptions. If you catch yourself about to run a write operation, stop.
</HARD-GATE>

Carry this gate into every role prompt — the role files each restate it, so dispatch them verbatim rather than paraphrasing.

## Scope

Read `$ARGUMENTS` to set scope:

- **path**: optional directory or file. Default to `.`.
- **focus**: optional comma-separated reviewer list. Valid values are `structure`, `error`, `style`, and `security`. Default to all four.

Reject unknown focus values with a concise usage note instead of guessing.

## Step 1: Enumerate Files

Find non-test Go files under the scoped path. Prefer `rg --files <path>` filtered to `.go` and excluding `*_test.go`; fall back to `Glob` with `**/*.go` if `rg` is unavailable. Test files are out of scope.

If no production Go files are found, report that and stop.

If the file list is very large (roughly 150+ files), tell the user the count and offer to narrow the path before spending four full reviewer passes on it.

## Step 2: Dispatch Reviewers

Standard mode is the default: launch the selected reviewers in a single message so they run concurrently.

Workflow mode is opt-in only, when the user asks for a thorough, comprehensive, or deep review, or says "workflow mode". Be honest about cost before starting: it spawns roughly 8-15 agents on a normal repo. Structure it as a `pipeline()` — one `agent()` per selected role returning schema-validated findings from [findings-schema.md](findings-schema.md), each role's findings flowing straight into adversarial verification without waiting for the other roles. Verifiers are prompted to **refute**; a finding that survives keeps its severity, one that does not is downgraded with the doubt recorded rather than dropped. Verify only findings marked `needs_verification` — re-litigating a missing `defer Close()` wastes a turn.

Every verifier prompt must include this self-contained gate verbatim:

```text
<HARD-GATE>
This is a read-only verification pass.
- Do not edit, create, delete, rename, or format files.
- Shell and git commands must be read-only.
- Do not mutate git state, branches, commits, or the working tree.
- Do not mutate GitHub or the pull request.
</HARD-GATE>
```

Workflow agents are not persistent, so a follow-up deep-dive after workflow mode spawns a fresh focused pass rather than continuing a reviewer.

In both modes, build each prompt from the matching role file with the `[REVIEW CONTEXT]` block filled in:

```
[REVIEW CONTEXT]
- Repo root: <absolute path>
- Scope path: <path or "." if not specified>
- Files to review: <the enumerated non-test .go files>
```

Reviewers inherit the session model by default. Pass a cheaper model per reviewer only if the user asks to economize — the role files assume a careful read of every assigned file.

Keep only findings in main context. Do not paste raw reviewer transcripts into the report.

## Step 3: Consolidate

Once every dispatched reviewer has returned:

1. Deduplicate. The same issue is often flagged by two reviewers — merge into one finding and note the agreement, which raises its weight.
2. Map each finding onto a priority tier.
3. Drop noise. Prefer fewer high-signal items over exhaustive lists.
4. Keep concrete `file/path.go:LINE` references. A finding without a location is not actionable — either locate it or drop it.

### Priority Tiers

- **P0 (Bug risk):** runtime failures, exploitable vulnerabilities, data races, or silent data loss.
- **P1 (Robustness):** missing error checks, resource leaks, context propagation gaps, defensive hardening, or medium-risk security findings.
- **P2 (Maintainability):** duplication, large functions, unclear abstractions, package coupling, meaningful simplification.
- **P3 (Style):** naming, idioms, comments, documentation, low-risk cleanup.

## Step 4: Report

Output one numbered list grouped by priority tier:

```
N. file/path.go:LINE — [Category]
   Description of the issue.
   Suggested fix: concrete recommendation.
```

If a dispatched reviewer produced no findings, say so in one line rather than omitting it — a silent omission reads as "clean" when it may mean "did not run".

Offer to deep-dive on any finding. Do not apply fixes unless the user explicitly asks in a later turn, and treat that as leaving this skill.

## Common Mistakes

| Mistake | Fix |
|---------|-----|
| Forking the whole skill into a subagent | Run inline; dispatch only leaf roles. |
| Reviewing `*_test.go` files | Filter them out during enumeration. |
| Dispatching reviewers sequentially | Send them in one message so they run concurrently. |
| Reaching for workflow mode unprompted | It is opt-in; state the agent cost before starting. |
| Findings without file:line | Locate it or drop it. |
| Pasting whole reviewer reports into the output | Consolidate and deduplicate; keep conclusions. |
| Applying a fix because it looked trivial | The skill is read-only. Report it. |
| Silently dropping a reviewer that found nothing | Say "no findings" explicitly. |
| Dropping a finding a verifier could not confirm | Downgrade it and record the doubt. |

## Red Flags

- You are about to run a command that modifies a file or git state.
- You are about to suggest a fix by making it instead of describing it.
- Your report has findings with no line references.
- You reviewed files the user did not scope you to.
