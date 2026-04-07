package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/pdonorio/claude-context-proxy/internal/cli"
	"github.com/pdonorio/claude-context-proxy/internal/config"
	"github.com/pdonorio/claude-context-proxy/internal/proxy"
	"github.com/pdonorio/claude-context-proxy/internal/stats"
)

const version = "0.1.0"

// Type aliases so that tests (package main) can use the unqualified names.
type Session = stats.Session
type HistoryEntry = stats.HistoryEntry
type StatuslineState = stats.StatuslineState
type Config = config.Config
type ModelPrice = config.ModelPrice

// Package globals — accessed directly by tests.
var (
	mu  sync.Mutex
	session *Session
	cfg     *config.Config
	wg      sync.WaitGroup // tracks in-flight recordTokens goroutines
)

// ── Forwarding functions ────────────────────────────────────────────────────

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

func statuslinePath() string                          { return stats.StatuslinePath(cfg) }
func costUSD(model string, in, out int64) float64     { return stats.CostUSD(cfg, model, in, out) }
func extractModel(body []byte) string                 { return stats.ExtractModel(cfg, body) }
func writeStatusline(s *Session)                      { stats.WriteStatusline(cfg, s) }
func newSSEInspector(r io.Reader) *proxy.SSEInspector { return proxy.NewSSEInspector(r) }
func fmtInt64(n int64) string                         { return cli.FmtInt64(n) }
func fmtCompact(n int64) string                       { return cli.FmtCompact(n) }
func fmtInt(n int) string                             { return cli.FmtInt(n) }
func defaultConfig() *config.Config                   { return config.Default() }
func loadConfig() *config.Config                      { return config.Load() }

// recordTokens applies token counts to the current session and persists stats.
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

// ── Daemon management ───────────────────────────────────────────────────────

// cmdStart launches the proxy as a background daemon.
// If _CCP_DAEMON=1 is set we are already the daemon child — run the server.
func cmdStart() {
	if os.Getenv("_CCP_DAEMON") == "1" {
		runServer()
		return
	}

	// Check if already running.
	if pid := stats.ReadPID(); pid != 0 {
		if proc, err := os.FindProcess(pid); err == nil {
			if proc.Signal(syscall.Signal(0)) == nil {
				fmt.Fprintf(os.Stderr, "proxy already running (pid %d)\n", pid)
				os.Exit(1)
			}
		}
		stats.RemovePID() // stale PID file
	}

	self, err := exec.LookPath(os.Args[0])
	if err != nil {
		self = os.Args[0]
	}

	logPath := stats.LogFile()
	if err := os.MkdirAll(stats.CacheBase(), 0o755); err != nil {
		log.Fatalf("start: mkdir: %v", err)
	}
	lf, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		log.Fatalf("start: open log %s: %v", logPath, err)
	}

	cmd := exec.Command(self)
	cmd.Env = append(os.Environ(), "_CCP_DAEMON=1")
	cmd.Stdout = lf
	cmd.Stderr = lf
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		log.Fatalf("start: %v", err)
	}
	lf.Close()

	fmt.Printf("proxy started (pid %d) — logs: %s\n", cmd.Process.Pid, logPath)
}

// cmdStop sends SIGTERM to the running proxy.
func cmdStop() {
	pid := stats.ReadPID()
	if pid == 0 {
		fmt.Fprintln(os.Stderr, "stop: proxy not running (no pid file)")
		os.Exit(1)
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		log.Fatalf("stop: find process %d: %v", pid, err)
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		log.Fatalf("stop: signal %d: %v", pid, err)
	}
	fmt.Printf("proxy stopped (pid %d)\n", pid)
}

// cmdRestart stops the running proxy and starts a new daemon.
func cmdRestart() {
	pid := stats.ReadPID()
	if pid != 0 {
		if proc, err := os.FindProcess(pid); err == nil {
			_ = proc.Signal(syscall.SIGTERM)
			fmt.Printf("restart: sent SIGTERM to pid %d\n", pid)
			// Wait up to 3 s for old process to exit.
			for i := 0; i < 30; i++ {
				time.Sleep(100 * time.Millisecond)
				if stats.ReadPID() == 0 {
					break
				}
			}
		}
	}
	cmdStart()
}

// cmdLog tails the daemon log file (last 40 lines, then follows).
func cmdLog() {
	logPath := stats.LogFile()
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "log: no log file at %s — has the proxy been started?\n", logPath)
		os.Exit(1)
	}
	cmd := exec.Command("tail", "-n", "40", "-f", logPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}

// ── Server ──────────────────────────────────────────────────────────────────

func runServer() {
	stats.WritePID()
	defer stats.RemovePID()

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

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("claude-context-proxy v%s listening on :%d → %s", version, cfg.Port, proxy.Upstream)
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

// ── main ────────────────────────────────────────────────────────────────────

func main() {
	cfg = config.Load()
	config.EnsureFile()

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "start":
			cmdStart()
			return
		case "stop":
			cmdStop()
			return
		case "restart":
			cmdRestart()
			return
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
		case "log":
			cmdLog()
			return
		case "version", "--version", "-v":
			fmt.Printf("claude-context-proxy v%s\n", version)
			return
		case "--foreground", "-f":
			runServer()
			return
		}
	}

	// Default: daemon start (or run server if already the daemon child).
	cmdStart()
}
