package bazel

import (
	"context"
	"os"
	"os/exec"
	"time"

	"fmt"
	buildpb "github.com/bazelbuild/buildtools/build_proto"
	"go.uber.org/zap"
)

const (
	// TODO: Make this configurable
	_queryTimeout = 15 * time.Minute
)

type QueryRequest struct {
	Query          string
	StartupOptions []string
	AdditionalArgs []string
}

type QueryResponse struct {
	Result *buildpb.QueryResult
}

type Bazel interface {
	ExecuteQuery(ctx context.Context, req *QueryRequest) (*QueryResponse, error)
}

// BazelClient is a client for interacting with Bazel.
type BazelClient struct {
	workspacePath      string
	envVarsMap         map[string]string
	bazelCommand       string
	logger             *zap.SugaredLogger
	execCommandContext func(ctx context.Context, name string, arg ...string) commander
	queryTimeout       time.Duration
}

type Params struct {
	BazelCommand       string
	WorkspacePath      string
	EnvVarsMap         map[string]string
	Logger             *zap.SugaredLogger
	ExecCommandContext func(ctx context.Context, name string, arg ...string) commander
	QueryTimeout       time.Duration
}

func NewBazelClient(p Params) *BazelClient {
	execCmd := p.ExecCommandContext
	if execCmd == nil {
		execCmd = func(ctx context.Context, name string, arg ...string) commander {
			cmd := exec.CommandContext(ctx, name, arg...)
			cmd.Dir = p.WorkspacePath
			for key, value := range p.EnvVarsMap {
				cmd.Env = append(cmd.Env, key+"="+value)
			}
			cmd.Stdin = nil
			return cmd
		}
	}
	timeout := p.QueryTimeout
	if timeout == 0 {
		timeout = _queryTimeout
	}
	return &BazelClient{
		workspacePath:      p.WorkspacePath,
		envVarsMap:         p.EnvVarsMap,
		bazelCommand:       p.BazelCommand,
		logger:             p.Logger,
		execCommandContext: execCmd,
		queryTimeout:       timeout,
	}
}

func DetectBazelExecutable() (string, error) {
	// TODO: read from config file to get the bazel executable path. Fallback to the following logic if not provided.
	// Most correct: honor Bazelisk wrapper env vars
	if p, ok := bazelFromEnv(); ok {
		return p, nil
	}

	// Otherwise fall back to PATH search
	path, err := exec.LookPath("bazel")
	if err != nil {
		return "", fmt.Errorf("could not locate bazel: %w", err)
	}
	return path, nil
}

func bazelFromEnv() (string, bool) {
	for _, key := range []string{"BAZEL_REAL", "BAZELISK_BIN", "BAZEL"} {
		if v := os.Getenv(key); v != "" {
			return v, true
		}
	}
	return "", false
}
