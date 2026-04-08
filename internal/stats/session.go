package stats

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pdonorio/claude-context-proxy/internal/config"
)

const cacheSubDir = ".cache/ai-proxy"
const cacheLegacySubDir = ".cache/claude-context-proxy"

// CacheBase returns the directory used for session and history files.
// Migrates from the legacy directory name on first call if needed.
func CacheBase() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	newDir := filepath.Join(home, cacheSubDir)
	oldDir := filepath.Join(home, cacheLegacySubDir)
	if _, err := os.Stat(oldDir); err == nil {
		if _, err := os.Stat(newDir); os.IsNotExist(err) {
			_ = os.Rename(oldDir, newDir)
		}
	}
	return newDir
}

// SessionFile returns the path to session.json.
func SessionFile() string { return filepath.Join(CacheBase(), "session.json") }

// HistoryFile returns the path to history.jsonl.
func HistoryFile() string { return filepath.Join(CacheBase(), "history.jsonl") }

// PIDFile returns the path to proxy.pid.
func PIDFile() string { return filepath.Join(CacheBase(), "proxy.pid") }

// LogFile returns the path to the daemon log file.
func LogFile() string { return filepath.Join(CacheBase(), "proxy.log") }

// WritePID writes the current process PID to proxy.pid.
func WritePID() {
	if err := os.MkdirAll(CacheBase(), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(PIDFile(), []byte(fmt.Sprintf("%d", os.Getpid())), 0o644)
}

// RemovePID deletes proxy.pid.
func RemovePID() { _ = os.Remove(PIDFile()) }

// ReadPID reads the PID from proxy.pid. Returns 0 if absent or invalid.
func ReadPID() int {
	data, err := os.ReadFile(PIDFile())
	if err != nil {
		return 0
	}
	var pid int
	fmt.Sscanf(string(data), "%d", &pid)
	return pid
}

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
