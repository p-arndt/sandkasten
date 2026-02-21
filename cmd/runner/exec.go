package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/p-arndt/sandkasten/protocol"
)

var ansiRegex = regexp.MustCompile("[\u001b\u009b][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))")

func (s *server) handleExec(req protocol.Request) protocol.Response {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(req.Cmd) > protocol.MaxExecInlineCmdBytes {
		return errorResponse(req.ID, fmt.Sprintf("command too large: %d bytes (max %d); use staged exec path", len(req.Cmd), protocol.MaxExecInlineCmdBytes))
	}

	// Stateless mode: direct exec, no PTY
	if s.ptmx == nil {
		return s.handleExecStateless(req)
	}

	timeout := getTimeout(req.TimeoutMs)

	// Drain any pending output
	s.shellBuf.ReadAndReset()

	// Build and execute command
	beginMarker, endMarker := buildSentinels(req.ID)
	cmdStr := buildWrappedCommand(beginMarker, endMarker, req.Cmd)

	start := time.Now()
	if _, err := s.ptmx.Write([]byte(cmdStr)); err != nil {
		return errorResponse(req.ID, "write to pty: "+err.Error())
	}

	// Wait for command completion
	return s.waitForCompletion(req.ID, beginMarker, endMarker, req.RawOutput, timeout, start)
}

// handleExecStateless runs the command directly via exec.Command, no persistent shell.
// Saves memory and startup time. No cwd/env persistence between execs.
func (s *server) handleExecStateless(req protocol.Request) protocol.Response {
	timeout := getTimeout(req.TimeoutMs)
	shell := findShell()

	cmd := exec.Command(shell, "-c", req.Cmd)
	cmd.Dir = "/workspace"
	cmd.Env = append(cmd.Env,
		"PATH=/usr/local/bin:/usr/bin:/bin",
		"HOME=/home/sandbox",
		"TERM=xterm",
		"LANG=C.UTF-8",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	if err := cmd.Start(); err != nil {
		return errorResponse(req.ID, "exec start: "+err.Error())
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	var execErr error
	select {
	case execErr = <-done:
		// Command finished
	case <-time.After(timeout):
		cmd.Process.Kill()
		<-done
		return timeoutResponse(req.ID, timeout, start)
	}

	output := stdout.String() + stderr.String()
	if !req.RawOutput {
		output = normalizeLineEndings(output)
		output = stripANSI(output)
	}
	truncated := truncateOutput(&output)

	exitCode := 0
	if execErr != nil {
		if exitErr, ok := execErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
			output = output + "\nexec error: " + execErr.Error()
		}
	}

	return protocol.Response{
		ID:         req.ID,
		Type:       protocol.ResponseExec,
		ExitCode:   exitCode,
		Cwd:        "/workspace",
		Output:     output,
		Truncated:  truncated,
		DurationMs: time.Since(start).Milliseconds(),
	}
}

// getTimeout returns command timeout with 30s default.
func getTimeout(timeoutMs int) time.Duration {
	timeout := time.Duration(timeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return timeout
}

// buildSentinels creates unique begin/end markers for command output.
func buildSentinels(requestID string) (begin, end string) {
	begin = fmt.Sprintf("%s:%s", protocol.SentinelBegin, requestID)
	end = fmt.Sprintf("%s:%s", protocol.SentinelEnd, requestID)
	return
}

// buildWrappedCommand wraps user command with sentinels for output capture.
// The command is base64-encoded so it is never interpreted as part of the wrapper
// script, preventing injection via newlines or shell metacharacters.
func buildWrappedCommand(beginMarker, endMarker, cmd string) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(cmd))
	return fmt.Sprintf(
		"printf '%%s\\n' '%s'\n__b64='%s'; echo \"$__b64\" | base64 -d | bash\nprintf '\\n%s:%%d:%%s\\n' \"$?\" \"$PWD\"\n",
		beginMarker, encoded, endMarker,
	)
}

// endSentinelLine is the real end line we print (newline + marker); the PTY also
// echoes the printf command line, so we must look for this to avoid matching the echo.
func endSentinelLine(endMarker string) string { return "\n" + endMarker }

// waitForCompletion polls for command output until end sentinel or timeout.
func (s *server) waitForCompletion(requestID, beginMarker, endMarker string, rawOutput bool, timeout time.Duration, start time.Time) protocol.Response {
	deadline := time.After(timeout)
	var accumulated []byte
	endLine := endSentinelLine(endMarker)

	for {
		select {
		case <-deadline:
			return timeoutResponse(requestID, timeout, start)

		case <-time.After(50 * time.Millisecond):
			chunk := s.shellBuf.ReadAndReset()
			if len(chunk) > 0 {
				accumulated = append(accumulated, chunk...)
			}

			full := string(accumulated)
			if idx := strings.Index(full, endLine); idx >= 0 {
				return buildExecResponse(requestID, full, beginMarker, endMarker, rawOutput, start)
			}

			// Guard against runaway output
			if len(accumulated) > protocol.MaxOutputBytes*2 {
				// Keep the first chunk (containing beginMarker) and the last chunk (for endMarker)
				firstPart := accumulated[:protocol.MaxOutputBytes]
				lastPart := accumulated[len(accumulated)-4096:]

				newAccumulated := make([]byte, 0, len(firstPart)+len(lastPart))
				newAccumulated = append(newAccumulated, firstPart...)
				newAccumulated = append(newAccumulated, lastPart...)
				accumulated = newAccumulated
			}
		}
	}
}

// buildExecResponse parses command output and builds response.
func buildExecResponse(requestID, full, beginMarker, endMarker string, rawOutput bool, start time.Time) protocol.Response {
	exitCode, cwd := parseEndSentinel(full, endMarker)
	output := extractOutput(full, beginMarker, endMarker)
	if rawOutput {
		output = extractRawOutput(full, beginMarker, endMarker)
	}
	if !rawOutput {
		output = removeSentinelLines(output)
		output = normalizeLineEndings(output)
		output = stripANSI(output)
	}
	truncated := truncateOutput(&output)

	return protocol.Response{
		ID:         requestID,
		Type:       protocol.ResponseExec,
		ExitCode:   exitCode,
		Cwd:        cwd,
		Output:     output,
		Truncated:  truncated,
		DurationMs: time.Since(start).Milliseconds(),
	}
}

func normalizeLineEndings(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "")
}

// parseEndSentinel extracts exit code and cwd from end marker line.
func parseEndSentinel(full, endMarker string) (exitCode int, cwd string) {
	exitCode = 0
	cwd = "/workspace"

	// Find the real sentinel line (printf output), not the echoed command
	idx := strings.Index(full, endSentinelLine(endMarker))
	if idx < 0 {
		return
	}
	// Line is "\n" + endMarker + ":" + exitCode + ":" + cwd; take the full line
	endLine := full[idx+1:]
	if newlineIdx := strings.Index(endLine, "\n"); newlineIdx >= 0 {
		endLine = endLine[:newlineIdx]
	}
	endLine = strings.TrimRight(endLine, "\r")

	// Parse: __SANDKASTEN_END__:<id>:<exit_code>:<cwd>
	parts := strings.SplitN(endLine, ":", 5)
	if len(parts) >= 3 {
		fmt.Sscanf(parts[2], "%d", &exitCode)
	}
	if len(parts) >= 4 {
		cwd = parts[3]
	}

	return
}

// removeSentinelLines drops any line containing Sandkasten sentinels (in case they leak into output).
func removeSentinelLines(s string) string {
	lines := strings.Split(s, "\n")
	var out []string
	for _, line := range lines {
		if strings.Contains(line, protocol.SentinelBegin) || strings.Contains(line, protocol.SentinelEnd) {
			continue
		}
		out = append(out, line)
	}
	return strings.TrimRight(strings.Join(out, "\n"), "\n")
}

// stripANSI removes ANSI escape sequences (CSI, OSC, DCS, etc.) so output is plain text.
func stripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

// extractOutput extracts command output between begin and end markers.
// We look for the real sentinel lines (the printf output), not the echoed command
// lines, so that "printf '...'" never appears in the returned output.
func extractOutput(full, beginMarker, endMarker string) string {
	output := full

	// Skip to after the real begin marker line (printf output is marker then \n)
	beginLine := "\n" + beginMarker
	if idx := strings.Index(output, beginLine); idx >= 0 {
		output = output[idx+len(beginLine):]
		if nl := strings.Index(output, "\n"); nl >= 0 {
			output = output[nl+1:]
		}
	} else if strings.HasPrefix(output, beginMarker) {
		output = output[len(beginMarker):]
		if nl := strings.Index(output, "\n"); nl >= 0 {
			output = output[nl+1:]
		}
	}

	// Trim at the real end sentinel line (newline + marker), not the echoed "printf '\n..."
	endLine := endSentinelLine(endMarker)
	if endIdx := strings.Index(output, endLine); endIdx >= 0 {
		output = output[:endIdx]
	}

	return strings.TrimRight(output, "\n")
}

func extractRawOutput(full, beginMarker, endMarker string) string {
	output := full

	beginLine := "\n" + beginMarker
	if idx := strings.Index(output, beginLine); idx >= 0 {
		output = output[idx+len(beginLine):]
		if nl := strings.Index(output, "\n"); nl >= 0 {
			output = output[nl+1:]
		}
	} else if strings.HasPrefix(output, beginMarker) {
		output = output[len(beginMarker):]
		if nl := strings.Index(output, "\n"); nl >= 0 {
			output = output[nl+1:]
		}
	}

	endLine := endSentinelLine(endMarker)
	if endIdx := strings.Index(output, endLine); endIdx >= 0 {
		output = output[:endIdx]
	}

	return output
}

// truncateOutput limits output size to protocol maximum.
func truncateOutput(output *string) bool {
	if len(*output) > protocol.MaxOutputBytes {
		*output = (*output)[:protocol.MaxOutputBytes]
		return true
	}
	return false
}

// timeoutResponse creates timeout error response.
func timeoutResponse(requestID string, timeout time.Duration, start time.Time) protocol.Response {
	return protocol.Response{
		ID:         requestID,
		Type:       protocol.ResponseExec,
		ExitCode:   -1,
		Output:     "timeout: command exceeded " + timeout.String(),
		Truncated:  false,
		DurationMs: time.Since(start).Milliseconds(),
	}
}

// errorResponse creates generic error response.
func errorResponse(requestID, message string) protocol.Response {
	return protocol.Response{
		ID:    requestID,
		Type:  protocol.ResponseError,
		Error: message,
	}
}
