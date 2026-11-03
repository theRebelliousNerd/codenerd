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

// The Go checkpoint must execute with the workspace's configured build profile,
// not the nil-config environment used for non-Go commands.
func TestRunTestsCheckpoint_GoCommandAppliesConfiguredProfile(t *testing.T) {
	tests := []struct {
		name       string
		configJSON string
		wantEnv    string
		wantFlag   string
	}{
		{
			name:       "configured environment reaches Go test",
			configJSON: `{"build":{"env_vars":{"CHECKPOINT_GO_PROFILE":"enabled"}}}`,
			wantEnv:    "CHECKPOINT_GO_PROFILE=enabled",
		},
		{
			name:       "configured Go flag reaches Go test",
			configJSON: `{"build":{"go_flags":["-tags=checkpointprofile"]}}`,
			wantFlag:   "-tags=checkpointprofile",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ws := t.TempDir()
			if err := os.WriteFile(filepath.Join(ws, "go.mod"), []byte("module example.com/checkpoint\n\ngo 1.22\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Join(ws, ".nerd"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(ws, ".nerd", "config.json"), []byte(tt.configJSON), 0o600); err != nil {
				t.Fatal(err)
			}
			fake := &buildProfileCaptureExecutor{}
			cr := &CheckpointRunner{executor: fake, workspace: ws}
			if _, _, err := cr.runTestsCheckpoint(context.Background()); err != nil {
				t.Fatalf("runTestsCheckpoint: %v", err)
			}
			if fake.captured == nil {
				t.Fatal("checkpoint ran no command")
			}
			if fake.captured.Binary != "go" || len(fake.captured.Arguments) == 0 || fake.captured.Arguments[0] != "test" {
				t.Fatalf("checkpoint command = %q %q, want go test", fake.captured.Binary, fake.captured.Arguments)
			}
			if tt.wantEnv != "" && !slices.Contains(fake.captured.Environment, tt.wantEnv) {
				t.Errorf("Go test environment lacks configured entry %q", tt.wantEnv)
			}
			if tt.wantFlag != "" && !slices.Contains(fake.captured.Arguments, tt.wantFlag) {
				t.Errorf("Go test arguments %q lack configured flag %q", fake.captured.Arguments, tt.wantFlag)
			}
		})
	}
}

func TestRunTestsCheckpoint_NonGoBuildEnvironment(t *testing.T) {
	tests := []struct {
		name       string
		configJSON string
		headers    bool
	}{
		{
			name:    "npm receives detected workspace headers",
			headers: true,
		},
		{
			name:       "nil user config ignores workspace settings but detects headers",
			configJSON: `{"build":{"env_vars":{"CHECKPOINT_NON_GO_PROBE":"profile-applied"},"go_flags":["-tags=checkpoint_non_go"]}}`,
			headers:    true,
		},
		{
			name: "npm without workspace headers",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("CGO_CFLAGS", "")
			ws := t.TempDir()
			if err := os.WriteFile(filepath.Join(ws, "package.json"), []byte(`{"name":"checkpoint-test"}`), 0o600); err != nil {
				t.Fatal(err)
			}
			if tt.headers {
				if err := os.MkdirAll(filepath.Join(ws, "sqlite_headers"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if tt.configJSON != "" {
				if err := os.MkdirAll(filepath.Join(ws, ".nerd"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(ws, ".nerd", "config.json"), []byte(tt.configJSON), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			fake := &buildProfileCaptureExecutor{}
			cr := &CheckpointRunner{executor: fake, workspace: ws}
			if _, _, err := cr.runTestsCheckpoint(context.Background()); err != nil {
				t.Fatalf("runTestsCheckpoint: %v", err)
			}
			if fake.captured == nil {
				t.Fatal("checkpoint ran no command")
			}
			if fake.captured.Binary != "npm" {
				t.Fatalf("binary = %q, want npm", fake.captured.Binary)
			}
			if !slices.Contains(fake.captured.Arguments, "test") {
				t.Errorf("npm arguments = %q, want test", fake.captured.Arguments)
			}
			if slices.Contains(fake.captured.Arguments, "-tags=checkpoint_non_go") {
				t.Errorf("Go-only flag leaked into npm arguments: %q", fake.captured.Arguments)
			}
			if slices.Contains(fake.captured.Environment, "CHECKPOINT_NON_GO_PROBE=profile-applied") {
				t.Error("npm environment unexpectedly contains the nil user config's probe")
			}
			wantHeaders := "CGO_CFLAGS=-I" + filepath.Join(ws, "sqlite_headers")
			if got := slices.Contains(fake.captured.Environment, wantHeaders); got != tt.headers {
				t.Errorf("npm environment contains detected header flags = %t, want %t", got, tt.headers)
			}
		})
	}
}
