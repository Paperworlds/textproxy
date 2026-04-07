# Phase 6 — Refactor & Production Hardening

## Summary

`main.go` was split from a 879-line monolith into a proper package structure.
All 16 existing tests pass without any changes to test logic.
Graceful shutdown, error-handling improvements, benchmarks, and Makefile polish were added.

---

## Final Package Structure

```
claude-context-proxy/
├── main.go                        # entry point: arg dispatch + forwarding wrappers
├── bench_test.go                  # benchmarks (new)
├── main_test.go                   # existing tests (unchanged)
├── Makefile                       # updated with bench/lint targets
├── go.mod
└── internal/
    ├── config/
    │   └── config.go              # Config, ModelPrice, Default, Load, EnsureFile, ExpandHome
    ├── stats/
    │   ├── types.go               # Session, HistoryEntry, StatuslineState
    │   ├── session.go             # CacheBase, SessionFile, HistoryFile, LoadSession, SaveSession, ApplyTokens
    │   ├── history.go             # AppendHistory, ReadHistory
    │   └── statusline.go         # StatuslinePath, CostUSD, ExtractModel, WriteStatusline
    ├── proxy/
    │   └── handler.go             # SSEInspector, NewSSEInspector, Handler (callback-based)
    └── cli/
        └── stats.go               # CmdStats, CmdSessions, CmdHistory, CmdStatusline, CmdConfig, Fmt*
```

**Dependency graph** (no circular imports):
```
main  →  internal/config
main  →  internal/stats  →  internal/config
main  →  internal/proxy  →  internal/config
main  →  internal/cli    →  internal/stats, internal/config
```

**Test compatibility strategy**: `main.go` exports type aliases
(`Session = stats.Session`, `HistoryEntry`, `StatuslineState`, `Config`, `ModelPrice`)
and forwarding functions with identical signatures (`recordTokens`, `saveSession`,
`loadSession`, `cmdStats`, etc.) so that `main_test.go` compiles unchanged.

---

## Benchmark Results

Platform: Apple M4, darwin/arm64, Go 1.21

```
BenchmarkProxyHandler-10               32296      117980 ns/op
BenchmarkRecordTokens-10               32320      120533 ns/op
BenchmarkSSETeeParser/RawCopy-10   100000000          33.65 ns/op    14383 MB/s
BenchmarkSSETeeParser/SSEInspector-10   659352        5564 ns/op        87 MB/s
```

**Key findings:**

| Benchmark | ns/op | Notes |
|---|---|---|
| ProxyHandler | ~118 µs | Round-trip to loopback mock upstream; dominated by HTTP stack |
| RecordTokens | ~121 µs | JSON marshal + file I/O (session.json + history.jsonl) |
| SSEInspector overhead | ~5.5 µs | ~165× slower than raw copy per stream pass |
| RawCopy | 34 ns | Baseline: memcpy + discard |

The SSEInspector overhead (~5.5 µs per stream flush chunk) is only active when
`CTX_INSPECT=1`. In the default path the inspector is not instantiated and there
is zero overhead.

---

## Error Handling Audit

### Changes made

| Location | Before | After |
|---|---|---|
| `saveSession` | `_ = json.Marshal / _ = os.WriteFile` | returns `error`; caller `log.Printf` |
| `appendHistory` | all errors silently dropped | returns `error`; caller `log.Printf` |
| `config.EnsureFile` | `_ = os.WriteFile` | `log.Printf` on failure |
| `proxyHandler` SSE write | `_, _ = w.Write(...)` | early-break on write error (client disconnect) |
| `io.Copy` (non-SSE path) | `_, _ = io.Copy(...)` | kept silent — client disconnect is expected |
| `bodyBuf, _ = io.ReadAll(r.Body)` | silent | returns HTTP 400 with error message |

### Findings

- **No `log.Fatal` outside `main()`** — confirmed clean before and after.
- The original `_ = os.Remove(tmp)` in `writeStatusline` is intentional best-effort cleanup; kept with a comment.
- The non-SSE `io.Copy` error is silently swallowed by design — a client closing the connection mid-stream is not an error worth logging at level ERROR.
- SSE write errors now break the stream loop immediately rather than continuing to read from the upstream (avoids wasted work when the client has disconnected).

---

## Graceful Shutdown

```go
// SIGINT / SIGTERM
signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
<-stop
srv.Shutdown(ctx5s)     // stop accepting new requests
wg.Wait() / ctx timeout // flush pending recordTokens goroutines
```

- `wg.Add(1) / wg.Done()` wrap each `go recordTokens(...)` goroutine in the proxy callback.
- `srv.Shutdown` has a 5-second context; `wg.Wait` shares the same deadline.
- On clean exit: logs "all stats flushed". On timeout: logs warning about potential data loss.

---

## Verification Checklist

- [x] `go build ./...` succeeds
- [x] `go test ./...` passes — 16 tests, same as before refactor
- [x] `go vet ./...` clean
- [x] `go test -bench=. -benchtime=3s ./...` runs and prints results
- [x] `make build` / `make install` / `make test` / `make bench` / `make lint` all work
- [x] Binary strips (`-ldflags="-s -w"`) — confirmed smaller output
- [x] Proxy handles SIGTERM gracefully (SIGINT/SIGTERM → clean exit within 5 s)
- [x] All 5 CLI subcommands work end-to-end: stats, sessions, history, statusline, config
