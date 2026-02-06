//go:build integration
package core_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/tactile"
	"github.com/stretchr/testify/suite"
)

type VirtualStoreIntegrationSuite struct {
	suite.Suite
	vs      *core.VirtualStore
	tempDir string
	ctx     context.Context
}

func (s *VirtualStoreIntegrationSuite) SetupTest() {
	s.ctx = context.Background()
	s.tempDir = s.T().TempDir()

	// Use DirectExecutor for real execution
	executor := tactile.NewDirectExecutor()

	config := core.VirtualStoreConfig{
		WorkingDir:      s.tempDir,
		AllowedEnvVars:  []string{"PATH", "HOME"},
		AllowedBinaries: []string{"bash", "sh", "echo", "ls", "cat", "rm", "mkdir", "touch", "go", "git"},
	}

	s.vs = core.NewVirtualStoreWithConfig(executor, config)
	// Disable boot guard to allow actions immediately
	s.vs.DisableBootGuard()
}

func (s *VirtualStoreIntegrationSuite) toFact(req core.ActionRequest) core.Fact {
	args := []interface{}{
		req.ActionID,
		string(req.Type),
		req.Target,
		req.Payload,
	}
	return core.Fact{
		Predicate: "next_action",
		Args:      args,
	}
}

func (s *VirtualStoreIntegrationSuite) TestFileOperations() {
	filename := "testfile.txt"
	content := "Hello, Integration World!"
	fullPath := filepath.Join(s.tempDir, filename)

	// 1. Write File
	reqWrite := core.ActionRequest{
		ActionID: "test-write-1",
		Type:     core.ActionWriteFile,
		Target:   filename,
		Payload: map[string]interface{}{
			"content": content,
		},
	}
	output, err := s.vs.RouteAction(s.ctx, s.toFact(reqWrite))
	s.Require().NoError(err)
	s.Require().Contains(output, "Written")

	// Verify on disk
	data, err := os.ReadFile(fullPath)
	s.Require().NoError(err)
	s.Require().Equal(content, string(data))

	// 2. Read File
	reqRead := core.ActionRequest{
		ActionID: "test-read-1",
		Type:     core.ActionReadFile,
		Target:   filename,
	}
	output, err = s.vs.RouteAction(s.ctx, s.toFact(reqRead))
	s.Require().NoError(err)
	s.Require().Equal(content, output)

	// 3. Delete File
	reqDelete := core.ActionRequest{
		ActionID: "test-delete-1",
		Type:     core.ActionDeleteFile,
		Target:   filename,
		Payload: map[string]interface{}{
			"confirmed": true,
		},
	}
	output, err = s.vs.RouteAction(s.ctx, s.toFact(reqDelete))
	s.Require().NoError(err)
	s.Require().Contains(output, "Deleted")

	// Verify gone from disk
	_, err = os.Stat(fullPath)
	s.Require().True(os.IsNotExist(err), "File should not exist")
}

func (s *VirtualStoreIntegrationSuite) TestShellExecution() {
	// Execute echo
	req := core.ActionRequest{
		ActionID: "test-exec-1",
		Type:     core.ActionExecCmd,
		Target:   "echo 'integration test'",
	}
	output, err := s.vs.RouteAction(s.ctx, s.toFact(req))
	s.Require().NoError(err)
	s.Require().Contains(output, "integration test")
}

func (s *VirtualStoreIntegrationSuite) TestDirectoryListing() {
	// Setup: Create a file
	filename := "listme.txt"
	err := os.WriteFile(filepath.Join(s.tempDir, filename), []byte("content"), 0644)
	s.Require().NoError(err)

	// Action: Read Directory
	req := core.ActionRequest{
		ActionID: "test-list-1",
		Type:     core.ActionReadFile,
		Target:   ".", // Current dir (tempDir)
	}
	output, err := s.vs.RouteAction(s.ctx, s.toFact(req))
	s.Require().NoError(err)

	// Verify output contains file info
	s.Require().Contains(output, "Files:")
	s.Require().Contains(output, filename)
}

func TestVirtualStoreIntegrationSuite(t *testing.T) {
	suite.Run(t, new(VirtualStoreIntegrationSuite))
}
