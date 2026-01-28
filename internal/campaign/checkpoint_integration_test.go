//go:build integration

package campaign_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"codenerd/internal/campaign"
	"codenerd/internal/tactile"
	"github.com/stretchr/testify/require"
)

func setupGoProject(t *testing.T, dir string, valid bool, passingTests bool) {
	t.Helper()

	// 1. Create go.mod
	modContent := `module codenerd-test

go 1.21
`
	err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(modContent), 0644)
	require.NoError(t, err)

	// 2. Create main.go
	var mainContent string
	if valid {
		mainContent = `package main

import "fmt"

func Add(a, b int) int {
	return a + b
}

func main() {
	fmt.Println(Add(1, 2))
}
`
	} else {
		// Syntax error
		mainContent = `package main

func Add(a, b int) int {
	return a + b // Missing closing brace
`
	}
	err = os.WriteFile(filepath.Join(dir, "main.go"), []byte(mainContent), 0644)
	require.NoError(t, err)

	// 3. Create main_test.go
	var testContent string
	if passingTests {
		testContent = `package main

import "testing"

func TestAdd(t *testing.T) {
	if got := Add(1, 2); got != 3 {
		t.Errorf("Add(1, 2) = %d; want 3", got)
	}
}
`
	} else {
		testContent = `package main

import "testing"

func TestAdd(t *testing.T) {
	if got := Add(1, 2); got != 5 { // Intentionally fail
		t.Errorf("Add(1, 2) = %d; want 5", got)
	}
}
`
	}
	err = os.WriteFile(filepath.Join(dir, "main_test.go"), []byte(testContent), 0644)
	require.NoError(t, err)
}

func TestCheckpointRunner_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Use a real executor
	executor := tactile.NewDirectExecutor()

	// Common context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	t.Run("VerifyBuilds_Success", func(t *testing.T) {
		workspace := t.TempDir()
		setupGoProject(t, workspace, true, true)

		runner := campaign.NewCheckpointRunner(executor, nil, workspace)
		passed, details, err := runner.Run(ctx, nil, campaign.VerifyBuilds)

		require.NoError(t, err)
		require.True(t, passed, "Build should pass")
		require.Contains(t, details, "Build succeeded")
	})

	t.Run("VerifyBuilds_Failure", func(t *testing.T) {
		workspace := t.TempDir()
		setupGoProject(t, workspace, false, true)

		runner := campaign.NewCheckpointRunner(executor, nil, workspace)
		passed, details, err := runner.Run(ctx, nil, campaign.VerifyBuilds)

		require.NoError(t, err) // It returns no error, just passed=false
		require.False(t, passed, "Build should fail")
		require.Contains(t, details, "Build failed")
	})

	t.Run("VerifyTestsPass_Success", func(t *testing.T) {
		workspace := t.TempDir()
		setupGoProject(t, workspace, true, true)

		runner := campaign.NewCheckpointRunner(executor, nil, workspace)
		passed, details, err := runner.Run(ctx, nil, campaign.VerifyTestsPass)

		require.NoError(t, err)
		require.True(t, passed, "Tests should pass")
		require.Contains(t, details, "All 1 tests passed")
	})

	t.Run("VerifyTestsPass_Failure", func(t *testing.T) {
		workspace := t.TempDir()
		setupGoProject(t, workspace, true, false)

		runner := campaign.NewCheckpointRunner(executor, nil, workspace)
		passed, details, err := runner.Run(ctx, nil, campaign.VerifyTestsPass)

		require.NoError(t, err)
		require.False(t, passed, "Tests should fail")
		// Output usually says "Tests: X passed, Y failed"
		require.Contains(t, details, "failed")
	})
}
