//go:build integration

package e2e_test

import (
	"os"
	"testing"

	"codenerd/internal/tools"
)

// Real Cortex boot binds process-global workspace state. These fixtures are
// deliberately nonparallel. Register before boot so its cleanup runs only
// after every later-registered Cortex.Close, including same-workspace reboots.
func ownGeneratedFixtureWorkspace(t *testing.T, root string) {
	t.Helper()
	owned, err := tools.CanonicalWorkspaceRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	registry := tools.Global()
	previousRoot := registry.WorkspaceRoot()
	previousEnv, hadEnv := os.LookupEnv("CODENERD_WORKSPACE_ROOT")
	t.Cleanup(func() {
		if tools.Global() == registry && registry.WorkspaceRoot() == owned {
			registry.SetWorkspaceRoot(previousRoot)
		}
		if current, ok := os.LookupEnv("CODENERD_WORKSPACE_ROOT"); ok && current == owned {
			if hadEnv {
				if err := os.Setenv("CODENERD_WORKSPACE_ROOT", previousEnv); err != nil {
					t.Errorf("restore fixture workspace environment: %v", err)
				}
			} else if err := os.Unsetenv("CODENERD_WORKSPACE_ROOT"); err != nil {
				t.Errorf("restore absent fixture workspace environment: %v", err)
			}
		}
	})
}

func TestGeneratedFixtureWorkspaceRestoration(t *testing.T) {
	registry := tools.Global()
	originalRoot := registry.WorkspaceRoot()
	originalEnv, hadOriginalEnv := os.LookupEnv("CODENERD_WORKSPACE_ROOT")
	t.Cleanup(func() {
		registry.SetWorkspaceRoot(originalRoot)
		if hadOriginalEnv {
			if err := os.Setenv("CODENERD_WORKSPACE_ROOT", originalEnv); err != nil {
				t.Error(err)
			}
		} else if err := os.Unsetenv("CODENERD_WORKSPACE_ROOT"); err != nil {
			t.Error(err)
		}
	})
	for _, mode := range []string{"prior_set", "prior_unset", "later_both", "later_root", "later_env"} {
		t.Run(mode, func(t *testing.T) {
			previous := t.TempDir()
			later := t.TempDir()
			wantRoot, wantEnv, wantSet := previous, previous, true
			if mode == "prior_unset" {
				previous, wantRoot, wantEnv, wantSet = "", "", "", false
			}
			registry.SetWorkspaceRoot(previous)
			if wantSet {
				if err := os.Setenv("CODENERD_WORKSPACE_ROOT", previous); err != nil {
					t.Fatal(err)
				}
			} else if err := os.Unsetenv("CODENERD_WORKSPACE_ROOT"); err != nil {
				t.Fatal(err)
			}
			t.Run("real_boot", func(t *testing.T) {
				fixture := newPipelineFixture(t, false)
				owned, err := tools.CanonicalWorkspaceRoot(fixture.root)
				if err != nil {
					t.Fatal(err)
				}
				if registry.WorkspaceRoot() != owned || os.Getenv("CODENERD_WORKSPACE_ROOT") != owned {
					t.Fatal("live production boot did not retain its own workspace containment")
				}
				if mode == "later_both" || mode == "later_root" {
					registry.SetWorkspaceRoot(later)
					wantRoot = later
				}
				if mode == "later_both" || mode == "later_env" {
					if err := os.Setenv("CODENERD_WORKSPACE_ROOT", later); err != nil {
						t.Fatal(err)
					}
					wantEnv, wantSet = later, true
				}
			})
			gotEnv, gotSet := os.LookupEnv("CODENERD_WORKSPACE_ROOT")
			if registry.WorkspaceRoot() != wantRoot || gotEnv != wantEnv || gotSet != wantSet {
				t.Fatalf("fixture leaked or overwrote another binding: registry=%q want=%q env=(%q,%v) want=(%q,%v)", registry.WorkspaceRoot(), wantRoot, gotEnv, gotSet, wantEnv, wantSet)
			}
		})
	}
}
