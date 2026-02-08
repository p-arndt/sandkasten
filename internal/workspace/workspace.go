package workspace

import (
	"context"
	"fmt"
	"time"

	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
)

// Manager handles persistent workspace volumes.
type Manager struct {
	docker *client.Client
}

// Workspace represents a persistent storage volume.
type Workspace struct {
	ID        string
	CreatedAt time.Time
	SizeMB    int64
	Labels    map[string]string
}

func NewManager(dockerClient *client.Client) *Manager {
	return &Manager{docker: dockerClient}
}

// Create creates a new persistent workspace volume.
func (m *Manager) Create(ctx context.Context, workspaceID string, labels map[string]string) error {
	if labels == nil {
		labels = make(map[string]string)
	}
	labels["sandkasten.workspace"] = "true"
	labels["sandkasten.workspace_id"] = workspaceID

	_, err := m.docker.VolumeCreate(ctx, volume.CreateOptions{
		Name:   workspaceID,
		Driver: "local",
		Labels: labels,
	})
	return err
}

// Exists checks if a workspace volume exists.
func (m *Manager) Exists(ctx context.Context, workspaceID string) (bool, error) {
	_, err := m.docker.VolumeInspect(ctx, workspaceID)
	if err != nil {
		if client.IsErrNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// List returns all workspace volumes.
func (m *Manager) List(ctx context.Context) ([]*Workspace, error) {
	vols, err := m.docker.VolumeList(ctx, volume.ListOptions{
		Filters: map[string][]string{
			"label": {"sandkasten.workspace=true"},
		},
	})
	if err != nil {
		return nil, err
	}

	workspaces := make([]*Workspace, 0, len(vols.Volumes))
	for _, v := range vols.Volumes {
		ws := &Workspace{
			ID:     v.Name,
			Labels: v.Labels,
		}

		// Parse created time if available
		if createdAt, err := time.Parse(time.RFC3339, v.CreatedAt); err == nil {
			ws.CreatedAt = createdAt
		}

		workspaces = append(workspaces, ws)
	}

	return workspaces, nil
}

// Delete removes a workspace volume.
func (m *Manager) Delete(ctx context.Context, workspaceID string) error {
	return m.docker.VolumeRemove(ctx, workspaceID, false)
}

// GetVolumeName returns the Docker volume name for a workspace.
func GetVolumeName(workspaceID string) string {
	return fmt.Sprintf("sandkasten-ws-%s", workspaceID)
}

// GenerateWorkspaceID generates a workspace ID from user context.
func GenerateWorkspaceID(userID, projectID string) string {
	if projectID != "" {
		return fmt.Sprintf("%s-%s", userID, projectID)
	}
	return userID
}
