package main

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/p-arndt/sandkasten/protocol"
)

func (s *server) handleExec(req protocol.Request) protocol.Response {
	s.mu.Lock()
	defer s.mu.Unlock()

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
	return s.waitForCompletion(req.ID, beginMarker, endMarker, timeout, start)
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
func (s *server) waitForCompletion(requestID, beginMarker, endMarker string, timeout time.Duration, start time.Time) protocol.Response {
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
				return buildExecResponse(requestID, full, beginMarker, endMarker, start)
			}

			// Guard against runaway output
			if len(accumulated) > protocol.MaxOutputBytes*2 {
				accumulated = accumulated[len(accumulated)-protocol.MaxOutputBytes:]
			}
		}
	}
}

// buildExecResponse parses command output and builds response.
func buildExecResponse(requestID, full, beginMarker, endMarker string, start time.Time) protocol.Response {
	exitCode, cwd := parseEndSentinel(full, endMarker)
	output := extractOutput(full, beginMarker, endMarker)
	output = removeSentinelLines(output)
	output = stripANSI(output)
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
// Handles ESC (0x1b) and the single-byte CSI introducer 0x9b.
func stripANSI(s string) string {
	const (
		esc = '\x1b'
		csi = '\x9b' // single-byte CSI introducer (equivalent to ESC [)
	)
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		if s[i] != esc && s[i] != csi {
			b.WriteByte(s[i])
			i++
			continue
		}
		if s[i] == csi {
			// Single-byte CSI: consume 0x9b then same as CSI sequence
			i++
			for i < len(s) && s[i] >= 0x20 && s[i] <= 0x3f {
				i++
			}
			if i < len(s) && s[i] >= 0x40 && s[i] <= 0x7e {
				i++
			}
			continue
		}
		// ESC
		i++
		if i >= len(s) {
			break
		}
		switch s[i] {
		case '[':
			// CSI: ESC [ ... intermediate (0x20-0x2f) and parameters (0x30-0x3f) ... final (0x40-0x7e)
			i++
			for i < len(s) && s[i] >= 0x20 && s[i] <= 0x3f {
				i++
			}
			if i < len(s) && s[i] >= 0x40 && s[i] <= 0x7e {
				i++
			}
		case ']':
			// OSC: ESC ] ... BEL (0x07) or ST (ESC \)
			i++
			for i < len(s) {
				if s[i] == '\x07' {
					i++
					break
				}
				if i+1 < len(s) && s[i] == esc && s[i+1] == '\\' {
					i += 2
					break
				}
				i++
			}
		case 'P', 'X', '^', '_':
			// DCS, SOS, PM, APC: skip until ST (ESC \)
			i++
			for i+1 < len(s) {
				if s[i] == esc && s[i+1] == '\\' {
					i += 2
					break
				}
				i++
			}
		default:
			if s[i] >= 0x40 && s[i] <= 0x5f {
				i++
			}
		}
	}
	return b.String()
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
