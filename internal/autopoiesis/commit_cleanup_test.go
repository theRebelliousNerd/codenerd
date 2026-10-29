package autopoiesis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/types"
)

func TestOuroborosCommitFailurePreservesCommittedToolAndNewFailureLeavesNoArtifacts(t *testing.T) {
	t.Setenv("CODE_NERD_WORKSPACE_ROOT", "")
	workspace := t.TempDir()
	cfg := DefaultOuroborosConfig(workspace)
	cfg.EnableThunderdome = false
	cfg.TargetOS = ""
	cfg.TargetArch = ""
	loop := NewOuroborosLoop(&MockLLMClient{}, cfg)
	// These fixtures are stdlib-only, so the temporary generated module does
	// not need a replace directive back to the workspace.
	loop.compiler.config.WorkspaceRoot = ""

	oldTool := commitFixtureTool(cfg.ToolsDir, "probe", "old", passingCommitFixtureTest())
	if err := loop.commitTool(context.Background(), oldTool, &LoopResult{}); err != nil {
		t.Fatalf("commit initial version: %v", err)
	}
	oldSource, err := os.ReadFile(oldTool.FilePath)
	if err != nil {
		t.Fatal(err)
	}
	oldTest, err := os.ReadFile(toolTestPath(oldTool.FilePath))
	if err != nil {
		t.Fatal(err)
	}
	oldRuntime, ok := loop.registry.Get("probe")
	if !ok {
		t.Fatal("initial version was not registered")
	}
	oldIdentityBytes, err := os.ReadFile(oldRuntime.BinaryPath + ".identity.json")
	if err != nil {
		t.Fatal(err)
	}
	oldBinary, err := os.ReadFile(oldRuntime.BinaryPath)
	if err != nil {
		t.Fatal(err)
	}
	oldDigest := sha256.Sum256(oldBinary)
	if got := hex.EncodeToString(oldDigest[:]); got != oldRuntime.Hash {
		t.Fatalf("registered digest %q does not match initial binary %q", oldRuntime.Hash, got)
	}

	before, err := loop.ExecuteTool(context.Background(), "probe", `{"value":"before"}`)
	if err != nil || before != "old:before" {
		t.Fatalf("initial execution = %q, %v", before, err)
	}

	rejected := commitFixtureTool(cfg.ToolsDir, "probe", "rejected", `package tools

import "testing"

func TestGeneratedProbe(t *testing.T) { t.Fatal("deliberate generated test failure") }
`)
	if err := loop.commitTool(context.Background(), rejected, &LoopResult{}); err == nil || !strings.Contains(err.Error(), "generated tool failed its own generated tests") {
		t.Fatalf("failed replacement error = %v, want generated-test failure", err)
	}
	gotSource, err := os.ReadFile(oldTool.FilePath)
	if err != nil || string(gotSource) != string(oldSource) {
		t.Fatalf("failed replacement changed source: err=%v source=%q", err, gotSource)
	}
	gotTest, err := os.ReadFile(toolTestPath(oldTool.FilePath))
	if err != nil || string(gotTest) != string(oldTest) {
		t.Fatalf("failed replacement changed tests: err=%v tests=%q", err, gotTest)
	}
	gotBinary, err := os.ReadFile(oldRuntime.BinaryPath)
	if err != nil || sha256.Sum256(gotBinary) != oldDigest {
		t.Fatalf("failed replacement changed binary: err=%v", err)
	}
	gotIdentity, err := os.ReadFile(oldRuntime.BinaryPath + ".identity.json")
	if err != nil || string(gotIdentity) != string(oldIdentityBytes) {
		t.Fatalf("failed replacement changed identity: err=%v identity=%s", err, gotIdentity)
	}
	currentRuntime, ok := loop.registry.Get("probe")
	if !ok || currentRuntime.Hash != oldRuntime.Hash || currentRuntime.BinaryPath != oldRuntime.BinaryPath {
		t.Fatalf("failed replacement changed registered tool: old=%+v current=%+v", oldRuntime, currentRuntime)
	}
	after, err := loop.ExecuteTool(context.Background(), "probe", `{"value":"after"}`)
	if err != nil || after != "old:after" {
		t.Fatalf("execution after failed replacement = %q, %v", after, err)
	}

	newTool := commitFixtureTool(cfg.ToolsDir, "probe", "new", passingCommitFixtureTest())
	if err := loop.commitTool(context.Background(), newTool, &LoopResult{}); err != nil {
		t.Fatalf("commit successful replacement: %v", err)
	}

	newRegistry := NewRuntimeRegistry(workspace)
	newRegistry.Restore(cfg.ToolsDir, cfg.CompiledDir)
	identity, err := newRegistry.GeneratedToolIdentity("probe")
	if err != nil {
		t.Fatalf("restore successful replacement: %v", err)
	}
	if identity.Workspace != workspace {
		t.Fatalf("reloaded workspace = %q, want %q", identity.Workspace, workspace)
	}
	canonicalArgs, err := types.CanonicalGeneratedArgs(map[string]any{"value": "reloaded"})
	if err != nil {
		t.Fatal(err)
	}
	receipt := RunGeneratedBinary(context.Background(), types.GeneratedToolRequest{
		ScopeID: "commit", CallID: "reloaded", AuthorizationID: "exec-reloaded", Action: "/probe", Target: "probe",
		CanonicalArgs: canonicalArgs, Tool: identity,
	})
	if receipt.BackendError != nil || receipt.Output != "new:reloaded" {
		t.Fatalf("reloaded execution = %+v, want new:reloaded", receipt)
	}

	badNewTool := commitFixtureTool(cfg.ToolsDir, "rejected_new", "never", `package tools

import "testing"

func TestGeneratedProbe(t *testing.T) { t.Fatal("deliberate generated test failure") }
`)
	if err := loop.commitTool(context.Background(), badNewTool, &LoopResult{}); err == nil {
		t.Fatal("failed first generation unexpectedly committed")
	}
	compiledPath := filepath.Join(cfg.CompiledDir, "rejected_new")
	if cfg.TargetOS == "windows" {
		compiledPath += ".exe"
	}
	for _, path := range []string{badNewTool.FilePath, toolTestPath(badNewTool.FilePath), compiledPath, compiledPath + ".identity.json"} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("failed first generation left artifact %s (stat error %v)", path, err)
		}
	}
	if _, exists := loop.registry.Get("rejected_new"); exists {
		t.Error("failed first generation left a runtime registration")
	}
	loaded := NewToolGenerator(&MockLLMClient{}, cfg.ToolsDir)
	if err := loaded.LoadExistingTools(); err != nil {
		t.Fatal(err)
	}
	if loaded.HasTool("rejected_new") {
		t.Error("failed first generation remained loadable as an existing tool")
	}
}

func TestRestoreGeneratedToolFilesReportsFailureAndKeepsRecoveryCopy(t *testing.T) {
	dir := t.TempDir()
	destination := filepath.Join(dir, "tool.go")
	backup := filepath.Join(dir, "tool.go.rollback")
	if err := os.Mkdir(destination, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(backup, []byte("previous source"), 0600); err != nil {
		t.Fatal(err)
	}

	err := restoreGeneratedToolFiles([]generatedFileSnapshot{{path: destination, backupPath: backup, existed: true}})
	if err == nil || !strings.Contains(err.Error(), destination) {
		t.Fatalf("rollback error = %v, want an error naming %s", err, destination)
	}
	if data, readErr := os.ReadFile(backup); readErr != nil || string(data) != "previous source" {
		t.Fatalf("failed rollback lost recovery copy: data=%q err=%v", data, readErr)
	}
}

func commitFixtureTool(toolsDir, name, prefix, testCode string) *GeneratedTool {
	code, _ := json.Marshal(prefix)
	return &GeneratedTool{
		Name: name,
		Code: `package tools

import (
	"context"
	"encoding/json"
)

func Probe(ctx context.Context, input string) (string, error) {
	var args struct { Value string ` + "`json:\"value\"`" + ` }
	if err := json.Unmarshal([]byte(input), &args); err != nil { return "", err }
	return ` + string(code) + ` + ":" + args.Value, nil
}
`,
		TestCode: testCode,
		FilePath: filepath.Join(toolsDir, name+".go"),
	}
}

func passingCommitFixtureTest() string {
	return `package tools

import (
	"context"
	"testing"
)

func TestGeneratedProbe(t *testing.T) {
	got, err := Probe(context.Background(), ` + "`{\"value\":\"test\"}`" + `)
	if err != nil || got != "old:test" && got != "new:test" {
		t.Fatalf("Probe() = %q, %v", got, err)
	}
}
`
}
