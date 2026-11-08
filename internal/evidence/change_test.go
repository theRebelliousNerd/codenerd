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

func TestContractValidate_PinBoundaries(t *testing.T) {
	valid := func() Contract {
		_, c := fixture(t)
		return c
	}
	if err := valid().Validate(); err != nil {
		t.Fatalf("fixture contract must validate: %v", err)
	}
	cases := map[string]func(*Contract){
		"empty task":      func(c *Contract) { c.Task = "  " },
		"empty authority": func(c *Contract) { c.Authority = "" },
		"no obligations":  func(c *Contract) { c.Obligations = nil },
		"missing reproducer": func(c *Contract) {
			for i := range c.Obligations {
				c.Obligations[i].Reproducer = false
			}
		},
		"duplicate id":        func(c *Contract) { c.Obligations[1].ID = c.Obligations[0].ID },
		"empty description":   func(c *Contract) { c.Obligations[0].Description = "" },
		"bad test name":       func(c *Contract) { c.Obligations[0].Test = "test_add" },
		"bad package parent":  func(c *Contract) { c.Obligations[0].Package = "../escape" },
		"bad package space":   func(c *Contract) { c.Obligations[0].Package = "./has space" },
		"missing test suffix": func(c *Contract) { c.Obligations[0].TestFile = "add.go" },
	}
	for name, mutate := range cases {
		c := valid()
		mutate(&c)
		if err := c.Validate(); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
	overflow := valid()
	for len(overflow.Obligations) <= 32 {
		extra := overflow.Obligations[0]
		extra.ID += string(rune('a' + len(overflow.Obligations)))
		overflow.Obligations = append(overflow.Obligations, extra)
	}
	if err := overflow.Validate(); err == nil {
		t.Fatal("more than 32 obligations accepted")
	}
}

func TestLoadContract_PinSingleObjectContract(t *testing.T) {
	dir := t.TempDir()
	valid := `{"task":"t","authority":"a","obligations":[{"id":"x","description":"d","package":".","test":"TestX","test_file":"x_test.go","reproducer":true}]}`
	validPath := filepath.Join(dir, "valid.json")
	if err := os.WriteFile(validPath, []byte(valid), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadContract(validPath); err != nil {
		t.Fatalf("valid contract rejected: %v", err)
	}
	unknownPath := filepath.Join(dir, "unknown.json")
	if err := os.WriteFile(unknownPath, []byte(`{"task":"t","authority":"a","extra":1,"obligations":[{"id":"x","description":"d","package":".","test":"TestX","test_file":"x_test.go","reproducer":true}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadContract(unknownPath); err == nil {
		t.Fatal("unknown field accepted")
	}
	trailingPath := filepath.Join(dir, "trailing.json")
	if err := os.WriteFile(trailingPath, []byte(valid+"\n{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadContract(trailingPath); err == nil {
		t.Fatal("trailing JSON accepted")
	}
	if _, err := LoadContract(filepath.Join(dir, "missing.json")); err == nil {
		t.Fatal("missing file accepted")
	}
	badPath := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(badPath, []byte(`{not json`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadContract(badPath); err == nil {
		t.Fatal("malformed JSON accepted")
	}
}

func TestPersist_PinIdentityValidation(t *testing.T) {
	root, _ := fixture(t)
	badID := Report{ContractID: "not-hex", After: ""}
	if _, err := Persist(root, badID); err == nil {
		t.Fatal("invalid contract ID accepted")
	}
	badAfter := Report{ContractID: strings.Repeat("a", 64), After: "zzz"}
	if _, err := Persist(root, badAfter); err == nil {
		t.Fatal("invalid after snapshot accepted")
	}
	ok := Report{ContractID: strings.Repeat("b", 64), After: "", Status: "unverified"}
	path, err := Persist(root, ok)
	if err != nil {
		t.Fatalf("empty after snapshot rejected: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("persisted report missing: %v", err)
	}
}

func TestVerify_RequiresArtifactChange(t *testing.T) {
	root, c := fixture(t)
	ctx := context.Background()
	tx, err := Begin(ctx, root, c)
	if err != nil {
		t.Fatal(err)
	}
	r := tx.Verify(ctx)
	if r.Status == "verified" {
		t.Fatal("unchanged workspace verified")
	}
	found := false
	for _, u := range r.Unknown {
		if strings.Contains(u, "no artifact change") {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing no-change signal: %+v", r)
	}
}

func TestWithContract_CopiesObligations(t *testing.T) {
	_, c := fixture(t)
	ctx := WithContract(context.Background(), c)
	c.Obligations[0].ID = "mutated"
	got, ok := ContractFromContext(ctx)
	if !ok {
		t.Fatal("contract missing from context")
	}
	if got.Obligations[0].ID == "mutated" {
		t.Fatal("WithContract retained caller slice")
	}
	got.Obligations[0].ID = "mutated-again"
	again, _ := ContractFromContext(ctx)
	if again.Obligations[0].ID == "mutated-again" {
		t.Fatal("ContractFromContext exposed stored slice")
	}
	if _, ok := ContractFromContext(context.Background()); ok {
		t.Fatal("empty context reported a contract")
	}
}

func TestReportSummaryAndUnverified_PinStatus(t *testing.T) {
	_, c := fixture(t)
	r := Report{Contract: c, ContractID: strings.Repeat("c", 64), Before: strings.Repeat("0", 64), After: strings.Repeat("1", 64), Status: "verified"}
	r.Witnesses = []Witness{{Obligation: "sum", Status: "passed"}}
	r.Unknown = []string{"note"}
	s := r.Summary()
	if !strings.Contains(s, "verified") || !strings.Contains(s, "sum: passed") || !strings.Contains(s, "Unverified: note") {
		t.Fatalf("summary omitted status, witness or unknown: %q", s)
	}
	shortAfter := s
	if !strings.Contains(shortAfter, r.After[:12]) {
		t.Fatalf("summary did not shorten after snapshot: %q", s)
	}
	unverifiedTx := &Transaction{report: Report{Contract: c}}
	u := unverifiedTx.Unverified("reason")
	if u.Status != "unverified" || len(u.Unknown) != 1 || u.Unknown[0] != "reason" {
		t.Fatalf("Unverified did not pin reason: %+v", u)
	}
	if got := short(strings.Repeat("x", 64)); len(got) != 12 {
		t.Fatalf("short did not truncate: %q", got)
	}
	if got := short("tiny"); got != "tiny" {
		t.Fatalf("short altered small input: %q", got)
	}
}
