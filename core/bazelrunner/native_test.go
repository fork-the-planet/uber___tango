package bazelrunner

import (
	"context"
	"testing"

	"github.com/uber/tango/core/bazel"
	"github.com/uber/tango/core/bazel/bazelmock"
	gitmock "github.com/uber/tango/core/git/gitmock"
	"github.com/uber/tango/core/workspace"
	buildpb "github.com/bazelbuild/buildtools/build_proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestCompute_CallsBazelAndReturnsResult(t *testing.T) {
	ctrl := gomock.NewController(t)
	bazelMock := bazelmock.NewMockBazel(ctrl)
	gitMock := gitmock.NewMockInterface(ctrl)
	gitMock.EXPECT().FileHashes(gomock.Any(), gomock.Any()).Return(map[string][]byte{}, nil)
	ruleName := "//:a"
	ruleClass := "go_library"
	bazelMock.EXPECT().ExecuteQuery(gomock.Any(), gomock.Any()).Return(&bazel.QueryResponse{Result: &buildpb.QueryResult{Target: []*buildpb.Target{
		{
			Type: buildpb.Target_RULE.Enum(),
			Rule: &buildpb.Rule{
				Name:      &ruleName,
				RuleClass: &ruleClass,
			},
		},
	}}}, nil)
	gr := NewNativeGraphRunner(NativeGraphRunnerParams{
		BazelClient: bazelMock,
		GitClient:   gitMock,
		// leave HashConfig zero; not asserted here
	})
	ws := workspace.NewWorkspace(workspace.WorkspaceParams{
		Path: "/tmp/ws",
	})

	res, err := gr.Compute(context.Background(), ws)
	// get target hash
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, 1, len(res.Targets))
	assert.Equal(t, ruleName, res.Targets[ruleName].Name)
	assert.Equal(t, ruleClass, res.Targets[ruleName].Rule.GetRuleClass())
}

func TestCompute_PropagatesError(t *testing.T) {
	ctrl := gomock.NewController(t)
	bazelMock := bazelmock.NewMockBazel(ctrl)
	bazelMock.EXPECT().ExecuteQuery(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)
	gr := NewNativeGraphRunner(NativeGraphRunnerParams{BazelClient: bazelMock})
	ws := workspace.NewWorkspace(workspace.WorkspaceParams{
		Path: "/tmp/ws",
	})

	res, err := gr.Compute(context.Background(), ws)
	require.Error(t, err)
	// Expect an empty result on error
	assert.NotNil(t, res.Targets)
	assert.Zero(t, len(res.Targets))
}
