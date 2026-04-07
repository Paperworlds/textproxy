---
id: "006"
title: "Phase 7 — context mode vs cost mode"
phase: "phase-7"
repo: "claude-context-proxy"
model: "sonnet"
depends_on: ["005"]
budget_usd: 1.50
---

# 006 — Phase 7: Context Mode vs Cost Mode

## IMPORTANT: Progress Logging
Before doing ANYTHING else, create the progress log file. After EVERY step, append a line:
```bash
echo "[$(date -u +%Y-%m-%dT%H:%M:%SZ)] STEP N: description — PASS/FAIL" >> /Users/projects/personal/claude-context-proxy/logs/006-phase7-context-mode.progress.log
```

## Context
The proxy currently shows dollar cost estimates, which is only useful for API users
paying per token. Users on a Claude subscription (Claude Code, Claude.ai) don't pay
per token — for them, the meaningful metric is **context window consumption**.

Read ALL of `main.go` and `internal/` before starting.

## Two modes

### `context` mode (default)
Frames everything around context window utilisation. No dollar amounts.

`claude-context-proxy stats` output:
```
Session: 2026-04-07 10:32 (47m)
─────────────────────────────────────
Requests:        38
Input tokens:   284,391  (1.4× windows)
Output tokens:   18,204
Context ratio:   15.6:1  (in:out)
─────────────────────────────────────
Top context spikes (last 10 req):
  req #3   82,341 tokens  (41% of window)
  req #12  61,204 tokens  (31% of window)
```

- **`N× windows`** — cumulative input tokens ÷ context window size for the model
- **`Context ratio`** — input:output ratio, useful for understanding how much re-sending of context is happening vs actual output
- **Spikes** shown as `% of window` instead of raw tokens

`claude-context-proxy statusline` output:
```
⬡ 284k in · 1.4w · 15.6:1
```
- `1.4w` = 1.4 context windows consumed
- `15.6:1` = context ratio

### `cost` mode
Current behaviour, unchanged. Dollar amounts shown, no window metrics.

`stats` output remains exactly as today.
`statusline` output remains exactly as today.

## Config changes

### `config.json`
Add two new fields:

```json
{
  "mode": "context",
  "context_windows": {
    "claude-sonnet-4":  200000,
    "claude-haiku-4":   200000,
    "claude-opus-4":    200000
  }
}
```

- `mode`: `"context"` (default) or `"cost"`
- `context_windows`: tokens per model; used to calculate `N× windows` and `% of window`
- Unknown models fall back to `200000`

### Env var override
`CTX_MODE=cost` or `CTX_MODE=context` overrides the config value.

### `config` subcommand
Should show the new fields in its output.

## Implementation notes

- Mode selection should happen at render time (in `internal/cli/stats.go`), not at record time — the underlying data (raw token counts) is model-agnostic. Both modes read the same `session.json` and `history.jsonl`.
- Add `contextWindow(model string) int64` helper in `internal/config/config.go`
- `fmtWindows(tokens, windowSize int64) string` — formats `1.4×` (1 decimal place; if < 0.1 show `0.1×` as minimum)
- `fmtWindowPct(tokens, windowSize int64) string` — formats `41%`

## Tests to add
- `TestContextModeStats` — known session in context mode; verify `×` and `%` strings present, no `$` present
- `TestCostModeStats` — known session in cost mode; verify `$` present, no `×` present
- `TestFmtWindows` — edge cases: 0 tokens, exactly 1 window, >10 windows
- `TestFmtWindowPct` — 0%, 100%, >100%
- `TestContextModeStatusline` — verify `w` and `:1` suffix, no `$`
- `TestCostModeStatusline` — verify `$` present

## Verification checklist
- [ ] `go build ./...` succeeds
- [ ] `go test ./...` passes
- [ ] Default (`mode: context`): `stats` shows `×` and `%`, no `$`
- [ ] `CTX_MODE=cost claude-context-proxy stats` shows `$`, no `×`
- [ ] `claude-context-proxy config` shows `mode` and `context_windows` fields
- [ ] Statusline compact output reflects mode correctly

## Commit instructions
- Commit after config changes (new fields, env var)
- Commit after context mode rendering + tests
- Push when done

## Report
Write `reports/006-phase7-context-mode.report.md` with what was built, decisions, test results.
