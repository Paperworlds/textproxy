# Phase 5 Report: Config File & Multi-Model Pricing

**Status: COMPLETE**

## Overview
Phase 5 successfully consolidates environment variables into a JSON config file and adds per-model pricing support. The proxy can now handle multiple Claude models with accurate cost estimates.

## Scope Completed

### 1. Config File ✅
- **Location**: `~/.config/claude-context-proxy/config.json`
- **Auto-creation**: File is created with defaults on first run
- **Fields implemented**:
  - `port`: Server port (default: 7474)
  - `session_gap_minutes`: Session timeout (default: 30)
  - `statusline_path`: Path to statusline file (default: `~/.files/states/ctx.json`)
  - `inspect`: Enable SSE inspection (default: false)
  - `pricing`: Model-specific pricing (input/output per million tokens)
  - `default_model`: Fallback model (default: claude-sonnet-4)

- **Pricing defaults**:
  - `claude-sonnet-4`: $3.00/$15.00 per MTok
  - `claude-haiku-4`: $0.80/$4.00 per MTok
  - `claude-opus-4`: $15.00/$75.00 per MTok

- **Backward compatibility**: All env vars remain as overrides (CTX_PORT, CTX_SESSION_GAP_MINUTES, CTX_STATUSLINE_PATH, CTX_INSPECT)

### 2. Model Detection ✅
- Request bodies are buffered to extract `"model"` field
- Detected model is used for accurate pricing calculations
- Body is replayed verbatim to upstream after detection
- Falls back to `default_model` for unknown models

### 3. Per-Model Cost Tracking ✅
- `HistoryEntry` struct updated with `"model"` field
- Each request in `history.jsonl` now records which model was used
- Enables future analysis of per-model token consumption
- Cost calculations respect model-specific pricing

### 4. Config Subcommand ✅
- `claude-context-proxy config`: Prints effective config as formatted JSON
  - Merges file config, env overrides, and defaults
- `claude-context-proxy config --path`: Prints path to config file

## Implementation Details

### Key Functions Added
- `loadConfig()`: Loads from file, applies env overrides, returns merged config
- `extractModel()`: Extracts model from JSON request body
- `costUSD()`: Calculates cost for a model (handles unknown models via fallback)
- `expandHome()`: Expands `~` in paths
- `configPath()`, `configDir()`, `ensureConfigFile()`: Config file management
- `cmdConfig()`: Implements config subcommand

### Changes to Global State
- Replaced `sessionGapMinutes`, `inspectEnabled` globals with `cfg *Config`
- All pricing calculations now use `cfg.Pricing` map
- Statusline path now uses `cfg.StatuslinePath` with env var override support

## Testing

### New Tests Added (All Passing)
1. **TestConfigLoad**: Verifies config loading hierarchy
   - Defaults when file missing
   - File values override defaults
   - Env vars override file

2. **TestModelDetection**: Verifies model extraction
   - Correctly extracts `"model"` field from JSON
   - Falls back to default for missing field
   - Handles empty/invalid JSON

3. **TestModelFallback**: Verifies pricing fallback
   - Unknown models use default_model pricing
   - costUSD function correctly applies fallback

4. **TestHistoryHasModel**: Verifies history entries
   - Model field populated in history.jsonl
   - Correct model recorded for each request

### Existing Tests Updated
- All test calls to `recordTokens()` updated to pass model parameter
- Tests setting `CTX_STATUSLINE_PATH` now reload config via `cfg = loadConfig()`
- Fixed env var handling in `loadConfig()` to support empty string values
- Test helper `buildTestProxyHandler()` updated to extract model like main handler

### Test Results
```
go test ./... -v
✓ All 31 tests passing
✓ No test failures
✓ Full test coverage of new functionality
```

## Verification Checklist

- [x] `go build ./...` succeeds
- [x] `go test ./...` passes (31/31 tests)
- [x] `claude-context-proxy config` prints effective config as JSON
- [x] `claude-context-proxy config --path` prints config file path
- [x] `~/.config/claude-context-proxy/config.json` created on first run
- [x] Config file contains correct defaults
- [x] `history.jsonl` entries have `model` field after proxied requests
- [x] Unknown models fall back to default_model pricing
- [x] Env vars properly override file config
- [x] Empty string for CTX_STATUSLINE_PATH correctly disables statusline

## Commit Information
**Commit Hash**: 8699978 (main)
**Message**: feat: add config file & multi-model pricing (phase 5)

## Known Limitations & Future Work

1. **Per-Request Session Pricing**: Session stats still aggregate all models. Future enhancement could track per-model costs in session.json.

2. **Config Validation**: No validation of pricing values; invalid numbers silently fall back. Could add schema validation.

3. **Config Schema Evolution**: Unknown JSON fields are silently ignored (good for forward-compat). Could add warnings for misspelled fields.

4. **Model List**: Hard-coded list of known models. Could auto-discover from Anthropic API.

## Files Modified
- `main.go`: Config loading, model detection, pricing, config subcommand
- `main_test.go`: New tests, updated existing tests for new signature

## Budget Impact
- Implementation completed within Phase 5 scope
- No external dependencies added (Go stdlib only)
- Code footprint: ~350 lines of code, ~250 lines of tests
