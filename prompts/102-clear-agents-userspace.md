---
id: "102"
title: "Clear agents from user space"
phase: "phase-2"
repo: "skills"
model: "sonnet"
depends_on: ["101"]
budget_usd: 0.50
---

# Phase 2 — Clear Agents from User Space

Remove all agent files from Claude's loaded agents directories and update
`install.sh` so they are no longer copied on reinstall.

## Context

Agents are currently copied to two locations at install time:
- `~/.claude/agents/`       (default + personal profile)
- `~/.claude-work/agents/`  (work profile)

This causes all 9 agents to be pre-loaded into every session.
After this phase, those dirs will be empty by default.
Agents are enabled on-demand via `jg custom-agents enable` (Phase 3).

## Tasks

1. Remove all `.md` files from both agents dirs:
   ```bash
   rm ~/.claude/agents/*.md
   rm ~/.claude-work/agents/*.md
   ```

2. In `~/projects/personal/skills/locals/install.sh`, find the section that
   copies or symlinks agents into `~/.claude*/agents/` and remove it.
   The agents dirs should be left alone by install.sh entirely.

3. Verify both dirs are empty:
   ```bash
   ls ~/.claude/agents/
   ls ~/.claude-work/agents/
   ```

4. Commit the install.sh change to the skills repo.

## Important

Do NOT delete the source files in `~/projects/personal/skills/claude-code/agents/`.
Those are the source of truth and stay in the repo.
