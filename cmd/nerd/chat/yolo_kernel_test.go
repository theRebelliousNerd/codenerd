package chat

import (
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/core"
)

// TestYolo_TogglesKernelFact proves /yolo mirrors the switch into the kernel:
// the yolo_mode fact appears when autonomy turns on and is retracted when it
// turns off, so the policy guards consult live state.
func TestYolo_TogglesKernelFact(t *testing.T) {
	t.Parallel()
	m := NewTestModel()
	m.workspace = t.TempDir()
	if err := os.MkdirAll(filepath.Join(m.workspace, ".nerd"), 0o755); err != nil {
		t.Fatal(err)
	}
	m.Config = config.DefaultUserConfig()
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	m.kernel = kernel
	hasFact := func() bool {
		rows, err := kernel.Query("yolo_mode")
		if err != nil {
			t.Fatalf("query yolo_mode: %v", err)
		}
		return len(rows) > 0
	}

	updated, _ := m.handleCmdYolo("/yolo on", []string{"/yolo", "on"})
	result := updated.(Model)
	if !hasFact() {
		t.Fatal("/yolo on must assert yolo_mode in the kernel")
	}
	updated, _ = result.handleCmdYolo("/yolo off", []string{"/yolo", "off"})
	_ = updated
	if hasFact() {
		t.Fatal("/yolo off must retract yolo_mode from the kernel")
	}
}

// TestYolo_BootRestoresFact proves the boot hook: a persisted yolo switch
// re-asserts yolo_mode once the kernel exists, because the fact is session
// state that does not survive restarts while the config does.
func TestYolo_BootRestoresFact(t *testing.T) {
	t.Parallel()
	m := NewTestModel()
	m.Config = config.DefaultUserConfig()
	m.Config.WorkMode = config.WorkModeYolo
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}

	updated, _ := m.Update(bootCompleteMsg{components: &SystemComponents{Kernel: kernel}})
	_ = updated

	rows, err := kernel.Query("yolo_mode")
	if err != nil {
		t.Fatalf("query yolo_mode: %v", err)
	}
	if len(rows) != 1 {
		t.Fatal("boot with persisted yolo must assert yolo_mode")
	}
}
