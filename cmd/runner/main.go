package main

import (
	"bufio"
	"fmt"
	"net"
	"os"

	"github.com/p-arndt/sandkasten/protocol"
)

const socketPath = "/run/sandkasten/runner.sock"

func main() {
	// Client mode: connect to running runner, send request, print response
	if len(os.Args) > 1 && os.Args[1] == "--client" {
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "usage: runner --client '<json>'\n")
			os.Exit(1)
		}
		runClient(os.Args[2])
		return
	}

	// Server mode: start shell (or stateless direct exec), listen on socket
	if isStatelessMode() {
		runStatelessServer()
	} else {
		runServer()
	}
}

func runClient(reqJSON string) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	if err := sendRequest(conn, reqJSON); err != nil {
		fmt.Fprintf(os.Stderr, "write: %v\n", err)
		os.Exit(1)
	}

	if err := readResponse(conn); err != nil {
		fmt.Fprintf(os.Stderr, "read: %v\n", err)
		os.Exit(1)
	}
}

func sendRequest(conn net.Conn, reqJSON string) error {
	_, err := conn.Write([]byte(reqJSON + "\n"))
	return err
}

func readResponse(conn net.Conn) error {
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, protocol.MaxOutputBytes+4096), protocol.MaxOutputBytes+4096)

	if scanner.Scan() {
		fmt.Println(scanner.Text())
	}

	return scanner.Err()
}
