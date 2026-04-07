package stats

import "time"

// Session holds per-session accumulated stats.
type Session struct {
	SessionID     string    `json:"session_id"`
	StartedAt     time.Time `json:"started_at"`
	Requests      int       `json:"requests"`
	InputTokens   int64     `json:"input_tokens"`
	OutputTokens  int64     `json:"output_tokens"`
	LastRequestAt time.Time `json:"last_request_at"`
}

// HistoryEntry is one line in history.jsonl.
type HistoryEntry struct {
	SessionID string    `json:"session_id,omitempty"`
	TS        time.Time `json:"ts"`
	Input     int64     `json:"input"`
	Output    int64     `json:"output"`
	Path      string    `json:"path"`
	Model     string    `json:"model,omitempty"`
	Tools     []string  `json:"tools,omitempty"`
}

// StatuslineState is the schema of ctx.json.
type StatuslineState struct {
	InputTokens  int64     `json:"input_tokens"`
	OutputTokens int64     `json:"output_tokens"`
	Requests     int       `json:"requests"`
	CostUSD      float64   `json:"cost_usd"`
	SessionID    string    `json:"session_id"`
	UpdatedAt    time.Time `json:"updated_at"`
}
