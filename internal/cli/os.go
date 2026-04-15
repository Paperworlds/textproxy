package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"textproxy/internal/stats"
)

const launchAgentLabel = "com.paperworlds.textproxy"

func launchAgentPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", launchAgentLabel+".plist")
}

func launchAgentPlist(binPath, logPath string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
		<string>--foreground</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
	<key>StandardOutPath</key>
	<string>%s</string>
	<key>StandardErrorPath</key>
	<string>%s</string>
</dict>
</plist>
`, launchAgentLabel, binPath, logPath, logPath)
}

// launchctlPID queries launchctl for the running PID of our service.
// Returns 0 if not running or not loaded.
func launchctlPID() int {
	out, err := exec.Command("launchctl", "list", launchAgentLabel).Output()
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, `"PID"`) {
			// "PID" = 12345;
			var pid int
			fmt.Sscanf(line, `"PID" = %d;`, &pid)
			return pid
		}
	}
	return 0
}

// CmdOS implements the "os" subcommand: show OS integration status,
// optionally install or uninstall the launchd agent.
//
// Usage:
//
//	textproxy os             — show status
//	textproxy os install     — write plist + load launchd agent
//	textproxy os uninstall   — unload + remove plist
func CmdOS(args []string) {
	plistPath := launchAgentPath()

	sub := "status"
	if len(args) > 0 {
		sub = args[0]
	}

	switch sub {
	case "install":
		osInstall(plistPath)
	case "uninstall":
		osUninstall(plistPath)
	default:
		osStatus(plistPath)
	}
}

func osStatus(plistPath string) {
	_, plistErr := os.Stat(plistPath)
	installed := plistErr == nil

	launchdPID := launchctlPID()
	proxyPID := stats.ReadPID()

	fmt.Println("OS Integration")
	fmt.Println("──────────────────────────────────────")

	if installed {
		fmt.Printf("launchd agent:   installed (%s)\n", plistPath)
	} else {
		fmt.Printf("launchd agent:   not installed\n")
	}

	if launchdPID > 0 {
		fmt.Printf("launchd status:  running (pid %d)\n", launchdPID)
	} else if installed {
		fmt.Printf("launchd status:  stopped\n")
	} else {
		fmt.Printf("launchd status:  n/a\n")
	}

	if proxyPID > 0 {
		fmt.Printf("proxy pid file:  %d\n", proxyPID)
	} else {
		fmt.Printf("proxy pid file:  none\n")
	}

	fmt.Printf("log:             %s\n", stats.LogFile())

	if !installed {
		fmt.Println()
		fmt.Println("Run 'textproxy os install' to install the launchd agent.")
		fmt.Println("The proxy will then start on login and restart automatically if killed.")
	}
}

func osInstall(plistPath string) {
	binPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "os: cannot determine binary path: %v\n", err)
		os.Exit(1)
	}
	if resolved, err := filepath.EvalSymlinks(binPath); err == nil {
		binPath = resolved
	}

	logPath := stats.LogFile()
	if err := os.MkdirAll(filepath.Dir(plistPath), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "os: mkdir: %v\n", err)
		os.Exit(1)
	}

	// Unload first so a re-install picks up the new binary path.
	exec.Command("launchctl", "unload", plistPath).Run() //nolint:errcheck

	if err := os.WriteFile(plistPath, []byte(launchAgentPlist(binPath, logPath)), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "os: write plist: %v\n", err)
		os.Exit(1)
	}

	cmd := exec.Command("launchctl", "load", "-w", plistPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "os: launchctl load: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Installed:  %s\n", plistPath)
	fmt.Printf("Binary:     %s\n", binPath)
	fmt.Printf("Log:        %s\n", logPath)
	fmt.Println()
	fmt.Println("textproxy will now start on login and restart automatically if killed.")
}

func osUninstall(plistPath string) {
	if _, err := os.Stat(plistPath); os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, "os: launchd agent not installed")
		os.Exit(1)
	}
	exec.Command("launchctl", "unload", plistPath).Run() //nolint:errcheck
	if err := os.Remove(plistPath); err != nil {
		fmt.Fprintf(os.Stderr, "os: remove plist: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Uninstalled launchd agent.")
}
