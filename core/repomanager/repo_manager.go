package repomanager

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/uber/tango/core/git"
	"github.com/uber/tango/core/workspace"
	"github.com/uber/tango/tangopb"
	"github.com/gofrs/flock"
	"go.uber.org/zap"
)

const (
	lockFileName   = ".tango.lease.lock"
	lockTimeout    = 5 * time.Minute
	lockRetryDelay = 15 * time.Second
)

type RepoManager interface {
	Lease(ctx context.Context, desc tangopb.BuildDescription) (workspace.Workspace, error)
}

type repoManager struct {
	git           git.Interface
	rootWorkspace string
	logger        *zap.SugaredLogger
}

type Params struct {
	Git           git.Interface
	Logger        *zap.SugaredLogger
	RootWorkspace string
}

// NewRepoManager creates a new repo manager with the given git interface and root workspace.
func NewRepoManager(p Params) RepoManager {
	return &repoManager{git: p.Git, rootWorkspace: p.RootWorkspace, logger: p.Logger}
}

// Lease tries to take an exclusive lease on the repo’s workspace.
// If another process holds it, wait with a timeout.
func (r *repoManager) Lease(ctx context.Context, desc tangopb.BuildDescription) (workspace.Workspace, error) {
	repo := toShortRemote(desc.Remote)
	repoDir := filepath.Join(r.rootWorkspace, repo)
	// Lock file must be in the root workspace as git clone cannot be executed on a non-empty directory.
	lockPath := filepath.Join(r.rootWorkspace, lockFileName)
	if err := os.MkdirAll(r.rootWorkspace, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir repo dir: %w", err)
	}

	fl := flock.New(lockPath)

	waitCtx, cancel := context.WithTimeout(ctx, lockTimeout)
	defer cancel()

	// This will block until:
	//   1) the lock is released by another process, OR
	//   2) waitCtx expires/canceled
	locked, err := fl.TryLockContext(waitCtx, lockRetryDelay)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire lock for %s: %w", r.rootWorkspace, err)
	}
	if !locked {
		return nil, fmt.Errorf("lock timeout after %s for %s", lockTimeout, r.rootWorkspace)
	}
	if _, err := os.Stat(filepath.Join(repoDir, ".git")); os.IsNotExist(err) {
		if err := r.git.Clone(ctx, desc.Remote, repoDir); err != nil {
			flockErr := fl.Unlock()
			if flockErr != nil {
				r.logger.Errorf("unlock failed: %w", flockErr)
			}
			removeErr := os.RemoveAll(repoDir)
			if removeErr != nil {
				r.logger.Errorf("remove repo dir failed: %w", removeErr)
			}
			return nil, fmt.Errorf("clone failed: %w", err)
		}
	}
	// Use a Git interface rooted at the repo directory so commands run in the correct working directory.
	repoGit := git.New(repoDir)
	return workspace.NewWorkspace(workspace.WorkspaceParams{
		Path:   repoDir,
		Lock:   fl,
		Git:    repoGit,
		Logger: r.logger,
	}), nil
}

func toShortRemote(remote string) string {
	strs := strings.Split(remote, ":")
	return strs[len(strs)-1]
}
