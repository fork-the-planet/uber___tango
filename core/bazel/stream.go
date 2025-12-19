package bazel

import (
	"bufio"
	"context"
	"io"

	buildpb "github.com/bazelbuild/buildtools/build_proto"
	"google.golang.org/protobuf/encoding/protodelim"
)

func streamOutput(ctx context.Context, src io.Reader, dst io.Writer) error {
	done := make(chan error, 1)
	go func() {
		_, err := io.Copy(dst, src)
		done <- err
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}

func streamAndParseTargets(ctx context.Context, src io.Reader, dst io.Writer) (*buildpb.QueryResult, error) {
	type result struct {
		queryResult *buildpb.QueryResult
		err         error
	}
	done := make(chan result, 1)

	go func() {
		queryResult, err := getQueryResult(src, dst)
		done <- result{queryResult: queryResult, err: err}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-done:
		return res.queryResult, res.err
	}
}

// getQueryResult reads a QueryResult containing targets from the stream and returns it.
func getQueryResult(src io.Reader, dst io.Writer) (*buildpb.QueryResult, error) {
	result := &buildpb.QueryResult{
		Target: make([]*buildpb.Target, 0),
	}
	tr := io.TeeReader(src, dst)
	br := bufio.NewReader(tr)

	for {
		var target buildpb.Target
		err := protodelim.UnmarshalFrom(br, &target)
		if err == io.EOF {
			break
		}
		if err != nil {
			return result, err
		}
		result.Target = append(result.Target, &target)
	}

	return result, nil
}
