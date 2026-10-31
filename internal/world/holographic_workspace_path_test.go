package world

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"codenerd/internal/core"
)

func writeHolographicWorkspaceFixture(t *testing.T, root, name string, tests bool) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod":         "module example.com/" + name + "\n\ngo 1.27\n",
		"pkg/target.go":  "package " + name + "\nimport \"example.org/" + name + "dep\"\n// TODO: " + name + "\nfunc " + name + "Target() {}\n",
		"pkg/sibling.go": "package " + name + "\nfunc " + name + "Sibling() {}\n",
	}
	if tests {
		files["pkg/target_test.go"] = "package " + name + "\n"
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Join(root, "pkg", "target.go")
}

func TestHolographicWorkspacePath_RelativeTargetUsesWorkspaceWithDecoyCWD(t *testing.T) {
	for _, hasTests := range []bool{true, false} {
		name := "workspace_tests"
		if !hasTests {
			name = "decoy_tests"
		}
		t.Run(name, func(t *testing.T) {
			base := t.TempDir()
			root, decoy := filepath.Join(base, "workspace"), filepath.Join(base, "decoy")
			target := writeHolographicWorkspaceFixture(t, root, "Wanted", hasTests)
			decoyTarget := writeHolographicWorkspaceFixture(t, decoy, "Decoy", !hasTests)
			q := &stubQuerier{facts: map[string][]core.Fact{
				"code_defines": {
					{Predicate: "code_defines", Args: []any{"pkg/target.go", "Wanted.WantedTarget", "/function", int64(4), int64(4)}},
					{Predicate: "code_defines", Args: []any{decoyTarget, "Decoy.DecoyTarget", "/function", int64(4), int64(4)}},
				},
				"code_calls": {
					{Predicate: "code_calls", Args: []any{"caller.WantedCaller", "Wanted.WantedTarget"}},
					{Predicate: "code_calls", Args: []any{"caller.DecoyCaller", "Decoy.DecoyTarget"}},
				},
				"dependency_link": {
					{Predicate: "dependency_link", Args: []any{"app/main.go", "pkg:example.com/Wanted/pkg", "example.com/Wanted/pkg"}},
					{Predicate: "dependency_link", Args: []any{filepath.Join(root, "pkg/sibling.go"), "pkg:example.com/Wanted/pkg", "example.com/Wanted/pkg"}},
				},
			}}
			h := NewHolographicProvider(q, root)
			t.Chdir(root)
			normal, err := h.GetContext(target)
			if err != nil || normal.TargetPkg != "Wanted" || normal.HasTests != hasTests {
				t.Fatalf("ordinary absolute workspace control: context=%+v error=%v", normal, err)
			}
			t.Chdir(decoy)
			relative, err := h.GetContext("pkg/target.go")
			if err != nil {
				t.Fatalf("relative workspace target: %v", err)
			}
			if relative.TargetFile != "pkg/target.go" || relative.TargetPkg != "Wanted" || relative.HasTests != hasTests || relative.TODOCount != 1 {
				t.Errorf("relative target mixed workspace and CWD: target=%q package=%q tests=%v TODOs=%d", relative.TargetFile, relative.TargetPkg, relative.HasTests, relative.TODOCount)
			}
			if !reflect.DeepEqual(relative.PackageSiblings, normal.PackageSiblings) || !reflect.DeepEqual(relative.PackageSignatures, normal.PackageSignatures) ||
				!reflect.DeepEqual(relative.DirectImports, normal.DirectImports) || !reflect.DeepEqual(relative.DirectImporters, normal.DirectImporters) || !reflect.DeepEqual(relative.CallGraph, normal.CallGraph) {
				t.Errorf("relative/absolute dimensions diverged: relative siblings=%v imports=%v importers=%v graph=%v; absolute siblings=%v imports=%v importers=%v graph=%v", relative.PackageSiblings, relative.DirectImports, relative.DirectImporters, relative.CallGraph, normal.PackageSiblings, normal.DirectImports, normal.DirectImporters, normal.CallGraph)
			}
			section := h.PromptSection(context.Background(), "pkg/target.go")
			for _, want := range []string{"WantedSibling", "WantedTarget", "caller.WantedCaller", "app/main.go"} {
				if !strings.Contains(section, want) {
					t.Errorf("workspace render omitted %q", want)
				}
			}
			if strings.Contains(section, "Decoy") {
				t.Errorf("decoy source or graph entered the workspace render: %s", section)
			}
			// A different CWD must not change cache identity or the render.
			t.Chdir(base)
			if next := h.PromptSection(context.Background(), "pkg/target.go"); next != section {
				t.Error("an unchanged workspace render changed with process CWD")
			}
		})
	}
}

func TestHolographicWorkspacePath_RelativeRootIsCapturedAtConstruction(t *testing.T) {
	base := t.TempDir()
	root, creator, elsewhere := filepath.Join(base, "workspace"), filepath.Join(base, "creator"), filepath.Join(base, "other", "elsewhere")
	writeHolographicWorkspaceFixture(t, root, "Wanted", true)
	for _, dir := range []string{creator, elsewhere} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(creator)
	h := NewHolographicProvider(nil, "../workspace")
	t.Chdir(elsewhere)
	section := h.PromptSection(context.Background(), "pkg/target.go")
	if !strings.Contains(section, "WantedSibling") || !strings.Contains(section, "WantedTarget") || !strings.Contains(section, "**Tests**: yes") {
		t.Errorf("relative workspace root followed a later CWD instead of construction identity: %s", section)
	}
}

func TestHolographicWorkspacePath_RefusesEscapesAndExcludedTargets(t *testing.T) {
	base := t.TempDir()
	root, outside := filepath.Join(base, "workspace"), filepath.Join(base, "workspace-sibling")
	target := writeHolographicWorkspaceFixture(t, root, "Wanted", false)
	out := writeHolographicWorkspaceFixture(t, outside, "Outside", false)
	if err := os.MkdirAll(filepath.Join(root, ".nerd"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".nerd", "config.json"), []byte(`{"world":{"ignore_patterns":["pkg/excluded.go","pkg/excluded-alias.go"]}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "pkg/excluded.go"), []byte("package Wanted\nfunc ExcludedTargetCanary() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".nerd/private.go"), []byte("package private\nfunc PrivateTargetCanary() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	linkErrors := map[string]error{
		"symlink escape":                  os.Symlink(out, filepath.Join(root, "pkg/escape.go")),
		"excluded alias to admitted file": os.Symlink(target, filepath.Join(root, "pkg/excluded-alias.go")),
	}
	t.Chdir(outside)
	h := NewHolographicProvider(nil, root)
	for name, path := range map[string]string{
		"foreign absolute with shared prefix": out,
		"relative traversal":                  "../workspace-sibling/pkg/target.go",
		"backslash traversal":                 `..\workspace-sibling\pkg\target.go`,
		"foreign drive":                       `C:\outside\target.go`,
		"symlink escape":                      "pkg/escape.go",
		"operator excluded":                   "pkg/excluded.go",
		"excluded alias to admitted file":     "pkg/excluded-alias.go",
		"always excluded internal directory":  ".nerd/private.go",
	} {
		t.Run(name, func(t *testing.T) {
			if err := linkErrors[name]; err != nil {
				if runtime.GOOS == "windows" {
					t.Skipf("symlink fixture unavailable: %v", err)
				}
				t.Fatal(err)
			}
			if _, err := h.GetContext(path); err == nil {
				t.Errorf("unadmitted target %q returned a context", path)
			}
			if section := h.PromptSection(context.Background(), path); section != "" {
				t.Errorf("unadmitted target %q entered rendered context: %s", path, section)
			}
		})
	}
	// An admitted target must not import excluded or escaped sibling content.
	section := h.PromptSection(context.Background(), "pkg/target.go")
	if !strings.Contains(section, "WantedTarget") || strings.Contains(section, "ExcludedTargetCanary") || strings.Contains(section, "OutsideTarget") {
		t.Errorf("admitted target leaked an excluded sibling: %s", section)
	}
}

func TestHolographicWorkspacePath_MembershipChangesInvalidateWarmParse(t *testing.T) {
	base := t.TempDir()
	root, decoy := filepath.Join(base, "workspace"), filepath.Join(base, "decoy")
	writeHolographicWorkspaceFixture(t, root, "Wanted", true)
	writeHolographicWorkspaceFixture(t, decoy, "Decoy", false)
	if err := os.MkdirAll(filepath.Join(root, ".nerd"), 0o755); err != nil {
		t.Fatal(err)
	}
	setPatterns := func(text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, ".nerd/config.json"), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(decoy)
	h := NewHolographicProvider(nil, root)
	setPatterns(`{"world":{"ignore_patterns":[]}}`)
	initial, err := h.GetContext(filepath.Join(root, "pkg/target.go"))
	if err != nil || initial.TargetPkg != "Wanted" || !initial.HasTests {
		t.Fatalf("initial context: %+v, %v", initial, err)
	}
	_, missesBefore := h.CacheStats()
	setPatterns(`{"world":{"ignore_patterns":["pkg/sibling.go","pkg/target_test.go"]}}`)
	filtered, err := h.GetContext(filepath.Join(root, "pkg/target.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, sig := range filtered.PackageSignatures {
		if sig.Name == "WantedSibling" {
			t.Error("warm package parse retained an operator-excluded signature")
		}
	}
	if filtered.HasTests || len(filtered.PackageSiblings) != 0 {
		t.Errorf("excluded members remain in context: tests=%v siblings=%v", filtered.HasTests, filtered.PackageSiblings)
	}
	_, missesAfter := h.CacheStats()
	if missesAfter <= missesBefore {
		t.Error("membership edit did not invalidate the warm filesystem parse")
	}
	setPatterns(`{"world":{"ignore_patterns":[]}}`)
	restored := h.PromptSection(context.Background(), "pkg/target.go")
	if !strings.Contains(restored, "WantedSibling") || !strings.Contains(restored, "**Tests**: yes") {
		t.Errorf("restored membership did not restore admitted context: %s", restored)
	}
}

func TestHolographicWorkspacePath_AliasesRemainContainedAndTestsDoNotEscape(t *testing.T) {
	base := t.TempDir()
	root, outside := filepath.Join(base, "workspace"), filepath.Join(base, "outside")
	writeHolographicWorkspaceFixture(t, root, "Wanted", false)
	writeHolographicWorkspaceFixture(t, outside, "Outside", false)
	alias := filepath.Join(base, "workspace-alias")
	if err := os.Symlink(root, alias); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("symlink fixture unavailable: %v", err)
		}
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "pkg/target.go"), filepath.Join(root, "pkg/target_test.go")); err != nil {
		t.Fatal(err)
	}
	t.Chdir(outside)
	h := NewHolographicProvider(nil, alias)
	for _, path := range []string{"pkg/target.go", filepath.Join(alias, "pkg/target.go"), filepath.Join(root, "pkg/target.go")} {
		hc, err := h.GetContext(path)
		if err != nil || hc.TargetPkg != "Wanted" || hc.HasTests {
			t.Errorf("legitimate root alias or test containment failed for %q: context=%+v error=%v", path, hc, err)
		}
	}
	// Module identity is a filesystem read too; it must not follow a go.mod
	// symlink outside the configured workspace.
	if err := os.Remove(filepath.Join(root, "go.mod")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "go.mod"), filepath.Join(root, "go.mod")); err != nil {
		t.Fatal(err)
	}
	if module := h.moduleName(); module != "" {
		t.Errorf("module identity escaped the workspace: %q", module)
	}
}

func TestHolographicWorkspacePath_NonGoSiblingsAndInvalidRootDoNotUseCWD(t *testing.T) {
	base := t.TempDir()
	root, decoy := filepath.Join(base, "workspace"), filepath.Join(base, "decoy")
	for _, dir := range []string{root, decoy} {
		if err := os.MkdirAll(filepath.Join(dir, "pkg"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for path, content := range map[string]string{
		filepath.Join(root, "pkg/target.py"):    "def WantedTarget():\n    pass\n",
		filepath.Join(root, "pkg/sibling.py"):   "def WantedSibling():\n    pass\n",
		filepath.Join(decoy, "pkg/target.py"):   "def DecoyTarget():\n    pass\n",
		filepath.Join(decoy, "pkg/sibling.py"):  "def DecoySibling():\n    pass\n",
		filepath.Join(decoy, "pkg/cwd-only.py"): "def DecoyOnly():\n    pass\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(decoy)
	h := NewHolographicProvider(nil, root)
	hc, err := h.GetContext("pkg/target.py")
	if err != nil || !reflect.DeepEqual(hc.PackageSiblings, []string{filepath.Join(root, "pkg/sibling.py")}) {
		t.Fatalf("non-Go context used CWD: context=%+v error=%v", hc, err)
	}
	section := h.PromptSection(context.Background(), "pkg/target.py")
	if !strings.Contains(section, "WantedTarget") || strings.Contains(section, "Decoy") {
		t.Errorf("non-Go outline/sibling identity diverged: %s", section)
	}
	invalid := NewHolographicProvider(nil, filepath.Join(base, "missing-root"))
	if _, err := invalid.GetContext("pkg/target.py"); err == nil {
		t.Error("invalid configured workspace fell back to a readable CWD target")
	}
	if got := invalid.PromptSection(context.Background(), "pkg/target.py"); got != "" {
		t.Errorf("invalid configured root produced CWD evidence: %s", got)
	}
}
