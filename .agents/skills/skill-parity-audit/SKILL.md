---
name: skill-parity-audit
description: Audit and maintain semantic parity between the Claude and Codex runtime forks of every first-party skill in this repository. Use when adding or changing a first-party skill, checking whether runtime-specific instructions expose the same workflow, or repairing drift under skills/*/runtimes/{claude,codex}.
---

# Skill Parity Audit

Ensure every first-party skill offers the same user-facing capability in its
Claude and Codex runtime forks while allowing runtime-native implementation
details.

## Parity Contract

Require each directory under `skills/` to contain:

- `shared/`
- `runtimes/claude/SKILL.md`
- `runtimes/codex/SKILL.md`

Require both runtime `SKILL.md` files to:

- Declare the directory name as `name`.
- Use the same `description` so both runtimes trigger for the same requests.
- Preserve the same user intent, scope, prerequisites, safety constraints,
  authority boundaries, workflow phases, checkpoints, outputs, validation,
  failure behavior, and composed-skill dependencies.

Allow differences that adapt the same behavior to runtime-native facilities:

- Tool and connector names.
- Skill invocation syntax and argument handling.
- Subagent dispatch and fallback mechanics.
- UI metadata such as Codex `agents/openai.yaml`.
- Runtime-only support files that implement an equivalent capability.
- Claude-only frontmatter fields ignored by Codex.

Do not require literal line parity when the runtimes need different mechanics.
Do not accept a shared title or outline as proof of semantic parity.

## Workflow

1. Run the deterministic repository audit:

   ```bash
   audit_dir="$(mktemp -d)"
   python3 .agents/skills/skill-parity-audit/scripts/audit_runtime_forks.py \
     --json-out "$audit_dir/runtime-fork-parity.json" \
     --markdown-out "$audit_dir/runtime-fork-parity.md"
   ```

2. Read both reports.
   - Treat `error` as blocking structural or trigger-metadata drift.
   - Treat `review` as a required semantic comparison, not a failure.
   - Treat `pass` as exact overlay parity.
3. For every skill marked `review`, read both complete `SKILL.md` files and
   every runtime-only file reported by the script. Compare the pair against
   every item in the parity contract.
4. Classify each difference as:
   - `equivalent adaptation`: different mechanism, same contract.
   - `parity gap`: a capability, constraint, checkpoint, output, or failure
     path exists in only one runtime.
   - `shared-source candidate`: runtime-neutral content is duplicated in the
     overlays and belongs under `shared/`.
5. When the user asked only for an audit, report gaps with exact paths and
   concise evidence. Do not modify files.
6. When the user asked to create, update, or restore parity:
   - Repair blocking errors first.
   - Mirror each semantic change in both runtime forks during the same edit.
   - Express equivalent behavior with each runtime's native tools.
   - Move genuinely runtime-neutral support material into `shared/`.
   - Preserve intentional runtime-only metadata and support files.
7. Rerun the deterministic audit. Then rerun:

   ```bash
   scripts/test-skill-parity-audit.py
   scripts/test-forked-skills-layout.sh
   scripts/test-forked-skills-install.sh
   ```

Do not declare parity until the deterministic audit has no errors and every
reported review item has been inspected against the semantic contract.

## Reporting

For each parity gap, include:

- Skill and exact paths.
- Missing or divergent behavior.
- Why the difference is not merely runtime adaptation.
- The smallest paired change that restores parity.

End with counts for skills checked, blocking errors, semantic reviews, parity
gaps, and accepted runtime adaptations.
