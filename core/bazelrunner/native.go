package bazelrunner

import (
	"context"

	"github.com/uber/tango/core/bazel"
	"github.com/uber/tango/core/git"
	"github.com/uber/tango/core/targethasher"
	"github.com/uber/tango/core/workspace"
)

type nativeGraphRunner struct {
	bazel                  bazel.Bazel
	git                    git.Interface
	excludeExternalTargets bool
	bzlmodEnabled          bool
}

type NativeGraphRunnerParams struct {
	BazelClient            bazel.Bazel
	GitClient              git.Interface
	ExcludeExternalTargets bool
	BzlmodEnabled          bool
}

// graph runner takes in a bazel query request and computes the graph
func NewNativeGraphRunner(p NativeGraphRunnerParams) GraphRunner {
	return &nativeGraphRunner{
		bazel:                  p.BazelClient,
		git:                    p.GitClient,
		excludeExternalTargets: p.ExcludeExternalTargets,
		bzlmodEnabled:          p.BzlmodEnabled,
	}
}

func (g *nativeGraphRunner) Compute(ctx context.Context, ws workspace.Workspace) (targethasher.Result, error) {
	query := "//external:all-targets + deps(//...:all-targets)"
	if g.excludeExternalTargets {
		query = "deps(//...:all-targets)"
	}
	queryResult, err := g.bazel.ExecuteQuery(ctx, &bazel.QueryRequest{
		Query: query,
		// --order_output=no will make Bazel execute query faster
		// --proto:locations: we need to get external file location to make CTC more accurate
		// --noproto: parameters exclude fields from the output that are not used for hashing anyways, making
		// proto blob smaller and serialization/deserialization faster
		AdditionalArgs: []string{"--order_output=no", "--proto:locations", "--noproto:default_values"},
	})
	if err != nil {
		return targethasher.EmptyResult(), err
	}
	knownSourceHashes, err := g.git.FileHashes(ctx, "HEAD")
	if err != nil {
		return targethasher.EmptyResult(), err
	}
	hashConfig := targethasher.HashConfig{
		KnownSourceHashes: knownSourceHashes,
		// TODO: Make this configurable
		FullHashRepos: []string{},
		ExcludedFiles: []string{},
	}
	res, err := targethasher.FromProto(ctx, queryResult.Result, ws.Path(), hashConfig, g.bzlmodEnabled)
	if err != nil {
		return targethasher.EmptyResult(), err
	}
	return res, nil
}
