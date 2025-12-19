package controller

import (
	"context"
	"errors"
	"fmt"
	"io"

	pb "github.com/uber/tango/tangopb"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

// job represents a single goroutine of getting a target graph
type job struct {
	graphStreamChunks []*pb.GetTargetGraphResponse
	err               error
	cancelled         bool
	completed         bool
	ctx               context.Context
	cancel            context.CancelFunc
}

// GetChangedTargets returns the changed targets between two revisions.
func (c *controller) GetChangedTargets(request *pb.GetChangedTargetsRequest, stream pb.TangoServiceGetChangedTargetsYARPCServer) error {
	ctx := context.Background()
	if err := validateGetChangedTargetsRequest(request); err != nil {
		c.logger.Error("GetChangedTargets: Invalid request", zap.Error(err))
		return err
	}

	c.logger.Info("GetChangedTargets: Processing request",
		zap.String("first_revision_remote", request.GetFirstRevision().GetRemote()),
		zap.String("first_revision_base_sha", request.GetFirstRevision().GetBaseSha()),
		zap.String("second_revision_remote", request.GetSecondRevision().GetRemote()),
		zap.String("second_revision_base_sha", request.GetSecondRevision().GetBaseSha()),
	)

	jobs := make([]*job, 2)

	for i := 0; i < 2; i++ {
		// create independent contexts for each job; if one of the jobs fails, the other one should be cancelled to save resources and improve latency
		ctxNew, cancelNew := context.WithCancel(ctx)
		defer cancelNew()
		jobs[i] = &job{ctx: ctxNew, cancel: cancelNew}
	}

	// Start jobs for both revisions. Success or failure, the result will report to the results channel.
	type graphResult struct {
		// order is 0 or 1, 0 is the base (first) revision, 1 is the target (second) revision
		order int
		// TODO: pb.GetTargetGraphResponse is a stream, so most likely we can't use GetTargetGraphResponse as a return type and we'll want to read it fully before joining the threads
		graph *pb.GetTargetGraphResponse
		err   error
	}
	results := make(chan graphResult, len(jobs))

	for i := 0; i < len(jobs); i++ {
		i := i
		go func(idx int) {
			var revision *pb.BuildDescription
			if idx == 0 {
				revision = request.GetFirstRevision()
			} else {
				revision = request.GetSecondRevision()
			}
			graphReader, err := c.getGraph(jobs[idx].ctx, revision, request.GetOutputConfig())
			if err != nil {
				results <- graphResult{order: idx, err: err}
				return
			}
			if graphReader == nil {
				results <- graphResult{order: idx, err: nil}
				return
			}
			graph, err := graphReader.Read()
			results <- graphResult{order: idx, graph: graph, err: err}
		}(i)
	}

	// Wait for both results to complete, either successfully or with an error.
	for range jobs {
		select {
		case res := <-results:
			if res.graph != nil {
				jobs[res.order].graphStreamChunks = append(jobs[res.order].graphStreamChunks, res.graph)
			}
			if res.graph == nil {
				jobs[res.order].completed = true
			}
			if res.err == io.EOF {
				res.err = nil
				jobs[res.order].completed = true
			}
			if res.err != nil {
				jobs[res.order].err = res.err

				// one of the computations failed, if the other one has not completed yet, cancel it and wait for the result to come in, which would be a context cancelled result then
				other := (res.order + 1) % 2
				if !jobs[other].completed {
					jobs[other].cancel()

					// explicitly mark that this job is cancelled, so we can ignore its error later
					jobs[other].cancelled = true
				}
			}
		}
	}

	if ctx.Err() != nil {
		// If the context was cancelled by the upstream, just return the original error without additional augmentation
		return ctx.Err()
	}

	// Process errors, only aggregating the ones that are original ones and not a result of the other job being cancelled
	var err error
	for i, job := range jobs {
		if job.err != nil {
			if job.cancelled {
				// this only happens as a result of the other job failing, so we can ignore the error
				continue
			}
			err = multierr.Append(err, fmt.Errorf("failed to get target graph #%d: %w", i+1, job.err))
		}
	}

	if err != nil {
		return err
	}

	// At this point we should have both graphs computed successfully. Let's compare them.
	firstGraph := jobs[0].graphStreamChunks
	secondGraph := jobs[1].graphStreamChunks

	// TODO: Implement graph comparison logic
	changedTargetsResponse, err := c.compareTargetGraphs(ctx, firstGraph, secondGraph, request.GetOutputConfig())
	if err != nil {
		c.logger.Error("GetChangedTargets: Failed to compare target graphs", zap.Error(err))
		return fmt.Errorf("failed to compare target graphs: %w", err)
	}

	if changedTargetsResponse != nil {
		if err := stream.Send(changedTargetsResponse); err != nil {
			c.logger.Error("GetChangedTargets: Failed to send response", zap.Error(err))
			return fmt.Errorf("failed to send response: %w", err)
		}
	}

	c.logger.Info("GetChangedTargets: Successfully processed request")
	return nil
}

func (c *controller) compareTargetGraphs(ctx context.Context, firstGraph, secondGraph []*pb.GetTargetGraphResponse, outputConfig *pb.OutputConfig) (*pb.GetChangedTargetsResponse, error) {
	c.logger.Info("compareTargetGraphs: Computing differences between target graphs")

	// TODO: Implement the actual change computation logic
	return &pb.GetChangedTargetsResponse{
		Item: &pb.GetChangedTargetsResponse_Metadata{
			Metadata: &pb.Metadata{},
		},
	}, nil
}

func validateGetChangedTargetsRequest(request *pb.GetChangedTargetsRequest) error {
	if request == nil {
		return errors.New("request cannot be nil")
	}
	if request.GetFirstRevision() == nil {
		return errors.New("first revision is required")
	}
	if request.GetSecondRevision() == nil {
		return errors.New("second revision is required")
	}
	firstRevision := request.GetFirstRevision()
	if firstRevision.GetRemote() == "" {
		return errors.New("first revision remote is required")
	}
	if firstRevision.GetBaseSha() == "" {
		return errors.New("first revision base_sha is required")
	}
	secondRevision := request.GetSecondRevision()
	if secondRevision.GetRemote() == "" {
		return errors.New("second revision remote is required")
	}
	if secondRevision.GetBaseSha() == "" {
		return errors.New("second revision base_sha is required")
	}
	// Validate that both revisions have the same remote
	if firstRevision.GetRemote() != secondRevision.GetRemote() {
		return errors.New("first and second revision must have the same remote")
	}
	return nil
}
