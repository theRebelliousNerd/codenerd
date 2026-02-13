//go:build integration

package core_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"codenerd/internal/core"
	"github.com/stretchr/testify/require"
)

func makeNextActionFact(req core.ActionRequest) core.Fact {
	payload := req.Payload
	if payload == nil {
		payload = map[string]interface{}{}
	}

	return core.Fact{
		Predicate: "next_action",
		Args: []interface{}{
			req.ActionID,
			string(req.Type),
			req.Target,
			payload,
		},
	}
}

func newTestVirtualStoreWithKernel(t *testing.T, workDir string) (*core.VirtualStore, *core.RealKernel) {
	t.Helper()

	kernel, err := core.NewRealKernel()
	require.NoError(t, err, "Failed to create real kernel")

	cfg := core.DefaultVirtualStoreConfig()
	cfg.WorkingDir = workDir

	vs := core.NewVirtualStoreWithConfig(nil, cfg)
	vs.SetKernel(kernel)
	vs.DisableBootGuard()

	return vs, kernel
}

func TestVirtualStore_Integration_WriteReadDirectory_AndKernelFacts(t *testing.T) {
	tempDir := t.TempDir()
	vs, kernel := newTestVirtualStoreWithKernel(t, tempDir)

	ctx := context.Background()

	fileName := "integration_test.txt"
	fullPath := filepath.Join(tempDir, fileName)
	content := "Hello Integration World"

	t.Run("write_file", func(t *testing.T) {
		req := core.ActionRequest{
			ActionID: "act_write_1",
			Type:     core.ActionWriteFile,
			Target:   fileName, // Relative path exercises resolvePath(workDir + target).
			Payload: map[string]interface{}{
				"content": content,
			},
		}

		out, err := vs.RouteAction(ctx, makeNextActionFact(req))
		require.NoError(t, err, "RouteAction(write_file) failed")
		require.Contains(t, out, "Written")

		readBytes, err := os.ReadFile(fullPath)
		require.NoError(t, err, "Failed to read file from disk")
		require.Equal(t, content, string(readBytes))

		facts, err := kernel.Query("file_written")
		require.NoError(t, err)

		found := false
		for _, f := range facts {
			if len(f.Args) >= 1 && f.Args[0] == fullPath {
				found = true
				break
			}
		}
		require.True(t, found, "file_written fact not found in kernel")
	})

	t.Run("read_file", func(t *testing.T) {
		req := core.ActionRequest{
			ActionID: "act_read_1",
			Type:     core.ActionReadFile,
			Target:   fileName,
		}

		out, err := vs.RouteAction(ctx, makeNextActionFact(req))
		require.NoError(t, err, "RouteAction(read_file) failed")
		require.Equal(t, content, out)

		readFacts, err := kernel.Query("file_read")
		require.NoError(t, err)

		found := false
		for _, f := range readFacts {
			if len(f.Args) >= 1 && f.Args[0] == fullPath {
				found = true
				break
			}
		}
		require.True(t, found, "file_read fact not found in kernel")
	})

	t.Run("read_directory", func(t *testing.T) {
		req := core.ActionRequest{
			ActionID: "act_dir_1",
			Type:     core.ActionReadFile,
			Target:   ".",
		}

		out, err := vs.RouteAction(ctx, makeNextActionFact(req))
		require.NoError(t, err, "RouteAction(read_directory) failed")

		require.Contains(t, out, "Files:")
		require.Contains(t, out, fileName)

		dirFacts, err := kernel.Query("dir_read")
		require.NoError(t, err)

		found := false
		for _, f := range dirFacts {
			if len(f.Args) >= 1 && f.Args[0] == tempDir {
				found = true
				break
			}
		}
		require.True(t, found, "dir_read fact not found in kernel")
	})
}

func TestVirtualStore_Integration_ExecCmd_EmitsAuditFacts(t *testing.T) {
	tempDir := t.TempDir()
	vs, kernel := newTestVirtualStoreWithKernel(t, tempDir)

	cmdText := "echo integration exec"

	var binary string
	var args []interface{}
	if runtime.GOOS == "windows" {
		binary = "cmd"
		args = []interface{}{"/c", cmdText}
	} else {
		binary = "sh"
		args = []interface{}{"-c", cmdText}
	}

	req := core.ActionRequest{
		ActionID: "act_exec_1",
		Type:     core.ActionExecCmd,
		Target:   cmdText,
		Payload: map[string]interface{}{
			"binary": binary,
			"args":   args,
		},
	}

	out, err := vs.RouteAction(context.Background(), makeNextActionFact(req))
	require.NoError(t, err, "RouteAction(exec_cmd) failed")
	require.Contains(t, out, "integration exec")

	// Verify execution_result emitted by VirtualStore itself.
	results, err := kernel.Query("execution_result")
	require.NoError(t, err)

	foundResult := false
	for _, f := range results {
		if len(f.Args) >= 4 && f.Args[0] == "act_exec_1" {
			foundResult = true
			break
		}
	}
	require.True(t, foundResult, "execution_result fact not found for exec_cmd")

	// Verify execution_success emitted by the modern executor audit layer.
	successFacts, err := kernel.Query("execution_success")
	require.NoError(t, err)

	foundSuccess := false
	for _, f := range successFacts {
		if len(f.Args) >= 1 && f.Args[0] == "act_exec_1" {
			foundSuccess = true
			break
		}
	}
	require.True(t, foundSuccess, "execution_success fact not found")
}
