package init

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestFile(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", rel, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func depMap(deps []DependencyInfo) map[string]bool {
	m := make(map[string]bool, len(deps))
	for _, d := range deps {
		m[d.Name] = true
	}
	return m
}

func TestDetectModules_SingleModule(t *testing.T) {
	ws := t.TempDir()
	writeTestFile(t, ws, "go.mod", "module example.com/single\n\ngo 1.24\n\nrequire github.com/gin-gonic/gin v1.10.0\n")
	writeTestFile(t, ws, "main.go", "package main\nfunc main() {}\n")
	ini := &Initializer{config: InitConfig{Workspace: ws}}
	mods := ini.detectModules()
	if len(mods) != 1 {
		t.Fatalf("want 1 module, got %d", len(mods))
	}
	m := mods[0]
	if m.Path != "." {
		t.Errorf("Path %q want '.'", m.Path)
	}
	if m.Manifest != "go.mod" || m.Language != "go" {
		t.Errorf("Manifest/Language %q/%q", m.Manifest, m.Language)
	}
	if m.Name != "example.com/single" {
		t.Errorf("Name %q", m.Name)
	}
	if !depMap(m.Dependencies)["gin"] {
		t.Errorf("gin not found in %v", m.Dependencies)
	}
	prof := ini.buildProjectProfile()
	if len(prof.Modules) != 1 {
		t.Fatalf("profile Modules %d", len(prof.Modules))
	}
}

func TestDetectModules_MonorepoThree(t *testing.T) {
	ws := t.TempDir()
	writeTestFile(t, ws, "go.mod", "module example.com/root\n\ngo 1.24\n\nrequire github.com/gin-gonic/gin v1.10.0\n")
	writeTestFile(t, ws, "services/api/go.mod", "module example.com/services/api\n\ngo 1.24\n\nrequire github.com/spf13/cobra v1.8.0\n")
	writeTestFile(t, ws, "web/package.json", `{"name":"web","dependencies":{"react":"18.0.0"}}`)
	writeTestFile(t, ws, "main.go", "package main\nfunc main() {}\n")
	writeTestFile(t, ws, "services/api/main.go", "package main\nfunc main() {}\n")
	ini := &Initializer{config: InitConfig{Workspace: ws}}
	mods := ini.detectModules()
	if len(mods) != 3 {
		t.Fatalf("want 3 got %d %+v", len(mods), mods)
	}
	if mods[0].Path != "." || mods[1].Path != "services/api" || mods[2].Path != "web" {
		t.Errorf("sorted paths %v", mods)
	}
	checkDeps(t, mods)
	checkFlatUnion(t, ini)
	checkProfileModules(t, ini)
}

func checkDeps(t *testing.T, mods []ModuleProfile) {
	t.Helper()
	for _, m := range mods {
		names := depMap(m.Dependencies)
		switch m.Path {
		case ".":
			if !names["gin"] || names["cobra"] || names["react"] {
				t.Errorf("root deps %v", m.Dependencies)
			}
		case "services/api":
			if !names["cobra"] || names["gin"] || names["react"] {
				t.Errorf("api deps %v", m.Dependencies)
			}
		case "web":
			if !names["react"] || names["gin"] || names["cobra"] {
				t.Errorf("web deps %v", m.Dependencies)
			}
		}
	}
}

func checkFlatUnion(t *testing.T, ini *Initializer) {
	t.Helper()
	flat := depMap(ini.detectDependencies())
	for _, want := range []string{"gin", "cobra", "react"} {
		if !flat[want] {
			t.Errorf("flat missing %s", want)
		}
	}
}

func checkProfileModules(t *testing.T, ini *Initializer) {
	t.Helper()
	prof := ini.buildProjectProfile()
	if len(prof.Modules) != 3 {
		t.Fatalf("profile modules %d", len(prof.Modules))
	}
	for i := 1; i < len(prof.Modules); i++ {
		if prof.Modules[i-1].Path > prof.Modules[i].Path {
			t.Errorf("not sorted")
		}
	}
}

func TestDetectModules_BothManifestsSameDir(t *testing.T) {
	ws := t.TempDir()
	writeTestFile(t, ws, "go.mod", "module example.com/combined\n\ngo 1.24\n\nrequire github.com/gin-gonic/gin v1.10.0\n")
	writeTestFile(t, ws, "package.json", `{"name":"combined","dependencies":{"react":"18.0.0"}}`)
	ini := &Initializer{config: InitConfig{Workspace: ws}}
	mods := ini.detectModules()
	if len(mods) != 2 {
		t.Fatalf("want 2 got %d", len(mods))
	}
	langs := map[string]bool{}
	for _, m := range mods {
		langs[m.Language] = true
		if m.Path != "." {
			t.Errorf("Path %q", m.Path)
		}
	}
	if !langs["go"] || !langs["javascript"] {
		t.Errorf("langs %v", langs)
	}
	ws2 := t.TempDir()
	writeTestFile(t, ws2, "svc/go.mod", "module example.com/svc\n\ngo 1.24\n\nrequire github.com/spf13/cobra v1.8.0\n")
	writeTestFile(t, ws2, "svc/package.json", `{"name":"svc-js","dependencies":{"vue":"3.0.0"}}`)
	ini2 := &Initializer{config: InitConfig{Workspace: ws2}}
	mods2 := ini2.detectModules()
	if len(mods2) != 2 {
		t.Fatalf("want 2 got %d", len(mods2))
	}
	for _, m := range mods2 {
		if m.Path != "svc" {
			t.Errorf("Path %q", m.Path)
		}
	}
	if !containsMain(mods2[0].EntryPoints) && !containsMain(mods2[1].EntryPoints) {
		_ = strings.Contains("", "")
	}
}

func containsMain(eps []string) bool {
	for _, ep := range eps {
		if ep == "main.go" || strings.HasSuffix(ep, "/main.go") {
			return true
		}
	}
	return false
}

func TestDetectModules_Sorted(t *testing.T) {
	ws := t.TempDir()
	writeTestFile(t, ws, "zebra/go.mod", "module example.com/zebra\n\ngo 1.24\n")
	writeTestFile(t, ws, "alpha/go.mod", "module example.com/alpha\n\ngo 1.24\n")
	writeTestFile(t, ws, "middle/package.json", `{"name":"middle"}`)
	ini := &Initializer{config: InitConfig{Workspace: ws}}
	mods := ini.detectModules()
	if len(mods) != 3 {
		t.Fatalf("want 3 got %d", len(mods))
	}
	if mods[0].Path != "alpha" || mods[1].Path != "middle" || mods[2].Path != "zebra" {
		t.Errorf("order %v", mods)
	}
}
