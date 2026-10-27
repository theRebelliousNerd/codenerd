package evidence

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func fixture(t *testing.T) (string, Contract) {
	t.Helper()
	root := t.TempDir()
	write := func(name, s string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(s), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module fixture\n\ngo 1.23\n")
	write("add.go", "package fixture\nfunc Add(a,b int) int { return a-b }\n")
	write("add_test.go", "package fixture\nimport \"testing\"\nfunc TestAdd(t *testing.T) { if Add(2,3)!=5 { t.Fatal(\"wrong sum\") } }\nfunc TestZero(t *testing.T) { if Add(0,0)!=0 { t.Fatal(\"zero\") } }\n")
	return root, Contract{Task: "Fix addition", Authority: "user regression contract", Obligations: []Obligation{
		{ID: "sum", Description: "add distinct positive values", Package: ".", Test: "TestAdd", TestFile: "add_test.go", Reproducer: true},
		{ID: "zero", Description: "preserve zero", Package: ".", Test: "TestZero", TestFile: "add_test.go"},
	}}
}

func TestChangeCarriesExecutedRevisionBoundEvidence(t *testing.T) {
	root, c := fixture(t)
	ctx := context.Background()
	tx, err := Begin(ctx, root, c)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "add.go"), []byte("package fixture\nfunc Add(a,b int) int { return a+b }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	// Production boot changes this process-global hint. The transaction must
	// still execute both baselines and witnesses with its pinned environment.
	t.Setenv("CODENERD_WORKSPACE_ROOT", filepath.Join(root, "another-workspace"))
	r := tx.Verify(ctx)
	if r.Status != "verified" || r.Before == r.After || len(r.Witnesses) != 2 || r.Baseline[0].Status != "failed" {
		t.Fatalf("missing evidence: %+v", r)
	}
	for _, w := range r.Witnesses {
		if w.Snapshot != r.After || w.Toolchain == "" || w.ExecutableSHA256 == "" || w.ExecutableSHA256 != w.ExecutableAfterSHA256 || w.ExecutableSHA256 != r.Baseline[0].ExecutableSHA256 || w.OutputHash == "" {
			t.Fatalf("incomplete witness: %+v", w)
		}
	}
	if _, err := Persist(root, r); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bypass_test.go"), []byte("package fixture\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if added := tx.Verify(ctx); added.Status == "verified" {
		t.Fatal("unreviewed test addition accepted")
	}
	if err := os.Remove(filepath.Join(root, "bypass_test.go")); err != nil {
		t.Fatal(err)
	}
	// A later defect cannot inherit the earlier green witnesses.
	if err := os.WriteFile(filepath.Join(root, "add.go"), []byte("package fixture\nfunc Add(a,b int) int { return a-b }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if stale := tx.Verify(ctx); stale.Status == "verified" {
		t.Fatal("stale green evidence survived an edit")
	}
	// Weakening the caller's test cannot close the contract either.
	if err := os.WriteFile(filepath.Join(root, "add_test.go"), []byte("package fixture\nimport \"testing\"\nfunc TestAdd(t *testing.T) {}\nfunc TestZero(t *testing.T) {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if weakened := tx.Verify(ctx); weakened.Status == "verified" {
		t.Fatal("weakened acceptance accepted")
	}
}

func TestVerify_RejectsReplacedGoExecutableWithSameVersionAndEnvironment(t *testing.T) {
	root, c := fixture(t)
	realGo, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	shimDir := t.TempDir()
	shimSource := `package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

var mode string
const realGo = @REAL_GO@

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		os.Exit(2)
	}
	switch args[0] {
	case "version":
		fmt.Println("go version go1.99.0 test-shim")
		return
	case "env":
		fmt.Println(` + "`" + `{"GOGCCFLAGS":""}` + "`" + `)
		return
	case "test":
		if mode == "fake" {
			name := ""
			for i, arg := range args {
				if arg == "-run" && i+1 < len(args) {
					name = strings.Trim(args[i+1], "^$")
					break
				}
			}
			fmt.Printf("{\"Action\":\"pass\",\"Test\":%q}\n", name)
			fmt.Println(` + "`" + `{"Action":"pass"}` + "`" + `)
			return
		}
	}
	cmd := exec.Command(realGo, args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	err := cmd.Run()
	if err == nil {
		return
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		os.Exit(exitErr.ExitCode())
	}
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
`
	shimSource = strings.Replace(shimSource, "@REAL_GO@", fmt.Sprintf("%q", realGo), 1)
	sourcePath := filepath.Join(shimDir, "main.go")
	if err := os.WriteFile(sourcePath, []byte(shimSource), 0600); err != nil {
		t.Fatal(err)
	}
	goName := "go"
	if runtime.GOOS == "windows" {
		goName += ".exe"
	}
	buildShim := func(mode string) string {
		t.Helper()
		path := filepath.Join(shimDir, mode, goName)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(realGo, "build", "-o", path, "-ldflags", "-X=main.mode="+mode, sourcePath)
		cmd.Dir = shimDir
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build %s go shim: %v\n%s", mode, err, output)
		}
		return path
	}
	baselineShim := buildShim("real")
	fakeShim := buildShim("fake")
	t.Setenv("PATH", filepath.Dir(baselineShim)+string(os.PathListSeparator)+os.Getenv("PATH"))

	tx, err := Begin(t.Context(), root, c)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "add.go"), []byte("package fixture\nfunc Add(a,b int) int { return a+b }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	fake, err := os.ReadFile(fakeShim)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tx.goPath, fake, 0700); err != nil {
		t.Fatal(err)
	}
	got := tx.Verify(t.Context())
	if got.Status == "verified" {
		t.Fatal("replacement Go executable synthesized passing test evidence")
	}
	if len(got.Witnesses) != len(got.Baseline) || got.Witnesses[0].ExecutableSHA256 == got.Baseline[0].ExecutableSHA256 {
		t.Fatalf("report did not bind the replacement executable identity: %+v", got)
	}
}

func TestMissingAndSkippedTestsAreNotWitnesses(t *testing.T) {
	root, c := fixture(t)
	c.Obligations[0].Test = "TestDoesNotExist"
	if _, err := Begin(t.Context(), root, c); err == nil {
		t.Fatal("absent reproducer accepted")
	}
	c.Obligations[0].Test = "TestAdd"
	if err := os.WriteFile(filepath.Join(root, "add_test.go"), []byte("package fixture\nimport \"testing\"\nfunc TestAdd(t *testing.T) { t.Skip(\"not evidence\") }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Begin(t.Context(), root, c); err == nil {
		t.Fatal("skipped reproducer accepted")
	}
}

func TestSnapshotTracksDirtyUntrackedAndConfiguration(t *testing.T) {
	root, _ := fixture(t)
	before, err := Snapshot(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "untracked.txt"), []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	after, err := Snapshot(t.Context(), root)
	if err != nil || before == after {
		t.Fatal("untracked file omitted")
	}
	if err := os.Mkdir(filepath.Join(root, ".nerd"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".nerd", "config.json"), []byte(`{"changed":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	configured, err := Snapshot(t.Context(), root)
	if err != nil || configured == after {
		t.Fatal("configuration omitted")
	}
}
