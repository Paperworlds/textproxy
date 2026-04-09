---
id: "103"
title: "Add jg custom-agents commands"
phase: "phase-3"
repo: "skills"
model: "sonnet"
depends_on: ["102"]
budget_usd: 1.00
---

# Phase 3 — jg custom-agents Commands

Add `custom-agents` recipes to `~/projects/personal/skills/locals/justx/claude.just`
for on-demand agent management.

## Commands to implement

```
jg custom-agents list              # show enabled (symlinked) vs available (source only)
jg custom-agents enable <name>     # symlink agent into both profile agents dirs
jg custom-agents disable <name>    # remove symlinks from both dirs
jg custom-agents disable --all     # remove all symlinks (clean slate)
```

## Source and target paths

- Source: `~/projects/personal/skills/claude-code/agents/<name>.md`
- Target 1: `~/.claude/agents/<name>.md`
- Target 2: `~/.claude-work/agents/<name>.md`

Enable = symlink source into both targets (use `ln -sf`).
Disable = remove both symlinks (use `rm -f`).

## list output format

```
ENABLED:
  pr-reviewer       ~/.claude/agents/pr-reviewer.md -> ...
  snowflake-cost    ~/.claude/agents/snowflake-cost.md -> ...

AVAILABLE (not enabled):
  aws-support
  daily-breach-news
  go-vuln-fix
  mcp-manager
  metabase-cost
  shr-upgrade
  wrap-it-up
```

## Implementation notes

- Use fish shell (`set shell := ["fish", "-c"]` — already set in claude.just)
- Add as a new recipe block at the bottom of claude.just under a `# Agent management` comment
- Use `custom-agents` as the recipe name with subcommands via args
- Name resolution: user passes `pr-reviewer`, not `pr-reviewer.md`
- `list` needs no args; `enable`/`disable` require exactly one arg (or `--all` for disable)
- If agent source file doesn't exist, print error and exit 1

## Verification

```bash
jg custom-agents list              # should show all as available
jg custom-agents enable pr-reviewer
ls -la ~/.claude/agents/           # should show symlink for pr-reviewer only
jg custom-agents list              # pr-reviewer shows under ENABLED
jg custom-agents disable pr-reviewer
ls ~/.claude/agents/               # empty again
```
