package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
	"github.com/p-arndt/sandkasten/protocol"
)

type server struct {
	ptmx     *os.File
	mu       sync.Mutex // serializes exec commands
	shellBuf *ringBuffer
}

func runServer() {
	shell := findShell()
	ptmx, cmd := startShell(shell)
	defer ptmx.Close()

	srv := &server{
		ptmx:     ptmx,
		shellBuf: newRingBuffer(protocol.MaxOutputBytes),
	}

	startPTYReader(srv, ptmx)
	waitForShellReady(srv)

	listener := setupSocket()
	defer listener.Close()

	signalReady()
	handleShutdown(listener, cmd)

	serveRequests(srv, listener)
}

// findShell locates bash or sh on the system.
func findShell() string {
	shell := "/bin/bash"
	if _, err := os.Stat(shell); err != nil {
		shell = "/bin/sh"
	}
	return shell
}

// startShell starts shell with PTY.
func startShell(shell string) (*os.File, *exec.Cmd) {
	cmd := exec.Command(shell, "-l")
	cmd.Dir = "/workspace"
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"PS1=$ ",    // simple prompt to reduce noise
		"PS2=> ",    // continuation prompt
		"HISTFILE=", // no history file
	)

	ptmx, err := pty.Start(cmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pty start: %v\n", err)
		os.Exit(1)
	}

	// Set PTY size
	pty.Setsize(ptmx, &pty.Winsize{Rows: 40, Cols: 120})

	return ptmx, cmd
}

// startPTYReader launches background goroutine to read PTY output.
func startPTYReader(srv *server, ptmx *os.File) {
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
}

// waitForShellReady waits for shell initialization and discards startup output.
func waitForShellReady(srv *server) {
	time.Sleep(200 * time.Millisecond)
	srv.shellBuf.ReadAndReset()
}

// setupSocket creates Unix socket listener.
func setupSocket() net.Listener {
	os.Remove(socketPath)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "listen: %v\n", err)
		os.Exit(1)
	}

	os.Chmod(socketPath, 0600) // Owner-only access for security
	return listener
}

// signalReady prints ready message to stdout.
func signalReady() {
	readyMsg, err := json.Marshal(protocol.Response{Type: protocol.ResponseReady})
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal ready message: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(readyMsg))
}

// handleShutdown sets up signal handler for graceful shutdown.
func handleShutdown(listener net.Listener, cmd *exec.Cmd) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sigCh
		listener.Close()
		cmd.Process.Signal(syscall.SIGTERM)
		os.Exit(0)
	}()
}

// serveRequests accepts and handles incoming connections.
func serveRequests(srv *server, listener net.Listener) {
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

	req, err := readRequest(conn)
	if err != nil {
		s.writeResponse(conn, protocol.Response{
			ID:    "",
			Type:  protocol.ResponseError,
			Error: "invalid request: " + err.Error(),
		})
		return
	}

	resp := s.routeRequest(req)
	s.writeResponse(conn, resp)
}

// readRequest reads and parses JSON request from connection.
func readRequest(conn net.Conn) (protocol.Request, error) {
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, protocol.MaxOutputBytes+4096), protocol.MaxOutputBytes+4096)

	if !scanner.Scan() {
		return protocol.Request{}, fmt.Errorf("no data")
	}

	var req protocol.Request
	err := json.Unmarshal(scanner.Bytes(), &req)
	return req, err
}

// routeRequest dispatches request to appropriate handler.
func (s *server) routeRequest(req protocol.Request) protocol.Response {
	switch req.Type {
	case protocol.RequestExec:
		return s.handleExec(req)
	case protocol.RequestWrite:
		return s.handleWrite(req)
	case protocol.RequestRead:
		return s.handleRead(req)
	default:
		return protocol.Response{
			ID:    req.ID,
			Type:  protocol.ResponseError,
			Error: fmt.Sprintf("unknown request type: %s", req.Type),
		}
	}
}

func (s *server) writeResponse(conn net.Conn, resp protocol.Response) {
	data, err := json.Marshal(resp)
	if err != nil {
		// Fallback error response if marshaling fails
		fmt.Fprintf(os.Stderr, "marshal response: %v\n", err)
		data = []byte(`{"id":"` + resp.ID + `","type":"error","error":"internal marshaling error"}`)
	}
	conn.Write(append(data, '\n'))
}
