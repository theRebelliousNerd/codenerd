//go:build integration
package core_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/core"
	"github.com/stretchr/testify/require"
)

func TestVirtualStore_Integration_FileOperations(t *testing.T) {
	// Setup
	tempDir := t.TempDir()

	kernel, err := core.NewRealKernel()
	require.NoError(t, err, "Failed to create real kernel")

	config := core.DefaultVirtualStoreConfig()
	config.WorkingDir = tempDir
	vs := core.NewVirtualStoreWithConfig(nil, config)
	vs.SetKernel(kernel)
	vs.DisableBootGuard()

	// Test WriteFile with absolute path
	fileName := "integration_test.txt"
	content := "Hello Integration World"
	filePath := filepath.Join(tempDir, fileName)

	writeReq := core.ActionRequest{
		ActionID: "act_write_1",
		Type:     core.ActionWriteFile,
		Target:   filePath,
		Payload: map[string]interface{}{
			"content": content,
		},
		SessionID: "test-session-1",
	}

	ctx := context.Background()

	writeFact := core.Fact{
		Predicate: "next_action",
		Args: []interface{}{
			writeReq.ActionID,
			string(writeReq.Type),
			writeReq.Target,
			writeReq.Payload,
		},
	}

	output, err := vs.RouteAction(ctx, writeFact)
	require.NoError(t, err, "RouteAction(WriteFile) failed")
	require.Contains(t, output, "Written")

	readBytes, err := os.ReadFile(filePath)
	require.NoError(t, err, "Failed to read file from disk")
	require.Equal(t, content, string(readBytes))

	// Verify Kernel Facts: "file_written"
	facts, err := kernel.Query("file_written")
	require.NoError(t, err)
	found := false
	for _, f := range facts {
		if len(f.Args) == 4 && f.Args[0] == filePath {
			found = true
			break
		}
	}
	require.True(t, found, "file_written fact not found in kernel")

	// Verify Kernel Facts: "execution_result"
	results, err := kernel.Query("execution_result")
	require.NoError(t, err)
	foundResult := false
	for _, f := range results {
		if len(f.Args) >= 4 && f.Args[0] == "act_write_1" {
			success := f.Args[3]
			// Check for boolean true or Mangle atom /true
			require.Contains(t, []interface{}{true, "/true"}, success)
			foundResult = true
			break
		}
	}
	require.True(t, foundResult, "execution_result fact not found")
}

func TestVirtualStore_Integration_ExecCmd(t *testing.T) {
	// Setup
	tempDir := t.TempDir()

	kernel, err := core.NewRealKernel()
	require.NoError(t, err)

	config := core.DefaultVirtualStoreConfig()
	config.WorkingDir = tempDir
	vs := core.NewVirtualStoreWithConfig(nil, config)
	vs.SetKernel(kernel)
	vs.DisableBootGuard()

	// Test ExecCmd
	cmd := "echo 'integration exec'"

	execReq := core.ActionRequest{
		ActionID: "act_exec_1",
		Type:     core.ActionExecCmd,
		Target:   cmd,
		Payload:  map[string]interface{}{},
		SessionID: "test-session-2",
	}

	execFact := core.Fact{
		Predicate: "next_action",
		Args: []interface{}{
			execReq.ActionID,
			string(execReq.Type),
			execReq.Target,
		},
	}

	output, err := vs.RouteAction(context.Background(), execFact)
	require.NoError(t, err, "RouteAction(ExecCmd) failed")
	require.Contains(t, output, "integration exec")

	// Verify execution_result
	results, err := kernel.Query("execution_result")
	require.NoError(t, err)

	found := false
	for _, f := range results {
		if len(f.Args) >= 4 && f.Args[0] == "act_exec_1" {
			found = true
			break
		}
	}
	require.True(t, found, "execution_result fact not found for exec_cmd")

	// Verify execution_success (Modern Executor)
	successFacts, err := kernel.Query("execution_success")
	require.NoError(t, err)
	foundSuccess := false
	for _, f := range successFacts {
		// execution_success(RequestID)
		if len(f.Args) >= 1 && f.Args[0] == "act_exec_1" {
			foundSuccess = true
			break
		}
	}
	require.True(t, foundSuccess, "execution_success fact not found")
}
