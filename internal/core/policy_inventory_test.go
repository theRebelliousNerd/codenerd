package core

import (
	"slices"
	"testing"
)

func TestDefaultAgentPolicySetsResolveToEmbeddedPolicyInventory(t *testing.T) {
	ids := DefaultAgentPolicySetIDs()
	if len(ids) == 0 {
		t.Fatal("DefaultAgentPolicySetIDs() returned no policy sets")
	}
	for _, id := range ids {
		files, ok := DefaultAgentPolicySetFiles(id)
		if !ok || len(files) == 0 {
			t.Fatalf("DefaultAgentPolicySetFiles(%q) = %v, %v; want non-empty set", id, files, ok)
		}
		for _, file := range files {
			if !IsDefaultPolicyFile(file) {
				t.Errorf("policy set %q references non-canonical or missing file %q", id, file)
			}
			if _, err := GetDefaultContent(file); err != nil {
				t.Errorf("policy set %q file %q is not embedded: %v", id, file, err)
			}
		}
	}
}

func TestDefaultPolicyFilesMatchKernelRootModuleInventory(t *testing.T) {
	files := DefaultPolicyFiles()
	for _, module := range DefaultCorePolicyModules() {
		if !slices.Contains(files, module) {
			t.Errorf("DefaultPolicyFiles() missing root module %q", module)
		}
	}

	for _, invalid := range []string{"base.mg", "coder.mg", "researcher.mg", "../policy/constitution.mg", `policy\\constitution.mg`, " policy/constitution.mg"} {
		if IsDefaultPolicyFile(invalid) {
			t.Errorf("IsDefaultPolicyFile(%q) = true; want false", invalid)
		}
	}
}
