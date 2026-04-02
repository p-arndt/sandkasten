package autopull

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/p-arndt/sandkasten/internal/config"
)

// WellKnownImages maps short image names to OCI references.
// When a user requests image "python", we pull "python:3.12-slim" automatically.
var WellKnownImages = map[string]string{
	"python": "python:3.12-slim",
	"node":   "node:22-slim",
	"base":   "alpine:latest",
	"ubuntu": "ubuntu:24.04",
	"golang": "golang:1.25-alpine",
	"ruby":   "ruby:3.3-slim",
	"rust":   "rust:1-slim",
}

// Puller implements session.ImagePuller by pulling OCI images from registries.
type Puller struct {
	cfg    *config.Config
	logger *slog.Logger

	// pullMu serializes pulls for the same image name to avoid duplicate work.
	pullMu sync.Map // image name -> *sync.Mutex
}

// New creates a new auto-pull handler.
func New(cfg *config.Config, logger *slog.Logger) *Puller {
	return &Puller{
		cfg:    cfg,
		logger: logger,
	}
}

// ImageExists returns true if the image directory exists in the data dir.
func (p *Puller) ImageExists(image string) bool {
	metaPath := filepath.Join(p.cfg.DataDir, "images", image, "meta.json")
	_, err := os.Stat(metaPath)
	return err == nil
}

// PullImage pulls an OCI image and stores it under the given name.
// It resolves well-known short names (e.g. "python" -> "python:3.12-slim").
func (p *Puller) PullImage(ctx context.Context, imageName string) error {
	// Serialize pulls for the same image name.
	muIface, _ := p.pullMu.LoadOrStore(imageName, &sync.Mutex{})
	mu := muIface.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()

	// Double-check after acquiring lock (another goroutine may have pulled it).
	if p.ImageExists(imageName) {
		return nil
	}

	ref := p.resolveRef(imageName)
	p.logger.Info("auto-pulling image", "image", imageName, "ref", ref)

	if err := p.pullOCI(ctx, imageName, ref); err != nil {
		return fmt.Errorf("pull %s (%s): %w", imageName, ref, err)
	}

	p.logger.Info("auto-pull complete", "image", imageName)
	return nil
}

// resolveRef maps an image name to an OCI reference.
func (p *Puller) resolveRef(imageName string) string {
	if ref, ok := WellKnownImages[imageName]; ok {
		return ref
	}
	// If not well-known, try the name as a direct OCI reference.
	// e.g. "python" -> "python:latest"
	return imageName + ":latest"
}

// pullOCI pulls an image from an OCI registry and extracts layers.
// This mirrors the logic in cmd/sandkasten/commands_linux.go pullImage(),
// but operates through the config's data directory.
func (p *Puller) pullOCI(ctx context.Context, imageName, ref string) error {
	dataDir := p.cfg.DataDir
	imageDir := filepath.Join(dataDir, "images", imageName)

	if err := os.MkdirAll(imageDir, 0755); err != nil {
		return fmt.Errorf("create image dir: %w", err)
	}

	// Clean up on failure.
	success := false
	defer func() {
		if !success {
			_ = os.RemoveAll(imageDir)
		}
	}()

	parsedRef, err := name.ParseReference(ref, name.WeakValidation)
	if err != nil {
		return fmt.Errorf("parse reference: %w", err)
	}

	img, err := remote.Image(parsedRef, remote.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("fetch image: %w", err)
	}

	layers, err := img.Layers()
	if err != nil {
		return fmt.Errorf("resolve layers: %w", err)
	}

	var layerIDs []string
	for _, layer := range layers {
		digest, err := layer.Digest()
		if err != nil {
			return fmt.Errorf("layer digest: %w", err)
		}
		layerID := digest.Hex
		layerIDs = append(layerIDs, layerID)

		layerRootfs := filepath.Join(dataDir, "layers", layerID, "rootfs")
		if _, err := os.Stat(layerRootfs); err == nil {
			continue // Already extracted
		}
		if err := os.MkdirAll(layerRootfs, 0755); err != nil {
			return fmt.Errorf("create layer rootfs: %w", err)
		}

		reader, err := layer.Uncompressed()
		if err != nil {
			return fmt.Errorf("open layer: %w", err)
		}

		if err := extractLayer(layerRootfs, reader); err != nil {
			reader.Close()
			return fmt.Errorf("extract layer %s: %w", layerID, err)
		}
		if err := reader.Close(); err != nil {
			return fmt.Errorf("close layer: %w", err)
		}
	}

	digest, err := img.Digest()
	if err != nil {
		return fmt.Errorf("compute digest: %w", err)
	}

	meta := imageMeta{
		Name:      imageName,
		Hash:      digest.String(),
		Layers:    layerIDs,
	}
	metaData, err := marshalJSON(meta)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(imageDir, "meta.json"), metaData, 0644); err != nil {
		return fmt.Errorf("write meta: %w", err)
	}

	// Inject runner binary into the layers directory.
	if err := injectRunner(dataDir); err != nil {
		return err
	}

	success = true
	return nil
}
