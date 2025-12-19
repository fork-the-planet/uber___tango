package bazelrunner

import (
	"context"

	"github.com/uber/tango/core/targethasher"
	"github.com/uber/tango/core/workspace"
)

// GraphRunner defines interface for computing graph in a bazel workspace
type GraphRunner interface {
	// Compute computes a graph for the given workspace
	Compute(ctx context.Context, ws workspace.Workspace) (targethasher.Result, error)
}
