package main

import (
	"encoding/base64"
	"io"
	"os"
	"path/filepath"

	"github.com/p-arndt/sandkasten/protocol"
)

func (s *server) handleWrite(req protocol.Request) protocol.Response {
	path := sanitizePath(req.Path)

	content, err := decodeContent(req)
	if err != nil {
		return protocol.Response{
			ID:    req.ID,
			Type:  protocol.ResponseError,
			Error: "invalid base64: " + err.Error(),
		}
	}

	if err := ensureParentDir(path); err != nil {
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

// decodeContent extracts content from request (base64 or text).
func decodeContent(req protocol.Request) ([]byte, error) {
	if req.ContentBase64 != "" {
		return base64.StdEncoding.DecodeString(req.ContentBase64)
	}
	return []byte(req.Text), nil
}

// ensureParentDir creates parent directory if it doesn't exist.
func ensureParentDir(path string) error {
	dir := filepath.Dir(path)
	return os.MkdirAll(dir, 0755)
}
