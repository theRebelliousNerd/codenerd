package core

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/types"
)

// devTree copies the embedded constitution into a temp source tree's
// internal/core and returns that directory.
func devTree(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "internal", "core")
	err := fs.WalkDir(coreLogic, "defaults", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		target := filepath.Join(root, filepath.FromSlash(p))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := coreLogic.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return root
}

const reloadProbe = `
Decl dev_reload_probe(Target) bound [/string].
dev_reload_probe(Target) :- user_intent(/current_intent, _, _, Target, _).
`

func appendPolicy(t *testing.T, root, text string) {
	t.Helper()
	path := filepath.Join(root, "defaults", "policy", "dev_reload_probe.mg")
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func probeTargets(t *testing.T, k *RealKernel) []string {
	t.Helper()
	rows, err := k.Query("dev_reload_probe")
	if err != nil {
		return nil
	}
	var out []string
	for _, r := range rows {
		out = append(out, types.ExtractString(r.Args[0]))
	}
	return out
}

// In dev mode the constitution is read from the source tree, and an edit is
// swapped into the running kernel only after the whole program compiles:
// a valid change applies with the kernel's facts kept and its derivations
// recomputed, an invalid one is reported and the old program keeps running.
func TestReloadConstitution_AppliesAValidEditAndKeepsTheProgramOnAnInvalidOne(t *testing.T) {
	root := devTree(t)
	if err := UseDevConstitution(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(UseEmbeddedConstitution)
	k, err := NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	if err := k.Assert(Fact{Predicate: "user_intent", Args: []any{
		types.MangleAtom("/current_intent"), types.MangleAtom("/mutation"), types.MangleAtom("/fix"), "internal/retry/policy.go", ""}}); err != nil {
		t.Fatal(err)
	}
	if got := probeTargets(t, k); len(got) != 0 {
		t.Fatalf("fixture: the probe derives before it exists: %v", got)
	}

	appendPolicy(t, root, reloadProbe)
	if _, err := k.ReloadConstitution(); err != nil {
		t.Fatalf("a valid edit did not reload: %v", err)
	}
	if got := probeTargets(t, k); len(got) != 1 || got[0] != "internal/retry/policy.go" {
		t.Fatalf("after the reload the probe derives %v; want the kept intent's target", got)
	}

	appendPolicy(t, root, reloadProbe+"\nthis is not mangle(\n")
	_, err = k.ReloadConstitution()
	if err == nil || !strings.Contains(err.Error(), "does not compile") {
		t.Fatalf("an invalid edit reloaded: %v", err)
	}
	if got := probeTargets(t, k); len(got) != 1 {
		t.Fatalf("the invalid edit disturbed the running program: %v", got)
	}

	// A stratification error is refused the same way.
	appendPolicy(t, root, reloadProbe+"\ndev_reload_probe(X) :- user_intent(/current_intent, _, _, X, _), !dev_reload_probe(X).\n")
	if _, err := k.ReloadConstitution(); err == nil {
		t.Fatal("an unstratifiable edit reloaded")
	}
	if got := probeTargets(t, k); len(got) != 1 {
		t.Fatalf("the unstratifiable edit disturbed the running program: %v", got)
	}
}

func TestUseDevConstitution_RefusesATreeWithoutTheCorpus(t *testing.T) {
	if err := UseDevConstitution(t.TempDir()); err == nil {
		t.Fatal("a directory without the corpus was accepted")
	}
	if DevConstitutionDir() != "" {
		t.Fatal("a refused tree became the constitution's source")
	}
}
