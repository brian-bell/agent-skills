---
name: slice-issues
description: Break an issue or work item into independently-grabbable sub-issues using tracer-bullet vertical slices. Use when user wants to slice an issue, create implementation tickets, or break down an issue into work items.
---

# Slice Issues

Break a large issue into independently-grabbable sub-issues using vertical slices (tracer bullets).

## Process

- Use the project's issue tracker, or ask the user if one is not given. When `bd where` resolves a Beads workspace, Beads is the tracker: follow the Beads route below instead of GitHub issues.
- Issue tracker is not mandatory, local files can be used instead.
- Fetch the parent issue and any linked discussion. Slices should be linked to the parent. In Beads, `bd show <parent-id>` includes the parent's children, blockers, notes, and comments.
- If you have not already explored the codebase, do so to understand the current state of the code.
- Draft vertical slices: Break the issue into **tracer bullet** sub-issues. Each issue is a thin vertical slice that cuts through ALL integration layers end-to-end, NOT a horizontal slice of one layer.
- Slices may be 'HITL' or 'AFK'. HITL slices require human interaction, such as an architectural decision or a design review. AFK slices can be implemented and merged without human interaction. Prefer AFK over HITL where possible.
  - Each slice delivers a narrow but COMPLETE path through every layer (schema, API, UI, tests)
  - A completed slice is demoable or verifiable on its own
  - Prefer many thin slices over few thick ones
- Present the proposed breakdown as a numbered list. For each slice, show:
  - **Title**: short descriptive name
  - **Type**: HITL / AFK
  - **Blocked by**: which other slices (if any) must complete first
- Ask the user:
  - Does the granularity feel right? (too coarse / too fine)
  - Are the dependency relationships correct?
  - Should any slices be merged or split further?
  - Are the correct slices marked as HITL and AFK?
- Iterate until the user approves the breakdown.
- Create the sub-issues: For each approved slice, create a sub-issue in the chosen tracker using the body template below, preserving the approved HITL/AFK type.
- Create issues in dependency order (blockers first) so you can reference real issue identifiers in the "Blocked by" field.
- Set parent/sub-issue and "Blocked by" relationships in the issue tracker if available. Always fill in the "Blocked by" section of the issue body as well, since not every reader sees tracker links. If the tracker has no parent link, reference the parent issue in the "Why" section.

### Beads route

When the tracker is Beads (`bd`), create each approved slice with the CLI rather than hand-writing links:

1. Write the body from the template below to a temporary file.
2. Create the slice under the parent, recording its type as a label:
   `bd create "<title>" --type task --priority <parent's priority> --parent <parent-id> --labels afk --body-file <file>` (use `hitl` for HITL slices).
3. Wire each blocker after both beads exist: `bd dep add <slice-id> --blocked-by <blocker-id>`.
4. Confirm the result with `bd children <parent-id>` and `bd blocked`.

Use bead IDs as the `<issue-reference>` in "Blocked by". Do not run `bd dolt push` unless the user or repository instructions ask for it.

<issue-template>
## Why

A short explanation of the reason for this slice. Describe how it fits into the overall parent issue and what value is provided by this slice.

## What to build

A concise description of this vertical slice. Describe the end-to-end behavior, not layer-by-layer implementation. Reference specific sections of the parent issue rather than duplicating content.

## Type

HITL or AFK. Use the exact classification approved in the breakdown.

## Acceptance criteria

Use as many acceptance criteria as needed to make the slice verifiable.

- [ ] Criterion 1
- [ ] Criterion 2
- [ ] Criterion 3

## Blocked by

- Blocked by <issue-reference> (if any)

Or "None - can start immediately" if no blockers.

</issue-template>

Do NOT close or modify the parent issue.
