package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
	"github.com/p-arndt/sandkasten/protocol"
)

const socketPath = "/run/runner.sock"

func main() {
	// Client mode: connect to running runner, send request, print response.
	if len(os.Args) > 1 && os.Args[1] == "--client" {
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "usage: runner --client '<json>'\n")
			os.Exit(1)
		}
		runClient(os.Args[2])
		return
	}

	// Server mode: start shell, listen on socket.
	runServer()
}

func runClient(reqJSON string) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	_, err = conn.Write([]byte(reqJSON + "\n"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "write: %v\n", err)
		os.Exit(1)
	}

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, protocol.MaxOutputBytes+4096), protocol.MaxOutputBytes+4096)
	if scanner.Scan() {
		fmt.Println(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "read: %v\n", err)
		os.Exit(1)
	}
}

type server struct {
	ptmx     *os.File
	mu       sync.Mutex // serializes exec commands
	shellBuf *ringBuffer
}

// ringBuffer is a simple bounded byte buffer for PTY output.
type ringBuffer struct {
	mu   sync.Mutex
	data []byte
	cap  int
}

func newRingBuffer(cap int) *ringBuffer {
	return &ringBuffer{data: make([]byte, 0, cap), cap: cap}
}

func (rb *ringBuffer) Write(p []byte) (int, error) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.data = append(rb.data, p...)
	if len(rb.data) > rb.cap {
		rb.data = rb.data[len(rb.data)-rb.cap:]
	}
	return len(p), nil
}

func (rb *ringBuffer) ReadAndReset() []byte {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	out := make([]byte, len(rb.data))
	copy(out, rb.data)
	rb.data = rb.data[:0]
	return out
}

func runServer() {
	// Find a shell.
	shell := "/bin/bash"
	if _, err := os.Stat(shell); err != nil {
		shell = "/bin/sh"
	}

	// Start shell with a PTY.
	cmd := exec.Command(shell, "-l")
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"PS1=$ ",     // simple prompt to reduce noise
		"PS2=> ",     // continuation prompt
		"HISTFILE=",  // no history file
	)

	ptmx, err := pty.Start(cmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pty start: %v\n", err)
		os.Exit(1)
	}
	defer ptmx.Close()

	// Set PTY size.
	pty.Setsize(ptmx, &pty.Winsize{Rows: 40, Cols: 120})

	srv := &server{
		ptmx:     ptmx,
		shellBuf: newRingBuffer(protocol.MaxOutputBytes),
	}

	// Background goroutine reads all PTY output into the ring buffer.
	go func() {
		buf := make([]byte, 32*1024)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				srv.shellBuf.Write(buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()

	// Wait for shell to be ready.
	time.Sleep(200 * time.Millisecond)
	srv.shellBuf.ReadAndReset() // discard startup output

	// Clean up old socket.
	os.Remove(socketPath)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "listen: %v\n", err)
		os.Exit(1)
	}
	defer listener.Close()
	os.Chmod(socketPath, 0666)

	// Signal ready.
	readyMsg, _ := json.Marshal(protocol.Response{Type: protocol.ResponseReady})
	fmt.Println(string(readyMsg))

	// Handle shutdown.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sigCh
		listener.Close()
		cmd.Process.Signal(syscall.SIGTERM)
		os.Exit(0)
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			return // listener closed
		}
		go srv.handleConn(conn)
	}
}

func (s *server) handleConn(conn net.Conn) {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, protocol.MaxOutputBytes+4096), protocol.MaxOutputBytes+4096)
	if !scanner.Scan() {
		return
	}

	var req protocol.Request
	if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
		s.writeResponse(conn, protocol.Response{
			ID:    req.ID,
			Type:  protocol.ResponseError,
			Error: "invalid request: " + err.Error(),
		})
		return
	}

	var resp protocol.Response
	switch req.Type {
	case protocol.RequestExec:
		resp = s.handleExec(req)
	case protocol.RequestWrite:
		resp = s.handleWrite(req)
	case protocol.RequestRead:
		resp = s.handleRead(req)
	default:
		resp = protocol.Response{
			ID:    req.ID,
			Type:  protocol.ResponseError,
			Error: fmt.Sprintf("unknown request type: %s", req.Type),
		}
	}

	s.writeResponse(conn, resp)
}

func (s *server) writeResponse(conn net.Conn, resp protocol.Response) {
	data, _ := json.Marshal(resp)
	conn.Write(append(data, '\n'))
}

func (s *server) handleExec(req protocol.Request) protocol.Response {
	s.mu.Lock()
	defer s.mu.Unlock()

	timeout := time.Duration(req.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	// Drain any pending output.
	s.shellBuf.ReadAndReset()

	beginMarker := fmt.Sprintf("%s:%s", protocol.SentinelBegin, req.ID)
	endMarker := fmt.Sprintf("%s:%s", protocol.SentinelEnd, req.ID)

	// Write the command wrapped in sentinels.
	cmdStr := fmt.Sprintf(
		"echo '%s'\n%s\nprintf '\\n%s:%%d:%%s\\n' \"$?\" \"$PWD\"\n",
		beginMarker, req.Cmd, endMarker,
	)

	start := time.Now()
	_, err := s.ptmx.Write([]byte(cmdStr))
	if err != nil {
		return protocol.Response{
			ID:    req.ID,
			Type:  protocol.ResponseError,
			Error: "write to pty: " + err.Error(),
		}
	}

	// Read output until we see the end sentinel or timeout.
	deadline := time.After(timeout)
	var accumulated []byte

	for {
		select {
		case <-deadline:
			return protocol.Response{
				ID:        req.ID,
				Type:      protocol.ResponseExec,
				ExitCode:  -1,
				Output:    "timeout: command exceeded " + timeout.String(),
				Truncated: false,
				DurationMs: time.Since(start).Milliseconds(),
			}
		case <-time.After(50 * time.Millisecond):
			chunk := s.shellBuf.ReadAndReset()
			if len(chunk) > 0 {
				accumulated = append(accumulated, chunk...)
			}

			full := string(accumulated)
			if idx := strings.Index(full, endMarker); idx >= 0 {
				// Parse: __SANDKASTEN_END__:<id>:<exit_code>:<cwd>
				endLine := full[idx:]
				newlineIdx := strings.Index(endLine, "\n")
				if newlineIdx < 0 {
					newlineIdx = len(endLine)
				}
				endLine = endLine[:newlineIdx]

				parts := strings.SplitN(endLine, ":", 5)
				// parts: [__SANDKASTEN_END__, <id>, <exit_code>, <cwd>]
				exitCode := 0
				cwd := "/workspace"
				if len(parts) >= 4 {
					fmt.Sscanf(parts[3], "%d", &exitCode)
				}
				if len(parts) >= 5 {
					cwd = parts[4]
				}

				// Extract output between begin and end markers.
				output := full
				if beginIdx := strings.Index(output, beginMarker); beginIdx >= 0 {
					// Skip the begin marker line.
					afterBegin := output[beginIdx+len(beginMarker):]
					if nlIdx := strings.Index(afterBegin, "\n"); nlIdx >= 0 {
						afterBegin = afterBegin[nlIdx+1:]
					}
					output = afterBegin
				}
				if endIdx := strings.Index(output, endMarker); endIdx >= 0 {
					output = output[:endIdx]
				}
				// Trim the echo of our printf command if visible.
				output = strings.TrimRight(output, "\n")

				truncated := false
				if len(output) > protocol.MaxOutputBytes {
					output = output[:protocol.MaxOutputBytes]
					truncated = true
				}

				return protocol.Response{
					ID:         req.ID,
					Type:       protocol.ResponseExec,
					ExitCode:   exitCode,
					Cwd:        cwd,
					Output:     output,
					Truncated:  truncated,
					DurationMs: time.Since(start).Milliseconds(),
				}
			}

			// Guard against runaway output.
			if len(accumulated) > protocol.MaxOutputBytes*2 {
				accumulated = accumulated[len(accumulated)-protocol.MaxOutputBytes:]
			}
		}
	}
}

func (s *server) handleWrite(req protocol.Request) protocol.Response {
	path := sanitizePath(req.Path)

	var content []byte
	var err error
	if req.ContentBase64 != "" {
		content, err = base64.StdEncoding.DecodeString(req.ContentBase64)
		if err != nil {
			return protocol.Response{
				ID:    req.ID,
				Type:  protocol.ResponseError,
				Error: "invalid base64: " + err.Error(),
			}
		}
	} else {
		content = []byte(req.Text)
	}

	// Ensure parent directory exists.
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return protocol.Response{
			ID:    req.ID,
			Type:  protocol.ResponseError,
			Error: "mkdir: " + err.Error(),
		}
	}

	if err := os.WriteFile(path, content, 0644); err != nil {
		return protocol.Response{
			ID:    req.ID,
			Type:  protocol.ResponseError,
			Error: "write: " + err.Error(),
		}
	}

	return protocol.Response{
		ID:   req.ID,
		Type: protocol.ResponseWrite,
		OK:   true,
	}
}

func (s *server) handleRead(req protocol.Request) protocol.Response {
	path := sanitizePath(req.Path)

	maxBytes := req.MaxBytes
	if maxBytes <= 0 {
		maxBytes = protocol.DefaultMaxReadBytes
	}

	f, err := os.Open(path)
	if err != nil {
		return protocol.Response{
			ID:    req.ID,
			Type:  protocol.ResponseError,
			Error: "open: " + err.Error(),
		}
	}
	defer f.Close()

	buf := make([]byte, maxBytes+1)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return protocol.Response{
			ID:    req.ID,
			Type:  protocol.ResponseError,
			Error: "read: " + err.Error(),
		}
	}

	truncated := n > maxBytes
	if truncated {
		n = maxBytes
	}

	return protocol.Response{
		ID:            req.ID,
		Type:          protocol.ResponseRead,
		ContentBase64: base64.StdEncoding.EncodeToString(buf[:n]),
		Truncated:     truncated,
	}
}

// sanitizePath ensures paths are absolute; relative paths resolve from /workspace.
func sanitizePath(p string) string {
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Clean(filepath.Join("/workspace", p))
}
