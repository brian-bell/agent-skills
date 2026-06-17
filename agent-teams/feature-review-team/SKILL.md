---
name: feature-review
description: Run a feature acceptance review. Usage - /feature-review <PR number or feature name> [focus]. Examples - /feature-review #42, /feature-review scanner, /feature-review #15 safety,quality
user_invocable: true
---

Invoke the `acceptance-lead` agent to coordinate a feature acceptance review.

Pass the user's arguments directly in the agent prompt:

```
Review this feature.
Subject: <PR number or feature name from user args>
Focus: <comma-separated reviewers or "all" if not specified>
```

Use the Agent tool with `subagent_type: "acceptance-lead"`.
