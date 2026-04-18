# textproxy — Caching Options

## Option A: Auto-inject Anthropic cache_control (recommended)

The proxy already parses the request body. It can transparently add
`"cache_control": {"type": "ephemeral"}` to:

- The system prompt (if not already present)
- The last tool definition
- Optionally the last N messages

Zero client changes, works today. Claude Code doesn't always inject
cache_control optimally — the proxy automates it. Given the 207:1 input:output
ratio and large tool/system payloads, cache hit rate would likely be high.

**Complexity:** low — a few dozen lines of Go behind a config flag (`auto_cache: true`).  
**Risk:** none — cache_control is a hint; Anthropic ignores it if caching isn't applicable.

---

## Option B: Local response cache (keyed by request hash)

Hash `(model + system + tools + messages)` → store SSE response on disk.
On cache hit, replay the stored stream instead of calling Anthropic.

Useful for:
- Repeated identical calls (test suites, prompt iteration)
- textlives agents replaying scenarios
- `--no-upstream` / dry-run mode for prompt development

**Complexity:** medium — streaming replay is straightforward but cache invalidation
needs a TTL or size limit to prevent unbounded growth.  
**Risk:** low for deterministic use cases; not useful for interactive sessions
(temperature > 0, messages change each call).

---

## Option C: In-flight deduplication

If two identical requests arrive concurrently (e.g., textlives spawning many
agents with the same prompt), serve both from a single upstream call using a
`sync.Map` keyed by request hash.

**Complexity:** low.  
**Risk:** none.  
**Usefulness:** low for current workload — concurrent identical requests are rare
in interactive sessions.

---

## Recommendation

Implement **A** first. **B** is a good follow-on if dry-run / offline replay
becomes useful for textlives or prompt development workflows.
