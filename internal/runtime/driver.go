// Package runtime defines the abstraction layer for sandbox lifecycle management.
//
// The runtime package provides the Driver interface, which implementations (e.g. linux) use to:
//   - Create isolated sessions with overlayfs rootfs, cgroups, and namespaces
//   - Execute commands inside sessions via the runner Unix socket
//   - Destroy sessions and clean up resources
//
// Communication flow:
//
//	Daemon → Driver.Create() → overlayfs + cgroup + nsinit → Runner (PID 1)
//	Daemon → Driver.Exec() → Unix socket → Runner → bash
package runtime

import (
	"context"

	"github.com/p-arndt/sandkasten/protocol"
)

// CreateOpts holds parameters for creating a new sandbox session.
// SessionID uniquely identifies the session. Image names the rootfs (e.g. "python").
// WorkspaceID, if non-empty, causes the workspace directory to be bind-mounted at /workspace.
type CreateOpts struct {
	SessionID   string
	Image       string
	WorkspaceID string
}

// SessionInfo is returned after a successful Create and contains all handles needed
// to communicate with and manage the session. InitPID is the PID of the nsinit/runner
// process on the host. RunnerSock is the path to the Unix socket (via /proc/<pid>/root).
type SessionInfo struct {
	SessionID  string
	InitPID    int
	CgroupPath string
	Mnt        string
	RunnerSock string
}

// Driver is the interface that platform-specific runtimes must implement.
// On Linux, the implementation uses overlayfs, cgroups v2, and namespaces.
type Driver interface {
	// Create builds a new sandbox: overlayfs rootfs, cgroup, namespaces, and launches runner.
	Create(ctx context.Context, opts CreateOpts) (*SessionInfo, error)
	// Exec sends a Request to the runner over the session's Unix socket and returns the Response.
	Exec(ctx context.Context, sessionID string, req protocol.Request) (*protocol.Response, error)
	// Destroy terminates the session, removes the cgroup, and unmounts the rootfs.
	Destroy(ctx context.Context, sessionID string) error
	// IsRunning reports whether the session's init process is still alive.
	IsRunning(ctx context.Context, sessionID string) (bool, error)
	// Stats returns memory/CPU usage from the session's cgroup.
	Stats(ctx context.Context, sessionID string) (*protocol.SessionStats, error)
	// Ping verifies the runtime is operational (e.g. cgroup v2 available).
	Ping(ctx context.Context) error
	// Close releases any resources held by the driver.
	Close() error

	// MountWorkspace bind-mounts the workspace directory into /workspace of an existing session.
	// Used when acquiring a pooled session for a request with workspace_id. The workspace
	// is mounted via nsenter into the session's mount namespace.
	MountWorkspace(ctx context.Context, sessionID string, workspaceID string) error
}
