package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/config"
)

// The file-read projection follows observation.* through the installed
// policy: the outline of a many-element file is capped by the installed
// MaxOutline, and the rest is counted, not dropped silently.
func TestHandleReadFile_ProjectionFollowsObservationPolicy(t *testing.T) {
	prev := config.ResolvedObservationLimits()
	t.Cleanup(func() { config.SetObservationLimits(prev) })

	vs, tmpDir := createActionsTestVS(t)
	var sb strings.Builder
	sb.WriteString("package widget\n\n")
	for i := 0; i < 20; i++ {
		fmt.Fprintf(&sb, "func Part%02d() int {\n\treturn %d\n}\n\n", i, i)
	}
	fileName := "widget.go"
	if err := os.WriteFile(filepath.Join(tmpDir, fileName), []byte(sb.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	read := func() string {
		res, err := vs.handleReadFile(context.Background(), ActionRequest{ActionID: "r1", Target: fileName})
		if err != nil {
			t.Fatalf("handleReadFile: %v", err)
		}
		if !res.Success {
			t.Fatalf("handleReadFile failed: %s", res.Error)
		}
		return res.Output
	}

	// Absent (defaults): the 83-line file arrives whole with no outline
	// at all, since every element is shown in the region.
	config.SetObservationLimits(config.DefaultObservationConfig().Resolve())
	if out := read(); strings.Contains(out, "not shown") || strings.Contains(out, "not listed") {
		t.Errorf("the default projection elided an 83-line file:\n%s", out)
	}

	// Installed: a 30-line region elides, and MaxOutline 2 names the rest
	// of the outline as omitted rather than dropping it silently.
	config.SetObservationLimits(config.ObservationLimits{
		MaxRegionLines: 30, PadLines: 8, MaxOutline: 2, MaxRegionBytes: 24 << 10,
	})
	out := read()
	if !strings.Contains(out, "line(s) not shown") {
		t.Errorf("MaxRegionLines 30 left no elision marker:\n%s", out)
	}
	if !strings.Contains(out, "more element(s) not listed") {
		t.Errorf("MaxOutline 2 left no omission marker:\n%s", out)
	}
}
