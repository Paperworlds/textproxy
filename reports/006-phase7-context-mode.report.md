# 006 — Phase 7: Context Mode vs Cost Mode

## Summary

Implemented two rendering modes for stats and statusline output:
- **`context` mode** (default): frames everything around context window utilisation — no dollar amounts
- **`cost` mode**: previous behaviour — dollar amounts, no window metrics

## What was built

### Config (`internal/config/config.go`)
- Added `Mode string` and `ContextWindows map[string]int64` fields to `Config`
- Default: `mode = "context"`, windows = 200,000 for all known models
- `Load()` now merges these fields from the config file
- `CTX_MODE=cost|context` env var overrides the config value
- Added `ContextWindow(model string) int64` method — falls back to 200,000 for unknown models

### Stats rendering (`internal/cli/stats.go`)
- `CmdStats` branches on `cfg.Mode`:
  - **context**: shows `N× windows`, `Context ratio X.X:1 (in:out)`, spikes with `(XX% of window)`, no `$`
  - **cost**: unchanged previous output with `~$X.XX` per line
- `CmdStatusline` branches on `cfg.Mode`:
  - **context**: `⬡ 284k in · 1.4w · 15.6:1`
  - **cost**: `⬡ 284k in · 18k out · $1.13` (unchanged)
- Added `FmtWindows(tokens, windowSize int64) string` — formats `1.4×`, minimum `0.1×`
- Added `FmtWindowPct(tokens, windowSize int64) string` — formats `41%`

### Tests
- `TestFmtWindows` — edge cases: 0 tokens, exactly 1 window, >10 windows, zero window size
- `TestFmtWindowPct` — 0%, 100%, 150%, typical case
- `TestContextModeStats` — verifies `×`, `%`, `Context ratio` present; `$` absent
- `TestCostModeStats` — verifies `$` present; `×` absent
- `TestContextModeStatusline` — verifies `w` suffix and `:1` ratio; `$` absent
- `TestCostModeStatusline` — verifies `$` present; `:1` absent
- Updated `TestStatsOutput` and `TestStatuslineCmd` to explicitly set `mode = "cost"` (they tested cost-mode strings)

## Test results

```
go test ./... → 36 passed in 5 packages
```

## Decisions

- Mode selection happens at render time (not record time) — raw token counts are stored the same way regardless of mode; both modes read the same `session.json` and `history.jsonl`.
- The statusline uses `1.4w` (with `w` suffix) instead of `1.4×` for compact display, computed inline rather than via `FmtWindows` which uses `×`.
- `CmdSessions` still shows cost regardless of mode — the spec only mentioned `stats` and `statusline`.
- `FmtWindows` enforces a 0.1× minimum display value (as spec'd) but `FmtWindowPct` has no minimum (0% is valid and meaningful).

## Commits

- `8d480bd` feat: add mode + context_windows config fields + CTX_MODE env var (phase 7)
- `0d4637d` feat: context mode rendering + tests (phase 7)
