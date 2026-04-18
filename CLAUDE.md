# textproxy

Lightweight local MITM proxy between Claude Code and api.anthropic.com.
Reads token usage headers from responses and reports per-session consumption.

## What it does
- Listens on localhost:7474, forwards all traffic to api.anthropic.com
- Extracts token counts from SSE responses (input, output, cache)
- Writes stats to ~/.cache/textproxy/session.json + history.jsonl
- CLI: `textproxy stats` prints a formatted summary

## Usage
```bash
textproxy start
ANTHROPIC_BASE_URL=http://localhost:7474 claude
textproxy stats
```

## Phases
- Phase 1 (000): minimal proxy + stats CLI — Go, stdlib only
- Phase 2 (001): session tracking + `textproxy stats` history query
- Phase 3 (002): statusline integration (fish + .files/states/ctx.json)
- Phase 4 (003): token breakdown by tool call (nice to have)

## Git
GPG signing is disabled for this repo (`commit.gpgsign=false` in `.git/config`).
Do not use `--no-gpg-sign` flag — the config already handles it.

## Language
Go — single binary, no deps, fast startup.

## Forums
Before starting work, run `textforums list --tag textproxy --status open` and check for open threads relevant to your task. Post a thread tagged `textproxy` if you hit a cross-repo blocker.
