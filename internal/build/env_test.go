package build

import (
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/config"
)

func clearEnvVars(t *testing.T, keys ...string) {
	t.Helper()
	for _, key := range keys {
		t.Setenv(key, "")
	}
}

func TestDeriveGOCACHE_Precedence(t *testing.T) {
	keys := []string{"LOCALAPPDATA", "USERPROFILE", "HOME", "TEMP", "TMP", "TMPDIR"}

	t.Run("none", func(t *testing.T) {
		clearEnvVars(t, keys...)
		if got := deriveGOCACHE(); got != "" {
			t.Fatalf("deriveGOCACHE() = %q, want empty", got)
		}
	})

	t.Run("localappdata", func(t *testing.T) {
		clearEnvVars(t, keys...)
		localAppData := t.TempDir()
		userProfile := t.TempDir()
		home := t.TempDir()
		temp := t.TempDir()

		t.Setenv("LOCALAPPDATA", localAppData)
		t.Setenv("USERPROFILE", userProfile)
		t.Setenv("HOME", home)
		t.Setenv("TEMP", temp)

		want := filepath.Join(localAppData, "go-build")
		if got := deriveGOCACHE(); got != want {
			t.Fatalf("deriveGOCACHE() = %q, want %q", got, want)
		}
	})

	t.Run("userprofile", func(t *testing.T) {
		clearEnvVars(t, keys...)
		userProfile := t.TempDir()
		home := t.TempDir()
		temp := t.TempDir()

		t.Setenv("USERPROFILE", userProfile)
		t.Setenv("HOME", home)
		t.Setenv("TEMP", temp)

		want := filepath.Join(userProfile, ".cache", "go-build")
		if got := deriveGOCACHE(); got != want {
			t.Fatalf("deriveGOCACHE() = %q, want %q", got, want)
		}
	})

	t.Run("home", func(t *testing.T) {
		clearEnvVars(t, keys...)
		home := t.TempDir()
		temp := t.TempDir()

		t.Setenv("HOME", home)
		t.Setenv("TEMP", temp)

		want := filepath.Join(home, ".cache", "go-build")
		if got := deriveGOCACHE(); got != want {
			t.Fatalf("deriveGOCACHE() = %q, want %q", got, want)
		}
	})

	t.Run("temp", func(t *testing.T) {
		clearEnvVars(t, keys...)
		temp := t.TempDir()

		t.Setenv("TEMP", temp)

		want := filepath.Join(temp, "go-build")
		if got := deriveGOCACHE(); got != want {
			t.Fatalf("deriveGOCACHE() = %q, want %q", got, want)
		}
	})
}

func TestEnvKeyHelpers(t *testing.T) {
	env := []string{"FOO=1", "BAR=2"}

	if !hasEnvKey(env, "FOO") {
		t.Fatalf("hasEnvKey(env, FOO) = false, want true")
	}
	if hasEnvKey(env, "BA") {
		t.Fatalf("hasEnvKey(env, BA) = true, want false")
	}

	updated := setEnvKey(append([]string{}, env...), "FOO", "3")
	if !hasEnvKey(updated, "FOO") {
		t.Fatalf("setEnvKey did not retain FOO key")
	}
	if updated[0] != "FOO=3" {
		t.Fatalf("setEnvKey updated[0] = %q, want %q", updated[0], "FOO=3")
	}

	added := setEnvKey(append([]string{}, env...), "BAZ", "9")
	if !hasEnvKey(added, "BAZ") {
		t.Fatalf("setEnvKey did not add BAZ key")
	}

	merged := MergeEnv(env, "BAR=7", "BAZ=9")
	if !hasEnvKey(merged, "BAR") || !hasEnvKey(merged, "BAZ") {
		t.Fatalf("MergeEnv missing expected keys: %v", merged)
	}
	for _, entry := range merged {
		if entry == "BAR=2" {
			t.Fatalf("MergeEnv did not override BAR: %v", merged)
		}
	}
}

func TestDetectCGOFlags(t *testing.T) {
	root := t.TempDir()

	// Create multiple known header directories to ensure deterministic flag order.
	dirs := []string{
		filepath.Join(root, "sqlite_headers"),
		filepath.Join(root, "include"),
		filepath.Join(root, "vendor", "include"),
		filepath.Join(root, "third_party", "include"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdirAll(%q): %v", dir, err)
		}
	}

	got := detectCGOFlags(root)
	want := "-I" + dirs[0] + " " + "-I" + dirs[1] + " " + "-I" + dirs[2] + " " + "-I" + dirs[3]
	if got != want {
		t.Fatalf("detectCGOFlags() = %q, want %q", got, want)
	}
}

func TestLoadBuildConfig_SqliteHeaders(t *testing.T) {
	root := t.TempDir()
	sqliteHeaders := filepath.Join(root, "sqlite_headers")
	if err := os.MkdirAll(sqliteHeaders, 0o755); err != nil {
		t.Fatalf("mkdirAll(%q): %v", sqliteHeaders, err)
	}

	t.Run("default_enables_sqlite_vec", func(t *testing.T) {
		t.Setenv("GOFLAGS", "")
		cfg := loadBuildConfig(nil, root)

		if got, want := cfg.EnvVars["CGO_CFLAGS"], "-I"+sqliteHeaders; got != want {
			t.Fatalf("cfg.EnvVars[CGO_CFLAGS] = %q, want %q", got, want)
		}
		if got, want := cfg.EnvVars["GOFLAGS"], "-tags=sqlite_vec"; got != want {
			t.Fatalf("cfg.EnvVars[GOFLAGS] = %q, want %q", got, want)
		}
		found := false
		for _, pkg := range cfg.CGOPackages {
			if pkg == "sqlite-vec" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("cfg.CGOPackages missing sqlite-vec: %v", cfg.CGOPackages)
		}
	})

	t.Run("does_not_override_env_goflags", func(t *testing.T) {
		t.Setenv("GOFLAGS", "-tags=custom")
		cfg := loadBuildConfig(nil, root)
		if got := cfg.EnvVars["GOFLAGS"]; got != "" {
			t.Fatalf("cfg.EnvVars[GOFLAGS] = %q, want empty (env GOFLAGS already set)", got)
		}
	})

	t.Run("user_config_overrides_cgo_cflags", func(t *testing.T) {
		t.Setenv("GOFLAGS", "")
		userCfg := &config.UserConfig{
			Build: &config.BuildConfig{
				EnvVars: map[string]string{
					"CGO_CFLAGS": "-Icustom",
					"GOFLAGS":    "-tags=custom",
				},
				CGOPackages: []string{"custompkg"},
			},
		}
		cfg := loadBuildConfig(userCfg, root)
		if got, want := cfg.EnvVars["CGO_CFLAGS"], "-Icustom"; got != want {
			t.Fatalf("cfg.EnvVars[CGO_CFLAGS] = %q, want %q", got, want)
		}
		if got, want := cfg.EnvVars["GOFLAGS"], "-tags=custom"; got != want {
			t.Fatalf("cfg.EnvVars[GOFLAGS] = %q, want %q", got, want)
		}

		foundCustom := false
		foundSqliteVec := false
		for _, pkg := range cfg.CGOPackages {
			switch pkg {
			case "custompkg":
				foundCustom = true
			case "sqlite-vec":
				foundSqliteVec = true
			}
		}
		if !foundCustom || !foundSqliteVec {
			t.Fatalf("cfg.CGOPackages missing expected packages (custom=%v sqlite-vec=%v): %v", foundCustom, foundSqliteVec, cfg.CGOPackages)
		}
	})
}
