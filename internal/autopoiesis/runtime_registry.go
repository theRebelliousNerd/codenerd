package autopoiesis

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// =============================================================================
// RUNTIME REGISTRY - THE MENAGERIE
// =============================================================================
// Manages registered tools available for runtime execution.

// RuntimeRegistry manages registered tools
type RuntimeRegistry struct {
	mu    sync.RWMutex
	tools map[string]*RuntimeTool
}

// NewRuntimeRegistry creates a new registry
func NewRuntimeRegistry() *RuntimeRegistry {
	return &RuntimeRegistry{
		tools: make(map[string]*RuntimeTool),
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
		tools = append(tools, tool)
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
	}
}

// Execute runs the tool with the given input
func (rt *RuntimeTool) Execute(ctx context.Context, input string) (string, error) {
	cleanPath := filepath.Clean(rt.BinaryPath)
	if !filepath.IsAbs(cleanPath) {
		return "", fmt.Errorf("tool binary path must be absolute for security: %s", cleanPath)
	}

	// Verify binary still exists
	if _, err := os.Stat(cleanPath); os.IsNotExist(err) {
		return "", fmt.Errorf("tool binary not found: %s", cleanPath)
	}

	// Prepare input
	inputJSON, err := json.Marshal(map[string]string{"input": input})
	if err != nil {
		return "", fmt.Errorf("failed to marshal input: %w", err)
	}

	// Execute the tool binary
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd := exec.CommandContext(ctx, cleanPath)
	cmd.Stdin = strings.NewReader(string(inputJSON))
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	cmd.Env = toolExecutionEnv()

	err = cmd.Run()
	if err != nil {
		stderrStr := strings.TrimSpace(stderrBuf.String())
		if stderrStr != "" {
			return "", fmt.Errorf("tool execution failed (stderr: %s): %w", stderrStr, err)
		}
		return "", fmt.Errorf("tool execution failed: %w", err)
	}
	output := stdoutBuf.Bytes()

	// Parse output
	var result struct {
		Output string `json:"output"`
		Error  string `json:"error,omitempty"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return "", fmt.Errorf("failed to parse tool output: %w", err)
	}

	if result.Error != "" {
		return result.Output, fmt.Errorf("tool error: %s", result.Error)
	}

	atomic.AddInt64(&rt.ExecuteCount, 1)
	return result.Output, nil
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
