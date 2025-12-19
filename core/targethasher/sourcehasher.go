package targethasher

import (
	"crypto/sha1"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	tangopb "github.com/uber/tango/tangopb"
	buildpb "github.com/bazelbuild/buildtools/build_proto"
	"github.com/bazelbuild/buildtools/labels"
)

var newHash = sha1.New

const (
	// With bazel query, repository rules defined in the WORKSPACE file
	// are queryable with this prefix. See:
	// https://docs.bazel.build/versions/master/query.html#external-repos
	externalWorkspaceRulePrefix  = "//external:"
	externalWorkspaceFilePrefix  = "@"
	_defaultSourceFileVisibility = "//visibility:private"
)

// SourceHasher provides hashes for source nodes in the target graph. These
// can be calculated based on disk contents or form other sources such as a
// vcs system.
type SourceHasher interface {
	HashSourceFile(s *buildpb.SourceFile, externalHashes map[string]Target) ([]byte, error)
}

// diskHashHelper is a SourceHasher that provides hashes based on disk
// contents for targets inside the main bazel workspace, and a hash of the
// associated repository rule from the WORKSPACE file for targets from
// external workspaces. knownFileHashes (e.g. from a vcs system) can be
// used in place of generating a hash from disk.
type diskHashHelper struct {
	workspaceroot   string
	knownFileHashes map[string][]byte
	fullHashRepos   map[string]struct{}
	excludedFiles   map[string]struct{}
	bzlmodEnabled   bool
}

// noOpHasher
type noOpHasher struct {
}

// Params contains the parameters for creating a new SourceHasher.
type Params struct {
	WorkspaceRoot string
	HashConfig    HashConfig
	BzlmodEnabled bool
}

// HashConfig fine tunes behaviors during target hashing.
type HashConfig struct {
	// KnownSourceHashes can be used in-place of generating a hash from disk for a given source file. The values typically
	// can be produced by a VCS (e.g. `git ls-tree`).
	KnownSourceHashes map[string][]byte
	// FullHashRepos is the list of repositories that requires hashing each individual files. By default, the hash
	// for a repository rule is used as the hash for all files in the repository. Hashing individual files is more
	// accurate, but slower.
	FullHashRepos []string
	ExcludedFiles []string
}

// NewSourceHasher creates a new SourceHasher.
func NewSourceHasher(p Params) SourceHasher {
	fullHashRepos := make(map[string]struct{})
	for _, repo := range p.HashConfig.FullHashRepos {
		fullHashRepos[repo] = struct{}{}
	}
	excludedFiles := make(map[string]struct{})
	for _, file := range p.HashConfig.ExcludedFiles {
		excludedFiles[file] = struct{}{}
	}
	return &diskHashHelper{
		workspaceroot:   p.WorkspaceRoot,
		knownFileHashes: p.HashConfig.KnownSourceHashes,
		fullHashRepos:   fullHashRepos,
		excludedFiles:   excludedFiles,
		bzlmodEnabled:   p.BzlmodEnabled,
	}
}

// HashSourceFile does a no-op hash for the noOpHasher.
func (hh *noOpHasher) HashSourceFile(sourceFile *buildpb.SourceFile, externalHashes map[string]Target) ([]byte, error) {
	return nil, nil
}

func (hh *diskHashHelper) HashSourceFile(sourceFile *buildpb.SourceFile, externalHashes map[string]Target) ([]byte, error) {
	targetName := sourceFile.GetName()

	// If file is excluded, return empty hash
	for expr := range hh.excludedFiles {
		match, err := regexp.MatchString(expr, targetName)
		if err != nil {
			return nil, fmt.Errorf("Failed to check if excluded: %q", targetName)
		}
		if match {
			return []byte{}, nil
		}
	}

	label := labels.Parse(targetName)
	if _, ok := hh.fullHashRepos[label.Repository]; !ok {
		// Use the hash for repository rules instead of hashing files one by one.
		if h, ok := externalHashes[externalTargetForRule(targetName)]; ok {
			return []byte(h.Hash), nil
		}
		if !hh.bzlmodEnabled {
			return nil, fmt.Errorf("expected '//external:...' rule for: %q", targetName)
		}
	}
	nonDefaultVisibilities := filterVisibilityLabels(sourceFile.GetVisibilityLabel())
	// The location may look like /foo/decl.go:1:1
	location, _, _ := strings.Cut(sourceFile.GetLocation(), ":")
	// check knownFileHashes for a match, fallback to generating hashes from disk if there is no match
	// or if the file has a non-default visibility set
	if h, ok := hh.knownFileHashes[strings.TrimPrefix(location, filepath.Clean(hh.workspaceroot)+string(filepath.Separator))]; ok && len(nonDefaultVisibilities) == 0 {
		return h, nil
	}

	fi, err := os.Stat(location)
	if os.IsNotExist(err) {
		// Not considered a fatal error, e.g. a file is added to a dependency graph which is not
		// present on the file system in certain scenarios, like flaky test filter. This is
		// potentially dangerous as it can silently render an empty hash target graph.
		return []byte{}, nil
	} else if err != nil {
		return nil, err
	}

	var hash hash.Hash
	if fi.IsDir() {
		hash, err = hashDir(location)
	} else {
		hash, err = hashFile(location)
	}
	if err != nil {
		return nil, err
	}

	for _, v := range nonDefaultVisibilities {
		hash.Write([]byte(v))
	}

	return hash.Sum(nil), nil
}

func hashFile(path string) (hash.Hash, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}

	hash := newHash()
	// Using same SHA1 hashing algorithm as git to ensure file hashes
	// are always the same: https://alblue.bandlem.com/2011/08/git-tip-of-week-objects.html
	hash.Write([]byte(fmt.Sprintf("blob %d\000", fi.Size())))
	if _, err := io.Copy(hash, f); err != nil {
		return nil, err
	}
	return hash, nil
}

func hashDir(root string) (hash.Hash, error) {
	dirHash := newHash()
	walkDirFunc := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.Type().IsRegular() {
			fileHash, err := hashFile(path)
			if err != nil {
				return err
			}
			dirHash.Write(fileHash.Sum(nil))
		}
		return nil
	}

	err := filepath.WalkDir(root, walkDirFunc)
	return dirHash, err
}

func filterVisibilityLabels(labels []string) (res []string) {
	for _, v := range labels {
		if v != _defaultSourceFileVisibility {
			res = append(res, v)
		}
	}
	return
}

func externalTargetForRule(t string) string {
	// @workspace//path:target -> //external:workspace
	return externalWorkspaceRulePrefix + strings.TrimPrefix(strings.Split(t, "//")[0], "@")
}

func pathForTarget(root, target string) string {
	// //path/to:target.go -> $root/path/to/target.go
	parts := strings.SplitN(strings.TrimPrefix(target, "//"), ":", 2)
	return filepath.Join(root, parts[0], parts[1])
}

func hashRule(f *buildpb.Rule, targetHashes map[string]tangopb.OptimizedTarget) ([]byte, error) {
	h := newHash()
	HashRuleCommon(f, h)
	for _, dep := range f.GetRuleInput() {
		if dephash, ok := targetHashes[dep]; ok {
			h.Write([]byte(dephash.Hash))
		} else {
			return nil, fmt.Errorf("%q missing hash for dependency %q", f.GetName(), dep)
		}
	}
	return h.Sum(nil), nil
}

// HashRuleCommon hashes the common elements of a buildpb.Rule.
func HashRuleCommon(r *buildpb.Rule, h hash.Hash) {
	// Name                        *string
	io.WriteString(h, r.GetName())
	// RuleClass                   *string
	io.WriteString(h, r.GetRuleClass())
	// Location                    *string
	// don't hash location, as it machine local paths
	// Attribute                   []*Attribute
	// Before hashing, sort to guarantee consistency
	attributes := r.GetAttribute()
	sort.Slice(attributes, func(i, j int) bool {
		return attributes[i].GetName() < attributes[j].GetName()
	})
	for _, attr := range attributes {
		hashAttributes(h, attr)
	}
	// RuleInput                   []string
	// Before hashing, sort to guarantee consistency
	ruleInputs := r.GetRuleInput()
	sort.Strings(ruleInputs)
	for _, ri := range ruleInputs {
		io.WriteString(h, ri)
	}
	// RuleOutput                  []string
	// Before hashing, sort to guarantee consistency
	ruleOutputs := r.GetRuleOutput()
	sort.Strings(ruleOutputs)
	for _, ro := range ruleOutputs {
		io.WriteString(h, ro)
	}
	// DefaultSetting              []string
	// Before hashing, sort to guarantee consistency
	defaultSettings := r.GetDefaultSetting()
	sort.Strings(defaultSettings)
	for _, d := range defaultSettings {
		io.WriteString(h, d)
	}
	// SkylarkAttributeAspects     []*AttributeAspect
	// don't need to hash this, aspects don't appear in our query.
	// SkylarkEnvironmentHashCode  *string
	io.WriteString(h, r.GetSkylarkEnvironmentHashCode())
}

func hashAttributes(h hash.Hash, a *buildpb.Attribute) {
	// generator_location is present if the rule is generated from a macro, but
	// should not be hashed as the generating macro's location in a build file
	// is irrelevant for our purposes
	if a.GetName() == "generator_location" {
		return
	}
	// Name                           *string
	io.WriteString(h, a.GetName())
	// DEPRECATEDParseableLocation    *Location
	// note: Location contains machine specific elements, don't hash
	// ExplicitlySpecified            *bool
	if a.GetExplicitlySpecified() {
		h.Write([]byte{1})
	}
	// Nodep                          *bool
	if a.GetNodep() {
		h.Write([]byte{1})
	}
	// Type                           *Attribute_Discriminator
	io.WriteString(h, a.GetType().String())
	// IntValue                       *int32
	if a.IntValue != nil {
		io.WriteString(h, strconv.Itoa(int(a.GetIntValue())))
	}
	// StringValue                    *string
	io.WriteString(h, a.GetStringValue())
	// BooleanValue                   *bool
	if a.GetBooleanValue() {
		h.Write([]byte{1})
	}
	// TristateValue                  *Attribute_Tristate
	if a.TristateValue != nil {
		io.WriteString(h, a.GetTristateValue().String())
	}
	// StringListValue                []string
	// Before hashing, sort to guarantee consistency
	stringListValue := a.GetStringListValue()
	sort.Strings(stringListValue)
	for _, s := range stringListValue {
		io.WriteString(h, s)
	}
	// License                        *License
	// StringDictValue                []*StringDictEntry
	// Before hashing, sort to guarantee consistency
	stringDictValue := a.GetStringDictValue()
	sort.Slice(stringDictValue, func(i, j int) bool {
		return stringDictValue[i].GetKey() < stringDictValue[j].GetKey()
	})
	for _, d := range stringDictValue {
		io.WriteString(h, d.GetKey())
		io.WriteString(h, d.GetValue())
	}
	// FilesetListValue               []*FilesetEntry
	// Before hashing, sort to guarantee consistency
	filesetListValue := a.GetFilesetListValue()
	sort.Slice(filesetListValue, func(i, j int) bool {
		return filesetListValue[i].GetSource() < filesetListValue[j].GetSource()
	})
	for _, f := range filesetListValue {
		// Source               *string
		io.WriteString(h, f.GetSource())
		// DestinationDirectory *string
		io.WriteString(h, f.GetDestinationDirectory())
		// FilesPresent         *bool
		if f.GetFilesPresent() {
			h.Write([]byte{1})
		}
		// File                 []string
		// Before hashing, sort to guarantee consistency
		files := f.GetFile()
		sort.Strings(files)
		for _, file := range files {
			io.WriteString(h, file)
		}
		// Exclude              []string
		// Before hashing, sort to guarantee consistency
		excludedFiles := f.GetExclude()
		sort.Strings(excludedFiles)
		for _, file := range excludedFiles {
			io.WriteString(h, file)
		}
		// SymlinkBehavior      *FilesetEntry_SymlinkBehavior
		io.WriteString(h, f.GetSymlinkBehavior().String())
		// StripPrefix          *string
		io.WriteString(h, f.GetStripPrefix())
	}
	// LabelListDictValue             []*LabelListDictEntry
	// Before hashing, sort to guarantee consistency
	labelListDictValue := a.GetLabelListDictValue()
	sort.Slice(labelListDictValue, func(i, j int) bool {
		return labelListDictValue[i].GetKey() < labelListDictValue[j].GetKey()
	})
	for _, ll := range labelListDictValue {
		io.WriteString(h, ll.GetKey())
		// Before hashing, sort to guarantee consistency
		llv := ll.GetValue()
		sort.Strings(llv)
		for _, v := range llv {
			io.WriteString(h, v)
		}
	}
	// StringListDictValue            []*StringListDictEntry
	// Before hashing, sort to guarantee consistency
	stringListDictValue := a.GetStringListDictValue()
	sort.Slice(stringListDictValue, func(i, j int) bool {
		return stringListDictValue[i].GetKey() < stringListDictValue[j].GetKey()
	})
	for _, sl := range stringListDictValue {
		io.WriteString(h, sl.GetKey())
		// Before hashing, sort to guarantee consistency
		slv := sl.GetValue()
		sort.Strings(slv)
		for _, v := range slv {
			io.WriteString(h, v)
		}
	}
	// IntListValue                   []int32
	// Before hashing, sort to guarantee consistency
	intListValue := a.GetIntListValue()
	sort.Slice(intListValue, func(i, j int) bool {
		return intListValue[i] < intListValue[j]
	})
	for _, i := range intListValue {
		io.WriteString(h, strconv.Itoa(int(i)))
	}
	// LabelDictUnaryValue            []*LabelDictUnaryEntry
	// Before hashing, sort to guarantee consistency
	labelDictUnaryValue := a.GetLabelDictUnaryValue()
	sort.Slice(labelDictUnaryValue, func(i, j int) bool {
		return labelDictUnaryValue[i].GetKey() < labelDictUnaryValue[j].GetKey()
	})
	for _, d := range labelDictUnaryValue {
		io.WriteString(h, d.GetKey())
		io.WriteString(h, d.GetValue())
	}
	// LabelKeyedStringDictValue      []*LabelKeyedStringDictEntry
	// Before hashing, sort to guarantee consistency
	labelKeyedStringDictValue := a.GetLabelKeyedStringDictValue()
	sort.Slice(labelKeyedStringDictValue, func(i, j int) bool {
		return labelKeyedStringDictValue[i].GetKey() < labelKeyedStringDictValue[j].GetKey()
	})
	for _, d := range labelKeyedStringDictValue {
		io.WriteString(h, d.GetKey())
		io.WriteString(h, d.GetValue())
	}
	// SelectorList                   *Attribute_SelectorList
	// pass on this for now
}
