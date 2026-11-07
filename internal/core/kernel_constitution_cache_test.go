package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"unsafe"
)

func TestAssembleConstitution_CachedErrorIsPropagated(t *testing.T) {
	UseEmbeddedConstitution()
	if _, _, _, err := assembleConstitution(constitutionFS()); err != nil {
		t.Fatalf("embedded constitution should boot cleanly, got: %v", err)
	}
	oldErr := embeddedConstitutionErr
	injected := errors.New("injected constitution failure for coverage")
	embeddedConstitutionErr = injected
	t.Cleanup(func() { embeddedConstitutionErr = oldErr })

	schemas, policy, mods, err := assembleConstitution(constitutionFS())
	if !errors.Is(err, injected) {
		t.Fatalf("expected injected cached error, got schemas=%d policy=%d err=%v", len(schemas), len(policy), err)
	}
	if schemas != "" || policy != "" || mods != nil {
		t.Fatalf("cached error must return empty constitution, got schemas=%d policy=%d mods=%v", len(schemas), len(policy), mods)
	}

	if _, _, _, err := assembleConstitutionUncached(fstest.MapFS{}); err == nil {
		t.Fatalf("uncached assembly over an empty FS must fail (missing defaults/schemas.mg)")
	}
}

func TestLoadMangleFiles_LearnedOnly_PreservesConstitution(t *testing.T) {
	UseEmbeddedConstitution()
	dir := t.TempDir()
	mangleDir := filepath.Join(dir, ".nerd", "mangle")
	if err := os.MkdirAll(mangleDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(mangleDir, "learned.mg"), []byte("# empty learned for coverage\n"), 0o644); err != nil {
		t.Fatalf("write learned.mg: %v", err)
	}

	k, err := NewRealKernelWithWorkspace(dir)
	if err != nil {
		t.Fatalf("boot with learned-only workspace failed: %v", err)
	}
	if k.constSchemas == "" || k.constPolicy == "" {
		t.Fatalf("constitution must be loaded, got schemas=%d policy=%d", len(k.constSchemas), len(k.constPolicy))
	}
	if k.schemas != k.constSchemas {
		t.Fatalf("schemas-only extension absent: k.schemas must equal constSchemas, got %d vs %d", len(k.schemas), len(k.constSchemas))
	}
	if k.policy != k.constPolicy {
		t.Fatalf("policy override absent: k.policy must equal constPolicy, got %d vs %d", len(k.policy), len(k.constPolicy))
	}
}

func TestLoadMangleFiles_EmptyWorkspace_AliasesConstitution(t *testing.T) {
	UseEmbeddedConstitution()
	dir := t.TempDir()
	k, err := NewRealKernelWithWorkspace(dir)
	if err != nil {
		t.Fatalf("empty workspace boot failed: %v", err)
	}
	if k.schemas != k.constSchemas || k.constSchemas == "" {
		t.Fatalf("fast path must alias constitution schemas, got schemas=%d const=%d", len(k.schemas), len(k.constSchemas))
	}
	if k.policy != k.constPolicy || k.constPolicy == "" {
		t.Fatalf("fast path must alias constitution policy, got policy=%d const=%d", len(k.policy), len(k.constPolicy))
	}

	k2, err := NewRealKernelWithWorkspace(dir)
	if err != nil {
		t.Fatalf("second boot failed: %v", err)
	}
	if k2.schemas != k.schemas || k2.policy != k.policy {
		t.Fatalf("cached constitution must be identical across boots")
	}
}

func TestLoadMangleFiles_EmptyWorkspace_ZeroCopyAlias(t *testing.T) {
	UseEmbeddedConstitution()
	dir := t.TempDir()
	k, err := NewRealKernelWithWorkspace(dir)
	if err != nil {
		t.Fatalf("empty workspace boot failed: %v", err)
	}
	if len(k.schemas) == 0 || len(k.constSchemas) == 0 {
		t.Fatalf("constitution must be non-empty, got schemas=%d const=%d", len(k.schemas), len(k.constSchemas))
	}
	if unsafe.StringData(k.schemas) != unsafe.StringData(k.constSchemas) {
		t.Fatalf("fast path must alias constitution schemas without copying")
	}
	if unsafe.StringData(k.policy) != unsafe.StringData(k.constPolicy) {
		t.Fatalf("fast path must alias constitution policy without copying")
	}
}

func TestLoadMangleFiles_SchemasOnlyAndPolicyOnly_PreservesOtherConst(t *testing.T) {
	UseEmbeddedConstitution()

	t.Run("schemas-only keeps policy const", func(t *testing.T) {
		dir := t.TempDir()
		mangleDir := filepath.Join(dir, ".nerd", "mangle")
		if err := os.MkdirAll(mangleDir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(mangleDir, "extensions.mg"), []byte("# test extension\n"), 0o644); err != nil {
			t.Fatalf("write extensions.mg: %v", err)
		}
		k, err := NewRealKernelWithWorkspace(dir)
		if err != nil {
			t.Fatalf("boot failed: %v", err)
		}
		if !strings.HasPrefix(k.schemas, k.constSchemas) || len(k.schemas) <= len(k.constSchemas) {
			t.Fatalf("schemas must prepend constitution, got %d vs const %d", len(k.schemas), len(k.constSchemas))
		}
		if k.policy != k.constPolicy {
			t.Fatalf("policy must stay const when no override exists, got %d vs %d", len(k.policy), len(k.constPolicy))
		}
	})

	t.Run("policy-only keeps schemas const", func(t *testing.T) {
		dir := t.TempDir()
		mangleDir := filepath.Join(dir, ".nerd", "mangle")
		if err := os.MkdirAll(mangleDir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(mangleDir, "policy_overrides.mg"), []byte("# test override\n"), 0o644); err != nil {
			t.Fatalf("write policy_overrides.mg: %v", err)
		}
		k, err := NewRealKernelWithWorkspace(dir)
		if err != nil {
			t.Fatalf("boot failed: %v", err)
		}
		if k.schemas != k.constSchemas {
			t.Fatalf("schemas must stay const when no schema extension exists, got %d vs %d", len(k.schemas), len(k.constSchemas))
		}
		if !strings.HasPrefix(k.policy, k.constPolicy) || len(k.policy) <= len(k.constPolicy) {
			t.Fatalf("policy must prepend constitution, got %d vs const %d", len(k.policy), len(k.constPolicy))
		}
	})
}

func TestLoadMangleFiles_LearnedWithExtensions_PrependsConstitution(t *testing.T) {
	UseEmbeddedConstitution()
	dir := t.TempDir()
	mangleDir := filepath.Join(dir, ".nerd", "mangle")
	if err := os.MkdirAll(mangleDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(mangleDir, "extensions.mg"), []byte("# test extension\n"), 0o644); err != nil {
		t.Fatalf("write extensions.mg: %v", err)
	}
	if err := os.WriteFile(filepath.Join(mangleDir, "policy_overrides.mg"), []byte("# test override\n"), 0o644); err != nil {
		t.Fatalf("write policy_overrides.mg: %v", err)
	}
	if err := os.WriteFile(filepath.Join(mangleDir, "learned.mg"), []byte("# test learned\n"), 0o644); err != nil {
		t.Fatalf("write learned.mg: %v", err)
	}
	k, err := NewRealKernelWithWorkspace(dir)
	if err != nil {
		t.Fatalf("boot failed: %v", err)
	}
	if !strings.HasPrefix(k.schemas, k.constSchemas) || len(k.schemas) <= len(k.constSchemas) {
		t.Fatalf("learned finalization must prepend constitution to schemas, got %d vs const %d", len(k.schemas), len(k.constSchemas))
	}
	if !strings.HasPrefix(k.policy, k.constPolicy) || len(k.policy) <= len(k.constPolicy) {
		t.Fatalf("learned finalization must prepend constitution to policy, got %d vs const %d", len(k.policy), len(k.constPolicy))
	}
}
