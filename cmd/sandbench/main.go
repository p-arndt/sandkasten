package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

type hardwareInfo struct {
	Hostname      string `json:"hostname"`
	Kernel        string `json:"kernel"`
	OS            string `json:"os"`
	Arch          string `json:"arch"`
	GoVersion     string `json:"go_version"`
	CPUModel      string `json:"cpu_model"`
	LogicalCPUs   int    `json:"logical_cpus"`
	MemoryTotalMB int64  `json:"memory_total_mb"`
}

type benchmarkReport struct {
	GeneratedAt time.Time     `json:"generated_at"`
	Hardware    hardwareInfo  `json:"hardware"`
	Sandkasten  *sandReport   `json:"sandkasten,omitempty"`
	Docker      *dockerReport `json:"docker,omitempty"`
}

type workspaceOptions struct {
	Mode    string `json:"mode"`
	ID      string `json:"id,omitempty"`
	Prefix  string `json:"prefix,omitempty"`
	Cleanup bool   `json:"cleanup"`
}

type sandReport struct {
	Host          string            `json:"host"`
	Image         string            `json:"image"`
	Workspace     workspaceOptions  `json:"workspace"`
	ColdRuns      []sandRun         `json:"cold_runs"`
	WarmRuns      []sandRun         `json:"warm_runs"`
	ExistingRuns  []sandExistingRun `json:"existing_runs"`
	ColdSummary   sandSummary       `json:"cold_summary"`
	WarmSummary   sandSummary       `json:"warm_summary"`
	ExistingStats existingSummary   `json:"existing_summary"`
}

type sandRun struct {
	Mode             string        `json:"mode"`
	SessionID        string        `json:"session_id"`
	WorkspaceID      string        `json:"workspace_id,omitempty"`
	StartupMs        float64       `json:"startup_ms"`
	ClassifiedPooled bool          `json:"classified_pooled"`
	AcquireDetail    string        `json:"acquire_detail,omitempty"`
	StartupMemoryB   int64         `json:"startup_memory_bytes"`
	StartupCPUUsec   int64         `json:"startup_cpu_usec"`
	Workload         *sandWorkload `json:"workload,omitempty"`
}

type sandExistingRun struct {
	SessionID string        `json:"session_id"`
	MemoryB   int64         `json:"memory_bytes"`
	CPUUsec   int64         `json:"cpu_usage_usec"`
	Workload  *sandWorkload `json:"workload,omitempty"`
}

type sandWorkload struct {
	Command          string `json:"command"`
	ExitCode         int    `json:"exit_code"`
	DurationMs       int64  `json:"duration_ms"`
	CPUStartUsec     int64  `json:"cpu_start_usec"`
	CPUEndUsec       int64  `json:"cpu_end_usec"`
	CPUDeltaUsec     int64  `json:"cpu_delta_usec"`
	MemoryStartBytes int64  `json:"memory_start_bytes"`
	MemoryEndBytes   int64  `json:"memory_end_bytes"`
	MemoryPeakBytes  int64  `json:"memory_peak_bytes"`
}

type sandSummary struct {
	Count               int     `json:"count"`
	StartupAvgMs        float64 `json:"startup_avg_ms"`
	StartupMinMs        float64 `json:"startup_min_ms"`
	StartupMaxMs        float64 `json:"startup_max_ms"`
	StartupMemAvgMiB    float64 `json:"startup_mem_avg_mib"`
	StartupCPUAvgMs     float64 `json:"startup_cpu_avg_ms"`
	PooledHits          int     `json:"pooled_hits"`
	WorkloadCount       int     `json:"workload_count"`
	WorkloadAvgMs       float64 `json:"workload_avg_ms"`
	WorkloadCPUAvgMs    float64 `json:"workload_cpu_avg_ms"`
	WorkloadMemPeakMiB  float64 `json:"workload_mem_peak_avg_mib"`
	WorkloadExitNonZero int     `json:"workload_exit_non_zero"`
}

type existingSummary struct {
	Count               int     `json:"count"`
	MemoryAvgMiB        float64 `json:"memory_avg_mib"`
	CPUAvgMs            float64 `json:"cpu_avg_ms"`
	WorkloadCount       int     `json:"workload_count"`
	WorkloadAvgMs       float64 `json:"workload_avg_ms"`
	WorkloadCPUAvgMs    float64 `json:"workload_cpu_avg_ms"`
	WorkloadMemPeakMiB  float64 `json:"workload_mem_peak_avg_mib"`
	WorkloadExitNonZero int     `json:"workload_exit_non_zero"`
}

type dockerReport struct {
	Image         string                `json:"image"`
	Runs          []dockerRun           `json:"runs"`
	ExistingRuns  []dockerExistingRun   `json:"existing_runs"`
	Summary       dockerSummary         `json:"summary"`
	ExistingStats dockerExistingSummary `json:"existing_summary"`
}

type dockerRun struct {
	ContainerID    string          `json:"container_id"`
	StartupMs      float64         `json:"startup_ms"`
	StartupMemoryB int64           `json:"startup_memory_bytes"`
	StartupCPUPct  float64         `json:"startup_cpu_percent"`
	Workload       *dockerWorkload `json:"workload,omitempty"`
}

type dockerExistingRun struct {
	ContainerID string          `json:"container_id"`
	MemoryB     int64           `json:"memory_bytes"`
	CPUPct      float64         `json:"cpu_percent"`
	Workload    *dockerWorkload `json:"workload,omitempty"`
}

type dockerWorkload struct {
	Command       string  `json:"command"`
	ExitCode      int     `json:"exit_code"`
	DurationMs    int64   `json:"duration_ms"`
	MemStartBytes int64   `json:"mem_start_bytes"`
	MemEndBytes   int64   `json:"mem_end_bytes"`
	MemPeakBytes  int64   `json:"mem_peak_bytes"`
	CPUStartPct   float64 `json:"cpu_start_percent"`
	CPUEndPct     float64 `json:"cpu_end_percent"`
	CPUPeakPct    float64 `json:"cpu_peak_percent"`
}

type dockerSummary struct {
	Count               int     `json:"count"`
	StartupAvgMs        float64 `json:"startup_avg_ms"`
	StartupMinMs        float64 `json:"startup_min_ms"`
	StartupMaxMs        float64 `json:"startup_max_ms"`
	StartupMemAvgMiB    float64 `json:"startup_mem_avg_mib"`
	StartupCPUAvgPct    float64 `json:"startup_cpu_avg_percent"`
	WorkloadCount       int     `json:"workload_count"`
	WorkloadAvgMs       float64 `json:"workload_avg_ms"`
	WorkloadMemPeakMiB  float64 `json:"workload_mem_peak_avg_mib"`
	WorkloadCPUPeakAvg  float64 `json:"workload_cpu_peak_avg_percent"`
	WorkloadExitNonZero int     `json:"workload_exit_non_zero"`
}

type dockerExistingSummary struct {
	Count               int     `json:"count"`
	MemoryAvgMiB        float64 `json:"memory_avg_mib"`
	CPUAvgPct           float64 `json:"cpu_avg_percent"`
	WorkloadCount       int     `json:"workload_count"`
	WorkloadAvgMs       float64 `json:"workload_avg_ms"`
	WorkloadMemPeakMiB  float64 `json:"workload_mem_peak_avg_mib"`
	WorkloadCPUPeakAvg  float64 `json:"workload_cpu_peak_avg_percent"`
	WorkloadExitNonZero int     `json:"workload_exit_non_zero"`
}

func main() {
	var (
		target = flag.String("target", "sandkasten", "benchmark target: sandkasten | docker | both")

		host              = flag.String("host", "http://127.0.0.1:8080", "Sandkasten API base URL")
		apiKey            = flag.String("api-key", strings.TrimSpace(os.Getenv("SANDKASTEN_API_KEY")), "API key (defaults to SANDKASTEN_API_KEY)")
		image             = flag.String("image", "", "Sandkasten image (empty uses daemon default)")
		ttlSeconds        = flag.Int("ttl-seconds", 1800, "Sandkasten session TTL")
		coldRuns          = flag.Int("cold-runs", 3, "number of Sandkasten cold runs")
		warmRuns          = flag.Int("warm-runs", 3, "number of Sandkasten warm prepooled runs")
		warmWaitSeconds   = flag.Int("warm-wait-seconds", 10, "seconds to wait for warm pool")
		workloadCmd       = flag.String("workload", "", "optional workload command (used for both backends)")
		workloadTimeoutMs = flag.Int("workload-timeout-ms", 300000, "workload timeout in ms")
		pollMs            = flag.Int("poll-ms", 200, "resource polling interval in ms")

		existingSessionIDs = flag.String("existing-session-ids", "", "comma-separated existing Sandkasten session IDs")
		existingPingCmd    = flag.String("existing-ping-cmd", ":", "command for existing Sandkasten sessions when --workload is empty")

		workspaceMode    = flag.String("workspace-mode", "none", "Sandkasten workspace mode: none | shared | per-run")
		workspaceID      = flag.String("workspace-id", "", "workspace ID for shared mode")
		workspacePrefix  = flag.String("workspace-prefix", "sandbench", "workspace ID prefix for per-run/shared")
		workspaceCleanup = flag.Bool("workspace-cleanup", true, "delete workspaces created by sandbench")
		freshEnvironment = flag.Bool("fresh-environment", false, "shortcut for --workspace-mode per-run")

		dockerImage       = flag.String("docker-image", "python:3.12-slim", "Docker image")
		dockerRuns        = flag.Int("docker-runs", 3, "number of Docker create/start runs")
		dockerExistingIDs = flag.String("docker-existing-ids", "", "comma-separated existing Docker container IDs")
		dockerKeepalive   = flag.String("docker-keepalive-cmd", "while true; do sleep 3600; done", "command to keep benchmark container alive")
		dockerExecShell   = flag.String("docker-exec-shell", "sh", "shell used for docker exec (sh or bash)")

		jsonOut = flag.Bool("json", false, "emit JSON report")
	)
	flag.Parse()

	if *pollMs <= 0 || *workloadTimeoutMs <= 0 || *coldRuns < 0 || *warmRuns < 0 || *dockerRuns < 0 {
		fail("invalid numeric flags")
	}

	t := strings.ToLower(strings.TrimSpace(*target))
	if t != "sandkasten" && t != "docker" && t != "both" {
		fail("target must be sandkasten, docker, or both")
	}

	if *freshEnvironment {
		*workspaceMode = "per-run"
	}

	ws := workspaceOptions{
		Mode:    strings.ToLower(strings.TrimSpace(*workspaceMode)),
		ID:      strings.TrimSpace(*workspaceID),
		Prefix:  strings.TrimSpace(*workspacePrefix),
		Cleanup: *workspaceCleanup,
	}
	if ws.Mode == "" {
		ws.Mode = "none"
	}
	if ws.Mode != "none" && ws.Mode != "shared" && ws.Mode != "per-run" {
		fail("workspace-mode must be none, shared, or per-run")
	}
	if ws.Mode == "shared" && ws.ID == "" {
		ws.ID = newWorkspaceID(ws.Prefix)
	}

	ctx := context.Background()
	rep := benchmarkReport{GeneratedAt: time.Now().UTC(), Hardware: collectHardware()}

	if t == "sandkasten" || t == "both" {
		sc := newSandClient(*host, *apiKey)
		report, err := runSandkasten(ctx, sc, sandRunConfig{
			image:             *image,
			ttlSeconds:        *ttlSeconds,
			coldRuns:          *coldRuns,
			warmRuns:          *warmRuns,
			warmWait:          time.Duration(*warmWaitSeconds) * time.Second,
			workload:          strings.TrimSpace(*workloadCmd),
			workloadTimeoutMs: *workloadTimeoutMs,
			pollInterval:      time.Duration(*pollMs) * time.Millisecond,
			existingIDs:       parseCSV(*existingSessionIDs),
			existingCmd:       strings.TrimSpace(*existingPingCmd),
			workspace:         ws,
		})
		if err != nil {
			fail("sandkasten benchmark failed: %v", err)
		}
		rep.Sandkasten = report
	}

	if t == "docker" || t == "both" {
		dc, err := newDockerClient(*dockerExecShell)
		if err != nil {
			if t == "both" {
				fmt.Fprintf(os.Stderr, "sandbench: docker benchmark skipped: %v\n", err)
			} else {
				fail("docker unavailable: %v", err)
			}
		} else {
			report, derr := runDocker(ctx, dc, dockerRunConfig{
				image:             strings.TrimSpace(*dockerImage),
				runs:              *dockerRuns,
				workload:          strings.TrimSpace(*workloadCmd),
				workloadTimeoutMs: *workloadTimeoutMs,
				pollInterval:      time.Duration(*pollMs) * time.Millisecond,
				existingIDs:       parseCSV(*dockerExistingIDs),
				keepaliveCmd:      *dockerKeepalive,
			})
			if derr != nil {
				if t == "both" {
					fmt.Fprintf(os.Stderr, "sandbench: docker benchmark failed: %v\n", derr)
				} else {
					fail("docker benchmark failed: %v", derr)
				}
			} else {
				rep.Docker = report
			}
		}
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(rep)
		return
	}
	printReport(rep)
}

type sandRunConfig struct {
	image             string
	ttlSeconds        int
	coldRuns          int
	warmRuns          int
	warmWait          time.Duration
	workload          string
	workloadTimeoutMs int
	pollInterval      time.Duration
	existingIDs       []string
	existingCmd       string
	workspace         workspaceOptions
}

func runSandkasten(ctx context.Context, client *sandClient, cfg sandRunConfig) (*sandReport, error) {
	out := &sandReport{
		Host:         client.baseURL,
		Image:        cfg.image,
		Workspace:    cfg.workspace,
		ColdRuns:     make([]sandRun, 0, cfg.coldRuns),
		WarmRuns:     make([]sandRun, 0, cfg.warmRuns),
		ExistingRuns: make([]sandExistingRun, 0, len(cfg.existingIDs)),
	}

	if cfg.workspace.Mode == "shared" {
		if err := client.ensureWorkspace(ctx, cfg.workspace.ID); err != nil {
			return nil, fmt.Errorf("ensure shared workspace: %w", err)
		}
		if cfg.workspace.Cleanup {
			defer client.deleteWorkspace(context.Background(), cfg.workspace.ID)
		}
	}

	for i := 0; i < cfg.coldRuns; i++ {
		wsID, wsCleanup, err := prepareWorkspace(ctx, client, cfg.workspace)
		if err != nil {
			return nil, err
		}
		run, err := runSandCold(ctx, client, cfg, wsID)
		if wsCleanup != nil {
			wsCleanup()
		}
		if err != nil {
			return nil, err
		}
		out.ColdRuns = append(out.ColdRuns, *run)
	}

	for i := 0; i < cfg.warmRuns; i++ {
		wsID, wsCleanup, err := prepareWorkspace(ctx, client, cfg.workspace)
		if err != nil {
			return nil, err
		}
		run, err := runSandWarm(ctx, client, cfg, wsID)
		if wsCleanup != nil {
			wsCleanup()
		}
		if err != nil {
			return nil, err
		}
		out.WarmRuns = append(out.WarmRuns, *run)
	}

	for _, id := range cfg.existingIDs {
		cmd := cfg.workload
		if cmd == "" {
			cmd = cfg.existingCmd
		}
		run, err := runSandExisting(ctx, client, id, cmd, cfg.workloadTimeoutMs, cfg.pollInterval)
		if err != nil {
			return nil, err
		}
		out.ExistingRuns = append(out.ExistingRuns, *run)
	}

	out.ColdSummary = summarizeSand(out.ColdRuns)
	out.WarmSummary = summarizeSand(out.WarmRuns)
	out.ExistingStats = summarizeSandExisting(out.ExistingRuns)
	return out, nil
}

func prepareWorkspace(ctx context.Context, client *sandClient, ws workspaceOptions) (string, func(), error) {
	if ws.Mode == "none" {
		return "", nil, nil
	}
	if ws.Mode == "shared" {
		return ws.ID, nil, nil
	}
	id := newWorkspaceID(ws.Prefix)
	if err := client.ensureWorkspace(ctx, id); err != nil {
		return "", nil, fmt.Errorf("ensure per-run workspace: %w", err)
	}
	cleanup := func() {
		if ws.Cleanup {
			_ = client.deleteWorkspace(context.Background(), id)
		}
	}
	return id, cleanup, nil
}

func runSandCold(ctx context.Context, client *sandClient, cfg sandRunConfig, workspaceID string) (*sandRun, error) {
	sessions, err := client.listSessions(ctx)
	if err != nil {
		return nil, err
	}
	idle := 0
	for _, s := range sessions {
		if s.Status == "pool_idle" && (cfg.image == "" || s.Image == cfg.image) {
			idle++
		}
	}

	created := make([]string, 0, idle+1)
	defer func() {
		for _, id := range created {
			_ = client.destroySession(context.Background(), id)
		}
	}()

	for i := 0; i < idle; i++ {
		probe, err := runSandOne(ctx, client, "cold-drain", cfg.image, cfg.ttlSeconds, "", cfg.workloadTimeoutMs, cfg.pollInterval, "")
		if err != nil {
			return nil, err
		}
		created = append(created, probe.SessionID)
	}
	measured, err := runSandOne(ctx, client, "cold", cfg.image, cfg.ttlSeconds, cfg.workload, cfg.workloadTimeoutMs, cfg.pollInterval, workspaceID)
	if err != nil {
		return nil, err
	}
	created = append(created, measured.SessionID)
	return measured, nil
}

func runSandWarm(ctx context.Context, client *sandClient, cfg sandRunConfig, workspaceID string) (*sandRun, error) {
	if err := waitPoolIdle(ctx, client, cfg.image, workspaceID, cfg.warmWait); err != nil {
		if workspaceID == "" {
			return nil, err
		}
		// Workspace-aware pools are built on demand. Prime once, then wait again.
		primer, _, createErr := client.createSession(ctx, cfg.image, cfg.ttlSeconds, workspaceID)
		if createErr != nil {
			return nil, fmt.Errorf("wait pool idle failed (%v), and workspace prime create failed: %w", err, createErr)
		}
		_ = client.destroySession(context.Background(), primer.ID)
		if err2 := waitPoolIdle(ctx, client, cfg.image, workspaceID, cfg.warmWait); err2 != nil {
			return nil, fmt.Errorf("workspace pool not ready after priming: %w", err2)
		}
	}
	run, err := runSandOne(ctx, client, "warm", cfg.image, cfg.ttlSeconds, cfg.workload, cfg.workloadTimeoutMs, cfg.pollInterval, workspaceID)
	if err != nil {
		return nil, err
	}
	defer client.destroySession(context.Background(), run.SessionID)
	return run, nil
}

func waitPoolIdle(ctx context.Context, client *sandClient, image string, workspaceID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		sessions, err := client.listSessions(ctx)
		if err == nil {
			for _, s := range sessions {
				if s.Status == "pool_idle" &&
					(image == "" || s.Image == image) &&
					(workspaceID == "" || s.WorkspaceID == workspaceID) {
					return nil
				}
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for pool_idle (image=%q workspace_id=%q)", image, workspaceID)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func runSandOne(ctx context.Context, client *sandClient, mode, image string, ttl int, workload string, timeoutMs int, poll time.Duration, workspaceID string) (*sandRun, error) {
	created, latency, err := client.createSession(ctx, image, ttl, workspaceID)
	if err != nil {
		return nil, err
	}
	stats, err := client.getStats(ctx, created.ID)
	if err != nil {
		_ = client.destroySession(ctx, created.ID)
		return nil, err
	}
	pooled := strings.EqualFold(created.AcquireSource, "pool")
	out := &sandRun{
		Mode:             mode,
		SessionID:        created.ID,
		WorkspaceID:      workspaceID,
		StartupMs:        float64(latency.Microseconds()) / 1000.0,
		ClassifiedPooled: pooled,
		AcquireDetail:    created.AcquireDetail,
		StartupMemoryB:   stats.MemoryBytes,
		StartupCPUUsec:   stats.CPUUsageUsec,
	}
	if strings.TrimSpace(workload) != "" {
		work, err := runSandWorkload(ctx, client, created.ID, workload, timeoutMs, poll)
		if err != nil {
			_ = client.destroySession(ctx, created.ID)
			return nil, err
		}
		out.Workload = work
	}
	return out, nil
}

func runSandExisting(ctx context.Context, client *sandClient, sessionID, cmd string, timeoutMs int, poll time.Duration) (*sandExistingRun, error) {
	stats, err := client.getStats(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	out := &sandExistingRun{SessionID: sessionID, MemoryB: stats.MemoryBytes, CPUUsec: stats.CPUUsageUsec}
	if strings.TrimSpace(cmd) != "" {
		w, err := runSandWorkload(ctx, client, sessionID, cmd, timeoutMs, poll)
		if err != nil {
			return nil, err
		}
		out.Workload = w
	}
	return out, nil
}

func runSandWorkload(ctx context.Context, client *sandClient, sessionID, cmd string, timeoutMs int, poll time.Duration) (*sandWorkload, error) {
	before, err := client.getStats(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	type execResult struct {
		resp *sandExecResponse
		err  error
	}
	resultCh := make(chan execResult, 1)
	go func() {
		resp, e := client.exec(ctx, sessionID, cmd, timeoutMs)
		resultCh <- execResult{resp: resp, err: e}
	}()
	peak := before.MemoryBytes
	t := time.NewTicker(poll)
	defer t.Stop()
	var result execResult
	for {
		select {
		case result = <-resultCh:
			goto done
		case <-t.C:
			s, err := client.getStats(ctx, sessionID)
			if err == nil && s.MemoryBytes > peak {
				peak = s.MemoryBytes
			}
		}
	}
done:
	if result.err != nil {
		return nil, result.err
	}
	after, err := client.getStats(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if after.MemoryBytes > peak {
		peak = after.MemoryBytes
	}
	return &sandWorkload{
		Command:          cmd,
		ExitCode:         result.resp.ExitCode,
		DurationMs:       result.resp.DurationMs,
		CPUStartUsec:     before.CPUUsageUsec,
		CPUEndUsec:       after.CPUUsageUsec,
		CPUDeltaUsec:     max64(0, after.CPUUsageUsec-before.CPUUsageUsec),
		MemoryStartBytes: before.MemoryBytes,
		MemoryEndBytes:   after.MemoryBytes,
		MemoryPeakBytes:  peak,
	}, nil
}

type dockerRunConfig struct {
	image             string
	runs              int
	workload          string
	workloadTimeoutMs int
	pollInterval      time.Duration
	existingIDs       []string
	keepaliveCmd      string
}

func runDocker(ctx context.Context, client *dockerClient, cfg dockerRunConfig) (*dockerReport, error) {
	out := &dockerReport{
		Image:        cfg.image,
		Runs:         make([]dockerRun, 0, cfg.runs),
		ExistingRuns: make([]dockerExistingRun, 0, len(cfg.existingIDs)),
	}
	for i := 0; i < cfg.runs; i++ {
		id, latency, err := client.runContainer(ctx, cfg.image, cfg.keepaliveCmd)
		if err != nil {
			return nil, err
		}
		stat, err := client.stats(ctx, id)
		if err != nil {
			_ = client.remove(context.Background(), id)
			return nil, err
		}
		r := dockerRun{ContainerID: id, StartupMs: float64(latency.Microseconds()) / 1000.0, StartupMemoryB: stat.MemBytes, StartupCPUPct: stat.CPUPct}
		if cfg.workload != "" {
			w, err := client.workload(ctx, id, cfg.workload, cfg.workloadTimeoutMs, cfg.pollInterval)
			if err != nil {
				_ = client.remove(context.Background(), id)
				return nil, err
			}
			r.Workload = w
		}
		out.Runs = append(out.Runs, r)
		_ = client.remove(context.Background(), id)
	}
	for _, id := range cfg.existingIDs {
		stat, err := client.stats(ctx, id)
		if err != nil {
			return nil, err
		}
		r := dockerExistingRun{ContainerID: id, MemoryB: stat.MemBytes, CPUPct: stat.CPUPct}
		if cfg.workload != "" {
			w, err := client.workload(ctx, id, cfg.workload, cfg.workloadTimeoutMs, cfg.pollInterval)
			if err != nil {
				return nil, err
			}
			r.Workload = w
		}
		out.ExistingRuns = append(out.ExistingRuns, r)
	}
	out.Summary = summarizeDocker(out.Runs)
	out.ExistingStats = summarizeDockerExisting(out.ExistingRuns)
	return out, nil
}

type sandClient struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func newSandClient(baseURL, apiKey string) *sandClient {
	return &sandClient{baseURL: strings.TrimRight(baseURL, "/"), apiKey: strings.TrimSpace(apiKey), http: &http.Client{}}
}

type sandCreateSessionRequest struct {
	Image       string `json:"image,omitempty"`
	TTLSeconds  int    `json:"ttl_seconds,omitempty"`
	WorkspaceID string `json:"workspace_id,omitempty"`
}

type sandSessionInfo struct {
	ID            string    `json:"id"`
	Image         string    `json:"image"`
	Status        string    `json:"status"`
	WorkspaceID   string    `json:"workspace_id,omitempty"`
	AcquireSource string    `json:"acquire_source,omitempty"`
	AcquireDetail string    `json:"acquire_detail,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

type sandSessionStats struct {
	MemoryBytes  int64 `json:"memory_bytes"`
	CPUUsageUsec int64 `json:"cpu_usage_usec"`
}

type sandExecResponse struct {
	ExitCode   int   `json:"exit_code"`
	DurationMs int64 `json:"duration_ms"`
}

func (c *sandClient) createSession(ctx context.Context, image string, ttl int, workspaceID string) (*sandSessionInfo, time.Duration, error) {
	start := time.Now()
	req := sandCreateSessionRequest{Image: image, TTLSeconds: ttl, WorkspaceID: workspaceID}
	var out sandSessionInfo
	err := c.doJSON(ctx, http.MethodPost, "/v1/sessions", req, &out)
	if err != nil {
		return nil, 0, err
	}
	return &out, time.Since(start), nil
}

func (c *sandClient) listSessions(ctx context.Context) ([]sandSessionInfo, error) {
	var out []sandSessionInfo
	if err := c.doJSON(ctx, http.MethodGet, "/v1/sessions", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *sandClient) getStats(ctx context.Context, id string) (*sandSessionStats, error) {
	var out sandSessionStats
	if err := c.doJSON(ctx, http.MethodGet, "/v1/sessions/"+id+"/stats", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *sandClient) exec(ctx context.Context, id, cmd string, timeoutMs int) (*sandExecResponse, error) {
	var out sandExecResponse
	body := map[string]any{"cmd": cmd, "timeout_ms": timeoutMs}
	if err := c.doJSON(ctx, http.MethodPost, "/v1/sessions/"+id+"/exec", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *sandClient) destroySession(ctx context.Context, id string) error {
	return c.doJSON(ctx, http.MethodDelete, "/v1/sessions/"+id, nil, nil)
}

func (c *sandClient) ensureWorkspace(ctx context.Context, id string) error {
	body := map[string]any{"path": ".sandbench/created.txt", "text": time.Now().UTC().Format(time.RFC3339Nano)}
	return c.doJSON(ctx, http.MethodPost, "/v1/workspaces/"+id+"/fs/write", body, nil)
}

func (c *sandClient) deleteWorkspace(ctx context.Context, id string) error {
	return c.doJSON(ctx, http.MethodDelete, "/v1/workspaces/"+id, nil, nil)
}

func (c *sandClient) doJSON(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("%s %s: %s %s", method, path, resp.Status, strings.TrimSpace(string(raw)))
	}
	if out == nil || len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, out)
}

type dockerClient struct {
	binary string
	shell  string
}

type dockerStats struct {
	MemBytes int64
	CPUPct   float64
}

func newDockerClient(shell string) (*dockerClient, error) {
	b, err := exec.LookPath("docker")
	if err != nil {
		return nil, err
	}
	s := strings.TrimSpace(shell)
	if s == "" {
		s = "sh"
	}
	return &dockerClient{binary: b, shell: s}, nil
}

func (d *dockerClient) runContainer(ctx context.Context, image, keepalive string) (string, time.Duration, error) {
	start := time.Now()
	cmd := exec.CommandContext(ctx, d.binary, "run", "-d", "--rm", image, d.shell, "-lc", keepalive)
	out, err := cmd.Output()
	if err != nil {
		return "", 0, err
	}
	id := strings.TrimSpace(string(out))
	if id == "" {
		return "", 0, errors.New("docker returned empty container id")
	}
	return id, time.Since(start), nil
}

func (d *dockerClient) remove(ctx context.Context, id string) error {
	cmd := exec.CommandContext(ctx, d.binary, "rm", "-f", id)
	_ = cmd.Run()
	return nil
}

func (d *dockerClient) stats(ctx context.Context, id string) (*dockerStats, error) {
	cmd := exec.CommandContext(ctx, d.binary, "stats", "--no-stream", "--format", "{{json .}}", id)
	raw, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	line := strings.TrimSpace(string(raw))
	if line == "" {
		return nil, fmt.Errorf("empty docker stats output for %s", id)
	}
	var obj map[string]string
	if err := json.Unmarshal([]byte(line), &obj); err != nil {
		return nil, err
	}
	memRaw := firstPart(obj["MemUsage"], "/")
	memBytes, _ := parseHumanBytes(memRaw)
	cpuPct, _ := parsePercent(obj["CPUPerc"])
	return &dockerStats{MemBytes: memBytes, CPUPct: cpuPct}, nil
}

func (d *dockerClient) workload(ctx context.Context, id, cmdText string, timeoutMs int, poll time.Duration) (*dockerWorkload, error) {
	before, err := d.stats(ctx, id)
	if err != nil {
		return nil, err
	}
	peakMem := before.MemBytes
	peakCPU := before.CPUPct
	type result struct {
		exit int
		dur  int64
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		execCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
		defer cancel()
		start := time.Now()
		cmd := exec.CommandContext(execCtx, d.binary, "exec", id, d.shell, "-lc", cmdText)
		err := cmd.Run()
		exit := 0
		if err != nil {
			var ee *exec.ExitError
			if errors.As(err, &ee) {
				exit = ee.ExitCode()
			} else {
				exit = -1
			}
		}
		ch <- result{exit: exit, dur: time.Since(start).Milliseconds(), err: nil}
	}()
	tk := time.NewTicker(poll)
	defer tk.Stop()
	var r result
	for {
		select {
		case r = <-ch:
			goto done
		case <-tk.C:
			s, e := d.stats(ctx, id)
			if e == nil {
				if s.MemBytes > peakMem {
					peakMem = s.MemBytes
				}
				if s.CPUPct > peakCPU {
					peakCPU = s.CPUPct
				}
			}
		}
	}
done:
	if r.err != nil {
		return nil, r.err
	}
	after, err := d.stats(ctx, id)
	if err != nil {
		return nil, err
	}
	if after.MemBytes > peakMem {
		peakMem = after.MemBytes
	}
	if after.CPUPct > peakCPU {
		peakCPU = after.CPUPct
	}
	return &dockerWorkload{
		Command:       cmdText,
		ExitCode:      r.exit,
		DurationMs:    r.dur,
		MemStartBytes: before.MemBytes,
		MemEndBytes:   after.MemBytes,
		MemPeakBytes:  peakMem,
		CPUStartPct:   before.CPUPct,
		CPUEndPct:     after.CPUPct,
		CPUPeakPct:    peakCPU,
	}, nil
}

func summarizeSand(runs []sandRun) sandSummary {
	if len(runs) == 0 {
		return sandSummary{}
	}
	startup := make([]float64, 0, len(runs))
	mem := make([]float64, 0, len(runs))
	cpu := make([]float64, 0, len(runs))
	workDur := make([]float64, 0, len(runs))
	workCPU := make([]float64, 0, len(runs))
	workMemPeak := make([]float64, 0, len(runs))
	pooled := 0
	nonZero := 0
	for _, r := range runs {
		startup = append(startup, r.StartupMs)
		mem = append(mem, bytesToMiB(r.StartupMemoryB))
		cpu = append(cpu, float64(r.StartupCPUUsec)/1000.0)
		if r.ClassifiedPooled {
			pooled++
		}
		if r.Workload != nil {
			workDur = append(workDur, float64(r.Workload.DurationMs))
			workCPU = append(workCPU, float64(r.Workload.CPUDeltaUsec)/1000.0)
			workMemPeak = append(workMemPeak, bytesToMiB(r.Workload.MemoryPeakBytes))
			if r.Workload.ExitCode != 0 {
				nonZero++
			}
		}
	}
	return sandSummary{
		Count:               len(runs),
		StartupAvgMs:        avg(startup),
		StartupMinMs:        min(startup),
		StartupMaxMs:        max(startup),
		StartupMemAvgMiB:    avg(mem),
		StartupCPUAvgMs:     avg(cpu),
		PooledHits:          pooled,
		WorkloadCount:       len(workDur),
		WorkloadAvgMs:       avg(workDur),
		WorkloadCPUAvgMs:    avg(workCPU),
		WorkloadMemPeakMiB:  avg(workMemPeak),
		WorkloadExitNonZero: nonZero,
	}
}

func summarizeSandExisting(runs []sandExistingRun) existingSummary {
	if len(runs) == 0 {
		return existingSummary{}
	}
	mem := make([]float64, 0, len(runs))
	cpu := make([]float64, 0, len(runs))
	workDur := make([]float64, 0, len(runs))
	workCPU := make([]float64, 0, len(runs))
	workMemPeak := make([]float64, 0, len(runs))
	nonZero := 0
	for _, r := range runs {
		mem = append(mem, bytesToMiB(r.MemoryB))
		cpu = append(cpu, float64(r.CPUUsec)/1000.0)
		if r.Workload != nil {
			workDur = append(workDur, float64(r.Workload.DurationMs))
			workCPU = append(workCPU, float64(r.Workload.CPUDeltaUsec)/1000.0)
			workMemPeak = append(workMemPeak, bytesToMiB(r.Workload.MemoryPeakBytes))
			if r.Workload.ExitCode != 0 {
				nonZero++
			}
		}
	}
	return existingSummary{
		Count:               len(runs),
		MemoryAvgMiB:        avg(mem),
		CPUAvgMs:            avg(cpu),
		WorkloadCount:       len(workDur),
		WorkloadAvgMs:       avg(workDur),
		WorkloadCPUAvgMs:    avg(workCPU),
		WorkloadMemPeakMiB:  avg(workMemPeak),
		WorkloadExitNonZero: nonZero,
	}
}

func summarizeDocker(runs []dockerRun) dockerSummary {
	if len(runs) == 0 {
		return dockerSummary{}
	}
	startup := make([]float64, 0, len(runs))
	mem := make([]float64, 0, len(runs))
	cpu := make([]float64, 0, len(runs))
	workDur := make([]float64, 0, len(runs))
	workMem := make([]float64, 0, len(runs))
	workCPU := make([]float64, 0, len(runs))
	nonZero := 0
	for _, r := range runs {
		startup = append(startup, r.StartupMs)
		mem = append(mem, bytesToMiB(r.StartupMemoryB))
		cpu = append(cpu, r.StartupCPUPct)
		if r.Workload != nil {
			workDur = append(workDur, float64(r.Workload.DurationMs))
			workMem = append(workMem, bytesToMiB(r.Workload.MemPeakBytes))
			workCPU = append(workCPU, r.Workload.CPUPeakPct)
			if r.Workload.ExitCode != 0 {
				nonZero++
			}
		}
	}
	return dockerSummary{
		Count:               len(runs),
		StartupAvgMs:        avg(startup),
		StartupMinMs:        min(startup),
		StartupMaxMs:        max(startup),
		StartupMemAvgMiB:    avg(mem),
		StartupCPUAvgPct:    avg(cpu),
		WorkloadCount:       len(workDur),
		WorkloadAvgMs:       avg(workDur),
		WorkloadMemPeakMiB:  avg(workMem),
		WorkloadCPUPeakAvg:  avg(workCPU),
		WorkloadExitNonZero: nonZero,
	}
}

func summarizeDockerExisting(runs []dockerExistingRun) dockerExistingSummary {
	if len(runs) == 0 {
		return dockerExistingSummary{}
	}
	mem := make([]float64, 0, len(runs))
	cpu := make([]float64, 0, len(runs))
	workDur := make([]float64, 0, len(runs))
	workMem := make([]float64, 0, len(runs))
	workCPU := make([]float64, 0, len(runs))
	nonZero := 0
	for _, r := range runs {
		mem = append(mem, bytesToMiB(r.MemoryB))
		cpu = append(cpu, r.CPUPct)
		if r.Workload != nil {
			workDur = append(workDur, float64(r.Workload.DurationMs))
			workMem = append(workMem, bytesToMiB(r.Workload.MemPeakBytes))
			workCPU = append(workCPU, r.Workload.CPUPeakPct)
			if r.Workload.ExitCode != 0 {
				nonZero++
			}
		}
	}
	return dockerExistingSummary{
		Count:               len(runs),
		MemoryAvgMiB:        avg(mem),
		CPUAvgPct:           avg(cpu),
		WorkloadCount:       len(workDur),
		WorkloadAvgMs:       avg(workDur),
		WorkloadMemPeakMiB:  avg(workMem),
		WorkloadCPUPeakAvg:  avg(workCPU),
		WorkloadExitNonZero: nonZero,
	}
}

func printReport(rep benchmarkReport) {
	fmt.Printf("Sandbench report (%s)\n", rep.GeneratedAt.Format(time.RFC3339))
	fmt.Printf("Host: %s | Kernel: %s | CPU: %s (%d cores) | RAM: %d MiB\n\n",
		rep.Hardware.Hostname,
		rep.Hardware.Kernel,
		rep.Hardware.CPUModel,
		rep.Hardware.LogicalCPUs,
		rep.Hardware.MemoryTotalMB,
	)

	if rep.Sandkasten != nil {
		fmt.Printf("Sandkasten (%s, image=%s, workspace=%s)\n", rep.Sandkasten.Host, valueOrDefault(rep.Sandkasten.Image, "<default>"), rep.Sandkasten.Workspace.Mode)
		printSandRuns("Cold", rep.Sandkasten.ColdRuns, rep.Sandkasten.ColdSummary)
		printSandRuns("Warm", rep.Sandkasten.WarmRuns, rep.Sandkasten.WarmSummary)
		printSandExisting(rep.Sandkasten.ExistingRuns, rep.Sandkasten.ExistingStats)
		fmt.Println()
	}

	if rep.Docker != nil {
		fmt.Printf("Docker (image=%s)\n", rep.Docker.Image)
		printDockerRuns(rep.Docker.Runs, rep.Docker.Summary)
		printDockerExisting(rep.Docker.ExistingRuns, rep.Docker.ExistingStats)
	}
}

func printSandRuns(label string, runs []sandRun, s sandSummary) {
	fmt.Printf("  %s: runs=%d avg=%.2fms min=%.2fms max=%.2fms pooled=%d mem=%.2fMiB cpu=%.2fms\n",
		label, s.Count, s.StartupAvgMs, s.StartupMinMs, s.StartupMaxMs, s.PooledHits, s.StartupMemAvgMiB, s.StartupCPUAvgMs)
	if s.WorkloadCount > 0 {
		fmt.Printf("    workload: avg=%.2fms cpu=%.2fms mem_peak=%.2fMiB non_zero=%d\n",
			s.WorkloadAvgMs, s.WorkloadCPUAvgMs, s.WorkloadMemPeakMiB, s.WorkloadExitNonZero)
	}
	ordered := append([]sandRun(nil), runs...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].StartupMs < ordered[j].StartupMs })
	for _, r := range ordered {
		fmt.Printf("    - id=%s startup=%.2fms pooled=%t mem=%.2fMiB cpu=%.2fms\n",
			r.SessionID,
			r.StartupMs,
			r.ClassifiedPooled,
			bytesToMiB(r.StartupMemoryB),
			float64(r.StartupCPUUsec)/1000.0,
		)
	}
}

func printSandExisting(runs []sandExistingRun, s existingSummary) {
	if len(runs) == 0 {
		return
	}
	fmt.Printf("  Existing sessions: count=%d mem_avg=%.2fMiB cpu_avg=%.2fms\n", s.Count, s.MemoryAvgMiB, s.CPUAvgMs)
	if s.WorkloadCount > 0 {
		fmt.Printf("    workload: avg=%.2fms cpu=%.2fms mem_peak=%.2fMiB non_zero=%d\n",
			s.WorkloadAvgMs, s.WorkloadCPUAvgMs, s.WorkloadMemPeakMiB, s.WorkloadExitNonZero)
	}
}

func printDockerRuns(runs []dockerRun, s dockerSummary) {
	fmt.Printf("  Runs: count=%d avg=%.2fms min=%.2fms max=%.2fms mem=%.2fMiB cpu=%.2f%%\n",
		s.Count, s.StartupAvgMs, s.StartupMinMs, s.StartupMaxMs, s.StartupMemAvgMiB, s.StartupCPUAvgPct)
	if s.WorkloadCount > 0 {
		fmt.Printf("    workload: avg=%.2fms mem_peak=%.2fMiB cpu_peak=%.2f%% non_zero=%d\n",
			s.WorkloadAvgMs, s.WorkloadMemPeakMiB, s.WorkloadCPUPeakAvg, s.WorkloadExitNonZero)
	}
	for _, r := range runs {
		fmt.Printf("    - id=%s startup=%.2fms mem=%.2fMiB cpu=%.2f%%\n", r.ContainerID, r.StartupMs, bytesToMiB(r.StartupMemoryB), r.StartupCPUPct)
	}
}

func printDockerExisting(runs []dockerExistingRun, s dockerExistingSummary) {
	if len(runs) == 0 {
		return
	}
	fmt.Printf("  Existing containers: count=%d mem_avg=%.2fMiB cpu_avg=%.2f%%\n", s.Count, s.MemoryAvgMiB, s.CPUAvgPct)
	if s.WorkloadCount > 0 {
		fmt.Printf("    workload: avg=%.2fms mem_peak=%.2fMiB cpu_peak=%.2f%% non_zero=%d\n",
			s.WorkloadAvgMs, s.WorkloadMemPeakMiB, s.WorkloadCPUPeakAvg, s.WorkloadExitNonZero)
	}
}

func collectHardware() hardwareInfo {
	host, _ := os.Hostname()
	return hardwareInfo{
		Hostname:      host,
		Kernel:        readOneLine("/proc/sys/kernel/osrelease"),
		OS:            runtime.GOOS,
		Arch:          runtime.GOARCH,
		GoVersion:     runtime.Version(),
		CPUModel:      readCPUModel(),
		LogicalCPUs:   runtime.NumCPU(),
		MemoryTotalMB: readMemTotalMiB(),
	}
}

func readCPUModel() string {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return "unknown"
	}
	for _, ln := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(ln, "model name") {
			parts := strings.SplitN(ln, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return "unknown"
}

func readMemTotalMiB() int64 {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	for _, ln := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(ln, "MemTotal:") {
			f := strings.Fields(ln)
			if len(f) >= 2 {
				kb, _ := strconv.ParseInt(f[1], 10, 64)
				return kb / 1024
			}
		}
	}
	return 0
}

func readOneLine(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func parseCSV(v string) []string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

func newWorkspaceID(prefix string) string {
	p := strings.TrimSpace(prefix)
	if p == "" {
		p = "sandbench"
	}
	return fmt.Sprintf("%s-%d", p, time.Now().UnixNano())
}

func firstPart(v, sep string) string {
	parts := strings.SplitN(strings.TrimSpace(v), sep, 2)
	return strings.TrimSpace(parts[0])
}

func parsePercent(v string) (float64, error) {
	v = strings.TrimSuffix(strings.TrimSpace(v), "%")
	return strconv.ParseFloat(v, 64)
}

func parseHumanBytes(v string) (int64, error) {
	v = strings.TrimSpace(strings.ReplaceAll(v, " ", ""))
	if v == "" {
		return 0, nil
	}
	units := []string{"KiB", "MiB", "GiB", "TiB", "KB", "MB", "GB", "TB", "B"}
	mults := map[string]float64{"B": 1, "KB": 1000, "MB": 1000 * 1000, "GB": 1000 * 1000 * 1000, "TB": 1000 * 1000 * 1000 * 1000, "KiB": 1024, "MiB": 1024 * 1024, "GiB": 1024 * 1024 * 1024, "TiB": 1024 * 1024 * 1024 * 1024}
	for _, u := range units {
		if strings.HasSuffix(v, u) {
			n := strings.TrimSuffix(v, u)
			f, err := strconv.ParseFloat(n, 64)
			if err != nil {
				return 0, err
			}
			return int64(f * mults[u]), nil
		}
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, err
	}
	return int64(f), nil
}

func valueOrDefault(v, d string) string {
	if strings.TrimSpace(v) == "" {
		return d
	}
	return v
}

func avg(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	var s float64
	for _, x := range v {
		s += x
	}
	return s / float64(len(v))
}

func min(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	m := math.MaxFloat64
	for _, x := range v {
		if x < m {
			m = x
		}
	}
	return m
}

func max(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	m := -math.MaxFloat64
	for _, x := range v {
		if x > m {
			m = x
		}
	}
	return m
}

func bytesToMiB(v int64) float64 {
	return float64(v) / (1024.0 * 1024.0)
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func fail(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, "sandbench: "+msg+"\n", args...)
	os.Exit(1)
}
