package main

import (
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
func buildWrappedCommand(beginMarker, endMarker, cmd string) string {
	return fmt.Sprintf(
		"printf '%%s\\n' '%s'\n%s\nprintf '\\n%s:%%d:%%s\\n' \"$?\" \"$PWD\"\n",
		beginMarker, cmd, endMarker,
	)
}

// waitForCompletion polls for command output until end sentinel or timeout.
func (s *server) waitForCompletion(requestID, beginMarker, endMarker string, timeout time.Duration, start time.Time) protocol.Response {
	deadline := time.After(timeout)
	var accumulated []byte

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
			if idx := strings.Index(full, endMarker); idx >= 0 {
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

	idx := strings.Index(full, endMarker)
	if idx < 0 {
		return
	}

	endLine := full[idx:]
	if newlineIdx := strings.Index(endLine, "\n"); newlineIdx >= 0 {
		endLine = endLine[:newlineIdx]
	}

	// Parse: __SANDKASTEN_END__:<id>:<exit_code>:<cwd>
	parts := strings.SplitN(endLine, ":", 5)
	if len(parts) >= 4 {
		fmt.Sscanf(parts[3], "%d", &exitCode)
	}
	if len(parts) >= 5 {
		cwd = parts[4]
	}

	return
}

// extractOutput extracts command output between begin and end markers.
func extractOutput(full, beginMarker, endMarker string) string {
	output := full

	// Skip begin marker line
	if beginIdx := strings.Index(output, beginMarker); beginIdx >= 0 {
		afterBegin := output[beginIdx+len(beginMarker):]
		if nlIdx := strings.Index(afterBegin, "\n"); nlIdx >= 0 {
			afterBegin = afterBegin[nlIdx+1:]
		}
		output = afterBegin
	}

	// Trim at end marker
	if endIdx := strings.Index(output, endMarker); endIdx >= 0 {
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
