package repomanager

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	gitmock "github.com/uber/tango/core/git/gitmock"
	"github.com/uber/tango/tangopb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func TestLease_CreatesRepoDirAndClones_WhenGitMissing(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	g := gitmock.NewMockInterface(ctrl)

	root := t.TempDir()
	remote := "git@github.com:org/repo"
	repoDir := filepath.Join(root, "org/repo")

	g.EXPECT().Clone(gomock.Any(), remote, repoDir).Return(nil)

	rm := NewRepoManager(Params{Git: g, Logger: zap.NewNop().Sugar(), RootWorkspace: root})
	ctx := context.Background()
	ws, err := rm.Lease(ctx, tangopb.BuildDescription{Remote: remote})
	require.NoError(t, err)
	require.NotNil(t, ws)
	defer func() { require.NoError(t, ws.Release()) }()
	assert.Equal(t, repoDir, ws.Path())
}

func TestLease_SkipsClone_WhenGitPresent(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	g := gitmock.NewMockInterface(ctrl)

	root := t.TempDir()
	remote := "git@github.com:org/repo"
	repoDir := filepath.Join(root, "org/repo")
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755))

	// No expectation for Clone; if called, gomock will fail the test
	rm := NewRepoManager(Params{Git: g, Logger: zap.NewNop().Sugar(), RootWorkspace: root})
	ws, err := rm.Lease(context.Background(), tangopb.BuildDescription{Remote: remote})
	require.NoError(t, err)
	require.NotNil(t, ws)
	defer func() { require.NoError(t, ws.Release()) }()

	assert.Equal(t, repoDir, ws.Path())
}

func TestLease_CloneFails_ReleasesLock(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	g := gitmock.NewMockInterface(ctrl)

	root := t.TempDir()
	remote := "git@github.com:org/repo"
	repoDir := filepath.Join(root, "org/repo")

	g.EXPECT().Clone(gomock.Any(), remote, repoDir).Return(assert.AnError)

	rm := NewRepoManager(Params{Git: g, Logger: zap.NewNop().Sugar(), RootWorkspace: root})
	_, err := rm.Lease(context.Background(), tangopb.BuildDescription{Remote: remote})
	require.Error(t, err)

	// Repo dir should be removed on clone failure
	_, statErr := os.Stat(repoDir)
	require.Error(t, statErr)
	assert.True(t, os.IsNotExist(statErr), "repo dir should be removed after clone failure")
}

func TestLease_LockUnavailable_CtxCanceled(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	root := t.TempDir()
	remote := "git@github.com:org/repo"

	rm := NewRepoManager(Params{Git: gitmock.NewMockInterface(ctrl), Logger: zap.NewNop().Sugar(), RootWorkspace: root})
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately so TryLockContext returns quickly
	ws, err := rm.Lease(ctx, tangopb.BuildDescription{Remote: remote})
	require.Error(t, err)
	require.Nil(t, ws)
	assert.Contains(t, err.Error(), "failed to acquire lock")
}
