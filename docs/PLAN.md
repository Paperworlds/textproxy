# textproxy — Project Plan

## Purpose

A lightweight local daemon that sits between Claude Code and `api.anthropic.com`.
It captures token usage from every API response and reports context consumption
in real time — without modifying Claude Code's behaviour in any way.

The primary audience is **subscription users** (Claude Code, Claude.ai) who don't
pay per token but want to understand how much context window they're consuming
per session, per request, and per tool call.

---

## Current state (v0.1.0)

All seven planned phases are complete. The binary is a single self-contained Go
executable with no external dependencies.

### What's working

**Proxy daemon**
- HTTP server on `:7474`, forwards all traffic to `api.anthropic.com` verbatim
- Streaming SSE responses forwarded chunk-by-chunk via `http.Flusher` (zero buffering)
- Graceful shutdown: drains in-flight stats goroutines before exit
- Daemon lifecycle: `start` (default), `stop`, `restart`, `log`
- PID file at `~/.cache/textproxy/proxy.pid`
- Log file at `~/.cache/textproxy/proxy.log`

**Stats tracking**
- `~/.cache/textproxy/session.json` — live session state
- `~/.cache/textproxy/history.jsonl` — one line per request
- Sessions auto-reset after configurable inactivity gap (default 30 min)
- Each history entry records: timestamp, input/output tokens, path, model, session ID, tool calls (when inspect mode is on)

**CLI subcommands**
| Command | Description |
|---------|-------------|
| `stats` | Current session — tokens, context windows consumed, ratio |
| `sessions` | All past sessions grouped by session ID |
| `history` | Per-request log with filters: `--last`, `--today`, `--since=DATE` |
| `statusline` | Compact one-liner for fish/shell prompt embedding |
| `config` | Show effective config (merged file + env overrides) |
| `log` | Tail daemon log |
| `version` | Print version |

**Two display modes**
- `context` (default): frames output around context window utilisation
  - `284k in · 1.4w · 15.6:1` — windows consumed, input:output ratio
  - Spikes shown as `% of window`
  - No dollar amounts — designed for subscription users
- `cost`: dollar cost estimates per model (for API users)
- Switch via `CTX_MODE=cost` env var or `"mode"` in config

**Tool attribution (opt-in)**
- `CTX_INSPECT=1` enables inline SSE tee-parsing
- Extracts tool names from `content_block_start` events without buffering
- Annotates history entries with `"tools": ["Read", "Bash", ...]`
- `stats --tools` shows per-tool call frequency for the current session
- Zero overhead when disabled (default)

**Config file** at `~/.config/textproxy/config.json`
- Port, session gap, statusline path, inspect mode, pricing, model context windows
- All env vars (`CTX_PORT`, `CTX_MODE`, `CTX_INSPECT`, etc.) override config

**Statusline integration**
- Writes `~/.files/states/ctx.json` after every request (atomic rename)
- Fish prompt can read this file directly without spawning a subprocess
- `CTX_STATUSLINE_PATH` to override; `=""` to disable

---

## Architecture

```
main.go                        ← entry point, arg dispatch, forwarding wrappers
internal/
  config/config.go             ← Config, Load, Default, EnsureFile
  stats/
    types.go                   ← Session, HistoryEntry, StatuslineState
    session.go                 ← LoadSession, SaveSession, ApplyTokens, PID/log files
    history.go                 ← AppendHistory, ReadHistory
    statusline.go              ← WriteStatusline, CostUSD, ExtractModel
  proxy/handler.go             ← HTTP handler, SSEInspector (tee-parser)
  cli/stats.go                 ← all subcommand renderers
```

**Dependency graph** (no circular imports):
```
main → config
main → stats  → config
main → proxy  → config
main → cli    → stats, config
```

**Design principles**
- Zero overhead on the proxy hot path — stats writes are async goroutines
- Inspect mode is opt-in — SSE parsing adds ~5µs/chunk and is off by default
- Mode selection at render time — raw token counts are model/mode-agnostic
- Stats writes are best-effort — disk errors log and continue, never crash the proxy
- Atomic writes for statusline — `.tmp` + `os.Rename` prevents partial reads

---

## Usage

```bash
# Start proxy (daemonizes by default)
textproxy

# Point Claude Code at it
export ANTHROPIC_BASE_URL=http://localhost:7474

# Check context usage
textproxy stats
textproxy history --last

# Lifecycle
textproxy stop
textproxy restart
textproxy log
```

---

## Roadmap

### Known bugs

~~1. **Token capture broken after cache-token fix**~~ **Fixed (v0.1.1)** — Changed
   `if inputTokens == 0 && outputTokens == 0` fallback to always prefer SSE-parsed totals
   (which include `cache_read_input_tokens` + `cache_creation_input_tokens`) over header
   counts. Headers only report the tiny raw `input_tokens` value; SSE has the full picture.

~~2. **`ctx:12%` statusline source unknown**~~ **Not a bug** — `ctx:N%` is produced by
   `~/.claude/statusline-command.sh`, Claude Code's own statusline hook. It reads
   `.context_window.used_percentage` from Claude Code's native telemetry. Unrelated to the
   proxy's `statusline` command or `ctx.json`.

~~3. **Stale session after daemon restart**~~ **Fixed by #1** — Was downstream of the token
   capture bug; `recordTokens` now fires correctly.

### Near term
- **Live proxy routing debug** — `textproxy status` command to check whether
  proxy is reachable and `HTTPS_PROXY` / `NODE_EXTRA_CA_CERTS` are set in the current shell.

### Phase 8 — shell integration
- Fish function that auto-starts the proxy on shell init if not running
- `fish_prompt` integration reading `~/.files/states/ctx.json` directly
- Auto-set `ANTHROPIC_BASE_URL` when proxy is detected running

### Phase 9 — richer context analytics
- Context window % per request shown in `history` output
- Daily/weekly summaries: `textproxy summary --week`
- Alert when a single request exceeds a configurable threshold (e.g. >50% of window)

### Phase 10 — multi-profile support
- Separate session tracking per Claude Code profile (`claude-work`, `claude-personal`)
- Detected from the `Authorization` header prefix or a configurable mapping
- Per-profile stats in `sessions` output

### Future / nice-to-have
- Web UI dashboard (local only, `textproxy serve`)
- Export to CSV / JSON for external analysis
- Homebrew formula for easy install
