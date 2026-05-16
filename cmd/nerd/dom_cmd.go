package main

import (
	"codenerd/internal/core"
	coresys "codenerd/internal/system"
	"codenerd/internal/tactile"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	domDemoKeep        bool
	domDemoDeepWorker  int
	domInspectLimit    int
	domGetIncludeBody  bool
	domEditGoFmt       bool
	domEditGoTest      bool
	domEditContent     string
	domEditContentFile string
)

// domCmd groups Code DOM utilities.
var domCmd = &cobra.Command{
	Use:   "dom",
	Short: "Code DOM tools (semantic code editing)",
}

// domDemoCmd runs a self-contained end-to-end Code DOM demo.
var domDemoCmd = &cobra.Command{
	Use:   "demo",
	Short: "Run an end-to-end Code DOM demo in a temp workspace",
	RunE:  runDomDemo,
}

// domInspectCmd opens a real file and prints a Code DOM scope summary.
var domInspectCmd = &cobra.Command{
	Use:   "inspect <file>",
	Short: "Inspect Code DOM scope for a Go file",
	Args:  cobra.ExactArgs(1),
	RunE:  runDomInspect,
}

// domGetCmd opens a file and prints a single element (optionally with body).
var domGetCmd = &cobra.Command{
	Use:   "get <file> <ref>",
	Short: "Get a Code DOM element by ref",
	Args:  cobra.ExactArgs(2),
	RunE:  runDomGet,
}

// domEditCmd opens a file and replaces an element by ref.
var domEditCmd = &cobra.Command{
	Use:   "edit <file> <ref>",
	Short: "Edit a Code DOM element by ref (semantic replace)",
	Args:  cobra.ExactArgs(2),
	RunE:  runDomEdit,
}

func init() {
	domCmd.AddCommand(domDemoCmd)
	domCmd.AddCommand(domInspectCmd)
	domCmd.AddCommand(domGetCmd)
	domCmd.AddCommand(domEditCmd)
	domDemoCmd.Flags().BoolVar(&domDemoKeep, "keep", false, "Keep the temporary demo workspace on disk")
	domDemoCmd.Flags().IntVar(&domDemoDeepWorker, "deep-workers", 0, "Deep fact workers (0=auto)")

	domInspectCmd.Flags().IntVar(&domDemoDeepWorker, "deep-workers", 0, "Deep fact workers (0=auto)")
	domInspectCmd.Flags().IntVar(&domInspectLimit, "limit", 25, "Max elements to print")

	domGetCmd.Flags().BoolVar(&domGetIncludeBody, "include-body", false, "Include element body content")
	domGetCmd.Flags().IntVar(&domDemoDeepWorker, "deep-workers", 0, "Deep fact workers (0=auto)")

	domEditCmd.Flags().BoolVar(&domEditGoFmt, "gofmt", false, "Run gofmt on the target file after edit")
	domEditCmd.Flags().BoolVar(&domEditGoTest, "test", false, "Run `go test ./...` in workspace after edit")
	domEditCmd.Flags().StringVar(&domEditContent, "content", "", "Replacement content (full element text, including signature)")
	domEditCmd.Flags().StringVar(&domEditContentFile, "content-file", "", "Path to a file containing replacement content")
}

func runDomDemo(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
	defer cancel()

	ws, err := os.MkdirTemp("", "nerd-dom-demo-")
	if err != nil {
		return fmt.Errorf("create temp workspace: %w", err)
	}
	if !domDemoKeep {
		defer func() { _ = os.RemoveAll(ws) }()
	}
	if err := os.MkdirAll(filepath.Join(ws, ".nerd", "mangle"), 0755); err != nil {
		return fmt.Errorf("create demo .nerd dirs: %w", err)
	}

	modulePath := "example.com/nerd-dom-demo"
	if err := writeDomDemoWorkspace(ws, modulePath); err != nil {
		return err
	}

	origWD, _ := os.Getwd()
	if err := os.Chdir(ws); err != nil {
		return fmt.Errorf("chdir demo workspace: %w", err)
	}
	defer func() { _ = os.Chdir(origWD) }()

	kernel, err := core.NewRealKernel()
	if err != nil {
		return fmt.Errorf("create kernel: %w", err)
	}
	kernel.SetWorkspace(ws)

	executor := tactile.NewDirectExecutor()
	vsCfg := core.DefaultVirtualStoreConfig()
	vsCfg.WorkingDir = ws
	vs := core.NewVirtualStoreWithConfig(executor, vsCfg)
	vs.SetKernel(kernel)
	vs.DisableBootGuard() // CLI commands are user-initiated, disable boot guard

	scope := coresys.NewHolographicCodeScope(ws, kernel, nil, domDemoDeepWorker)
	vs.SetCodeScope(scope)

	fileEditor := tactile.NewFileEditor()
	fileEditor.SetWorkingDir(ws)
	vs.SetFileEditor(core.NewTactileFileEditorAdapter(fileEditor))

	demoFile := filepath.Join(ws, "demo.go")

	fmt.Printf("Code DOM demo workspace: %s\n", ws)
	fmt.Printf("Opening: %s\n", demoFile)

	if _, err := vs.RouteAction(ctx, core.Fact{Predicate: "next_action", Args: []interface{}{"dom-open", "/open_file", demoFile}}); err != nil {
		return fmt.Errorf("open_file failed: %w", err)
	}

	elementsJSON, err := vs.RouteAction(ctx, core.Fact{Predicate: "next_action", Args: []interface{}{"dom-elements", "/get_elements", ""}})
	if err != nil {
		return fmt.Errorf("get_elements failed: %w", err)
	}

	var elements []core.CodeElement
	if err := json.Unmarshal([]byte(elementsJSON), &elements); err != nil {
		return fmt.Errorf("parse get_elements output: %w", err)
	}

	sort.Slice(elements, func(i, j int) bool {
		if elements[i].File != elements[j].File {
			return elements[i].File < elements[j].File
		}
		return elements[i].StartLine < elements[j].StartLine
	})

	fmt.Printf("Elements in scope: %d\n", len(elements))
	printElementSample(elements, 10)

	printPredicateCounts(kernel, []string{
		"file_in_scope",
		"code_element",
		"build_tag",
		"embed_directive",
		"api_client_function",
		"api_handler_function",
		"generated_code",
		"edit_unsafe",
		"code_defines",
		"code_calls",
	})

	// Edit a known function to prove semantic editing works.
	refToEdit := "fn:domdemo.Add"
	newAdd := "func Add(a, b int) int {\n\tsum := a + b\n\treturn sum\n}"

	fmt.Printf("Editing element: %s\n", refToEdit)
	if _, err := vs.RouteAction(ctx, core.Fact{
		Predicate: "next_action",
		Args: []interface{}{
			"/edit_element",
			refToEdit,
			map[string]interface{}{"content": newAdd},
		},
	}); err != nil {
		return fmt.Errorf("edit_element failed: %w", err)
	}

	afterJSON, err := vs.RouteAction(ctx, core.Fact{
		Predicate: "next_action",
		Args: []interface{}{
			"/get_element",
			refToEdit,
			map[string]interface{}{"include_body": true},
		},
	})
	if err != nil {
		return fmt.Errorf("get_element after edit failed: %w", err)
	}

	var after core.CodeElement
	if err := json.Unmarshal([]byte(afterJSON), &after); err != nil {
		return fmt.Errorf("parse get_element output: %w", err)
	}
	if !strings.Contains(after.Body, "sum := a + b") {
		return fmt.Errorf("edit verification failed: expected updated body to contain %q", "sum := a + b")
	}
	fmt.Printf("Edit verified (Add now uses a local sum variable).\n")

	if err := runGoTest(ctx, ws); err != nil {
		return err
	}

	if _, err := vs.RouteAction(ctx, core.Fact{Predicate: "next_action", Args: []interface{}{"dom-close", "/close_scope", ""}}); err != nil {
		return fmt.Errorf("close_scope failed: %w", err)
	}

	fmt.Printf("Scope closed.\n")
	if domDemoKeep {
		fmt.Printf("Kept demo workspace: %s\n", ws)
	}
	return nil
}

func runDomInspect(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
	defer cancel()

	target := args[0]
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return fmt.Errorf("resolve target path: %w", err)
	}

	ws := workspace
	if ws == "" {
		if root, err := findGoWorkspaceRoot(absTarget); err == nil && root != "" {
			ws = root
		} else {
			ws, _ = os.Getwd()
		}
	}
	ws, _ = filepath.Abs(ws)

	origWD, _ := os.Getwd()
	if err := os.Chdir(ws); err != nil {
		return fmt.Errorf("chdir workspace: %w", err)
	}
	defer func() { _ = os.Chdir(origWD) }()

	kernel, err := core.NewRealKernel()
	if err != nil {
		return fmt.Errorf("create kernel: %w", err)
	}
	kernel.SetWorkspace(ws)

	executor := tactile.NewDirectExecutor()
	vsCfg := core.DefaultVirtualStoreConfig()
	vsCfg.WorkingDir = ws
	vs := core.NewVirtualStoreWithConfig(executor, vsCfg)
	vs.SetKernel(kernel)
	vs.DisableBootGuard() // CLI commands are user-initiated, disable boot guard

	scope := coresys.NewHolographicCodeScope(ws, kernel, nil, domDemoDeepWorker)
	vs.SetCodeScope(scope)

	fileEditor := tactile.NewFileEditor()
	fileEditor.SetWorkingDir(ws)
	vs.SetFileEditor(core.NewTactileFileEditorAdapter(fileEditor))

	fmt.Printf("Workspace: %s\n", ws)
	fmt.Printf("Opening:   %s\n", absTarget)

	if _, err := vs.RouteAction(ctx, core.Fact{Predicate: "next_action", Args: []interface{}{"dom-open", "/open_file", absTarget}}); err != nil {
		return fmt.Errorf("open_file failed: %w", err)
	}

	inScope := scope.GetInScopeFiles()
	fmt.Printf("Files in scope: %d\n", len(inScope))

	fileElementsJSON, err := vs.RouteAction(ctx, core.Fact{Predicate: "next_action", Args: []interface{}{"dom-elements", "/get_elements", absTarget}})
	if err != nil {
		return fmt.Errorf("get_elements failed: %w", err)
	}

	var fileElements []core.CodeElement
	if err := json.Unmarshal([]byte(fileElementsJSON), &fileElements); err != nil {
		return fmt.Errorf("parse get_elements output: %w", err)
	}

	sort.Slice(fileElements, func(i, j int) bool { return fileElements[i].StartLine < fileElements[j].StartLine })
	fmt.Printf("Elements in file: %d\n", len(fileElements))
	printElementSample(fileElements, domInspectLimit)

	printPredicateCounts(kernel, []string{
		"file_in_scope",
		"code_element",
		"parse_error",
		"file_not_found",
		"encoding_issue",
		"large_file_warning",
		"generated_code",
		"cgo_code",
		"build_tag",
		"embed_directive",
		"api_client_function",
		"api_handler_function",
		"code_defines",
		"code_calls",
	})

	_ = closeScopeQuiet(ctx, vs)
	return nil
}

func runDomGet(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
	defer cancel()

	target := args[0]
	ref := args[1]
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return fmt.Errorf("resolve target path: %w", err)
	}

	ws := workspace
	if ws == "" {
		if root, err := findGoWorkspaceRoot(absTarget); err == nil && root != "" {
			ws = root
		} else {
			ws, _ = os.Getwd()
		}
	}
	ws, _ = filepath.Abs(ws)

	origWD, _ := os.Getwd()
	if err := os.Chdir(ws); err != nil {
		return fmt.Errorf("chdir workspace: %w", err)
	}
	defer func() { _ = os.Chdir(origWD) }()

	_, vs, _, err := newDOMHarness(ws, domDemoDeepWorker)
	if err != nil {
		return err
	}

	if _, err := vs.RouteAction(ctx, core.Fact{Predicate: "next_action", Args: []interface{}{"dom-open", "/open_file", absTarget}}); err != nil {
		return fmt.Errorf("open_file failed: %w", err)
	}

	out, err := vs.RouteAction(ctx, core.Fact{
		Predicate: "next_action",
		Args: []interface{}{
			"/get_element",
			ref,
			map[string]interface{}{"include_body": domGetIncludeBody},
		},
	})
	if err != nil {
		return fmt.Errorf("get_element failed: %w", err)
	}

	// Pretty-print for humans.
	var obj any
	if jsonErr := json.Unmarshal([]byte(out), &obj); jsonErr == nil {
		if pretty, err := json.MarshalIndent(obj, "", "  "); err == nil {
			out = string(pretty)
		}
	}
	fmt.Println(out)

	_ = closeScopeQuiet(ctx, vs)
	return nil
}

func runDomEdit(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
	defer cancel()

	target := args[0]
	ref := args[1]
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return fmt.Errorf("resolve target path: %w", err)
	}

	content := domEditContent
	if strings.TrimSpace(domEditContentFile) != "" {
		b, err := os.ReadFile(domEditContentFile)
		if err != nil {
			return fmt.Errorf("read --content-file: %w", err)
		}
		content = string(b)
	}
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("edit requires --content or --content-file")
	}

	ws := workspace
	if ws == "" {
		if root, err := findGoWorkspaceRoot(absTarget); err == nil && root != "" {
			ws = root
		} else {
			ws, _ = os.Getwd()
		}
	}
	ws, _ = filepath.Abs(ws)

	origWD, _ := os.Getwd()
	if err := os.Chdir(ws); err != nil {
		return fmt.Errorf("chdir workspace: %w", err)
	}
	defer func() { _ = os.Chdir(origWD) }()

	_, vs, _, err := newDOMHarness(ws, domDemoDeepWorker)
	if err != nil {
		return err
	}

	fmt.Printf("Workspace: %s\n", ws)
	fmt.Printf("Opening:   %s\n", absTarget)

	if _, err := vs.RouteAction(ctx, core.Fact{Predicate: "next_action", Args: []interface{}{"dom-open", "/open_file", absTarget}}); err != nil {
		return fmt.Errorf("open_file failed: %w", err)
	}

	fmt.Printf("Editing:   %s\n", ref)
	if _, err := vs.RouteAction(ctx, core.Fact{
		Predicate: "next_action",
		Args: []interface{}{
			"/edit_element",
			ref,
			map[string]interface{}{"content": content},
		},
	}); err != nil {
		return fmt.Errorf("edit_element failed: %w", err)
	}

	if domEditGoFmt {
		gofmtCmd := exec.CommandContext(ctx, "gofmt", "-w", absTarget)
		gofmtCmd.Dir = ws
		if out, err := gofmtCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("gofmt failed: %w\n%s", err, string(out))
		}
		fmt.Printf("gofmt: OK\n")
	}

	if domEditGoTest {
		// Default to testing the main CLI + internal packages (excludes cmd/tools, which may be in-flight).
		fmt.Printf("Running: go test -count=1 ./cmd/nerd/... ./internal/... (workspace)\n")
		testCmd := exec.CommandContext(ctx, "go", "test", "-count=1", "./cmd/nerd/...", "./internal/...")
		testCmd.Dir = ws
		out, err := testCmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("go test failed: %w\n%s", err, string(out))
		}
		fmt.Printf("go test: PASS\n")
	}

	return nil
}

func newDOMHarness(ws string, deepWorkers int) (*core.RealKernel, *core.VirtualStore, *coresys.HolographicCodeScope, error) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create kernel: %w", err)
	}
	kernel.SetWorkspace(ws)

	executor := tactile.NewDirectExecutor()
	vsCfg := core.DefaultVirtualStoreConfig()
	vsCfg.WorkingDir = ws
	vs := core.NewVirtualStoreWithConfig(executor, vsCfg)
	vs.SetKernel(kernel)
	vs.DisableBootGuard() // CLI commands are user-initiated, disable boot guard

	scope := coresys.NewHolographicCodeScope(ws, kernel, nil, deepWorkers)
	vs.SetCodeScope(scope)

	fileEditor := tactile.NewFileEditor()
	fileEditor.SetWorkingDir(ws)
	vs.SetFileEditor(core.NewTactileFileEditorAdapter(fileEditor))

	return kernel, vs, scope, nil
}

func writeDomDemoWorkspace(workspace, modulePath string) error {
	if err := os.MkdirAll(filepath.Join(workspace, "util"), 0755); err != nil {
		return fmt.Errorf("create util dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(workspace, "consumer"), 0755); err != nil {
		return fmt.Errorf("create consumer dir: %w", err)
	}

	goMod := fmt.Sprintf("module %s\n\ngo 1.22\n", modulePath)
	if err := os.WriteFile(filepath.Join(workspace, "go.mod"), []byte(goMod), 0644); err != nil {
		return fmt.Errorf("write go.mod: %w", err)
	}

	if err := os.WriteFile(filepath.Join(workspace, "hello.txt"), []byte("hello from go:embed\n"), 0644); err != nil {
		return fmt.Errorf("write hello.txt: %w", err)
	}

	demoGo := fmt.Sprintf(`//go:build !ignore

package domdemo

import (
	_ "embed"
	"fmt"
	"net/http"

	"%s/util"
)

//go:embed hello.txt
var embeddedHello string

func Add(a, b int) int {
	return a + b
}

func TwiceViaUtil(n int) int {
	return util.Twice(n)
}

func FetchURL(url string) (int, error) {
	resp, err := http.Get(url) // triggers api_client_function
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}

func RegisterHandlers() {
	http.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "pong")
	})
	fmt.Println(embeddedHello)
}
`, modulePath)
	if err := os.WriteFile(filepath.Join(workspace, "demo.go"), []byte(demoGo), 0644); err != nil {
		return fmt.Errorf("write demo.go: %w", err)
	}

	utilGo := `package util

func Twice(n int) int {
	return n * 2
}
`
	if err := os.WriteFile(filepath.Join(workspace, "util", "util.go"), []byte(utilGo), 0644); err != nil {
		return fmt.Errorf("write util.go: %w", err)
	}

	consumerGo := fmt.Sprintf(`package consumer

import dd "%s"

func UsesDomDemo() int {
	return dd.Add(20, 22)
}
`, modulePath)
	if err := os.WriteFile(filepath.Join(workspace, "consumer", "consumer.go"), []byte(consumerGo), 0644); err != nil {
		return fmt.Errorf("write consumer.go: %w", err)
	}

	testGo := `package domdemo

import "testing"

func TestAdd(t *testing.T) {
	if Add(20, 22) != 42 {
		t.Fatalf("expected 42")
	}
}
`
	if err := os.WriteFile(filepath.Join(workspace, "demo_test.go"), []byte(testGo), 0644); err != nil {
		return fmt.Errorf("write demo_test.go: %w", err)
	}

	return nil
}

func findGoWorkspaceRoot(filePath string) (string, error) {
	dir := filepath.Dir(filePath)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("no go.mod found above %s", filePath)
}

func printElementSample(elements []core.CodeElement, limit int) {
	if limit <= 0 || len(elements) == 0 {
		return
	}
	if limit > len(elements) {
		limit = len(elements)
	}
	fmt.Printf("Sample elements:\n")
	for i := 0; i < limit; i++ {
		e := elements[i]
		fmt.Printf("  - %s (%s) %s:%d-%d\n", e.Ref, e.Type, filepath.Base(e.File), e.StartLine, e.EndLine)
	}
}

func printPredicateCounts(kernel core.Kernel, predicates []string) {
	for _, p := range predicates {
		facts, err := kernel.Query(p)
		if err != nil {
			fmt.Printf("%s: (query error: %v)\n", p, err)
			continue
		}
		fmt.Printf("%s: %d\n", p, len(facts))
	}
}

func closeScopeQuiet(ctx context.Context, vs *core.VirtualStore) error {
	_, err := vs.RouteAction(ctx, core.Fact{Predicate: "next_action", Args: []interface{}{"dom-close", "/close_scope", ""}})
	return err
}

func runGoTest(ctx context.Context, dir string) error {
	fmt.Printf("Running: go test ./... (demo workspace)\n")
	testCmd := exec.CommandContext(ctx, "go", "test", "./...")
	testCmd.Dir = dir
	out, err := testCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go test failed: %w\n%s", err, string(out))
	}
	fmt.Printf("go test: PASS\n")
	return nil
}
