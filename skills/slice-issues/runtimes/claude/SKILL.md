---
name: slice-issues
description: Break an issue or work item into independently-grabbable sub-issues using tracer-bullet vertical slices. Use when user wants to slice an issue, create implementation tickets, or break down an issue into work items.
---

# Slice Issues

Break a large issue into independently-grabbable sub-issues using vertical slices (tracer bullets).

## Process

- Use the project's issue tracker, or ask the user if one is not given. 
- Issue tracker is not mandatory, local files can be used instead.
- Slices should be linked to a parent issue.
- If you have not already explored the codebase, do so to understand the current state of the code.
- Draft vertical slices: Break the issue into **tracer bullet** sub-issues. Each issue is a thin vertical slice that cuts through ALL integration layers end-to-end, NOT a horizontal slice of one layer.
- Slices may be 'HITL' or: AFK'. HITL slices require human interaction, such as an architectural decision or a design review. AFK slices can be implemented and merged without human interaction. Prefer AFK over HITL where possible.
  - Each slice delivers a narrow but COMPLETE path through every layer (schema, API, UI, tests)
  - A completed slice is demoable or verifiable on its own
  - Prefer many thin slices over few thick ones
- Present the proposed breakdown as a numbered list. For each slice, show:
  - **Title**: short descriptive name
  - **Type**: HITL / AFK
  - **Blocked by**: which other slices (if any) must complete first
  - **User stories covered**: which user stories from the issue this addresses. Infer a set of user stories if one does not exist in the issue.

- Ask the user:
  - Does the granularity feel right? (too coarse / too fine)
  - Are the dependency relationships correct?
  - Should any slices be merged or split further?
  - Are the correct slices marked as HITL and AFK?
- Iterate until the user approves the breakdown.
- Create the sub-issues: For each approved slice, create a sub-issue in the chosen tracker using the body template below, preserving the approved HITL/AFK type.
- Create issues in dependency order (blockers first) so you can reference real issue identifiers in the "Blocked by" field.

<issue-template>
## Parent

Reference to the parent issue (e.g. `#<issue-number>`, an ID, or a filename).

## Type

HITL or AFK. Use the exact classification approved in step 5.

## What to build

A concise description of this vertical slice. Describe the end-to-end behavior, not layer-by-layer implementation. Reference specific sections of the parent issue rather than duplicating content.

## Acceptance criteria

- [ ] Criterion 1
- [ ] Criterion 2
- [ ] Criterion 3

## Blocked by

- Blocked by <issue-reference> (if any)

Or "None - can start immediately" if no blockers.

## User stories addressed

Reference by number from the parent issue:

- User story 3
- User story 7

</issue-template>

Do NOT close or modify the parent issue.
