package common

import (
	"github.com/uber/tango/tangopb"
	"encoding/base64"
	"path/filepath"
	"strings"
)

// ToShortRemote returns the short remote name given a git ssh remote string.
// For example, "git@github:uber/tango" will return "uber/tango".
func ToShortRemote(remote string) string {
	strs := strings.Split(remote, ":")
	return strs[len(strs)-1]
}

// GetGraphByTreeHash returns the cache path for the target graph by treehash.
func GetGraphByTreeHash(remote, treehash string) string {
	return filepath.Join(ToShortRemote(remote), treehash)
}

// GetTreehashCachePath returns the cache path for the treehash.
func GetTreehashCachePath(buildDescription *tangopb.BuildDescription) string {
	return filepath.Join(ToShortRemote(buildDescription.Remote), buildDescription.BaseSha, getReqsBase64(buildDescription.RequestUrls))
}

// getReqsBase64 returns the base64 encoded request URLs.
func getReqsBase64(requestURLs []string) string {
	encodedURLs := make([]string, 0, len(requestURLs))
	for _, url := range requestURLs {
		encoded := base64.RawURLEncoding.EncodeToString([]byte(url))
		encodedURLs = append(encodedURLs, encoded)
	}
	return strings.Join(encodedURLs, "-")
}
