package session

import (
	"bytes"
	"go/format"
	"os"
	"path/filepath"
	"strings"

	"codenerd/internal/atomicfile"
	"codenerd/internal/logging"
	"codenerd/internal/tactile"
)

// formatWrittenGoFiles gofmts the Go files this turn wrote, once, after the
// build gate passed. Formatting at turn end rather than inside the write
// tools means no line number the model is still editing by can shift under
// it. A file that does not parse is left alone (the build gate already
// failed it or it is not compiled); a file already formatted is not
// rewritten. The file keeps its own line endings, because later stages diff
// it against its pre-turn snapshot. It returns the workspace-relative paths
// it rewrote.
func formatWrittenGoFiles(workspace string, writtenPaths []string) []string {
	var formattedPaths []string
	for _, path := range writtenPaths {
		if !strings.HasSuffix(strings.ToLower(path), ".go") {
			continue
		}
		var abs string
		if filepath.IsAbs(path) {
			abs = path
		} else {
			abs = filepath.Join(workspace, filepath.FromSlash(NormalizeCoverPath(path)))
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			continue
		}
		ending := tactile.DetectLineEnding(data)
		formatted, err := format.Source([]byte(tactile.NormalizeLineEnding(string(data), "\n")))
		if err != nil {
			continue
		}
		out := []byte(tactile.NormalizeLineEnding(string(formatted), ending))
		if bytes.Equal(data, out) {
			continue
		}
		if err := atomicfile.WriteFilePreservingMode(abs, out, 0o644); err != nil {
			logging.Get(logging.CategorySession).Warn("gofmt: skipping %s: %v", path, err)
			continue
		}
		formattedPaths = append(formattedPaths, path)
	}
	return formattedPaths
}
