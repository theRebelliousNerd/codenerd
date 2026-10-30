package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A kernel with no workspace root writes its failed-program dump to the
// system temp directory, not under the process's working directory: under
// go test that is the package's source tree, and the dump showed up there as
// a broken .mg file.
func TestFailedProgramDump_RootlessKernelStaysOutOfTheWorkingDirectory(t *testing.T) {
	wd := t.TempDir()
	t.Chdir(wd)
	k := &RealKernel{}
	path, err := k.writeFailedProgramDump("broken(.")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(path) })
	if strings.HasPrefix(path, wd) || strings.HasPrefix(path, ".nerd") {
		t.Fatalf("dump written to %s, inside the working directory", path)
	}
	if _, err := os.Stat(filepath.Join(wd, ".nerd")); err == nil {
		t.Fatal("a .nerd directory was created in the working directory")
	}

	ws := t.TempDir()
	rooted := &RealKernel{workspaceRoot: ws}
	got, err := rooted.writeFailedProgramDump("broken(.")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(ws, ".nerd", "debug", "debug_program_ERROR.mg"); got != want {
		t.Fatalf("a rooted kernel wrote %s, want %s", got, want)
	}
}
