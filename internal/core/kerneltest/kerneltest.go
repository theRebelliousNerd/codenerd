// Package kerneltest hands tests an evaluated core.RealKernel without paying the
// constitution's first fixpoint for every test.
//
// A fresh kernel parses and stratifies the whole constitution and evaluates it
// once; a test that only needs "a booted kernel" repeated that work hundreds of
// times per package run. The constitution does not depend on the test, so one
// evaluated template is built per process and every caller gets a Clone: an
// independent kernel that shares the template's immutable program and reads
// through to its derived store (RealKernel.Clone), so a test's own facts never
// reach the template or another test.
//
// Only a workspace with no user Mangle extensions can be served from the
// template. A workspace that carries any of the files the boot reads from it
// (see userExtensionFiles) gets a real boot, so the kernel a test receives is
// always the kernel core.NewRealKernelWithWorkspace would have built.
package kerneltest

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"codenerd/internal/core"
)

// userExtensionFiles are the workspace files loadMangleFiles reads on top of
// the embedded constitution, relative to the workspace's .nerd directory.
var userExtensionFiles = []string{
	filepath.Join("mangle", "extensions.mg"),
	filepath.Join("mangle", "policy_overrides.mg"),
	filepath.Join("mangle", "learned.mg"),
	"northstar.mg",
	filepath.Join("orientation", "orientation.mg"),
}

var (
	templateOnce sync.Once
	template     *core.RealKernel
	templateErr  error
)

// base is the process's evaluated, never-mutated template kernel.
func base() (*core.RealKernel, error) {
	templateOnce.Do(func() {
		template, templateErr = core.NewRealKernelWithWorkspace("")
	})
	return template, templateErr
}

// New returns a kernel equivalent to core.NewRealKernel().
func New(tb testing.TB) *core.RealKernel {
	tb.Helper()
	k, err := base()
	if err != nil {
		tb.Fatalf("kerneltest: boot the template kernel: %v", err)
	}
	return k.Clone()
}

// WithWorkspace returns a kernel equivalent to
// core.NewRealKernelWithWorkspace(root).
func WithWorkspace(tb testing.TB, root string) *core.RealKernel {
	tb.Helper()
	root = strings.TrimSpace(root)
	if root == "" {
		return New(tb)
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = root
	}
	if hasUserExtensions(abs) {
		k, err := core.NewRealKernelWithWorkspace(abs)
		if err != nil {
			tb.Fatalf("kerneltest: boot a kernel for %s: %v", abs, err)
		}
		return k
	}
	k := New(tb)
	k.SetWorkspace(abs)
	return k
}

func hasUserExtensions(workspace string) bool {
	for _, rel := range userExtensionFiles {
		if fileExists(filepath.Join(workspace, ".nerd", rel)) {
			return true
		}
	}
	return false
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
