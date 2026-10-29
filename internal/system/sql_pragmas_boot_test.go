package system

import (
	"os"
	"testing"

	"codenerd/internal/sqlpragmas"
)

// core_limits.sql_host_class reaches the package that sizes SQLite's caches.
// Before this, SetHostClass had no caller: a config could not choose a class,
// only the environment could. Not parallel: the host class is process state.
func TestConfigureSQLPragmas_WhenConfigDeclaresAClass_ShouldApplyIt(t *testing.T) {
	unsetEnv(t, sqlpragmas.EnvHostClass)
	t.Cleanup(sqlpragmas.ClearHostClass)

	configureSQLPragmas("laptop")
	if got := sqlpragmas.ActiveHostClass(); got != sqlpragmas.HostLaptop {
		t.Fatalf("host class = %s, want laptop from core_limits.sql_host_class", got)
	}

	// A later Cortex whose workspace declares nothing does not inherit it.
	configureSQLPragmas("")
	if got := sqlpragmas.ActiveHostClass(); got != sqlpragmas.HostWorkstation {
		t.Fatalf("host class = %s after an empty key, want the default", got)
	}
}

// The environment wins over the config key, as it does for every override.
func TestConfigureSQLPragmas_WhenEnvironmentSetsAClass_ShouldLeaveItAlone(t *testing.T) {
	t.Setenv(sqlpragmas.EnvHostClass, "micro")
	t.Cleanup(sqlpragmas.ClearHostClass)

	configureSQLPragmas("laptop")
	if got := sqlpragmas.ActiveHostClass(); got != sqlpragmas.HostMicro {
		t.Fatalf("host class = %s, want micro from %s", got, sqlpragmas.EnvHostClass)
	}
}

// unsetEnv removes key for the rest of the test and restores it afterwards.
func unsetEnv(t *testing.T, key string) {
	t.Helper()
	t.Setenv(key, "") // registers the restore
	if err := os.Unsetenv(key); err != nil {
		t.Fatal(err)
	}
}
