package init

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func sortedDepNames(deps []DependencyInfo) []string {
	var out []string
	for _, d := range deps {
		out = append(out, d.Name)
	}
	slices.Sort(out)
	return out
}

// Each ecosystem's imports are spelled with its own separator, so the kernel
// matches a file's import of the package and of its subpaths, and never a
// longer name.
func TestTechnologyImportRootsSpellImportsPerEcosystem(t *testing.T) {
	want := map[string]ImportRoot{
		"go:cobra":     {Ecosystem: EcosystemGo, Exact: "github.com/spf13/cobra", Prefix: "github.com/spf13/cobra/", Technology: "cobra"},
		"npm:nextjs":   {Ecosystem: EcosystemNPM, Exact: "next", Prefix: "next/", Technology: "nextjs"},
		"py:django":    {Ecosystem: EcosystemPython, Exact: "django", Prefix: "django.", Technology: "django"},
		"py:gcp-sdk":   {Ecosystem: EcosystemPython, Exact: "google.cloud", Prefix: "google.cloud.", Technology: "gcp-sdk"},
		"rust:actix":   {Ecosystem: EcosystemRust, Exact: "actix_web", Prefix: "actix_web::", Technology: "actix-web"},
		"rust:tokio":   {Ecosystem: EcosystemRust, Exact: "tokio", Prefix: "tokio::", Technology: "tokio"},
		"npm:@prisma":  {Ecosystem: EcosystemNPM, Exact: "@prisma/client", Prefix: "@prisma/client/", Technology: "prisma"},
		"go:bubbletea": {Ecosystem: EcosystemGo, Exact: "github.com/charmbracelet/bubbletea", Prefix: "github.com/charmbracelet/bubbletea/", Technology: "bubbletea"},
	}
	roots := TechnologyImportRoots()
	for name, w := range want {
		if !slices.Contains(roots, w) {
			t.Errorf("%s: %+v missing from the import roots", name, w)
		}
	}
}

func TestParseRequirementsTxt(t *testing.T) {
	got := sortedDepNames(parseRequirementsTxt("# web\nDjango>=4.2\nfastapi[all]==0.110 ; python_version>'3.8'\n-r base.txt\nunknownpkg\nSQLAlchemy\n"))
	if !slices.Equal(got, []string{"django", "fastapi", "sqlalchemy"}) {
		t.Errorf("requirements.txt deps = %v", got)
	}
}

func TestParsePyprojectDependencies(t *testing.T) {
	pep621 := "[project]\nname = \"svc\"\ndependencies = [\n  \"flask>=3\",\n  \"pydantic\",\n]\n\n[project.optional-dependencies]\ndev = [\"pytest\"]\n"
	if got := sortedDepNames(parsePyprojectDependencies(pep621)); !slices.Equal(got, []string{"flask", "pydantic"}) {
		t.Errorf("PEP 621 deps = %v", got)
	}
	poetry := "[tool.poetry.dependencies]\npython = \"^3.11\"\ncelery = \"^5\"\n\n[tool.poetry.group.dev.dependencies]\npytest = \"*\"\n"
	if got := sortedDepNames(parsePyprojectDependencies(poetry)); !slices.Equal(got, []string{"celery", "pytest"}) {
		t.Errorf("Poetry deps = %v", got)
	}
}

func TestParseCargoTomlDependencies(t *testing.T) {
	cargo := "[package]\nname = \"svc\"\n\n[dependencies]\ntokio = { version = \"1\", features = [\"full\"] }\nserde = \"1\"\nunknown-crate = \"0.1\"\n\n[dev-dependencies]\nclap = \"4\"\n\n[dependencies.axum]\nversion = \"0.7\"\n"
	if got := sortedDepNames(parseCargoTomlDependencies(cargo)); !slices.Equal(got, []string{"axum", "clap", "serde", "tokio"}) {
		t.Errorf("Cargo.toml deps = %v", got)
	}
}

// A Python or Rust project with no lock file still declares its dependencies:
// detectDependencies reads requirements.txt, pyproject.toml and Cargo.toml.
func TestDetectDependenciesReadsPythonAndRustManifests(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "requirements.txt"), []byte("django\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(ws, "engine"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "engine", "Cargo.toml"), []byte("[dependencies]\ntokio = \"1\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	i := &Initializer{config: InitConfig{Workspace: ws}}
	got := sortedDepNames(i.detectDependencies())
	for _, want := range []string{"django", "tokio"} {
		if !slices.Contains(got, want) {
			t.Errorf("detectDependencies = %v, want %s", got, want)
		}
	}
}
