package build

import (
	"os"
	"path/filepath"
	"strings"
)

// TestTagsForWorkspace returns the extra `go test` flags a workspace needs.
//
// codeNERD's own tree gates sqlite-vec code behind the sqlite_vec build tag,
// so a tagless `go test ./...` silently tests a different build: tag-gated
// files are excluded and store tests exercise the fallback path. Campaign
// test runs and phase gates must test the real build, but the generic `go
// test ./...` default must stay untouched for every other workspace — so this
// detects the codeNERD tree (go.mod module plus the vendored sqlite headers)
// and returns nil anywhere else.
func TestTagsForWorkspace(workspace string) []string {
	if !isCodeNERDWorkspace(workspace) {
		return nil
	}
	return []string{"-tags", "sqlite_vec"}
}

func isCodeNERDWorkspace(workspace string) bool {
	if workspace == "" {
		return false
	}
	if _, err := os.Stat(filepath.Join(workspace, "sqlite_headers", "sqlite3.h")); err != nil {
		return false
	}
	data, err := os.ReadFile(filepath.Join(workspace, "go.mod"))
	if err != nil {
		return false
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if mod, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			return strings.Contains(strings.TrimSpace(mod), "codenerd")
		}
	}
	return false
}
