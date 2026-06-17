---
name: skill-parity-audit
description: Compare two local skill directories for parity, including missing skills, shared skill drift, broken symlinks, non-skill entries, and support files. Use when auditing Claude/Codex/agents skill folders, planning migration between skill roots, or checking whether two AI skill installations expose equivalent workflows.
---

# Skill Parity Audit

Audit two skill roots and report what must change for parity.

## Workflow

1. Identify the roots to compare.
   - Default Claude root: `~/.claude/skills`
   - Default agents root: `~/.agents/skills`
   - Use explicit paths if the user names different skill directories.
2. Run the bundled script:

   ```bash
   python3 ~/.agents/skills/skill-parity-audit/scripts/audit_skill_parity.py \
     ~/.claude/skills ~/.agents/skills \
     --left-label Claude --right-label Agents \
     --markdown-out outputs/skill-parity-audit.md \
     --json-out outputs/skill-parity-audit.json
   ```

3. Read the Markdown report and summarize:
   - Usable skills in each root
   - Shared identical skills
   - Shared drifted skills
   - Skills missing from either side
   - Broken symlinks or entries with no `SKILL.md`
   - High-risk migrations, especially skills that reference platform-specific tools or subagents
4. If the user asks to fix parity, make scoped changes:
   - Prefer a canonical source for portable skills and symlink the other root to it.
   - Keep platform-specific skills separate when their instructions name platform-specific tools.
   - Back up replaced folders before converting them to symlinks.

## Compatibility Notes

Treat a skill as portable only when its `SKILL.md` and support files do not depend on unavailable platform primitives. Use the script's platform-specific token hints to find instructions that likely require adaptation for Codex.

For agent-team skills, audit both the skill folder and the referenced agent definitions. A launcher `SKILL.md` without its subagents is not parity.
