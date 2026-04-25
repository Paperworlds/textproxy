# textproxy

> Local proxy for Claude Code — token observability today, response cache tomorrow.

Lightweight local MITM proxy between Claude Code and `api.anthropic.com`.
Captures token usage from every API response and reports context consumption
in real time — without modifying Claude Code's behaviour.

Designed for **subscription users** (Claude Team, Claude.ai) who don't pay
per token but want to understand how much context window they're consuming
per session, per request, and per tool call.

## Why a proxy

The proxy is the right seat for **everything that wants to observe or shape
the API conversation**. Today that's just token observability. The roadmap is
broader:

- **Response caching** — identical prompts return cached answers. Agentic
  loops that re-ask the same question (especially in DAG retries and
  multi-shot evaluators) stop paying twice for the same work.
- **Request rewriting / policy** — strip secrets, redact paths, enforce
  per-session budgets at the wire.
- **Cross-vendor adapters** — once an open agent spec stabilises (A2A and
  friends), the proxy is the natural place to translate.

If it crosses the wire, it can live here. The MITM cert is the leverage.

### Pairs with RTK

textproxy is the **output side** of the token economy. The **input side** is
[RTK](https://github.com/snichols/rtk) — a CLI proxy that compresses tool
output (`ls`, `tree`, `git`, `gh`, `find`, …) before it reaches the LLM.
Same goal — fewer tokens crossing the boundary — different end of the pipe.

Install RTK separately (`brew install rtk`). textworkspace will pick it up and
register it. Together they cover both directions of the conversation between
agent and model.

## How it works

```
claude-ctx  (fish alias)
  ↓  HTTPS_PROXY=http://localhost:7474
  ↓  NODE_EXTRA_CA_CERTS=~/.config/textproxy/ca.crt
textproxy daemon  (localhost:7474)
  ↓  MITM: terminates TLS, inspects response, re-encrypts
api.anthropic.com
  ↓  SSE response with usage.input_tokens / cache tokens
proxy extracts token counts + writes stats
  ↓
~/.cache/textproxy/session.json   ← live session state
~/.cache/textproxy/history.jsonl  ← per-request log
~/.files/states/ctx.json          ← statusline state (atomic write)
```

## Quick start

```bash
# 1. Install
make install

# 2. Generate CA cert and install to macOS keychain (one-time)
textproxy setup

# 3. Start the daemon
textproxy start

# 4. Run Claude Code through the proxy
#    (add HTTPS_PROXY + NODE_EXTRA_CA_CERTS, or use the claude-ctx alias)
HTTPS_PROXY=http://localhost:7474 \
NODE_EXTRA_CA_CERTS=~/.config/textproxy/ca.crt \
claude

# 5. Check usage
textproxy stats
```

### Shell alias

**Fish** — `~/.config/fish/functions/claude-ctx.fish`
```fish
function claude-ctx
    set -lx HTTPS_PROXY http://localhost:7474
    set -lx NODE_EXTRA_CA_CERTS $HOME/.config/textproxy/ca.crt
    claude $argv
end
```

**Bash** — add to `~/.bashrc`
```bash
claude-ctx() {
    HTTPS_PROXY=http://localhost:7474 \
    NODE_EXTRA_CA_CERTS="$HOME/.config/textproxy/ca.crt" \
    claude "$@"
}
```

**Zsh** — add to `~/.zshrc`
```zsh
claude-ctx() {
    HTTPS_PROXY=http://localhost:7474 \
    NODE_EXTRA_CA_CERTS="$HOME/.config/textproxy/ca.crt" \
    claude "$@"
}
```

## Commands

| Command | Description |
|---------|-------------|
| `start` | Start proxy as background daemon |
| `stop` | Stop the running daemon |
| `restart` | Stop and restart the daemon |
| `setup` | Generate CA cert and install to macOS keychain |
| `log` | Tail the daemon log |
| `stats` | Current session — tokens, context windows consumed |
| `sessions` | All past sessions |
| `history` | Per-request log (`--last`, `--today`, `--since=DATE`) |
| `statusline` | Compact one-liner for shell prompt embedding |
| `config` | Show effective config |
| `os` | Show OS integration status (launchd agent) |
| `os install` | Install launchd agent — auto-start on login, restart if killed |
| `os uninstall` | Remove launchd agent |
| `version` | Print version |

## Auto-restart with launchd (recommended)

By default `textproxy start` runs a plain background daemon — if the process
is killed it stays dead. For a more resilient setup, hand it over to launchd:

```bash
textproxy os install
```

This writes a plist to `~/Library/LaunchAgents/com.paperworlds.textproxy.plist`
and loads it. launchd will then:

- Start textproxy automatically at login
- Restart it within ~1 second if it is killed for any reason

```bash
textproxy os          # show current status
textproxy os install  # install / re-install (e.g. after moving the binary)
textproxy os uninstall # remove and stop
```

> **macOS background activity notification** — after `os install` macOS may
> show a notification saying textproxy can run in the background. This is
> expected. You can manage it in **System Settings → General → Login Items &
> Extensions**. The proxy runs entirely locally; no data leaves your machine
> except the API calls it forwards to `api.anthropic.com`.

## Stats output

```
Session: 2026-04-07 14:28 (47m)
─────────────────────────────────────
Requests:       38
Input tokens:   284,391  (1.4× windows)
Output tokens:   18,204
Context ratio:  15.6:1  (in:out)
─────────────────────────────────────
Top context spikes (last 10 req):
  req #3   82,341 tokens  (41% of window)
  req #12  61,204 tokens  (31% of window)
```

## Configuration

Config file: `~/.config/textproxy/config.json`

All fields can be overridden with environment variables:

| Env var | Default | Description |
|---------|---------|-------------|
| `CTX_PORT` | `7474` | Proxy listen port |
| `CTX_SESSION_GAP_MINUTES` | `30` | Inactivity gap before session resets |
| `CTX_MODE` | `context` | Display mode: `context` or `cost` |
| `CTX_INSPECT` | `0` | Set to `1` to enable tool call attribution |
| `CTX_DEBUG` | `0` | Set to `1` to enable debug logging |
| `CTX_STATUSLINE_PATH` | `~/.files/states/ctx.json` | Statusline output path (empty to disable) |

## Build

```bash
make build    # compile
make install  # install to ~/.local/bin + codesign (macOS)
make test     # run tests
make bench    # benchmarks
```

## Notes

- Token counts include prompt cache tokens (`cache_read_input_tokens` +
  `cache_creation_input_tokens`) — essential for accurate context tracking
  since Claude Code aggressively caches tool definitions and system prompts
- The proxy only MITMs `api.anthropic.com`; all other HTTPS traffic is
  tunnelled transparently
- CA private key never leaves `~/.config/textproxy/ca.key` (mode 0600)

## License

[Elastic License 2.0](LICENSE)
