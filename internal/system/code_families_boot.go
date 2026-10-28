package system

import (
	"fmt"
	"os"
	"path/filepath"

	"codenerd/internal/codefamily"
	"codenerd/internal/core"
)

// loadCodeFamilies projects .nerd/code_families.json into the kernel at boot.
// A malformed file is reported and loads nothing: half a contract would derive
// obligations for some members and silently none for the rest.
func loadCodeFamilies(workspace string, kernel core.Kernel) error {
	families, err := codefamily.Load(filepath.Join(workspace, ".nerd"))
	if err != nil {
		return err
	}
	if len(families) == 0 {
		return nil
	}
	facts := codefamily.Facts(families)
	if err := kernel.LoadFacts(facts); err != nil {
		return fmt.Errorf("load code families: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Code families: %d loaded from .nerd/%s\n", len(families), codefamily.FileName)
	return nil
}
