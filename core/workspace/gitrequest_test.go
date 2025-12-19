package workspace

import (
	"testing"
	"context"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	gitmock "github.com/uber/tango/core/git/gitmock"
)

func TestNewGitRequest_InvalidPath(t *testing.T) {
	req := NewGitRequest(nil, "invalid", "baseRef")
	require.NotNil(t, req)
}

func TestNewGitRequest_ExtractsID(t *testing.T) {
	r := NewGitRequest(nil, "/org/repo/pull/456", "baseRef")
	gr, ok := r.(*gitRequest)
	assert.True(t, ok, "expected *gitRequest, got %T", r)
	assert.Equal(t, "456", gr.requestID)
	assert.Equal(t, "baseRef", gr.baseRef)
}

func TestGitRequest_Apply_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	git := gitmock.NewMockInterface(ctrl)
	git.EXPECT().Fetch(gomock.Any(), "origin", gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().Diff(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
	git.EXPECT().ApplyPatch(gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().Commit(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().SubmoduleUpdate(gomock.Any()).Return(nil)
	req := NewGitRequest(git, "123", "baseRef")
	err := req.Apply(context.Background())
	require.NoError(t, err)
}
