package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/config"
)

// Limits cleanup 2026-09-29: executeReadFile built observation.ReadLimits{}
// directly, so the observation.* keys never reached this read path and the
// codec's own defaults always won. The read resolves the installed policy
// now, the way VirtualStore.handleReadFile does.
//
// Not parallel: the installed policy is process-wide, and a parallel sibling
// reading a file mid-test would observe the tiny outline installed here.
func TestReadFile_ShouldHonorInstalledObservationLimits(t *testing.T) {
	t.Cleanup(func() { config.SetObservationLimits(config.DefaultObservationConfig().Resolve()) })
	limits := config.DefaultObservationConfig().Resolve()
	limits.MaxOutline = 2
	config.SetObservationLimits(limits)

	root := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	name := fmt.Sprintf("outline_%d.go", readFixtureSeq.Add(1))
	var src strings.Builder
	src.WriteString("package a\n")
	for i := 0; i < 200; i++ {
		fmt.Fprintf(&src, "\nfunc fn%d() int {\n\treturn %d\n}\n", i, i)
	}
	if err := os.WriteFile(filepath.Join(root, name), []byte(src.String()), 0o600); err != nil {
		t.Fatalf("seed long file: %v", err)
	}

	out, err := executeReadFile(wsCtx(root), map[string]any{"path": name})
	if err != nil {
		t.Fatalf("read_file: %v", err)
	}
	// Outline rows render as "  function fn12 61-64"; the numbered region
	// never contains that shape, so every hit is one listed element.
	if got := strings.Count(out, "\n  function fn"); got != 2 {
		t.Fatalf("outline listed %d elements, want the installed max_outline of 2:\n%s", got, firstLines(out, 6))
	}
	if !strings.Contains(out, "more element(s) not listed") {
		t.Fatalf("a capped outline must name its remainder:\n%s", firstLines(out, 6))
	}
}
