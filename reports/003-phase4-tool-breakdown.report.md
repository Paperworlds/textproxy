# 003 — Phase 4: Token Breakdown by Tool Call

## What Was Built

### 1. SSE Tee-Parser (`sseInspector`)

A new `sseInspector` type wraps an `io.Reader` and extracts tool names from the Anthropic SSE stream inline as bytes pass through. It implements `io.Reader` so the SSE streaming loop in `proxyHandler` is unchanged from the caller's perspective.

**How it works:**
- `Read(p)` calls the underlying reader, then calls `ingest(chunk)` on the bytes just read
- `ingest` accumulates bytes in an internal buffer and scans for `\n\n` SSE event boundaries
- When a complete event is found, `parseEvent` splits it into lines and looks for `data: ` lines
- `content_block_start` events with `content_block.type == "tool_use"` yield a tool name appended to `inspector.Tools`
- The buffer is compacted after each parsed event — memory stays small

### 2. Zero-Overhead Default Path

`CTX_INSPECT=1` is checked once at startup and stored in a global `inspectEnabled` bool. In `proxyHandler`, the inspector is only instantiated when `inspectEnabled && isSSE`:

```go
var inspector *sseInspector
if isSSE {
    bodyReader := io.Reader(resp.Body)
    if inspectEnabled {
        inspector = newSSEInspector(resp.Body)
        bodyReader = inspector
    }
    // SSE loop uses bodyReader — unchanged when inspector is nil
}
```

When `CTX_INSPECT=0` (default): `inspector` is nil, `bodyReader` aliases `resp.Body` directly, and no extra allocations occur per request.

### 3. `HistoryEntry.Tools []string`

Added `Tools []string \`json:"tools,omitempty"\`` to `HistoryEntry`. The field is only written when non-nil (i.e., when `CTX_INSPECT=1` and at least one tool was called). Old entries without the field parse fine — backward compatible.

### 4. `stats --tools`

`cmdStats` now accepts `args []string` and parses a `--tools` flag. When set, it calls `printToolBreakdown` which:
- Reads history, filters to the current session
- Counts tool call frequencies
- Prints sorted descending by count (ties broken alphabetically)

```
Tool call breakdown (current session):
  Bash       34 calls
  Read       28 calls
  Glob       11 calls
```

### 5. Statusline Tool Suffix

`cmdStatusline` now reads history for the current session (matched by `state.SessionID`) and computes the most-called tool. If found, appends ` · Tool×N` to the output:

```
⬡ 284k in · 18k out · $1.13 · Bash×34
```

Falls back to the existing format when no tool data is available.

## Test Results

```
ok  github.com/pdonorio/claude-context-proxy  0.882s
```

All 21 tests pass including 5 new Phase 4 tests:

| Test | What it verifies |
|------|-----------------|
| `TestSSETeeParser` | Full SSE stream: tool names extracted, byte output identical to input |
| `TestSSETeeParserChunked` | SSE event split across 10-byte Read chunks — parser reassembles correctly |
| `TestHistoryToolField` | Tools written to history.jsonl and parsed round-trip |
| `TestHistoryToolFieldNil` | nil tools → no `"tools"` key in JSON (omitempty) |
| `TestStatsToolsFlag` | Known history with tools → frequency table with correct counts and ordering |

## Design Decisions

**Why not buffer the whole body before parsing?** The proxy's value is zero-latency passthrough. Buffering would block the client until the full response arrives. The tee approach adds negligible overhead (a `bytes.Index` scan per 4096-byte chunk).

**Why `CTX_INSPECT` env var rather than always-on?** SSE parsing adds a per-Read allocation and scan. For users who don't need tool attribution (the majority), this is pure overhead. The env var makes the tradeoff explicit and the default safe.

**Why read history in `cmdStatusline`?** The state file (`ctx.json`) only carries token totals. Adding tool counts there would require changes to `writeStatusline` and the state schema. Reading history on statusline query is cheap (the file is small) and keeps the write path simple.

## Verification

- `go build ./...` — passes
- `go test ./...` — passes (21 tests)
- `CTX_INSPECT=0` (default): `proxyHandler` SSE loop is identical to Phase 3
- `CTX_INSPECT=1`: after a Claude session, `history.jsonl` entries include `"tools":[...]`
- `claude-context-proxy stats --tools` shows tool frequency table
