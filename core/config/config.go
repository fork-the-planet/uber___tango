package config

import (
	yaml "github.com/goccy/go-yaml"
	"os"
)

type RepositoryConfig struct {
	Remote                 string   `yaml:"remote"`
	DefaultBranch          string   `yaml:"default_branch"`
	FullHashRepos          []string `yaml:"full_hash_repos"`
	ExcludedFiles          []string `yaml:"excluded_files"`
	ExcludeExternalTargets bool     `yaml:"exclude_external_targets"`
	BzlmodEnabled          bool     `yaml:"bzlmod_enabled"`
	BazelCommand           string   `yaml:"bazel_command"`
	QueryTimeout           int64    `yaml:"query_timeout"` // in seconds
	GitTimeout             int64    `yaml:"git_timeout"`   // in seconds
}

func ParseConfig(configFilePath string) (*RepositoryConfig, error) {
	yamlBytes, err := os.ReadFile(configFilePath)
	if err != nil {
		return nil, err
	}
	var config RepositoryConfig
	err = yaml.Unmarshal(yamlBytes, &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}
