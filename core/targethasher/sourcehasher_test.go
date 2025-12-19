package targethasher

import (
	"os"
	"path/filepath"
	"testing"

	buildpb "github.com/bazelbuild/buildtools/build_proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TODO: Add tests to check for bazel query for proto.bin output.
func TestNewSourceHasher_buildsMaps(t *testing.T) {
	h := NewSourceHasher(Params{
		WorkspaceRoot: "/ws",
		HashConfig: HashConfig{
			KnownSourceHashes: map[string][]byte{"a/b.txt": []byte("abc")},
			FullHashRepos:     []string{"", "extA"},
			ExcludedFiles:     []string{`.*\.gen\.go`},
		},
	})
	dh, ok := h.(*diskHashHelper)
	assert.True(t, ok, "expected *diskHashHelper, got %T", h)
	_, ok = dh.fullHashRepos[""]
	assert.True(t, ok, "fullHashRepos should contain main repo key \"\"")
	_, ok = dh.fullHashRepos["extA"]
	assert.True(t, ok, "fullHashRepos should contain \"extA\"")
	_, ok = dh.excludedFiles[`.*\.gen\.go`]
	assert.True(t, ok, "excludedFiles should contain the provided regex")
	assert.Equal(t, "abc", string(dh.knownFileHashes["a/b.txt"]), "knownFileHashes mismatch")
}

func TestNoOpHasher_returnsNil(t *testing.T) {
	h := &noOpHasher{}
	sf := &buildpb.SourceFile{Name: strPtr("//:dummy")}
	got, err := h.HashSourceFile(sf, nil)
	assert.NoError(t, err, "unexpected error: %v", err)
	assert.Nil(t, got, "expected nil hash, got %v", got)
}

func TestDiskHashHelper_ExcludedFile(t *testing.T) {
	tmp := t.TempDir()
	wsRoot := tmp

	// File that matches the exclude pattern.
	path := filepath.Join(wsRoot, "foo.gen.go")
	require.NoError(t, os.WriteFile(path, []byte("hello"), 0o644))

	h := &diskHashHelper{
		workspaceroot:   wsRoot,
		knownFileHashes: map[string][]byte{},
		fullHashRepos:   map[string]struct{}{"": {}}, // main repo uses full hashing
		excludedFiles:   map[string]struct{}{`.*\.gen\.go`: {}},
	}

	sf := &buildpb.SourceFile{
		Name:            strPtr("//:foo.gen.go"),
		Location:        strPtr(path + ":1:1"),
		VisibilityLabel: []string{"//visibility:private"},
	}
	hash, err := h.HashSourceFile(sf, nil)
	assert.NoError(t, err, "unexpected error: %v", err)
	assert.Equal(t, []byte{}, hash, "expected empty hash for excluded file, got %x", hash)
}

func TestDiskHashHelper_KnownFileHashUsed(t *testing.T) {
	tmp := t.TempDir()
	wsRoot := tmp
	rel := filepath.Join("pkg", "file.txt")
	abs := filepath.Join(wsRoot, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		require.NoError(t, err, "mkdir: %v", err)
	}
	if err := os.WriteFile(abs, []byte("contents"), 0o644); err != nil {
		require.NoError(t, err, "write: %v", err)
	}

	known := []byte("KNOWN")
	h := &diskHashHelper{
		workspaceroot:   wsRoot,
		knownFileHashes: map[string][]byte{filepath.ToSlash(rel): known},
		fullHashRepos:   map[string]struct{}{"": {}}, // main repo full hashing enabled
		excludedFiles:   map[string]struct{}{},
	}
	sf := &buildpb.SourceFile{
		Name:            strPtr("//" + filepath.ToSlash(filepath.Dir(rel)) + ":" + filepath.Base(rel)),
		Location:        strPtr(abs + ":1:1"),
		VisibilityLabel: []string{"//visibility:private"},
	}
	got, err := h.HashSourceFile(sf, nil)
	assert.NoError(t, err, "unexpected err: %v", err)
	assert.Equal(t, string(known), string(got), "expected known hash %q, got %q", known, got)
}

func TestDiskHashHelper_NonDefaultVisibilityForcesDiskHash(t *testing.T) {
	tmp := t.TempDir()
	wsRoot := tmp
	rel := filepath.Join("pkg", "file.txt")
	abs := filepath.Join(wsRoot, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		require.NoError(t, err, "mkdir: %v", err)
	}
	if err := os.WriteFile(abs, []byte("X"), 0o644); err != nil {
		require.NoError(t, err, "write: %v", err)
	}

	h := &diskHashHelper{
		workspaceroot:   wsRoot,
		knownFileHashes: map[string][]byte{filepath.ToSlash(rel): []byte("KNOWN")},
		fullHashRepos:   map[string]struct{}{"": {}},
		excludedFiles:   map[string]struct{}{},
	}
	sf := &buildpb.SourceFile{
		Name:            strPtr("//" + filepath.ToSlash(filepath.Dir(rel)) + ":" + filepath.Base(rel)),
		Location:        strPtr(abs + ":1:1"),
		VisibilityLabel: []string{"//visibility:public"}, // non-default
	}
	got, err := h.HashSourceFile(sf, nil)
	assert.NoError(t, err, "unexpected err: %v", err)
	assert.NotEqual(t, string(got), "KNOWN", "expected disk hash, but got known hash")
	assert.NotEqual(t, []byte{}, got, "expected non-empty disk hash")
}

func TestDiskHashHelper_ExternalRepo_UsesExternalRuleHash(t *testing.T) {
	// Label in an external repo; not in fullHashRepos → use external rule hash
	h := &diskHashHelper{
		workspaceroot:   "/ws",
		knownFileHashes: map[string][]byte{},
		fullHashRepos:   map[string]struct{}{}, // empty: not using full hashing for any repo
		excludedFiles:   map[string]struct{}{},
	}
	sf := &buildpb.SourceFile{
		Name:            strPtr("@ext//pkg:lib.go"),
		Location:        strPtr("/ws/pkg/lib.go:1:1"),
		VisibilityLabel: []string{"//visibility:private"},
	}
	extKey := externalTargetForRule(sf.GetName())
	external := map[string]Target{
		extKey: {Hash: []byte("EXT-HASH")},
	}
	got, err := h.HashSourceFile(sf, external)
	assert.NoError(t, err, "unexpected err: %v", err)
	assert.Equal(t, "EXT-HASH", string(got), "expected EXT-HASH, got %q", string(got))
}

func TestDiskHashHelper_ExternalRepo_MissingRuleErrors(t *testing.T) {
	h := &diskHashHelper{
		workspaceroot:   "/ws",
		knownFileHashes: map[string][]byte{},
		fullHashRepos:   map[string]struct{}{}, // empty: not using full hashing for any repo
		excludedFiles:   map[string]struct{}{},
	}
	sf := &buildpb.SourceFile{
		Name:            strPtr("@ext//pkg:lib.go"),
		Location:        strPtr("/ws/pkg/lib.go:1:1"),
		VisibilityLabel: []string{"//visibility:private"},
	}
	_, err := h.HashSourceFile(sf, map[string]Target{})
	assert.Error(t, err, "expected error about missing //external rule, got %v", err)
	assert.ErrorContains(t, err, "expected '//external:...' rule")
}

func strPtr(s string) *string { return &s }
