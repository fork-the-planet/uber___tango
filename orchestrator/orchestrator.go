package orchestrator

import (
	"github.com/uber/tango/core/storage"
	"context"

	"github.com/uber/tango/tangopb"
)

// GetTargetGraphParam is the input of GetTargetGraph
type GetTargetGraphParam struct {
	Req *tangopb.GetTargetGraphRequest
}

// ChangedTargetsParam is the input of ComputeChangedTargets
type ChangedTargetsParam struct {
	Req *tangopb.GetChangedTargetsRequest
}

// Orchestrator defines high-level execution interface that "does everything"
type Orchestrator interface {
	GetTargetGraph(ctx context.Context, param GetTargetGraphParam) (storage.GraphReader, error)
}
