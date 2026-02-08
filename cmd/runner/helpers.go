package main

import (
	"path/filepath"
	"strings"
	"sync"
)

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

// sanitizePath ensures all paths resolve within /workspace.
// Prevents path traversal attacks by verifying the cleaned path stays within bounds.
func sanitizePath(p string) string {
	// Always resolve relative to /workspace
	target := p
	if !filepath.IsAbs(p) {
		target = filepath.Join("/workspace", p)
	}
	target = filepath.Clean(target)

	// Verify result is within /workspace (prevent escaping via absolute paths or ..)
	if !strings.HasPrefix(target, "/workspace/") && target != "/workspace" {
		return "/workspace" // Safe fallback for invalid paths
	}
	return target
}
