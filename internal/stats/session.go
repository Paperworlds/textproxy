package stats

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pdonorio/claude-context-proxy/internal/config"
)

const cacheSubDir = ".cache/claude-context-proxy"

// CacheBase returns the directory used for session and history files.
func CacheBase() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, cacheSubDir)
}

// SessionFile returns the path to session.json.
func SessionFile() string { return filepath.Join(CacheBase(), "session.json") }

// HistoryFile returns the path to history.jsonl.
func HistoryFile() string { return filepath.Join(CacheBase(), "history.jsonl") }

// LoadSession reads session.json from disk. Returns nil if absent or corrupt.
func LoadSession() *Session {
	data, err := os.ReadFile(SessionFile())
	if err != nil {
		return nil
	}
	var s Session
	if json.Unmarshal(data, &s) != nil {
		return nil
	}
	return &s
}

// SaveSession persists s to session.json.
func SaveSession(s *Session) error {
	if err := os.MkdirAll(CacheBase(), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(SessionFile(), data, 0o644)
}

// ApplyTokens updates or creates a session with new token counts.
// Returns the (potentially new) session. Caller must hold the mutex.
func ApplyTokens(s *Session, cfg *config.Config, input, output int64) *Session {
	now := time.Now().UTC()
	gap := time.Duration(cfg.SessionGapMinutes) * time.Minute
	if s == nil || (s.LastRequestAt != time.Time{} && now.Sub(s.LastRequestAt) > gap) {
		s = &Session{
			SessionID: fmt.Sprintf("%d", now.Unix()),
			StartedAt: now,
		}
	}
	s.Requests++
	s.InputTokens += input
	s.OutputTokens += output
	s.LastRequestAt = now
	return s
}
