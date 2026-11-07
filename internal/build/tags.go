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
	return strings.Contains(modulePath(workspace), "codenerd")
}

// modulePath is the module path go.mod declares in dir, "" when there is no
// go.mod or it declares none.
func modulePath(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return ""
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if mod, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			return strings.Trim(strings.TrimSpace(mod), `"`)
		}
	}
	return ""
}

// SelfBinaryPackage is the package whose binary is codeNERD itself.
const SelfBinaryPackage = "./cmd/nerd"

// IsCodeNERDModule reports whether workspace is the codeNERD module itself,
// the tree the running `nerd` binary is built from: go.mod's module path is
// codenerd (as a whole path or its last element) and the tree holds the
// SelfBinaryPackage main package. go.mod decides, not a nerd.md declaration:
// nerd.md is the workspace's own text, and a workspace must not be able to
// name an arbitrary build for a run to exec.
func IsCodeNERDModule(workspace string) bool {
	if workspace == "" {
		return false
	}
	mod := modulePath(workspace)
	if mod != "codenerd" && !strings.HasSuffix(mod, "/codenerd") {
		return false
	}
	st, err := os.Stat(filepath.Join(workspace, filepath.FromSlash(SelfBinaryPackage), "main.go"))
	return err == nil && !st.IsDir()
}
