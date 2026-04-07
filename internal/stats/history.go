package stats

import (
	"encoding/json"
	"os"
	"strings"
)

// AppendHistory appends a single HistoryEntry to history.jsonl.
func AppendHistory(e HistoryEntry) error {
	if err := os.MkdirAll(CacheBase(), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(HistoryFile(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	line, err := json.Marshal(e)
	if err != nil {
		return err
	}
	_, err = f.Write(append(line, '\n'))
	return err
}

// ReadHistory reads all entries from history.jsonl. Returns nil if absent.
func ReadHistory() []HistoryEntry {
	data, err := os.ReadFile(HistoryFile())
	if err != nil {
		return nil
	}
	var entries []HistoryEntry
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var e HistoryEntry
		if json.Unmarshal([]byte(line), &e) == nil {
			entries = append(entries, e)
		}
	}
	return entries
}
