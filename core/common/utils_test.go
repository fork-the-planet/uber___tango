package common

import (
	"encoding/base64"
	"path/filepath"
	"testing"

	pb "github.com/uber/tango/tangopb"
)

func TestToShortRemote(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		remote string
		want   string
	}{
		{
			name:   "ssh remote with host",
			remote: "git@github:uber/tango",
			want:   "uber/tango",
		},
		{
			name:   "already short",
			remote: "uber/tango",
			want:   "uber/tango",
		},
		{
			name:   "with nested path",
			remote: "git@github:org/project/sub",
			want:   "org/project/sub",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := ToShortRemote(tt.remote)
			if got != tt.want {
				t.Fatalf("ToShortRemote(%q) = %q, want %q", tt.remote, got, tt.want)
			}
		})
	}
}

func TestGetGraphByTreeHash(t *testing.T) {
	t.Parallel()
	remote := "git@github:uber/tango"
	treehash := "abcd1234"
	got := GetGraphByTreeHash(remote, treehash)
	want := filepath.Join("uber/tango", treehash)
	if got != want {
		t.Fatalf("GetGraphByTreeHash(%q,%q) = %q, want %q", remote, treehash, got, want)
	}
}

func TestGetTreehashCachePath(t *testing.T) {
	t.Parallel()
	reqs := []string{"github://org/repo/pull/1", "custom://foo/bar"}
	desc := &pb.BuildDescription{
		Remote:      "git@github:uber/tango",
		BaseSha:     "deadbeef",
		RequestUrls: reqs,
	}
	got := GetTreehashCachePath(desc)
	// Build expected with the same encoding semantics used by the implementation
	encoded := []string{
		base64.RawURLEncoding.EncodeToString([]byte(reqs[0])),
		base64.RawURLEncoding.EncodeToString([]byte(reqs[1])),
	}
	want := filepath.Join("uber/tango", "deadbeef", encoded[0]+"-"+encoded[1])
	if got != want {
		t.Fatalf("GetTreehashCachePath(..) = %q, want %q", got, want)
	}
}

func TestGetReqsBase64(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   []string
		want string
	}{
		{
			name: "empty",
			in:   []string{},
			want: "",
		},
		{
			name: "single",
			in:   []string{"github://org/repo/pull/42"},
			want: base64.RawURLEncoding.EncodeToString([]byte("github://org/repo/pull/42")),
		},
		{
			name: "multiple",
			in:   []string{"a", "b"},
			want: base64.RawURLEncoding.EncodeToString([]byte("a")) + "-" + base64.RawURLEncoding.EncodeToString([]byte("b")),
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := getReqsBase64(tt.in)
			if got != tt.want {
				t.Fatalf("getReqsBase64(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
