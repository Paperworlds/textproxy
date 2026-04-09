---
id: "101"
title: "Trim agent .md files"
phase: "phase-1"
repo: "skills"
model: "sonnet"
depends_on: []
budget_usd: 1.00
---

# Phase 1 — Trim Agent Definitions

Cut every agent `.md` file in `~/projects/personal/skills/claude-code/agents/`
from its current verbose form (~1.5k tokens each) down to a compact form (~150 tokens each).

## Goal

All 9 files combined should total under 2k tokens.
Each file must remain functional — Claude still needs enough context to use the agent correctly.

## Format for each trimmed file

```markdown
---
name: <agent-name>
description: <one sentence, max 120 chars>
---

**Use when:** <one line trigger description>
**Tools:** <comma-separated list of key tools/MCPs used>

<2-3 sentence body: what it does, key behaviors, any critical gotchas>
```

## Files to trim

All 9 files in `~/projects/personal/skills/claude-code/agents/`:
- aws-support.md
- daily-breach-news.md
- go-vuln-fix.md
- mcp-manager.md
- metabase-cost.md
- pr-reviewer.md
- shr-upgrade.md
- snowflake-cost.md
- wrap-it-up.md

Read each file fully before trimming. Preserve the agent's actual behavior and tool list.
Do not invent capabilities. When in doubt about what to keep, keep the trigger condition and tools.

## Verification

After trimming, run:
```bash
wc -c ~/projects/personal/skills/claude-code/agents/*.md
```
Each file should be under 600 bytes. Total under 5000 bytes.
