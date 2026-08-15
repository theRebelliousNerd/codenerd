package features

import (
	"strings"
	"testing"
)

// The NERD_* → CODENERD_* migration is dual-read: the legacy spelling still
// works so an upgrade does not break an operator's shell profile, but the
// canonical name wins and the legacy one is reported as deprecated.

func TestEnvMigration_WhenOnlyTheLegacyVarIsSet_ShouldStillHonourIt(t *testing.T) {
	SetActive(nil)
	t.Setenv("NERD_FLIGHTREC", "1")

	if !IsFlightRecorderEnabled() {
		t.Fatal("NERD_FLIGHTREC=1 no longer turns the flight recorder on; the migration broke an existing deployment")
	}
	if !IsFlightRecorderEnabled() {
		t.Fatal("accessor is not stable across calls")
	}
}

func TestEnvMigration_WhenBothVarsAreSet_ShouldPreferTheCanonicalOne(t *testing.T) {
	SetActive(nil)
	t.Setenv("NERD_SKIP_ONBOARDING", "1")
	t.Setenv("CODENERD_SKIP_ONBOARDING", "0")

	if IsOnboardingSkipped() {
		t.Fatal("the legacy variable beat the canonical one; precedence is inverted")
	}
}

func TestEnvMigration_WhenOnlyTheCanonicalVarIsSet_ShouldHonourIt(t *testing.T) {
	SetActive(nil)
	t.Setenv("CODENERD_FLIGHT_RECORDER", "true")

	if !IsFlightRecorderEnabled() {
		t.Fatal("CODENERD_FLIGHT_RECORDER=true did not enable the flight recorder")
	}
}

func TestEnvMigration_LegacyVarShouldStillBeatConfig(t *testing.T) {
	// Precedence is env > config > default, and a legacy variable is still an
	// env override — demoting it below config would silently change behavior
	// for anyone relying on the old name.
	f := false
	SetActive(&FeaturesConfig{FlightRecorder: &f})
	t.Cleanup(func() { SetActive(nil) })
	t.Setenv("NERD_FLIGHTREC", "1")

	if !IsFlightRecorderEnabled() {
		t.Fatal("config beat the legacy env override")
	}
}

func TestResolved_WhenALegacyVarDecides_ShouldReportLegacyEnvSource(t *testing.T) {
	SetActive(nil)
	t.Setenv("NERD_FLIGHTREC", "1")

	flag := findFlag(t, "flight_recorder")
	if flag.Source != SourceLegacyEnv {
		t.Fatalf("source = %q, want %q", flag.Source, SourceLegacyEnv)
	}
	if flag.EnvVar != "CODENERD_FLIGHT_RECORDER" {
		t.Fatalf("canonical env var = %q", flag.EnvVar)
	}
	if flag.LegacyEnvVar != "NERD_FLIGHTREC" {
		t.Fatalf("legacy env var = %q", flag.LegacyEnvVar)
	}
}

func TestResolved_WhenTheCanonicalVarDecides_ShouldReportEnvSource(t *testing.T) {
	SetActive(nil)
	t.Setenv("CODENERD_FLIGHT_RECORDER", "1")

	if flag := findFlag(t, "flight_recorder"); flag.Source != SourceEnv {
		t.Fatalf("source = %q, want %q", flag.Source, SourceEnv)
	}
}

func TestDeprecations_WhenNoLegacyVarIsSet_ShouldBeEmpty(t *testing.T) {
	for _, name := range legacyVarNames() {
		t.Setenv(name, "")
	}
	if got := Deprecations(); len(got) != 0 {
		t.Fatalf("expected no deprecations, got %v", got)
	}
}

func TestDeprecations_WhenALegacyVarIsSet_ShouldNameTheReplacement(t *testing.T) {
	for _, name := range legacyVarNames() {
		t.Setenv(name, "")
	}
	t.Setenv("NERD_FAST_SCAN_WORKERS", "4")

	got := Deprecations()
	if len(got) != 1 {
		t.Fatalf("expected exactly one deprecation, got %v", got)
	}
	if !strings.Contains(got[0], "NERD_FAST_SCAN_WORKERS") ||
		!strings.Contains(got[0], "CODENERD_FAST_SCAN_WORKERS") {
		t.Fatalf("deprecation must name both the old and the new spelling: %q", got[0])
	}
}

// TestDeprecations_WhenShadowedByTheCanonicalVar_ShouldSayItIsIgnored: a legacy
// variable that is set but inert is the case most likely to waste an
// operator's afternoon, so it has to be reported, not filtered out.
func TestDeprecations_WhenShadowedByTheCanonicalVar_ShouldSayItIsIgnored(t *testing.T) {
	for _, name := range legacyVarNames() {
		t.Setenv(name, "")
	}
	t.Setenv("NERD_FLIGHTREC", "1")
	t.Setenv("CODENERD_FLIGHT_RECORDER", "0")

	got := Deprecations()
	if len(got) != 1 {
		t.Fatalf("expected exactly one deprecation, got %v", got)
	}
	if !strings.Contains(got[0], "ignored") {
		t.Fatalf("a shadowed legacy variable must be reported as ignored: %q", got[0])
	}
}

func TestFastScanWorkers_ShouldDualReadTheLegacyVar(t *testing.T) {
	SetActive(nil)
	t.Setenv("NERD_FAST_SCAN_WORKERS", "6")
	if got := FastScanWorkers(); got != 6 {
		t.Fatalf("FastScanWorkers() = %d, want 6 from the legacy variable", got)
	}

	t.Setenv("CODENERD_FAST_SCAN_WORKERS", "3")
	if got := FastScanWorkers(); got != 3 {
		t.Fatalf("FastScanWorkers() = %d, want 3 from the canonical variable", got)
	}
}

func TestFastASTMaxBytes_ShouldDualReadTheLegacyVar(t *testing.T) {
	SetActive(nil)
	t.Setenv("NERD_FAST_AST_MAX_BYTES", "2048")
	if got := FastASTMaxBytes(); got != 2048 {
		t.Fatalf("FastASTMaxBytes() = %d, want 2048 from the legacy variable", got)
	}

	t.Setenv("CODENERD_FAST_AST_MAX_BYTES", "4096")
	if got := FastASTMaxBytes(); got != 4096 {
		t.Fatalf("FastASTMaxBytes() = %d, want 4096 from the canonical variable", got)
	}
}

// TestEnvMigration_EveryCanonicalVarShouldUseTheCodenerdPrefix is the ratchet:
// a new flag added with a bare NERD_ name fails here instead of quietly
// re-opening the inconsistency this migration closed.
func TestEnvMigration_EveryCanonicalVarShouldUseTheCodenerdPrefix(t *testing.T) {
	check := func(name, envVar string) {
		if !strings.HasPrefix(envVar, "CODENERD_") {
			t.Errorf("flag %q has canonical env var %q; canonical names must use the CODENERD_ prefix", name, envVar)
		}
	}
	for _, f := range boolFlags {
		check(f.name, f.envVar)
	}
	for _, f := range intFlags {
		check(f.name, f.envVar)
	}
}

// TestEnvMigration_LegacyVarsShouldBeTheKnownFour pins the migration's scope.
// It fails both when a legacy name is dropped without the doc updates the
// package comment requires, and when a new bare NERD_ name sneaks in.
func TestEnvMigration_LegacyVarsShouldBeTheKnownFour(t *testing.T) {
	want := map[string]string{
		"NERD_FLIGHTREC":          "flight_recorder",
		"NERD_SKIP_ONBOARDING":    "skip_onboarding",
		"NERD_FAST_SCAN_WORKERS":  "fast_scan_workers",
		"NERD_FAST_AST_MAX_BYTES": "fast_ast_max_bytes",
	}
	got := map[string]string{}
	for _, f := range boolFlags {
		if f.legacyEnvVar != "" {
			got[f.legacyEnvVar] = f.name
		}
	}
	for _, f := range intFlags {
		if f.legacyEnvVar != "" {
			got[f.legacyEnvVar] = f.name
		}
	}
	if len(got) != len(want) {
		t.Fatalf("legacy variable set changed: got %v, want %v", got, want)
	}
	for legacy, flag := range want {
		if got[legacy] != flag {
			t.Errorf("legacy %s maps to %q, want %q", legacy, got[legacy], flag)
		}
	}
}

func findFlag(t *testing.T, name string) Flag {
	t.Helper()
	for _, f := range Resolved() {
		if f.Name == name {
			return f
		}
	}
	t.Fatalf("Resolved() has no flag %q", name)
	return Flag{}
}

func legacyVarNames() []string {
	var out []string
	for _, f := range boolFlags {
		if f.legacyEnvVar != "" {
			out = append(out, f.legacyEnvVar)
		}
	}
	for _, f := range intFlags {
		if f.legacyEnvVar != "" {
			out = append(out, f.legacyEnvVar)
		}
	}
	return out
}
