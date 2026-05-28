// Package features external test: cross-boundary verification that
// internal/config's LoadUserConfig correctly threads a FeaturesConfig
// through to the features registry. Lives in package features_test
// because importing internal/config from inside `package features`
// would create an import cycle (config → features).
package features_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"codenerd/internal/config"
	"codenerd/internal/features"
)

// resetActive restores a clean features registry between tests. Tests
// that flip the active config MUST register this in t.Cleanup so the
// next test starts from defaults; otherwise the package-global atomic
// pointer leaks state across test files.
func resetActive(t *testing.T) {
	t.Helper()
	features.SetActive(nil)
	t.Cleanup(func() { features.SetActive(nil) })
}

// writeUserConfig serialises a UserConfig as the on-disk JSON shape
// LoadUserConfig will parse. Returns the absolute path of the written
// .nerd/config.json so the caller can hand it straight to LoadUserConfig.
func writeUserConfig(t *testing.T, uc *config.UserConfig) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	data, err := json.MarshalIndent(uc, "", "  ")
	require.NoError(t, err, "marshal user config")
	require.NoError(t, os.WriteFile(path, data, 0o644), "write user config")
	return path
}

// TestLoadUserConfig_InstallsFeaturesIntoRegistry verifies that the
// FeaturesConfig parsed from .nerd/config.json is what IsDiffEvalEnabled()
// and friends consult. The boundary under test is the SetActive call
// inside LoadUserConfig — without it, the registry would still report
// compile-time defaults and downstream packages would see stale flags.
func TestLoadUserConfig_InstallsFeaturesIntoRegistry(t *testing.T) {
	t.Run("explicit_false_propagates", func(t *testing.T) {
		resetActive(t)

		fa := false
		ta := true
		uc := &config.UserConfig{
			Features: &features.FeaturesConfig{
				DiffEval:       &fa,
				FlightRecorder: &ta,
				Provenance:     &ta,
				TaxonomyFast:   &fa,
			},
		}
		path := writeUserConfig(t, uc)

		loaded, err := config.LoadUserConfig(path)
		require.NoError(t, err)
		require.NotNil(t, loaded.Features)

		require.False(t, features.IsDiffEvalEnabled(), "config wrote false; registry must report false")
		require.True(t, features.IsFlightRecorderEnabled(), "config wrote true; registry must report true")
		require.True(t, features.IsProvenanceEnabled(), "config wrote true; registry must report true")
		require.False(t, features.IsTaxonomyFastEnabled())
	})

	t.Run("absent_features_uses_compile_time_defaults", func(t *testing.T) {
		resetActive(t)

		uc := &config.UserConfig{Provider: "anthropic"} // no Features block
		path := writeUserConfig(t, uc)

		_, err := config.LoadUserConfig(path)
		require.NoError(t, err)

		// Defaults as declared in features.go's IsXxxEnabled accessors:
		//   DiffEval=false, FlightRecorder=true, Provenance=false,
		//   TaxonomyFast=true. We re-read the same accessors the kernel
		//   does so we're testing the actual contract, not a hardcoded
		//   table.
		require.False(t, features.IsDiffEvalEnabled(), "DiffEval default")
		require.True(t, features.IsFlightRecorderEnabled(), "FlightRecorder default")
		require.False(t, features.IsProvenanceEnabled(), "Provenance default")
		require.True(t, features.IsTaxonomyFastEnabled(), "TaxonomyFast default")
	})

	t.Run("nonexistent_file_preserves_active_registry", func(t *testing.T) {
		// Document the actual contract: when .nerd/config.json is
		// missing, LoadUserConfig returns an empty UserConfig early and
		// does NOT touch the active registry. Whatever was installed
		// previously (e.g. by a startup wizard) stays in effect. A test
		// that asserts the opposite would mask regressions in production
		// where the registry was preserved across reloads.
		ta := true
		fa := false
		features.SetActive(&features.FeaturesConfig{TaxonomyFast: &ta, FlightRecorder: &fa})
		t.Cleanup(func() { features.SetActive(nil) })
		require.False(t, features.IsFlightRecorderEnabled(),
			"precondition: registry says false for FlightRecorder")

		_, err := config.LoadUserConfig(filepath.Join(t.TempDir(), "nope.json"))
		require.NoError(t, err)

		require.False(t, features.IsFlightRecorderEnabled(),
			"missing file must NOT clobber a previously-installed registry")
	})
}

// TestEnvOverridesActiveConfig exercises the documented precedence chain:
// env > active config > compile-time default. The config file explicitly
// disables DiffEval; the env var explicitly enables it; the accessor MUST
// return true.
func TestEnvOverridesActiveConfig(t *testing.T) {
	resetActive(t)

	fa := false
	uc := &config.UserConfig{
		Features: &features.FeaturesConfig{DiffEval: &fa, FlightRecorder: &fa},
	}
	path := writeUserConfig(t, uc)

	_, err := config.LoadUserConfig(path)
	require.NoError(t, err)
	require.False(t, features.IsDiffEvalEnabled(), "precondition: active config says false")

	t.Setenv("CODENERD_DIFF_EVAL", "1")
	require.True(t, features.IsDiffEvalEnabled(), "env=1 must override active config")

	// Other accessors keep reading active config; verify env override is scoped.
	require.False(t, features.IsFlightRecorderEnabled(), "FlightRecorder env not set; active false stands")

	// Unrecognised env value falls back to active config (true).
	t.Setenv("CODENERD_DIFF_EVAL", "maybe")
	require.False(t, features.IsDiffEvalEnabled(), "garbage env falls through to active false")
}

// TestFullyEnabledConfigRoundTrip serialises FullyEnabledFeaturesConfig
// through the on-disk JSON path and confirms every flag the wizard sets
// to true actually reads back as true via the registry — except
// PerShardFacts, which short-circuits per the package comment.
func TestFullyEnabledConfigRoundTrip(t *testing.T) {
	resetActive(t)

	full := features.FullyEnabledFeaturesConfig()
	uc := &config.UserConfig{Features: &full}
	path := writeUserConfig(t, uc)

	_, err := config.LoadUserConfig(path)
	require.NoError(t, err)

	require.True(t, features.IsDiffEvalEnabled())
	require.True(t, features.IsFlightRecorderEnabled())
	require.True(t, features.IsProvenanceEnabled())
	require.True(t, features.IsSystemShardsEnabled())
	require.True(t, features.IsDarkModeEnabled())
	require.True(t, features.IsOnboardingSkipped())
	require.True(t, features.IsTaxonomyFastEnabled())
	// PerShardFacts is hard-wired off until the cross-shard coordinator ships.
	require.False(t, features.IsPerShardFactsEnabled(), "PerShardFacts must short-circuit to false")
}

// TestNumericOverrides verifies that integer overrides (FastScanWorkers,
// FastASTMaxBytes) thread through LoadUserConfig and that env vars take
// precedence. Zero from config still means "use default" so call sites
// keep their own fallbacks.
func TestNumericOverrides(t *testing.T) {
	resetActive(t)

	uc := &config.UserConfig{
		Features: &features.FeaturesConfig{
			FastScanWorkers: 8,
			FastASTMaxBytes: 1 << 20,
		},
	}
	path := writeUserConfig(t, uc)

	_, err := config.LoadUserConfig(path)
	require.NoError(t, err)

	require.Equal(t, 8, features.FastScanWorkers())
	require.Equal(t, int64(1<<20), features.FastASTMaxBytes())

	t.Setenv("NERD_FAST_SCAN_WORKERS", "16")
	require.Equal(t, 16, features.FastScanWorkers(), "env wins over config")
}
