package autopoiesis

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"codenerd/internal/atomicfile"
	"codenerd/internal/types"
)

// =============================================================================
// RUNTIME REGISTRY - THE MENAGERIE
// =============================================================================
// Manages registered tools available for runtime execution.

// RuntimeRegistry manages registered tools
type RuntimeRegistry struct {
	mu         sync.RWMutex
	tools      map[string]*RuntimeTool
	identities map[string]types.GeneratedToolIdentity
}

// NewRuntimeRegistry creates a new registry
func NewRuntimeRegistry() *RuntimeRegistry {
	return &RuntimeRegistry{
		tools:      make(map[string]*RuntimeTool),
		identities: make(map[string]types.GeneratedToolIdentity),
	}
}

// Register adds a tool to the registry
func (r *RuntimeRegistry) Register(tool *GeneratedTool, compiled *CompileResult) (*RuntimeTool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	rt := &RuntimeTool{
		Name:         tool.Name,
		Description:  tool.Description,
		BinaryPath:   compiled.OutputPath,
		Hash:         compiled.Hash,
		Schema:       tool.Schema,
		RegisteredAt: time.Now(),
	}
	identity := types.GeneratedToolIdentity{Name: rt.Name, BinaryPath: rt.BinaryPath, BinaryHash: rt.Hash, Protocol: types.GeneratedStdinV1}
	if err := verifyGeneratedBinary(identity); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(identity)
	if err != nil {
		return nil, err
	}
	if err := atomicfile.WriteFile(rt.BinaryPath+".identity.json", encoded, 0600); err != nil {
		return nil, err
	}
	if r.identities == nil {
		r.identities = make(map[string]types.GeneratedToolIdentity)
	}
	r.identities[tool.Name] = identity

	r.tools[tool.Name] = rt
	return rt, nil
}

// Get retrieves a tool by name
func (r *RuntimeRegistry) Get(name string) (*RuntimeTool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, exists := r.tools[name]
	return tool, exists
}

// List returns all registered tools
func (r *RuntimeRegistry) List() []*RuntimeTool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]*RuntimeTool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, &RuntimeTool{Name: tool.Name, Description: tool.Description, BinaryPath: tool.BinaryPath,
			Hash: tool.Hash, Schema: tool.Schema, RegisteredAt: tool.RegisteredAt, ExecuteCount: atomic.LoadInt64(&tool.ExecuteCount)})
	}
	return tools
}

// Restore rebuilds the registry from disk
func (r *RuntimeRegistry) Restore(toolsDir, compiledDir string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// List all binaries in compiled dir
	entries, err := os.ReadDir(compiledDir)
	if err != nil {
		return // Directory might not exist yet
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(entry.Name(), ".identity.json") {
			continue
		}

		name := entry.Name()
		// Strip extension (e.g. .exe on Windows)
		name = strings.TrimSuffix(name, ".exe")

		// Check if source exists
		srcPath := filepath.Join(toolsDir, name+".go")
		if _, err := os.Stat(srcPath); err != nil {
			continue // Orphaned binary
		}

		// Create runtime tool
		binaryPath := filepath.Join(compiledDir, entry.Name())

		// Calculate hash
		hash := ""
		if content, err := os.ReadFile(binaryPath); err == nil {
			h := sha256.Sum256(content)
			hash = hex.EncodeToString(h[:])
		}

		registeredAt := time.Now()
		if info, err := entry.Info(); err == nil {
			registeredAt = info.ModTime()
		}

		description := "Restored from disk"
		if srcBytes, err := os.ReadFile(srcPath); err == nil {
			lines := strings.SplitSeq(string(srcBytes), "\n")
			for line := range lines {
				line = strings.TrimSpace(line)
				if after, ok := strings.CutPrefix(line, "// Description:"); ok {
					description = strings.TrimSpace(after)
					break
				} else if after, ok := strings.CutPrefix(line, "//Description:"); ok {
					description = strings.TrimSpace(after)
					break
				}
				if strings.HasPrefix(line, "package ") {
					break
				}
			}
		}

		rt := &RuntimeTool{
			Name:         name,
			Description:  description,
			BinaryPath:   binaryPath,
			Hash:         hash,
			Schema:       ToolSchema{Name: name}, // Basic schema
			RegisteredAt: registeredAt,
		}

		r.tools[name] = rt
		delete(r.identities, name)
		data, err := os.ReadFile(binaryPath + ".identity.json")
		var identity types.GeneratedToolIdentity
		if err == nil && json.Unmarshal(data, &identity) == nil && identity.Name == name &&
			identity.BinaryPath == binaryPath && verifyGeneratedBinary(identity) == nil {
			if r.identities == nil {
				r.identities = make(map[string]types.GeneratedToolIdentity)
			}
			r.identities[name] = identity
		}
	}
}

// Execute runs the tool with the given input
func (rt *RuntimeTool) Execute(ctx context.Context, input string) (string, error) {
	identity := types.GeneratedToolIdentity{Name: rt.Name, BinaryPath: rt.BinaryPath, BinaryHash: rt.Hash, Protocol: types.GeneratedStdinV1}
	// The legacy wrapper still accepts arbitrary input strings.
	if identity.BinaryHash == "" {
		data, err := os.ReadFile(identity.BinaryPath)
		if err != nil {
			return "", err
		}
		digest := sha256.Sum256(data)
		identity.BinaryHash = hex.EncodeToString(digest[:])
	}
	receipt := executeGeneratedBinary(ctx, identity, input)
	if receipt.BackendError == nil {
		atomic.AddInt64(&rt.ExecuteCount, 1)
	}
	return receipt.Output, receipt.BackendError
}

func (r *RuntimeRegistry) GeneratedToolIdentity(name string) (types.GeneratedToolIdentity, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	identity, exists := r.identities[name]
	if !exists {
		return identity, fmt.Errorf("tool %s has no host protocol identity", name)
	}
	tool := r.tools[name]
	if tool == nil || tool.Name != identity.Name || tool.BinaryPath != identity.BinaryPath || tool.Hash != identity.BinaryHash {
		return identity, fmt.Errorf("runtime registration identity changed")
	}
	return identity, nil
}

func (r *RuntimeRegistry) recordGeneratedSuccess(identity types.GeneratedToolIdentity) {
	if r == nil {
		return
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, ok := r.tools[identity.Name]
	if ok && tool.BinaryPath == identity.BinaryPath && tool.Hash == identity.BinaryHash {
		atomic.AddInt64(&tool.ExecuteCount, 1)
	}
}

func verifyGeneratedBinary(identity types.GeneratedToolIdentity) error {
	if !filepath.IsAbs(identity.BinaryPath) {
		return fmt.Errorf("tool binary must be absolute")
	}
	if identity.Protocol != types.GeneratedStdinV1 && identity.Protocol != types.LegacyArgvV1 {
		return fmt.Errorf("unknown tool protocol %q", identity.Protocol)
	}
	data, err := os.ReadFile(identity.BinaryPath)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(data)
	if identity.BinaryHash != hex.EncodeToString(digest[:]) {
		return fmt.Errorf("registered binary digest changed")
	}
	return nil
}

type generatedOutputBuffer struct {
	bytes.Buffer
	truncated bool
}

func (b *generatedOutputBuffer) Write(data []byte) (int, error) {
	const maximum = 10 * 1024 * 1024
	length := len(data)
	remaining := maximum - b.Len()
	if len(data) > remaining {
		data = data[:remaining]
		b.truncated = true
	}
	_, _ = b.Buffer.Write(data)
	return length, nil
}

// RunGeneratedBinary launches only the immutable, host-resolved registration.
// Start and Wait are separate so a launch refusal cannot masquerade as execution.
func RunGeneratedBinary(ctx context.Context, request types.GeneratedToolRequest) types.GeneratedToolReceipt {
	if err := request.Validate(); err != nil {
		return types.GeneratedToolReceipt{Request: request, BackendError: err, ExitCode: -1}
	}
	receipt := executeGeneratedBinary(ctx, request.Tool, request.CanonicalArgs)
	receipt.Request = request
	return receipt
}

func executeGeneratedBinary(ctx context.Context, identity types.GeneratedToolIdentity, input string) types.GeneratedToolReceipt {
	receipt := types.GeneratedToolReceipt{Attempted: true, ExitCode: -1}
	if err := ctx.Err(); err != nil {
		receipt.BackendError = err
		return receipt
	}
	if err := verifyGeneratedBinary(identity); err != nil {
		receipt.BackendError = err
		return receipt
	}
	arguments := []string{}
	if identity.Protocol == types.LegacyArgvV1 {
		arguments = []string{input}
	}
	cmd := exec.CommandContext(ctx, identity.BinaryPath, arguments...)
	if identity.Protocol == types.GeneratedStdinV1 {
		encoded, err := json.Marshal(map[string]string{"input": input})
		if err != nil {
			receipt.BackendError = err
			return receipt
		}
		cmd.Stdin = bytes.NewReader(encoded)
	}
	cmd.Dir = identity.Workspace
	cmd.Env = toolExecutionEnv()
	cmd.WaitDelay = 2 * time.Second
	var stdout, stderr generatedOutputBuffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	receipt.StartedAt = time.Now()
	if err := cmd.Start(); err != nil {
		receipt.BackendError = errors.Join(ctx.Err(), err)
		return receipt
	}
	receipt.ProcessStarted = true
	waitErr := cmd.Wait()
	receipt.Duration = time.Since(receipt.StartedAt)
	receipt.ExitCode = cmd.ProcessState.ExitCode()
	receipt.Stdout, receipt.Stderr = stdout.String(), stderr.String()
	receipt.OutputTruncated = stdout.truncated || stderr.truncated
	receipt.Output = receipt.Stdout
	if identity.Protocol == types.GeneratedStdinV1 {
		var envelope struct {
			Output json.RawMessage `json:"output"`
			Error  string          `json:"error"`
		}
		if err := json.Unmarshal([]byte(receipt.Stdout), &envelope); err != nil {
			receipt.BackendError = fmt.Errorf("invalid generated output envelope: %w", err)
		} else if envelope.Output == nil && envelope.Error == "" {
			receipt.BackendError = fmt.Errorf("generated output envelope has neither output nor error")
		} else {
			receipt.Output = decodeToolOutput(envelope.Output)
			if envelope.Error != "" {
				receipt.BackendError = fmt.Errorf("tool error: %s", envelope.Error)
			}
		}
	} else if receipt.Stderr != "" {
		receipt.Output += receipt.Stderr
	}
	if waitErr != nil {
		receipt.BackendError = errors.Join(receipt.BackendError, ctx.Err(), waitErr)
	}
	if receipt.OutputTruncated {
		receipt.BackendError = errors.Join(receipt.BackendError, fmt.Errorf("generated output exceeds capture bound"))
	}
	receipt.PartialOutput = receipt.BackendError != nil && (receipt.Stdout != "" || receipt.Stderr != "")
	return receipt
}

// decodeToolOutput renders the wrapper's raw output field as text.
//
// A JSON string is unquoted (the wrapper marshals non-JSON returns, so this is
// the common case); anything else — object, array, number, bool — is returned
// as its JSON source, which is what the tool produced in the first place.
func decodeToolOutput(raw json.RawMessage) string {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return ""
	}
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return asString
	}
	return trimmed
}

// toolExecutionEnv returns a minimal runtime environment for executing generated tools.
// This limits accidental leakage of host secrets while preserving process launch stability.
func toolExecutionEnv() []string {
	if runtime.GOOS != "windows" {
		return []string{}
	}

	keys := []string{"SYSTEMROOT", "WINDIR", "TEMP", "TMP", "ComSpec"}
	env := make([]string, 0, len(keys))
	for _, key := range keys {
		if val := os.Getenv(key); val != "" {
			env = append(env, key+"="+val)
		}
	}
	return env
}
