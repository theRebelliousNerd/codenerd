package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestKernelAnalysisDumpUsesConfiguredWorkspace(t *testing.T) {
	caller := t.TempDir()
	t.Chdir(caller)
	workspace := t.TempDir()
	kernel, err := NewRealKernelWithWorkspace(workspace)
	if err != nil {
		t.Fatal(err)
	}
	kernel.SetSchemas("")
	kernel.SetLearned("")
	kernel.SetPolicy(`dump_identity_probe(X) :- dump_missing_predicate(X).`)
	if err := kernel.Evaluate(); err == nil {
		t.Fatal("expected undeclared program to fail analysis")
	}
	data, err := os.ReadFile(filepath.Join(workspace, ".nerd", "debug", "debug_program_ERROR.mg"))
	if err != nil || !strings.Contains(string(data), "dump_identity_probe") {
		t.Fatalf("configured workspace missing the actual failed program: %v", err)
	}
	if _, err := os.Stat(filepath.Join(caller, ".nerd", "debug", "debug_program_ERROR.mg")); !os.IsNotExist(err) {
		t.Fatalf("kernel wrote into its caller's workspace: %v", err)
	}
}

func TestNewRealKernelWithWorkspaceInitializesRuntimeDependencies(t *testing.T) {
	workspace := t.TempDir()
	kernel, err := NewRealKernelWithWorkspace(workspace)
	if err != nil {
		t.Fatalf("NewRealKernelWithWorkspace() error = %v", err)
	}

	wantWorkspace, err := filepath.Abs(workspace)
	if err != nil {
		t.Fatalf("filepath.Abs() error = %v", err)
	}
	if got := kernel.GetWorkspace(); got != wantWorkspace {
		t.Fatalf("GetWorkspace() = %q, want %q", got, wantWorkspace)
	}
	if kernel.GetEventBus() == nil {
		t.Fatal("GetEventBus() = nil; workspace constructor must match the default runtime constructor")
	}
}
