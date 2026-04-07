# 002 — Phase 3: Fish Statusline Integration

## Summary

All scope items implemented, all tests passing, committed and pushed.

## What was built

### Statusline state file writer (`writeStatusline`)

- Writes `~/.files/states/ctx.json` after every `saveSession` call in `recordTokens`
- Atomic write: temp file `.ctx.json.tmp` → `os.Rename` → `ctx.json`
- Creates `~/.files/states/` directory if missing; logs warning on failure, never crashes
- Schema: `input_tokens`, `output_tokens`, `requests`, `cost_usd`, `session_id`, `updated_at`

### Configurable path

- Default: `~/.files/states/ctx.json`
- Override: `CTX_STATUSLINE_PATH=/path/to/file.json`
- Disable: `CTX_STATUSLINE_PATH=""` — skips write entirely, no crash

### `statusline` subcommand

```
claude-context-proxy statusline
```

Prints compact one-liner suitable for fish prompt:
```
⬡ 284k in · 18k out · $1.13
```

- Numbers ≥ 1,000,000 → `NM` (rounded); ≥ 1,000 → `Nk` (rounded)
- Silent (exit 0) if file missing or `updated_at` > 35 min ago
- `--json` flag prints raw `ctx.json` for scripting

## Decisions

- `fmtCompact` uses integer rounding (`(n+500)/1_000`) so 1499 → `1k`, 1500 → `2k`
- `cost_usd` stored as raw float64 (no rounding in JSON); `cmdStatusline` formats with `$%.2f`
- `os.LookupEnv` used (not `os.Getenv`) to distinguish unset from empty string for the path

## Test results

All 17 tests pass (`go test ./...`):

| Test | Result |
|------|--------|
| TestTokenHeaderExtraction | PASS |
| TestStreamingPassthrough | PASS |
| TestSessionJSONWritten | PASS |
| TestStatsOutput | PASS |
| TestFmtInt64 | PASS |
| TestSessionID | PASS |
| TestSessionsCmd | PASS |
| TestHistoryFilter | PASS |
| TestOldHistoryNoSessionID | PASS |
| TestStatuslineWrite | PASS |
| TestStatuslineAtomic | PASS |
| TestStatuslineCmd | PASS |
| TestStatuslineCmdJSON | PASS |
| TestStatuslineDisabled | PASS |
| TestStatuslineStale | PASS |
| TestFmtCompact | PASS |
| TestSessionGapReset | PASS |

## Commits

- `f677abf` feat: add statusline state file writer
- `01b2543` feat: add statusline subcommand + tests

## Files changed

- `main.go` — added `StatuslineState`, `statuslinePath`, `writeStatusline`, `fmtCompact`, `cmdStatusline`; wired into `recordTokens` and `main`
- `main_test.go` — added 7 new tests for Phase 3 functionality
