package pool

import (
	"context"
	"testing"
	"time"

	"github.com/p-arndt/sandkasten/internal/config"
	storemod "github.com/p-arndt/sandkasten/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testPoolStore(t *testing.T) *storemod.Store {
	t.Helper()
	st, err := storemod.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	return st
}

func TestNew_Disabled(t *testing.T) {
	cfg := &config.Config{Pool: config.PoolConfig{Enabled: false, Images: map[string]int{"python": 3}}}
	pl := New(cfg, PoolConfig{Store: testPoolStore(t)})
	assert.Nil(t, pl)
}

func TestNew_EmptyImages(t *testing.T) {
	cfg := &config.Config{Pool: config.PoolConfig{Enabled: true, Images: map[string]int{}}}
	pl := New(cfg, PoolConfig{Store: testPoolStore(t)})
	assert.Nil(t, pl)
}

func TestGet_EmptyPool(t *testing.T) {
	cfg := &config.Config{
		Pool: config.PoolConfig{Enabled: true, Images: map[string]int{"python": 2}},
	}
	st := testPoolStore(t)
	createCount := 0
	pl := New(cfg, PoolConfig{
		Store:      st,
		PoolExpiry: 24 * time.Hour,
		CreateFunc: func(ctx context.Context, sessionID string, image string) (*CreateResult, error) {
			createCount++
			return &CreateResult{InitPID: 1, CgroupPath: "/cgroup/" + sessionID}, nil
		},
	})
	require.NotNil(t, pl)

	sid, ok := pl.Get(context.Background(), "python", "")
	assert.False(t, ok)
	assert.Empty(t, sid)
}

func TestRefillAndGet(t *testing.T) {
	cfg := &config.Config{
		Pool:        config.PoolConfig{Enabled: true, Images: map[string]int{"python": 2}},
		AllowedImages: []string{},
	}
	st := testPoolStore(t)
	var createdIDs []string
	pl := New(cfg, PoolConfig{
		Store:      st,
		PoolExpiry: 24 * time.Hour,
		CreateFunc: func(ctx context.Context, sessionID string, image string) (*CreateResult, error) {
			createdIDs = append(createdIDs, sessionID)
			return &CreateResult{InitPID: 1, CgroupPath: "/cgroup/" + sessionID}, nil
		},
	})
	require.NotNil(t, pl)

	require.NoError(t, pl.Refill(context.Background(), "python", 2))
	assert.Len(t, createdIDs, 2)

	sid1, ok := pl.Get(context.Background(), "python", "")
	require.True(t, ok)
	assert.NotEmpty(t, sid1)
	assert.Contains(t, createdIDs, sid1)

	sid2, ok := pl.Get(context.Background(), "python", "")
	require.True(t, ok)
	assert.NotEmpty(t, sid2)
	assert.NotEqual(t, sid1, sid2)

	_, ok = pl.Get(context.Background(), "python", "")
	assert.False(t, ok)
}

func TestGet_WithWorkspaceID(t *testing.T) {
	cfg := &config.Config{
		Pool: config.PoolConfig{Enabled: true, Images: map[string]int{"python": 1}},
	}
	st := testPoolStore(t)
	pl := New(cfg, PoolConfig{
		Store:      st,
		PoolExpiry: 24 * time.Hour,
		CreateFunc: func(ctx context.Context, sessionID string, image string) (*CreateResult, error) {
			return &CreateResult{InitPID: 1, CgroupPath: "/cgroup/" + sessionID}, nil
		},
	})
	require.NotNil(t, pl)
	require.NoError(t, pl.Refill(context.Background(), "python", 1))

	sid, ok := pl.Get(context.Background(), "python", "my-workspace")
	require.True(t, ok)
	assert.NotEmpty(t, sid)
}

func TestRefill_RespectsTarget(t *testing.T) {
	cfg := &config.Config{
		Pool: config.PoolConfig{Enabled: true, Images: map[string]int{"python": 3}},
	}
	st := testPoolStore(t)
	createCount := 0
	pl := New(cfg, PoolConfig{
		Store:      st,
		PoolExpiry: 24 * time.Hour,
		CreateFunc: func(ctx context.Context, sessionID string, image string) (*CreateResult, error) {
			createCount++
			return &CreateResult{InitPID: 1, CgroupPath: "/cgroup/" + sessionID}, nil
		},
	})
	require.NotNil(t, pl)

	require.NoError(t, pl.Refill(context.Background(), "python", 2))
	assert.Equal(t, 2, createCount)

	require.NoError(t, pl.Refill(context.Background(), "python", 1))
	assert.Equal(t, 2, createCount, "should not create more, already at 2")

	require.NoError(t, pl.Refill(context.Background(), "python", 3))
	assert.Equal(t, 3, createCount)
}

func TestRefill_FiltersAllowedImages(t *testing.T) {
	cfg := &config.Config{
		Pool:          config.PoolConfig{Enabled: true, Images: map[string]int{"python": 1, "node": 1}},
		AllowedImages: []string{"python"},
	}
	st := testPoolStore(t)
	pl := New(cfg, PoolConfig{
		Store:      st,
		PoolExpiry: 24 * time.Hour,
		CreateFunc: func(ctx context.Context, sessionID string, image string) (*CreateResult, error) {
			return &CreateResult{InitPID: 1, CgroupPath: "/cgroup/" + sessionID}, nil
		},
	})
	require.NotNil(t, pl)

	pl.RefillAll(context.Background())

	_, ok := pl.Get(context.Background(), "python", "")
	assert.True(t, ok)

	_, ok = pl.Get(context.Background(), "node", "")
	assert.False(t, ok, "node not in allowed list, should not be pooled")
}
