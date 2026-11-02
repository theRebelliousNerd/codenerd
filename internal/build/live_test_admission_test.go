package build

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Keep the existing environment contract for both go gates and other declared
// commands. Only a caller's existing explicit forwarding carries live consent.
func TestWorkspaceCommandLiveTestAdmissionEnvironment(t *testing.T) {
	t.Setenv("CODENERD_LIVE_TYPESAFE", "1")
	t.Setenv("CODENERD_TEST_UNLISTED_SECRET", "synthetic-secret")
	t.Setenv("HTTPS_PROXY", "http://synthetic.invalid:9")
	for _, argv := range [][]string{{"go", "test", "./..."}, {"synthetic-command"}} {
		for _, mode := range []string{"default", "owner_allowlist", "declared_env"} {
			t.Run(argv[0]+"/"+mode, func(t *testing.T) {
				root := t.TempDir()
				settings := map[string]any{}
				if mode == "owner_allowlist" {
					settings["execution"] = map[string]any{"allowed_env_vars": []string{"CODENERD_LIVE_TYPESAFE"}}
				}
				body, err := json.Marshal(settings)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(filepath.Join(root, ".nerd"), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, ".nerd", "config.json"), body, 0600); err != nil {
					t.Fatal(err)
				}
				overlay := map[string]string{}
				if mode == "declared_env" {
					overlay["CODENERD_LIVE_TYPESAFE"] = "1"
				}
				cmd := WorkspaceCommand(context.Background(), root, argv, overlay)
				want := "1"
				if mode == "default" {
					want = ""
				}
				if got := envValue(cmd.Env, "CODENERD_LIVE_TYPESAFE"); got != want {
					t.Fatalf("admission=%q want=%q", got, want)
				}
				for _, key := range []string{"CODENERD_TEST_UNLISTED_SECRET", "HTTPS_PROXY"} {
					if hasEnvKey(cmd.Env, key) {
						t.Errorf("unlisted %s reached command", key)
					}
				}
				if got := envValue(cmd.Env, "PATH"); got != os.Getenv("PATH") {
					t.Fatal("toolchain PATH changed")
				}
			})
		}
	}
}

func TestWorkspaceCommandAdaptsForeignSqliteHeadersPath(t *testing.T) {
	root := t.TempDir()
	headersDir := filepath.Join(root, "sqlite_headers")
	if err := os.MkdirAll(headersDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := WorkspaceCommand(context.Background(), root, []string{"go", "test", "./..."}, map[string]string{
		"CGO_CFLAGS": "-IC:/CodeProjects/codeNERD/sqlite_headers",
	})
	want := "-I" + headersDir
	if got := envValue(cmd.Env, "CGO_CFLAGS"); got != want {
		t.Fatalf("CGO_CFLAGS = %q, want %q", got, want)
	}
}
