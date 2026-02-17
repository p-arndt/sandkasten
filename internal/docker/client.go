package docker

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-units"

	"github.com/p-arndt/sandkasten/internal/config"
	"github.com/p-arndt/sandkasten/protocol"
)

const labelPrefix = "sandkasten."

type Client struct {
	docker *client.Client
}

func New() (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("docker client: %w", err)
	}
	return &Client{docker: cli}, nil
}

func (c *Client) Close() error {
	return c.docker.Close()
}

// DockerClient returns the underlying Docker client (for workspace manager).
func (c *Client) DockerClient() *client.Client {
	return c.docker
}

// Ping verifies the Docker daemon is reachable.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.docker.Ping(ctx)
	return err
}

type CreateOpts struct {
	SessionID   string
	Image       string
	Defaults    config.Defaults
	WorkspaceID string            // optional persistent workspace volume
	Labels      map[string]string // additional labels
}

// CreateContainer creates and starts a sandbox container.
func (c *Client) CreateContainer(ctx context.Context, opts CreateOpts) (string, error) {
	labels := map[string]string{
		labelPrefix + "session_id": opts.SessionID,
		labelPrefix + "managed":    "true",
	}

	// Add custom labels
	for k, v := range opts.Labels {
		labels[k] = v
	}

	// Add workspace label if present
	if opts.WorkspaceID != "" {
		labels[labelPrefix+"workspace_id"] = opts.WorkspaceID
	}

	// Resource limits
	resources := container.Resources{
		NanoCPUs:  int64(opts.Defaults.CPULimit * 1e9),
		Memory:    int64(opts.Defaults.MemLimitMB) * 1024 * 1024,
		PidsLimit: int64Ptr(int64(opts.Defaults.PidsLimit)),
	}

	// Determine workspace volume source
	workspaceSource := protocol.WorkspaceVolumePrefix + opts.SessionID // default: ephemeral
	if opts.WorkspaceID != "" {
		workspaceSource = protocol.WorkspaceVolumePrefix + opts.WorkspaceID // persistent
	}

	hostCfg := &container.HostConfig{
		Resources:      resources,
		AutoRemove:     false,
		ReadonlyRootfs: opts.Defaults.ReadonlyRootfs,
		SecurityOpt:    []string{"no-new-privileges"},
		CapDrop:        []string{"ALL"},
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeVolume,
				Source: workspaceSource,
				Target: "/workspace",
			},
			{
				Type:   mount.TypeTmpfs,
				Target: "/tmp",
				TmpfsOptions: &mount.TmpfsOptions{
					SizeBytes: 512 * units.MiB,
				},
			},
			{
				Type:   mount.TypeTmpfs,
				Target: "/run",
				TmpfsOptions: &mount.TmpfsOptions{
					SizeBytes: 16 * units.MiB,
				},
			},
			// Writable cache dir for sandbox user (root fs may be read-only)
			{
				Type:   mount.TypeTmpfs,
				Target: "/home/sandbox/.cache",
				TmpfsOptions: &mount.TmpfsOptions{
					SizeBytes: 128 * units.MiB,
				},
			},
		},
	}

	if opts.Defaults.NetworkMode == "none" {
		hostCfg.NetworkMode = "none"
	}

	containerCfg := &container.Config{
		Image:  opts.Image,
		Labels: labels,
		Tty:    false,
		Cmd:    nil, // entrypoint is the runner
	}

	resp, err := c.docker.ContainerCreate(ctx, containerCfg, hostCfg, nil, nil, "sandkasten-"+opts.SessionID)
	if err != nil {
		return "", fmt.Errorf("container create: %w", err)
	}

	if err := c.docker.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		// Clean up on start failure.
		c.docker.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		return "", fmt.Errorf("container start: %w", err)
	}

	return resp.ID, nil
}

// ExecRunner sends a protocol request to the runner inside the container
// and returns the response.
func (c *Client) ExecRunner(ctx context.Context, containerID string, req protocol.Request) (*protocol.Response, error) {
	reqJSON, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	execCfg := container.ExecOptions{
		Cmd:          []string{"/usr/local/bin/runner", "--client", string(reqJSON)},
		AttachStdout: true,
		AttachStderr: true,
	}

	execResp, err := c.docker.ContainerExecCreate(ctx, containerID, execCfg)
	if err != nil {
		return nil, fmt.Errorf("exec create: %w", err)
	}

	attachResp, err := c.docker.ContainerExecAttach(ctx, execResp.ID, container.ExecAttachOptions{})
	if err != nil {
		return nil, fmt.Errorf("exec attach: %w", err)
	}
	defer attachResp.Close()

	// Demultiplex Docker's stdout/stderr stream (8-byte headers) so we get raw stdout.
	var stdoutBuf, stderrBuf bytes.Buffer
	if _, err := stdcopy.StdCopy(&stdoutBuf, &stderrBuf, attachResp.Reader); err != nil {
		return nil, fmt.Errorf("exec read: %w", err)
	}
	output := stdoutBuf.Bytes()

	line := findJSONLine(output)
	if line == nil {
		return nil, fmt.Errorf("no JSON response from runner, got: %s", string(output))
	}

	var resp protocol.Response
	if err := json.Unmarshal(line, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &resp, nil
}

// RemoveContainer force-removes a container and its workspace volume.
func (c *Client) RemoveContainer(ctx context.Context, containerID string, sessionID string) error {
	err := c.docker.ContainerRemove(ctx, containerID, container.RemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	})
	if err != nil && !client.IsErrNotFound(err) {
		return fmt.Errorf("container remove: %w", err)
	}

	// Also remove the workspace volume.
	volName := protocol.WorkspaceVolumePrefix + sessionID
	c.docker.VolumeRemove(ctx, volName, true)

	return nil
}

// ContainerInfo holds basic info about a running sandbox container.
type ContainerInfo struct {
	ContainerID string
	SessionID   string
}

// ListSandboxContainers returns all containers with sandkasten labels.
func (c *Client) ListSandboxContainers(ctx context.Context) ([]ContainerInfo, error) {
	f := filters.NewArgs()
	f.Add("label", labelPrefix+"managed=true")

	containers, err := c.docker.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: f,
	})
	if err != nil {
		return nil, fmt.Errorf("container list: %w", err)
	}

	var result []ContainerInfo
	for _, ctr := range containers {
		sessionID := ctr.Labels[labelPrefix+"session_id"]
		if sessionID == "" {
			continue
		}
		result = append(result, ContainerInfo{
			ContainerID: ctr.ID,
			SessionID:   sessionID,
		})
	}
	return result, nil
}

// findJSONLine extracts the first line that starts with '{' from docker output.
func findJSONLine(data []byte) []byte {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, protocol.MaxOutputBytes+4096), protocol.MaxOutputBytes+4096)
	for scanner.Scan() {
		line := scanner.Bytes()
		trimmed := bytes.TrimLeft(line, "\x00\x01\x02\x03\x04\x05\x06\x07\x08")
		if idx := bytes.IndexByte(trimmed, '{'); idx >= 0 {
			return trimmed[idx:]
		}
	}
	// Fallback: find first '{' in raw data.
	if idx := bytes.IndexByte(data, '{'); idx >= 0 {
		end := bytes.IndexByte(data[idx:], '\n')
		if end < 0 {
			return data[idx:]
		}
		return data[idx : idx+end]
	}
	return nil
}

func int64Ptr(v int64) *int64 {
	return &v
}

// IsContainerRunning checks if a container is currently running.
func (c *Client) IsContainerRunning(ctx context.Context, containerID string) (bool, error) {
	info, err := c.docker.ContainerInspect(ctx, containerID)
	if err != nil {
		if client.IsErrNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return info.State.Running, nil
}

