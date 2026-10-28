package system

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"codenerd/internal/codefamily"
	"codenerd/internal/core"
	"codenerd/internal/world"
)

// loadCodeFamilies projects .nerd/code_families.json into the kernel at boot.
// A malformed file is reported and loads nothing: half a contract would derive
// obligations for some members and silently none for the rest.
func loadCodeFamilies(ctx context.Context, workspace string, kernel core.Kernel) error {
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
	// The structure index is built on first use and a large workspace takes a
	// while, so members are checked off the boot path.
	go checkCodeFamilyMembers(ctx, workspace, kernel, families)
	return nil
}

// checkCodeFamilyMembers asserts code_family_member_unresolved for every
// declared fn: member that names no function, or several, in the workspace. A
// failed check asserts nothing and says so: it must not look like a clean one.
func checkCodeFamilyMembers(ctx context.Context, workspace string, kernel core.Kernel, families []codefamily.Family) {
	refs, _, err := world.SharedStructureIndex(workspace).ElementRefs(ctx)
	if err != nil {
		if ctx.Err() == nil {
			fmt.Fprintf(os.Stderr, "Warning: code family members not checked: %v\n", err)
		}
		return
	}
	unresolved := codefamily.Resolve(families, refs)
	if len(unresolved) == 0 {
		return
	}
	if err := kernel.LoadFacts(codefamily.UnresolvedFacts(unresolved)); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: code family members not recorded: %v\n", err)
		return
	}
	for _, u := range unresolved {
		fmt.Fprintf(os.Stderr, "Warning: code family %s: member %s %s\n", u.Family, u.Ref, u.Why)
	}
}
