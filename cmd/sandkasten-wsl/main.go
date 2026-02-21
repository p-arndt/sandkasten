// sandkasten-wsl is a Windows helper to run the Sandkasten daemon inside WSL2.
// Build with: GOOS=windows go build -o sandkasten-wsl.exe ./cmd/sandkasten-wsl
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const usage = `sandkasten-wsl: run Sandkasten daemon inside WSL2

Usage:
  sandkasten-wsl start [options]     Start daemon in WSL (detached)
  sandkasten-wsl status [options]    Show daemon status in WSL
  sandkasten-wsl stop [options]       Stop daemon in WSL

Options (all commands):
  -d, --distro <name>   WSL distro (default: default distro)
  -c, --config <path>  Config path inside WSL (default: /var/lib/sandkasten/sandkasten.yaml or ~/sandkasten.yaml)
  -b, --binary <path>  Path to sandkasten binary inside WSL (default: sandkasten from PATH)

Examples:
  sandkasten-wsl start
  sandkasten-wsl start --distro Ubuntu-22.04 --config ~/sandkasten.yaml
  sandkasten-wsl status
  sandkasten-wsl stop
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	// Require Windows for wsl.exe
	if os.Getenv("WSL_DISTRO_NAME") != "" {
		fmt.Fprintln(os.Stderr, "sandkasten-wsl should be run from Windows (e.g. PowerShell), not from inside WSL.")
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "start":
		err = runStart(args)
	case "status":
		err = runStatus(args)
	case "stop":
		err = runStop(args)
	case "-h", "--help", "help":
		fmt.Print(usage)
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func parseFlags(args []string) (distro, configPath, binaryPath string) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-d", "--distro":
			if i+1 < len(args) {
				distro = args[i+1]
				i++
			}
		case "-c", "--config":
			if i+1 < len(args) {
				configPath = args[i+1]
				i++
			}
		case "-b", "--binary":
			if i+1 < len(args) {
				binaryPath = args[i+1]
				i++
			}
		}
	}
	return distro, configPath, binaryPath
}

func wslArgs(distro string, innerCmd string) []string {
	if distro != "" {
		return []string{"-d", distro, "--", "sh", "-c", innerCmd}
	}
	return []string{"--", "sh", "-c", innerCmd}
}

func runStart(args []string) error {
	distro, configPath, binaryPath := parseFlags(args)
	if binaryPath == "" {
		binaryPath = "sandkasten"
	}
	configFlag := ""
	if configPath != "" {
		configFlag = " --config " + configPath
	}
	// daemon -d runs in background
	innerCmd := "sudo " + binaryPath + " daemon -d" + configFlag
	argv := wslArgs(distro, innerCmd)
	cmd := exec.Command("wsl", argv...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("wsl start: %w", err)
	}
	fmt.Println("Daemon started in WSL. Use 'sandkasten-wsl status' to check.")
	return nil
}

func runStatus(args []string) error {
	distro, _, _ := parseFlags(args)
	innerCmd := "pgrep -a sandkasten || true"
	argv := wslArgs(distro, innerCmd)
	cmd := exec.Command("wsl", argv...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("wsl status: %w", err)
	}
	lines := strings.TrimSpace(string(out))
	if lines == "" {
		fmt.Println("Daemon is not running in WSL.")
		return nil
	}
	fmt.Println("Daemon process(es) in WSL:")
	fmt.Println(lines)
	return nil
}

func runStop(args []string) error {
	distro, _, binaryPath := parseFlags(args)
	if binaryPath == "" {
		binaryPath = "sandkasten"
	}
	innerCmd := "sudo " + binaryPath + " stop 2>/dev/null || sudo pkill -f 'sandkasten.*daemon' || true"
	argv := wslArgs(distro, innerCmd)
	cmd := exec.Command("wsl", argv...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("wsl stop: %w", err)
	}
	fmt.Println("Daemon stop requested in WSL.")
	return nil
}
