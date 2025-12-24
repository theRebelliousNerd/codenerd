package system_test

import (
	"codenerd/internal/core"
	"codenerd/internal/system"
	"codenerd/internal/tactile"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCodeDOM_EndToEnd(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, ".nerd", "mangle"), 0755); err != nil {
		t.Fatalf("mkdir .nerd: %v", err)
	}

	modulePath := "example.com/nerd-dom-demo"
	if err := writeDemoModule(ws, modulePath); err != nil {
		t.Fatalf("write demo module: %v", err)
	}

	origWD, _ := os.Getwd()
	if err := os.Chdir(ws); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("kernel: %v", err)
	}
	kernel.SetWorkspace(ws)

	executor := tactile.NewSafeExecutor()
	vsCfg := core.DefaultVirtualStoreConfig()
	vsCfg.WorkingDir = ws
	vs := core.NewVirtualStoreWithConfig(executor, vsCfg)
	vs.SetKernel(kernel)
	vs.DisableBootGuard()

	scope := system.NewHolographicCodeScope(ws, kernel, nil, 0)
	vs.SetCodeScope(scope)

	fileEditor := tactile.NewFileEditor()
	fileEditor.SetWorkingDir(ws)
	vs.SetFileEditor(core.NewTactileFileEditorAdapter(fileEditor))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	demoFile := filepath.Join(ws, "demo.go")
	if _, err := vs.RouteAction(ctx, core.Fact{Predicate: "next_action", Args: []interface{}{"test-open", "/open_file", demoFile}}); err != nil {
		t.Fatalf("open_file: %v", err)
	}

	if got := countFacts(t, kernel, "file_in_scope"); got < 2 {
		t.Fatalf("expected file_in_scope >= 2, got %d", got)
	}
	if got := countFacts(t, kernel, "code_element"); got == 0 {
		t.Fatalf("expected code_element > 0")
	}
	if got := countFacts(t, kernel, "build_tag"); got == 0 {
		t.Fatalf("expected build_tag > 0")
	}
	if got := countFacts(t, kernel, "embed_directive"); got == 0 {
		t.Fatalf("expected embed_directive > 0")
	}
	if got := countFacts(t, kernel, "api_client_function"); got == 0 {
		t.Fatalf("expected api_client_function > 0")
	}
	if got := countFacts(t, kernel, "code_defines"); got == 0 {
		t.Fatalf("expected code_defines > 0 (deep facts)")
	}

	ref := "fn:domdemo.Add"
	newAdd := "func Add(a, b int) int {\n\tsum := a + b\n\treturn sum\n}"
	if _, err := vs.RouteAction(ctx, core.Fact{
		Predicate: "next_action",
		Args: []interface{}{
			"test-edit",
			"/edit_element",
			ref,
			map[string]interface{}{"content": newAdd},
		},
	}); err != nil {
		t.Fatalf("edit_element: %v", err)
	}

	afterJSON, err := vs.RouteAction(ctx, core.Fact{
		Predicate: "next_action",
		Args: []interface{}{
			"test-get",
			"/get_element",
			ref,
			map[string]interface{}{"include_body": true},
		},
	})
	if err != nil {
		t.Fatalf("get_element: %v", err)
	}
	var after core.CodeElement
	if err := json.Unmarshal([]byte(afterJSON), &after); err != nil {
		t.Fatalf("unmarshal get_element: %v", err)
	}
	if !strings.Contains(after.Body, "sum := a + b") {
		t.Fatalf("expected edited body to contain %q", "sum := a + b")
	}

	if _, err := vs.RouteAction(ctx, core.Fact{Predicate: "next_action", Args: []interface{}{"test-close", "/close_scope", ""}}); err != nil {
		t.Fatalf("close_scope: %v", err)
	}
	if got := countFacts(t, kernel, "code_element"); got != 0 {
		t.Fatalf("expected code_element to be cleared after close_scope, got %d", got)
	}
}

func countFacts(t *testing.T, kernel core.Kernel, predicate string) int {
	t.Helper()
	facts, err := kernel.Query(predicate)
	if err != nil {
		t.Fatalf("query %s: %v", predicate, err)
	}
	return len(facts)
}

func writeDemoModule(ws, modulePath string) error {
	if err := os.MkdirAll(filepath.Join(ws, "util"), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(ws, "consumer"), 0755); err != nil {
		return err
	}

	goMod := "module " + modulePath + "\n\ngo 1.22\n"
	if err := os.WriteFile(filepath.Join(ws, "go.mod"), []byte(goMod), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(ws, "hello.txt"), []byte("hello from go:embed\n"), 0644); err != nil {
		return err
	}

	demoGo := `//go:build !ignore

package domdemo

import (
	_ "embed"
	"fmt"
	"net/http"

	"` + modulePath + `/util"
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
	resp, err := http.Get(url)
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
`
	if err := os.WriteFile(filepath.Join(ws, "demo.go"), []byte(demoGo), 0644); err != nil {
		return err
	}

	utilGo := `package util

func Twice(n int) int {
	return n * 2
}
`
	if err := os.WriteFile(filepath.Join(ws, "util", "util.go"), []byte(utilGo), 0644); err != nil {
		return err
	}

	consumerGo := `package consumer

import dd "` + modulePath + `"

func UsesDomDemo() int {
	return dd.Add(20, 22)
}
`
	return os.WriteFile(filepath.Join(ws, "consumer", "consumer.go"), []byte(consumerGo), 0644)
}
