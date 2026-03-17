---
name: skill-location-preference
description: User prefers skills to be placed in the project directory (.claude/skills/) rather than the global ~/.claude/skills/
type: feedback
---

Place custom skills in the project's `.claude/skills/` directory, not the global `~/.claude/skills/`.

**Why:** User explicitly corrected when the global path was used.

**How to apply:** When creating skills for a project, always use `{project_root}/.claude/skills/{skill-name}/SKILL.md`.
