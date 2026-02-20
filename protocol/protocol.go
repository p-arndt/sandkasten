// Package protocol defines the JSON-line message types exchanged between
// the sandbox daemon and the runner binary inside containers.
package protocol

// Request is the envelope sent from daemon → runner.
type Request struct {
	ID   string      `json:"id"`
	Type RequestType `json:"type"`

	// Exec fields
	Cmd       string `json:"cmd,omitempty"`
	TimeoutMs int    `json:"timeout_ms,omitempty"`

	// Write fields
	Path          string `json:"path,omitempty"`
	ContentBase64 string `json:"content_base64,omitempty"`
	Text          string `json:"text,omitempty"`

	// Read fields
	MaxBytes int `json:"max_bytes,omitempty"`
}

type RequestType string

const (
	RequestExec       RequestType = "exec"
	RequestExecStream RequestType = "exec_stream" // streaming exec
	RequestWrite      RequestType = "write"
	RequestRead       RequestType = "read"
)

// Response is the envelope sent from runner → daemon.
type Response struct {
	ID   string       `json:"id"`
	Type ResponseType `json:"type"`

	// Exec response fields
	ExitCode   int    `json:"exit_code,omitempty"`
	Cwd        string `json:"cwd,omitempty"`
	Output     string `json:"output,omitempty"`
	Truncated  bool   `json:"truncated,omitempty"`
	DurationMs int64  `json:"duration_ms,omitempty"`

	// Streaming exec fields (for exec_chunk)
	Chunk     string `json:"chunk,omitempty"`     // output chunk
	Timestamp int64  `json:"timestamp,omitempty"` // unix timestamp ms

	// Write response fields
	OK bool `json:"ok,omitempty"`

	// Read response fields
	ContentBase64 string `json:"content_base64,omitempty"`

	// Error fields
	Error string `json:"error,omitempty"`
}

type ResponseType string

const (
	ResponseExec      ResponseType = "exec"
	ResponseExecChunk ResponseType = "exec_chunk" // streaming output chunk
	ResponseExecDone  ResponseType = "exec_done"  // streaming complete
	ResponseWrite     ResponseType = "write"
	ResponseRead      ResponseType = "read"
	ResponseError     ResponseType = "error"
	ResponseReady     ResponseType = "ready"
)

// ReadyMessage is emitted by the runner on startup.
type ReadyMessage struct {
	Type ResponseType `json:"type"` // always "ready"
}

// MaxOutputBytes is the default cap on exec output.
const MaxOutputBytes = 5 * 1024 * 1024 // 5 MB

// DefaultMaxReadBytes is the default cap on file reads.
const DefaultMaxReadBytes = 10 * 1024 * 1024 // 10 MB

const DataDirPrefix = "/var/lib/sandkasten/"
const SessionDirPrefix = DataDirPrefix + "sessions/"
const ImageDirPrefix = DataDirPrefix + "images/"
const WorkspaceDirPrefix = DataDirPrefix + "workspaces/"
const RunnerSocketName = "runner.sock"
const RunDirName = "run"

const WorkspaceVolumePrefix = "sandkasten-ws-" // legacy Docker volume prefix

func RunnerSocketPath(sessionID string) string {
	return SessionDirPrefix + sessionID + "/" + RunDirName + "/" + RunnerSocketName
}

func SessionDir(sessionID string) string {
	return SessionDirPrefix + sessionID
}

func ImageRootfsPath(image string) string {
	return ImageDirPrefix + image + "/rootfs"
}

func WorkspacePath(workspaceID string) string {
	return WorkspaceDirPrefix + workspaceID
}

type SessionState struct {
	SessionID  string `json:"session_id"`
	InitPID    int    `json:"init_pid"`
	CgroupPath string `json:"cgroup_path"`
	Mnt        string `json:"mnt"`
	RunnerSock string `json:"runner_sock"`
}

type SessionStats struct {
	MemoryBytes  int64 `json:"memory_bytes"`
	MemoryLimit  int64 `json:"memory_limit,omitempty"`
	CPUUsageUsec int64 `json:"cpu_usage_usec"`
}

// SentinelBegin is the marker written before a command.
const SentinelBegin = "__SANDKASTEN_BEGIN__"

// SentinelEnd is the marker written after a command completes.
const SentinelEnd = "__SANDKASTEN_END__"
