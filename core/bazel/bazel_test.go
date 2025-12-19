package bazel

import (
	"context"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewBazelClient(t *testing.T) {
	tests := []struct {
		name   string
		params Params
	}{
		{
			name: "with exec command context",
			params: Params{
				BazelCommand:  "bazel",
				WorkspacePath: "/tmp/test",
				EnvVarsMap:    map[string]string{"FOO": "bar"},
				Logger:        zap.NewNop().Sugar(),
				ExecCommandContext: func(ctx context.Context, name string, arg ...string) commander {
					return nil
				},
			},
		},
		{
			name: "without exec command context",
			params: Params{
				BazelCommand:  "bazel",
				WorkspacePath: "/tmp/test",
				EnvVarsMap:    map[string]string{"FOO": "bar"},
				Logger:        zap.NewNop().Sugar(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewBazelClient(tt.params)
			require.NotNil(t, client)
			assert.Equal(t, tt.params.BazelCommand, client.bazelCommand)
			assert.Equal(t, tt.params.WorkspacePath, client.workspacePath)
			assert.Equal(t, tt.params.EnvVarsMap, client.envVarsMap)
			assert.Equal(t, tt.params.Logger, client.logger)
			assert.NotNil(t, client.execCommandContext)
		})
	}
}

func TestNewBazelClient_WithNilExecCommand(t *testing.T) {
	client := NewBazelClient(Params{
		BazelCommand:  "bazel",
		WorkspacePath: "/workspace",
		EnvVarsMap:    map[string]string{"KEY": "value"},
		Logger:        zap.NewNop().Sugar(),
	})
	require.NotNil(t, client)
	require.NotNil(t, client.execCommandContext)

	cmd := client.execCommandContext(context.Background(), "test", "arg1")
	require.NotNil(t, cmd)
	execCmd, ok := cmd.(*exec.Cmd)
	require.True(t, ok)
	assert.Equal(t, "/workspace", execCmd.Dir)
	assert.Contains(t, execCmd.Env, "KEY=value")
}
