package autopoiesis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"codenerd/internal/atomicfile"
	"codenerd/internal/types"
)

// The generated wrapper emits `{"output": <raw json>}`. Reading that back as a
// Go string silently worked only for tools whose return value was NOT valid
// JSON, because the wrapper marshals those into a JSON string. Every other
// tool — anything returning a count, a bool, or a JSON document — registered
// successfully and then failed on its first call.
func TestDecodeToolOutput_WhenWrapperEmitsNonStringJSON_ShouldRenderItAsText(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"marshalled plain text", `"processed: hi"`, "processed: hi"},
		{"bare number", `3`, "3"},
		{"bare bool", `true`, "true"},
		{"json object passthrough", `{"lines":3}`, `{"lines":3}`},
		{"json array passthrough", `[1,2,3]`, `[1,2,3]`},
		{"null", `null`, ""},
		{"empty", ``, ""},
		{"string containing json", `"{\"a\":1}"`, `{"a":1}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := decodeToolOutput(json.RawMessage(tt.raw)); got != tt.want {
				t.Errorf("decodeToolOutput(%s) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

// A copied test executable is the real subprocess fixture on every host OS.
func init() {
	if !strings.HasPrefix(filepath.Base(os.Args[0]), "generated_runtime_fixture") {
		return
	}
	var input string
	if len(os.Args) > 1 {
		input = os.Args[1]
	} else {
		var envelope struct {
			Input string `json:"input"`
		}
		if json.NewDecoder(os.Stdin).Decode(&envelope) != nil {
			os.Exit(9)
		}
		input = envelope.Input
	}
	var args struct {
		Mode      string          `json:"mode"`
		Value     json.RawMessage `json:"value"`
		Admission string          `json:"admission"`
	}
	if json.Unmarshal([]byte(input), &args) != nil {
		os.Exit(8)
	}
	switch args.Mode {
	case "error":
		fmt.Print(`{"output":"partial","error":"fixture failure"}`)
		os.Exit(3)
	case "malformed":
		fmt.Print("partial malformed output")
		fmt.Fprint(os.Stderr, "fixture diagnostic")
		os.Exit(4)
	case "block":
		connection, err := net.Dial("tcp", args.Admission)
		if err != nil {
			os.Exit(7)
		}
		_, _ = connection.Write([]byte{1})
		var release [1]byte
		_, _ = connection.Read(release[:])
		_ = connection.Close()
	case "argv":
		fmt.Print(input)
		os.Exit(0)
	case "cwd":
		cwd, err := os.Getwd()
		if err != nil {
			os.Exit(6)
		}
		args.Value, _ = json.Marshal(cwd)
	}
	if args.Value == nil {
		args.Value = json.RawMessage(`"literal"`)
	}
	_, _ = os.Stdout.Write(append(append([]byte(`{"output":`), args.Value...), '}'))
	os.Exit(0)
}

func runtimeFixtureRequest(t *testing.T, args map[string]any, protocol types.GeneratedToolProtocol) types.GeneratedToolRequest {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "generated_runtime_fixture.exe")
	if err := os.WriteFile(path, data, 0700); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	canonical, err := types.CanonicalGeneratedArgs(args)
	if err != nil {
		t.Fatal(err)
	}
	return types.GeneratedToolRequest{ScopeID: "runtime", CallID: "call", AuthorizationID: "exec-call", Action: "/fixture", Target: "unknown", CanonicalArgs: canonical,
		Tool: types.GeneratedToolIdentity{Name: "fixture", BinaryPath: path, BinaryHash: hex.EncodeToString(digest[:]), Protocol: protocol, Workspace: t.TempDir()}}
}

func TestGeneratedRuntimeOutputAndFailures(t *testing.T) {
	for _, scenario := range []struct {
		name, mode string
		value      any
		output     string
		failed     bool
	}{
		{"string", "", "text", "text", false}, {"number", "", 3, "3", false},
		{"object", "", map[string]any{"n": 2}, `{"n":2}`, false}, {"null", "", nil, "", false},
		{"nonzero", "error", nil, "partial", true}, {"malformed", "malformed", nil, "partial malformed output", true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			request := runtimeFixtureRequest(t, map[string]any{"mode": scenario.mode, "value": scenario.value}, types.GeneratedStdinV1)
			receipt := RunGeneratedBinary(context.Background(), request)
			if !receipt.ProcessStarted || receipt.Output != scenario.output || (receipt.BackendError != nil) != scenario.failed {
				t.Fatalf("receipt: %+v", receipt)
			}
			if scenario.failed && !receipt.PartialOutput {
				t.Fatal("failure discarded partial output")
			}
		})
	}
}

func TestGeneratedRuntimeProtocolAndIdentityRefusals(t *testing.T) {
	request := runtimeFixtureRequest(t, map[string]any{"mode": "argv"}, types.LegacyArgvV1)
	receipt := RunGeneratedBinary(context.Background(), request)
	if receipt.BackendError != nil || receipt.Output != request.CanonicalArgs {
		t.Fatalf("legacy argv: %+v", receipt)
	}
	request.Tool.Protocol = "unknown"
	if receipt := RunGeneratedBinary(context.Background(), request); receipt.ProcessStarted || receipt.BackendError == nil {
		t.Fatalf("unknown protocol admitted: %+v", receipt)
	}
	request.Tool.Protocol = types.GeneratedStdinV1
	if err := atomicfile.WriteFile(request.Tool.BinaryPath, []byte("mutated"), 0700); err != nil {
		t.Fatal(err)
	}
	if receipt := RunGeneratedBinary(context.Background(), request); receipt.ProcessStarted || receipt.BackendError == nil {
		t.Fatalf("changed binary admitted: %+v", receipt)
	}
}

func TestRuntimeRegistryGeneratedIdentity_PinsWorkspaceAcrossRestoreAndExecution(t *testing.T) {
	workspace := t.TempDir()
	cfg := DefaultOuroborosConfig(workspace)
	cfg.EnableThunderdome = false
	loop := NewOuroborosLoop(&MockLLMClient{}, cfg)

	if err := os.MkdirAll(cfg.ToolsDir, 0755); err != nil {
		t.Fatal(err)
	}
	toolName := "generated_runtime_fixture"
	if err := os.WriteFile(filepath.Join(cfg.ToolsDir, toolName+".go"), []byte("// Description: report working directory\npackage tools\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cfg.CompiledDir, 0755); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	binary, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	binaryPath := filepath.Join(cfg.CompiledDir, toolName+".exe")
	if err := os.WriteFile(binaryPath, binary, 0700); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(binary)
	hash := hex.EncodeToString(digest[:])
	if _, err := loop.registry.Register(&GeneratedTool{Name: toolName}, &CompileResult{Success: true, OutputPath: binaryPath, Hash: hash}); err != nil {
		t.Fatal(err)
	}

	registered, err := loop.registry.GeneratedToolIdentity(toolName)
	if err != nil {
		t.Fatal(err)
	}
	if registered.Workspace != workspace {
		t.Fatalf("registered workspace = %q, want %q", registered.Workspace, workspace)
	}
	canonical, err := types.CanonicalGeneratedArgs(map[string]any{"mode": "cwd"})
	if err != nil {
		t.Fatal(err)
	}
	request := types.GeneratedToolRequest{
		ScopeID: "workspace", CallID: "before-restore", AuthorizationID: "exec-before-restore", Action: types.MangleAtom("/" + toolName), Target: toolName, CanonicalArgs: canonical, Tool: registered,
	}
	beforeRestore := RunGeneratedBinary(context.Background(), request)
	if beforeRestore.BackendError != nil {
		t.Fatalf("execution before restore failed: %+v", beforeRestore)
	}
	if beforeRestore.Output != workspace {
		t.Fatalf("execution before restore ran in %q, want pinned workspace %q", beforeRestore.Output, workspace)
	}

	restored := NewRuntimeRegistry(workspace)
	restored.Restore(cfg.ToolsDir, cfg.CompiledDir)
	identity, err := restored.GeneratedToolIdentity(toolName)
	if err != nil {
		t.Fatal(err)
	}
	if identity != registered {
		t.Fatalf("restored identity = %+v, want %+v", identity, registered)
	}
	receipt := RunGeneratedBinary(context.Background(), types.GeneratedToolRequest{
		ScopeID: "workspace", CallID: "call", AuthorizationID: "exec-call", Action: types.MangleAtom("/" + toolName), Target: toolName, CanonicalArgs: canonical, Tool: identity,
	})
	if receipt.BackendError != nil {
		t.Fatalf("execution failed: %+v", receipt)
	}
	if receipt.Output != workspace {
		t.Fatalf("generated tool ran in %q, want pinned workspace %q", receipt.Output, workspace)
	}
}

func TestRuntimeRegistrySourceOnlyRegistrationRevokesCompiledIdentityAcrossRestore(t *testing.T) {
	workspace := t.TempDir()
	cfg := DefaultOuroborosConfig(workspace)
	cfg.ExecutionMode = ExecuteInterpreted
	cfg.EnableThunderdome = false
	loop := NewOuroborosLoop(&MockLLMClient{}, cfg)
	toolName := "generated_runtime_fixture"
	toolSource := `package main

import "context"

func RunTool(ctx context.Context, input string) (string, error) {
	return "source:" + input, nil
}
`
	if err := os.MkdirAll(cfg.ToolsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.ToolsDir, toolName+".go"), []byte(toolSource), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cfg.CompiledDir, 0755); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	binary, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	binaryPath := filepath.Join(cfg.CompiledDir, toolName)
	if runtime.GOOS == "windows" {
		binaryPath += ".exe"
	}
	if err := os.WriteFile(binaryPath, binary, 0700); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(binary)
	if _, err := loop.registry.Register(&GeneratedTool{Name: toolName}, &CompileResult{
		Success: true, OutputPath: binaryPath, Hash: hex.EncodeToString(digest[:]),
	}); err != nil {
		t.Fatalf("register compiled tool: %v", err)
	}
	if _, err := loop.registry.GeneratedToolIdentity(toolName); err != nil {
		t.Fatalf("compiled tool identity missing before revocation: %v", err)
	}
	if _, err := os.Stat(binaryPath + ".identity.json"); err != nil {
		t.Fatalf("compiled sidecar missing before revocation: %v", err)
	}

	interpreted, err := loop.registry.Register(&GeneratedTool{Name: toolName}, &CompileResult{Success: true})
	if err != nil {
		t.Fatalf("register source-only tool: %v", err)
	}
	if interpreted.BinaryPath != "" || interpreted.Hash != "" {
		t.Fatalf("source-only runtime retained compiled identity: %+v", interpreted)
	}
	if _, err := loop.registry.GeneratedToolIdentity(toolName); err == nil {
		t.Fatal("source-only registration retained a host execution identity")
	}
	for _, path := range []string{binaryPath, binaryPath + ".identity.json"} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("source-only registration left stale compiled artifact %s (stat error %v)", path, err)
		}
	}
	if output, err := loop.ExecuteTool(context.Background(), toolName, "live"); err != nil || output != "source:live" {
		t.Fatalf("explicit interpreted execution = %q, %v; want source:live", output, err)
	}

	compiledCfg := cfg
	compiledCfg.ExecutionMode = ExecuteCompiled
	reloaded := NewOuroborosLoop(&MockLLMClient{}, compiledCfg)
	if _, exists := reloaded.registry.Get(toolName); exists {
		t.Fatal("Restore resurrected the revoked compiled runtime")
	}
	if _, err := reloaded.registry.GeneratedToolIdentity(toolName); err == nil {
		t.Fatal("Restore resurrected the revoked host execution identity")
	}
	if _, err := reloaded.ExecuteTool(context.Background(), toolName, "after-reload"); err == nil {
		t.Fatal("default compiled execution admitted a revoked tool after Restore")
	}
}

func TestRuntimeRegistrySourceOnlyRegistrationRevokesPersistedIdentityWithoutRestore(t *testing.T) {
	workspace := t.TempDir()
	toolsDir := filepath.Join(workspace, "tools")
	compiledDir := filepath.Join(workspace, "compiled")
	if err := os.MkdirAll(toolsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(compiledDir, 0755); err != nil {
		t.Fatal(err)
	}
	toolName := "stale_tool"
	if err := os.WriteFile(filepath.Join(toolsDir, toolName+".go"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	binary, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	binaryPath := filepath.Join(compiledDir, toolName)
	if runtime.GOOS == "windows" {
		binaryPath += ".exe"
	}
	if err := os.WriteFile(binaryPath, binary, 0700); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(binary)
	identity := types.GeneratedToolIdentity{
		Name: toolName, BinaryPath: binaryPath, BinaryHash: hex.EncodeToString(digest[:]),
		Protocol: types.GeneratedStdinV1, Workspace: workspace,
	}
	encodedIdentity, err := json.Marshal(identity)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binaryPath+".identity.json", encodedIdentity, 0600); err != nil {
		t.Fatal(err)
	}

	// No Restore call populated the registry maps: the configured compiled
	// directory still makes source-only registration a durable revocation.
	registry := NewRuntimeRegistry(workspace, compiledDir)
	if _, err := registry.Register(&GeneratedTool{Name: toolName}, &CompileResult{Success: true}); err != nil {
		t.Fatalf("register source-only tool without Restore: %v", err)
	}
	for _, path := range []string{binaryPath, binaryPath + ".identity.json"} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("source-only registration left persisted artifact %s (stat error %v)", path, err)
		}
	}

	restored := NewRuntimeRegistry(workspace, compiledDir)
	restored.Restore(toolsDir, compiledDir)
	if _, exists := restored.Get(toolName); exists {
		t.Fatal("Restore found a compiled tool revoked without a prior in-memory registration")
	}
}

func TestRuntimeRegistrySourceOnlyRegistrationRejectsTraversalWithoutChangingFiles(t *testing.T) {
	workspace := t.TempDir()
	compiledDir := filepath.Join(workspace, "compiled")
	if err := os.MkdirAll(compiledDir, 0755); err != nil {
		t.Fatal(err)
	}
	outsideBinary := filepath.Join(workspace, "outside_tool")
	outsideSidecar := outsideBinary + ".identity.json"
	if err := os.WriteFile(outsideBinary, []byte("protected binary"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outsideSidecar, []byte("protected identity"), 0600); err != nil {
		t.Fatal(err)
	}
	registry := NewRuntimeRegistry(workspace, compiledDir)
	registry.tools["../outside_tool"] = &RuntimeTool{Name: "../outside_tool", BinaryPath: outsideBinary}
	registry.identities["../outside_tool"] = types.GeneratedToolIdentity{Name: "../outside_tool", BinaryPath: outsideBinary}
	if _, err := registry.Register(&GeneratedTool{Name: "../outside_tool"}, &CompileResult{Success: true}); err == nil {
		t.Fatal("source-only registration accepted a traversal name")
	}
	if data, err := os.ReadFile(outsideBinary); err != nil || string(data) != "protected binary" {
		t.Fatalf("traversal registration changed outside binary: %q, %v", data, err)
	}
	if data, err := os.ReadFile(outsideSidecar); err != nil || string(data) != "protected identity" {
		t.Fatalf("traversal registration changed outside sidecar: %q, %v", data, err)
	}
}

func TestRuntimeRegistrySourceOnlyRegistrationRejectsSymlinkArtifact(t *testing.T) {
	workspace := t.TempDir()
	compiledDir := filepath.Join(workspace, "compiled")
	if err := os.MkdirAll(compiledDir, 0755); err != nil {
		t.Fatal(err)
	}
	outsideBinary := filepath.Join(workspace, "protected_binary")
	if err := os.WriteFile(outsideBinary, []byte("protected"), 0700); err != nil {
		t.Fatal(err)
	}
	toolName := "symlink_tool"
	artifact := filepath.Join(compiledDir, toolName)
	if err := os.Symlink(outsideBinary, artifact); err != nil {
		t.Skipf("symlink fixture unavailable: %v", err)
	}
	registry := NewRuntimeRegistry(workspace, compiledDir)
	if _, err := registry.Register(&GeneratedTool{Name: toolName}, &CompileResult{Success: true}); err == nil {
		t.Fatal("source-only registration accepted a symlink compiled artifact")
	}
	if info, err := os.Lstat(artifact); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("source-only registration changed the symlink artifact: info=%v err=%v", info, err)
	}
	if data, err := os.ReadFile(outsideBinary); err != nil || string(data) != "protected" {
		t.Fatalf("source-only registration changed symlink target: %q, %v", data, err)
	}
}

func TestGeneratedRuntimeCancellationJoinsProcess(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	request := runtimeFixtureRequest(t, map[string]any{"mode": "block", "admission": listener.Addr().String()}, types.GeneratedStdinV1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan types.GeneratedToolReceipt, 1)
	processDone, acceptDone := make(chan struct{}), make(chan struct{})
	go func() { defer close(processDone); result <- RunGeneratedBinary(ctx, request) }()
	accepted := make(chan net.Conn, 1)
	go func() { defer close(acceptDone); connection, _ := listener.Accept(); accepted <- connection }()
	var connection net.Conn
	defer func() {
		cancel()
		listener.Close()
		<-processDone
		<-acceptDone
		if connection != nil {
			connection.Close()
		} else {
			select {
			case remaining := <-accepted:
				if remaining != nil {
					remaining.Close()
				}
			default:
			}
		}
	}()
	select {
	case connection = <-accepted:
	case <-time.After(15 * time.Second):
		t.Fatal("process did not enter fixture")
	}
	if connection == nil {
		t.Fatal("no child connection")
	}
	if err := connection.SetReadDeadline(time.Now().Add(15 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var marker [1]byte
	if _, err := connection.Read(marker[:]); err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case receipt := <-result:
		if !receipt.ProcessStarted || !errors.Is(receipt.BackendError, context.Canceled) {
			t.Fatalf("cancel receipt: %+v", receipt)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("canceled process was not joined")
	}
	if _, err := connection.Read(marker[:]); !errors.Is(err, io.EOF) && !(runtime.GOOS == "windows" && errors.Is(err, syscall.Errno(10054))) {
		t.Fatalf("child connection was not joined: %v", err)
	}
}
