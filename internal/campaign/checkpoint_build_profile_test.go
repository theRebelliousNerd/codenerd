package campaign

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"codenerd/internal/tactile"
)

// The tests checkpoint must run under the workspace's configured build
// profile. checkpoint.go runTestsCheckpoint calls build.GetBuildEnv(nil, ...)
// with a nil config, so build env_vars and go_flags from the workspace's
// .nerd/config.json never reach the go test subprocess.

// buildProfileCaptureExecutor records the command the checkpoint executes and
// reports a successful, empty test run.
type buildProfileCaptureExecutor struct {
	captured *tactile.Command
}

func (f *buildProfileCaptureExecutor) Execute(ctx context.Context, cmd tactile.Command) (*tactile.ExecutionResult, error) {
	c := cmd
	f.captured = &c
	return &tactile.ExecutionResult{Success: true, ExitCode: 0}, nil
}

func (f *buildProfileCaptureExecutor) Capabilities() tactile.ExecutorCapabilities {
	return tactile.ExecutorCapabilities{}
}

func (f *buildProfileCaptureExecutor) Validate(cmd tactile.Command) error { return nil }

func TestRunTestsCheckpoint_UsesWorkspaceBuildProfile(t *testing.T) {
	tests := []struct {
		name       string
		configJSON string
		wantEnv    string
		wantArg    string
	}{
		{
			name:       "workspace build env var reaches the test command",
			configJSON: `{"build":{"env_vars":{"CHECKPOINT_PROFILE_PROBE":"profile-applied"}}}`,
			wantEnv:    "CHECKPOINT_PROFILE_PROBE=profile-applied",
		},
		{
			name:       "workspace go flags reach the test argv",
			configJSON: `{"build":{"go_flags":["-tags=checkpointexample"]}}`,
			wantArg:    "-tags=checkpointexample",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ws := t.TempDir()
			if err := os.MkdirAll(filepath.Join(ws, ".nerd"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(ws, ".nerd", "config.json"), []byte(tt.configJSON), 0o600); err != nil {
				t.Fatal(err)
			}

			fake := &buildProfileCaptureExecutor{}
			cr := &CheckpointRunner{executor: fake, workspace: ws}
			if _, _, err := cr.runTestsCheckpoint(context.Background()); err != nil {
				t.Fatalf("runTestsCheckpoint = %v", err)
			}
			if fake.captured == nil {
				t.Fatal("checkpoint ran no command; want the go test invocation")
			}
			if tt.wantEnv != "" && !slices.Contains(fake.captured.Environment, tt.wantEnv) {
				t.Errorf("test command environment lacks %q; the workspace build profile was not applied", tt.wantEnv)
			}
			if tt.wantArg != "" && !slices.Contains(fake.captured.Arguments, tt.wantArg) {
				t.Errorf("test command argv %q lacks %q; the workspace go flags were not applied",
					fake.captured.Arguments, tt.wantArg)
			}
		})
	}
}
