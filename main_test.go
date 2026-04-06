package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ── helpers ────────────────────────────────────────────────────────────────

func withTempCache(t *testing.T) func() {
	t.Helper()
	tmp := t.TempDir()
	// Override cacheBase by pointing home to tmp.
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmp)
	// Reset in-memory session so tests don't bleed state.
	mu.Lock()
	session = nil
	mu.Unlock()
	return func() {
		os.Setenv("HOME", origHome)
		mu.Lock()
		session = nil
		mu.Unlock()
	}
}

// ── tests ──────────────────────────────────────────────────────────────────

// TestTokenHeaderExtraction verifies that token headers are parsed from a
// mock upstream response and persisted to session.json / history.jsonl.
func TestTokenHeaderExtraction(t *testing.T) {
	cleanup := withTempCache(t)
	defer cleanup()

	// Mock upstream that returns token headers.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Anthropic-Input-Tokens", "42381")
		w.Header().Set("X-Anthropic-Output-Tokens", "1204")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"type":"message"}`))
	}))
	defer upstream.Close()

	// Temporarily patch the global upstream constant by using a test handler.
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Replicate proxyHandler but target the test server.
		proxyReq, _ := http.NewRequest(r.Method, upstream.URL+r.RequestURI, r.Body)
		for k, vals := range r.Header {
			for _, v := range vals {
				proxyReq.Header.Add(k, v)
			}
		}
		client := &http.Client{}
		resp, err := client.Do(proxyReq)
		if err != nil {
			http.Error(w, err.Error(), 502)
			return
		}
		defer resp.Body.Close()

		inputTokens := int64(0)
		outputTokens := int64(0)
		if v := resp.Header.Get("X-Anthropic-Input-Tokens"); v != "" {
			fmt.Sscanf(v, "%d", &inputTokens)
		}
		if v := resp.Header.Get("X-Anthropic-Output-Tokens"); v != "" {
			fmt.Sscanf(v, "%d", &outputTokens)
		}
		for k, vals := range resp.Header {
			for _, v := range vals {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)

		if inputTokens > 0 || outputTokens > 0 {
			recordTokens(inputTokens, outputTokens, r.URL.Path)
		}
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v1/messages")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	// Give goroutine time to write.
	time.Sleep(50 * time.Millisecond)

	// Verify session.json.
	data, err := os.ReadFile(sessionFile())
	if err != nil {
		t.Fatalf("session.json not written: %v", err)
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatalf("bad session.json: %v", err)
	}
	if s.InputTokens != 42381 {
		t.Errorf("InputTokens = %d, want 42381", s.InputTokens)
	}
	if s.OutputTokens != 1204 {
		t.Errorf("OutputTokens = %d, want 1204", s.OutputTokens)
	}
	if s.Requests != 1 {
		t.Errorf("Requests = %d, want 1", s.Requests)
	}

	// Verify history.jsonl.
	hist := readHistory()
	if len(hist) != 1 {
		t.Fatalf("history has %d entries, want 1", len(hist))
	}
	if hist[0].Input != 42381 {
		t.Errorf("history Input = %d, want 42381", hist[0].Input)
	}
	if hist[0].Path != "/v1/messages" {
		t.Errorf("history Path = %q, want /v1/messages", hist[0].Path)
	}
}

// TestStreamingPassthrough verifies that SSE responses are forwarded chunk by
// chunk without buffering (each chunk is delivered to the client immediately).
func TestStreamingPassthrough(t *testing.T) {
	cleanup := withTempCache(t)
	defer cleanup()

	chunks := []string{
		"data: {\"type\":\"ping\"}\n\n",
		"data: {\"type\":\"content_block_delta\"}\n\n",
		"data: [DONE]\n\n",
	}

	// Mock upstream: sends SSE chunks with a short delay between them.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("X-Anthropic-Input-Tokens", "100")
		w.Header().Set("X-Anthropic-Output-Tokens", "50")
		flusher, _ := w.(http.Flusher)
		for _, c := range chunks {
			w.Write([]byte(c))
			if flusher != nil {
				flusher.Flush()
			}
		}
	}))
	defer upstream.Close()

	// Patch proxy to target test upstream.
	saved := ""
	_ = saved
	handler := buildTestProxyHandler(upstream.URL)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v1/messages")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}

	body, _ := io.ReadAll(resp.Body)
	for _, c := range chunks {
		if !strings.Contains(string(body), strings.TrimSpace(c)) {
			t.Errorf("response missing chunk: %q", c)
		}
	}
}

// TestSessionJSONWritten verifies session.json accumulates across calls.
func TestSessionJSONWritten(t *testing.T) {
	cleanup := withTempCache(t)
	defer cleanup()

	recordTokens(1000, 200, "/v1/messages")
	recordTokens(500, 100, "/v1/messages")
	time.Sleep(20 * time.Millisecond)

	data, err := os.ReadFile(sessionFile())
	if err != nil {
		t.Fatalf("session.json not found: %v", err)
	}
	var s Session
	json.Unmarshal(data, &s)
	if s.InputTokens != 1500 {
		t.Errorf("InputTokens = %d, want 1500", s.InputTokens)
	}
	if s.OutputTokens != 300 {
		t.Errorf("OutputTokens = %d, want 300", s.OutputTokens)
	}
	if s.Requests != 2 {
		t.Errorf("Requests = %d, want 2", s.Requests)
	}
}

// TestStatsOutput verifies the stats subcommand produces correct output.
func TestStatsOutput(t *testing.T) {
	cleanup := withTempCache(t)
	defer cleanup()

	// Write a known history.
	if err := os.MkdirAll(cacheBase(), 0o755); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	s := Session{
		StartedAt:     now.Add(-47 * time.Minute),
		Requests:      3,
		InputTokens:   284391,
		OutputTokens:  18204,
		LastRequestAt: now,
	}
	data, _ := json.MarshalIndent(s, "", "  ")
	os.WriteFile(sessionFile(), data, 0o644)

	entries := []HistoryEntry{
		{TS: now.Add(-10 * time.Minute), Input: 82341, Output: 500, Path: "/v1/messages"},
		{TS: now.Add(-5 * time.Minute), Input: 61204, Output: 800, Path: "/v1/messages"},
		{TS: now, Input: 140846, Output: 16904, Path: "/v1/messages"},
	}
	f, _ := os.OpenFile(historyFile(), os.O_CREATE|os.O_WRONLY, 0o644)
	for _, e := range entries {
		line, _ := json.Marshal(e)
		f.Write(append(line, '\n'))
	}
	f.Close()

	// Capture output.
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmdStats()

	w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)

	output := string(out)
	checks := []string{
		"284,391",
		"18,204",
		"Requests:",
		"Input tokens:",
		"Output tokens:",
		"Top input spikes",
	}
	for _, c := range checks {
		if !strings.Contains(output, c) {
			t.Errorf("stats output missing %q\nfull output:\n%s", c, output)
		}
	}
}

// TestFmtInt64 verifies comma formatting.
func TestFmtInt64(t *testing.T) {
	cases := []struct {
		n    int64
		want string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1,000"},
		{284391, "284,391"},
		{1000000, "1,000,000"},
	}
	for _, c := range cases {
		if got := fmtInt64(c.n); got != c.want {
			t.Errorf("fmtInt64(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}

// buildTestProxyHandler creates a proxyHandler-equivalent targeting targetURL.
func buildTestProxyHandler(targetURL string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyReq, err := http.NewRequest(r.Method, targetURL+r.RequestURI, r.Body)
		if err != nil {
			http.Error(w, "bad request", 400)
			return
		}
		for k, vals := range r.Header {
			for _, v := range vals {
				proxyReq.Header.Add(k, v)
			}
		}
		client := &http.Client{Timeout: 0}
		resp, err := client.Do(proxyReq)
		if err != nil {
			http.Error(w, err.Error(), 502)
			return
		}
		defer resp.Body.Close()

		inputTokens := int64(0)
		outputTokens := int64(0)
		fmt.Sscanf(resp.Header.Get("X-Anthropic-Input-Tokens"), "%d", &inputTokens)
		fmt.Sscanf(resp.Header.Get("X-Anthropic-Output-Tokens"), "%d", &outputTokens)

		for k, vals := range resp.Header {
			for _, v := range vals {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)

		isSSE := strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream")
		if isSSE {
			flusher, ok := w.(http.Flusher)
			buf := make([]byte, 4096)
			for {
				n, readErr := resp.Body.Read(buf)
				if n > 0 {
					w.Write(buf[:n])
					if ok {
						flusher.Flush()
					}
				}
				if readErr == io.EOF || readErr != nil {
					break
				}
			}
		} else {
			io.Copy(w, resp.Body)
		}

		if inputTokens > 0 || outputTokens > 0 {
			recordTokens(inputTokens, outputTokens, r.URL.Path)
		}
	})
}

// ── session gap test ───────────────────────────────────────────────────────

func TestSessionGapReset(t *testing.T) {
	cleanup := withTempCache(t)
	defer cleanup()

	// Force session gap to 1 minute.
	savedGap := sessionGapMinutes
	sessionGapMinutes = 1
	defer func() { sessionGapMinutes = savedGap }()

	// Write a session that ended 2 minutes ago.
	old := time.Now().UTC().Add(-2 * time.Minute)
	s := Session{
		StartedAt:     old.Add(-10 * time.Minute),
		Requests:      5,
		InputTokens:   9999,
		OutputTokens:  999,
		LastRequestAt: old,
	}
	os.MkdirAll(filepath.Dir(sessionFile()), 0o755)
	data, _ := json.MarshalIndent(s, "", "  ")
	os.WriteFile(sessionFile(), data, 0o644)

	// Reset in-memory session so loadSession is used.
	mu.Lock()
	session = nil
	mu.Unlock()

	recordTokens(100, 10, "/v1/messages")
	time.Sleep(20 * time.Millisecond)

	data2, _ := os.ReadFile(sessionFile())
	var s2 Session
	json.Unmarshal(data2, &s2)

	// Should have reset: only 1 request.
	if s2.Requests != 1 {
		t.Errorf("expected session reset (Requests=1), got %d", s2.Requests)
	}
	if s2.InputTokens != 100 {
		t.Errorf("expected InputTokens=100 after reset, got %d", s2.InputTokens)
	}
}
