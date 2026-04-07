package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/pdonorio/claude-context-proxy/internal/cli"
	"github.com/pdonorio/claude-context-proxy/internal/config"
	"github.com/pdonorio/claude-context-proxy/internal/proxy"
	"github.com/pdonorio/claude-context-proxy/internal/stats"
)

// Type aliases so that tests (package main) can use the unqualified names.
type Session = stats.Session
type HistoryEntry = stats.HistoryEntry
type StatuslineState = stats.StatuslineState
type Config = config.Config
type ModelPrice = config.ModelPrice

// Package globals — accessed directly by tests.
var (
	mu      sync.Mutex
	session *Session
	cfg     *config.Config
	wg      sync.WaitGroup // tracks in-flight recordTokens goroutines
)

// ── Forwarding functions ────────────────────────────────────────────────────
// These keep the same names as the original functions so that existing tests
// (in package main) compile and pass without any changes to test logic.

func cacheBase() string     { return stats.CacheBase() }
func sessionFile() string   { return stats.SessionFile() }
func historyFile() string   { return stats.HistoryFile() }
func loadSession() *Session { return stats.LoadSession() }

func saveSession(s *Session) {
	if err := stats.SaveSession(s); err != nil {
		log.Printf("saveSession: %v", err)
	}
}

func readHistory() []HistoryEntry { return stats.ReadHistory() }

func appendHistory(e HistoryEntry) {
	if err := stats.AppendHistory(e); err != nil {
		log.Printf("appendHistory: %v", err)
	}
}

func statuslinePath() string                        { return stats.StatuslinePath(cfg) }
func costUSD(model string, in, out int64) float64   { return stats.CostUSD(cfg, model, in, out) }
func extractModel(body []byte) string               { return stats.ExtractModel(cfg, body) }
func writeStatusline(s *Session)                    { stats.WriteStatusline(cfg, s) }
func newSSEInspector(r io.Reader) *proxy.SSEInspector { return proxy.NewSSEInspector(r) }
func fmtInt64(n int64) string                       { return cli.FmtInt64(n) }
func fmtCompact(n int64) string                     { return cli.FmtCompact(n) }
func fmtInt(n int) string                           { return cli.FmtInt(n) }
func defaultConfig() *config.Config                 { return config.Default() }
func loadConfig() *config.Config                    { return config.Load() }

// recordTokens applies token counts to the current session and persists stats.
// It is called synchronously; the proxy hot path spawns it in a goroutine
// tracked by wg for graceful shutdown.
func recordTokens(input, output int64, path, model string, tools []string) {
	mu.Lock()
	session = stats.ApplyTokens(session, cfg, input, output)
	s := session
	mu.Unlock()

	if err := stats.SaveSession(s); err != nil {
		log.Printf("recordTokens: save session: %v", err)
	}
	stats.WriteStatusline(cfg, s)
	if err := stats.AppendHistory(HistoryEntry{
		SessionID: s.SessionID,
		TS:        s.LastRequestAt,
		Input:     input,
		Output:    output,
		Path:      path,
		Model:     model,
		Tools:     tools,
	}); err != nil {
		log.Printf("recordTokens: append history: %v", err)
	}
}

// ── CLI forwarding ──────────────────────────────────────────────────────────

func cmdStats(args []string)      { cli.CmdStats(args, cfg) }
func cmdSessions()                { cli.CmdSessions(cfg) }
func cmdHistory(args []string)    { cli.CmdHistory(args) }
func cmdStatusline(args []string) { cli.CmdStatusline(args, cfg) }
func cmdConfig(args []string)     { cli.CmdConfig(args, cfg) }

// ── main ────────────────────────────────────────────────────────────────────

func main() {
	cfg = config.Load()
	config.EnsureFile()

	// Subcommand dispatch.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "stats":
			cmdStats(os.Args[2:])
			return
		case "sessions":
			cmdSessions()
			return
		case "history":
			cmdHistory(os.Args[2:])
			return
		case "statusline":
			cmdStatusline(os.Args[2:])
			return
		case "config":
			cmdConfig(os.Args[2:])
			return
		}
	}

	// Load existing session from disk so we survive restarts within the gap.
	mu.Lock()
	session = loadSession()
	if session != nil {
		gap := time.Duration(cfg.SessionGapMinutes) * time.Minute
		if time.Since(session.LastRequestAt) > gap {
			session = nil
		}
	}
	mu.Unlock()

	mux := http.NewServeMux()
	mux.HandleFunc("/", proxy.Handler(proxy.Upstream, cfg, func(input, output int64, path, model string, tools []string) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			recordTokens(input, output, path, model, tools)
		}()
	}))

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: mux,
	}

	// Graceful shutdown on SIGINT / SIGTERM.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("claude-context-proxy listening on :%d → %s", cfg.Port, proxy.Upstream)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("server: %v", err)
		}
	}()

	<-stop
	log.Println("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("http shutdown: %v", err)
	}

	// Wait for all in-flight recordTokens goroutines to finish (max 5 s).
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		log.Println("all stats flushed")
	case <-ctx.Done():
		log.Println("flush timeout: some stats may not have been written")
	}
}
