package stats

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/pdonorio/claude-context-proxy/internal/config"
)

// StatuslinePath returns the expanded path to the statusline state file.
// Returns "" if statusline is disabled (empty config value).
func StatuslinePath(cfg *config.Config) string {
	return config.ExpandHome(cfg.StatuslinePath)
}

// CostUSD calculates the USD cost for the given model and token counts.
// Falls back to DefaultModel pricing if the model is not found.
func CostUSD(cfg *config.Config, model string, input, output int64) float64 {
	pricing, ok := cfg.Pricing[model]
	if !ok {
		pricing = cfg.Pricing[cfg.DefaultModel]
	}
	return float64(input)/1_000_000*pricing.InputPerMtok + float64(output)/1_000_000*pricing.OutputPerMtok
}

// ExtractModel extracts the model name from a request body JSON.
// Returns cfg.DefaultModel if the body is not JSON or has no "model" field.
func ExtractModel(cfg *config.Config, body []byte) string {
	var data map[string]interface{}
	if json.Unmarshal(body, &data) != nil {
		return cfg.DefaultModel
	}
	if model, ok := data["model"].(string); ok && model != "" {
		return model
	}
	return cfg.DefaultModel
}

// WriteStatusline atomically writes the statusline state file.
// Errors are logged and silently swallowed — statusline is best-effort.
func WriteStatusline(cfg *config.Config, s *Session) {
	path := StatuslinePath(cfg)
	if path == "" {
		return
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("statusline: cannot create dir %s: %v", dir, err)
		return
	}
	cost := CostUSD(cfg, cfg.DefaultModel, s.InputTokens, s.OutputTokens)
	state := StatuslineState{
		InputTokens:  s.InputTokens,
		OutputTokens: s.OutputTokens,
		Requests:     s.Requests,
		CostUSD:      cost,
		SessionID:    s.SessionID,
		UpdatedAt:    time.Now().UTC(),
	}
	data, err := json.Marshal(state)
	if err != nil {
		return
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		log.Printf("statusline: write tmp: %v", err)
		return
	}
	if err := os.Rename(tmp, path); err != nil {
		log.Printf("statusline: rename: %v", err)
		_ = os.Remove(tmp) // best-effort cleanup
	}
}
