// Package features is a leaf-level toggle registry for codeNERD's
// modernization flags. It exists as its own package (rather than a
// subdirectory of internal/config) so that internal/core and any
// other low-level subsystem can read flags without creating an import
// cycle with internal/config (config → store → ... → core).
//
// Layering rule:
//
//	internal/config (loads .nerd/config.json) → features.SetActive(...)
//	internal/core, internal/observability, internal/world, ...    → features.IsXXX()
//
// The package depends on nothing inside codeNERD. Add nothing here that
// pulls in another internal package.
//
// Env-var override semantics:
//
//	If the matching environment variable is present and non-empty, it
//	always wins (so tests can use t.Setenv and CI can flip a flag
//	without rewriting config.json). Otherwise the active FeaturesConfig
//	wins. If neither is set, the compile-time default wins.
//
// The flags themselves are intentionally narrow: each one corresponds
// to a specific marathon-era feature (DifferentialEngine, FlightRecorder,
// Provenance, ...). Adding a flag here means three things in lockstep:
//
//  1. A field on FeaturesConfig
//  2. A public IsXXX helper below that honours env-override + active +
//     default precedence
//  3. A default in DefaultFeaturesConfig (and a matching key written to
//     .nerd/config.json by the cli init flow)
package features

import (
	"fmt"
	"os"
	"sync/atomic"
)

// FeaturesConfig is the on-disk shape (JSON) of the feature toggles in
// .nerd/config.json. Every field is a pointer so we can tell apart
// "user wrote `false`" from "key is absent → use the default". When the
// user does not write the `features` block at all, every accessor
// falls back to DefaultFeaturesConfig().
type FeaturesConfig struct {
	// DiffEval activates internal/core/kernel_eval.go's
	// DifferentialEngine fast-path. Env var: CODENERD_DIFF_EVAL.
	DiffEval *bool `json:"diff_eval,omitempty"`

	// FlightRecorder turns on the runtime/trace ring buffer dumped on
	// panic and on /diag flightrec. Env var: NERD_FLIGHTREC.
	FlightRecorder *bool `json:"flight_recorder,omitempty"`

	// Provenance enables the Mangle DerivationRecorder so /explain can
	// answer "why was this fact derived?". Costs extra memory per
	// evaluation, so off by default unless explicitly requested.
	// Env var: CODENERD_PROVENANCE.
	Provenance *bool `json:"provenance,omitempty"`

	// SystemShards controls whether the chat session boots its
	// background system shards (autopoiesis, observer, etc).
	// Env var (inverted): NERD_DISABLE_SYSTEM_SHARDS.
	SystemShards *bool `json:"system_shards,omitempty"`

	// PerShardFacts gates the per-shard fact-store partition (Track D
	// in the marathon plan). Off by default until the cross-shard join
	// coordinator is fully wired.  Env var: CODENERD_PER_SHARD_FACTS.
	PerShardFacts *bool `json:"per_shard_facts,omitempty"`

	// DarkMode forces the TUI to load the dark palette regardless of
	// the user's theme preference. Env var: CODENERD_DARK_MODE.
	DarkMode *bool `json:"dark_mode,omitempty"`

	// SkipOnboarding bypasses the first-run wizard in internal/ux.
	// Env var: NERD_SKIP_ONBOARDING.
	SkipOnboarding *bool `json:"skip_onboarding,omitempty"`

	// TaxonomyFast switches cmd/tools/verify_taxonomy to its fast
	// codepath. Env var: CODENERD_TAXONOMY_FAST.
	TaxonomyFast *bool `json:"taxonomy_fast,omitempty"`

	// FastScanWorkers overrides the worker count used by
	// internal/world's fast scanner. Zero means "use default".
	// Env var: NERD_FAST_SCAN_WORKERS.
	FastScanWorkers int `json:"fast_scan_workers,omitempty"`

	// FastASTMaxBytes overrides the size cutoff above which fast AST
	// parsing is skipped. Zero means "use default".
	// Env var: NERD_FAST_AST_MAX_BYTES.
	FastASTMaxBytes int64 `json:"fast_ast_max_bytes,omitempty"`
}

// DefaultFeaturesConfig returns the compile-time defaults. These are
// the values an accessor returns when NO .nerd/config.json is loaded
// AND no env override is present — i.e. they govern unit tests, ad-hoc
// kernel constructions, and the very first boot before config is read.
//
// The choice is intentionally conservative: features that change the
// evaluation path (DiffEval), allocate per-derivation buffers
// (Provenance), or drive the execution tracer and can OOM the process
// under load (FlightRecorder) default OFF here so unit tests against the
// kernel see the canonical evaluation behaviour and no debug tracer runs
// unless explicitly requested. The user's `.nerd/config.json` (written by
// the boot wizard) opts INTO the modern paths explicitly — see
// FullyEnabledFeaturesConfig and the seed config.json.
//
// To "turn it all on by default for everyone", flip the file rather
// than this function; tests that need the modern paths should call
// SetActive(&FullyEnabledFeaturesConfig{}) in their setup.
func DefaultFeaturesConfig() FeaturesConfig {
	t, f := true, false
	return FeaturesConfig{
		DiffEval:       &f, // off in tests; .nerd/config.json sets true in production
		FlightRecorder: &f, // drives the execution tracer; can OOM under heavy load — opt-in only
		Provenance:     &f, // per-derivation buffers — off until /explain
		SystemShards:   &t,
		PerShardFacts:  &f,
		DarkMode:       &f,
		SkipOnboarding: &f,
		TaxonomyFast:   &t,
	}
}

// FullyEnabledFeaturesConfig returns a config where every boolean is
// explicitly true. This is what `nerd init` (or the auth wizard) writes
// to .nerd/config.json on first creation so the user inherits the most
// modern code paths out of the box.
//
// PerShardFacts is intentionally still false even here: the cross-shard
// join coordinator is not yet implemented, and enabling the flag without
// the coordinator would soft-brick the kernel. The accessor `IsPerShardFactsEnabled`
// also short-circuits to false for the same reason; this is the one
// flag that ships off until track D lands.
func FullyEnabledFeaturesConfig() FeaturesConfig {
	t := true
	f := false
	return FeaturesConfig{
		DiffEval:       &t,
		FlightRecorder: &t,
		Provenance:     &t,
		SystemShards:   &t,
		PerShardFacts:  &f, // see doc comment
		DarkMode:       &t,
		SkipOnboarding: &t,
		TaxonomyFast:   &t,
	}
}

// active holds the FeaturesConfig that internal/config writes after
// LoadUserConfig. Atomic-stored so calls to IsXXX from goroutines on
// hot paths (kernel_eval.go is called per evaluate) don't need a mutex.
// A nil pointer means "no config loaded yet; everyone gets defaults".
var active atomic.Pointer[FeaturesConfig]

// SetActive installs the FeaturesConfig parsed from .nerd/config.json.
// Idempotent: callers can install the same value repeatedly without
// side effects. Pass nil to reset to defaults.
//
// Layering note: features is intentionally a leaf package and must not
// import internal/logging. The caller (LoadUserConfig) is responsible
// for emitting a Boot-level "features: SetActive applied …" log line
// AFTER calling SetActive, so triage can tell at a glance which flags
// are live for a given run. Summary() below gives callers a stable
// string for that log line.
func SetActive(cfg *FeaturesConfig) {
	if cfg == nil {
		active.Store(nil)
		return
	}
	// Copy so callers can't mutate the active pointer's struct.
	c := *cfg
	active.Store(&c)
}

// Summary returns a short single-line description of the currently
// active FeaturesConfig (or "defaults" if none is installed). Suitable
// for boot-time logging by the caller of SetActive.
//
// Boolean pointer fields are rendered as "true"/"false"/"unset" rather
// than raw pointer addresses: FeaturesConfig uses *bool so that a
// missing JSON key (nil) can be distinguished from an explicit false.
func Summary() string {
	c := active.Load()
	if c == nil {
		return "features: defaults active"
	}
	return fmt.Sprintf(
		"features: diff_eval=%s flight_recorder=%s provenance=%s "+
			"system_shards=%s per_shard_facts=%s dark_mode=%s skip_onboarding=%s "+
			"taxonomy_fast=%s fast_scan_workers=%d fast_ast_max_bytes=%d",
		boolPtrString(c.DiffEval), boolPtrString(c.FlightRecorder), boolPtrString(c.Provenance),
		boolPtrString(c.SystemShards), boolPtrString(c.PerShardFacts), boolPtrString(c.DarkMode), boolPtrString(c.SkipOnboarding),
		boolPtrString(c.TaxonomyFast), c.FastScanWorkers, c.FastASTMaxBytes,
	)
}

// boolPtrString renders a *bool for human-readable logs.
// nil → "unset", otherwise "true"/"false".
func boolPtrString(p *bool) string {
	if p == nil {
		return "unset"
	}
	if *p {
		return "true"
	}
	return "false"
}

// Active returns the currently-installed FeaturesConfig or nil if
// none has been set. Reads are wait-free; callers must not mutate the
// returned pointer.
func Active() *FeaturesConfig { return active.Load() }

// resolveBool implements the env-override → active → default precedence
// for a boolean toggle. envVar is queried via os.Getenv and accepts
// "1" / "true" (case-insensitive) for true, "0" / "false" for false.
// Any other non-empty value is treated as "no override" (we don't want
// a stray export to silently flip a bit).
func resolveBool(envVar string, fromActive func(*FeaturesConfig) *bool, def bool) bool {
	if v := os.Getenv(envVar); v != "" {
		switch v {
		case "1", "true", "TRUE", "True":
			return true
		case "0", "false", "FALSE", "False":
			return false
		}
		// fall through to active/default
	}
	if a := active.Load(); a != nil {
		if p := fromActive(a); p != nil {
			return *p
		}
	}
	return def
}

// IsDiffEvalEnabled gates kernel_eval.go's DifferentialEngine path.
//
// Default OFF at the compile-time level so unit tests that construct a
// kernel directly (no .nerd/config.json) see the canonical full-eval
// path — the diff engine's first build is heavyweight (LoadSchemaString
// + Stratify on the whole constitution) and inflated test wall time
// before this default was set conservatively.
//
// Production opts IN via the `features.diff_eval: true` key in
// .nerd/config.json, which LoadUserConfig pushes into the active
// pointer at boot.
func IsDiffEvalEnabled() bool {
	return resolveBool("CODENERD_DIFF_EVAL",
		func(f *FeaturesConfig) *bool { return f.DiffEval }, false)
}

// IsFlightRecorderEnabled gates the runtime/trace ring buffer in main.go.
// Default OFF (opt-in). The recorder drives the Go execution tracer for
// the whole process lifetime; under heavy, long-running load (e.g. a
// campaign spawning many subprocesses) the tracer's region allocator can
// grow until the runtime aborts the process with the unrecoverable fatal
// error `throw("traceRegion: out of memory")`. A debug/observability
// feature must never be able to crash production by default, so it ships
// off and is opted into via NERD_FLIGHTREC=1 or features.flight_recorder.
// When enabled, StartFlightRecorder runs a memory watchdog that stops the
// recorder before that OOM point, so an explicit opt-in degrades
// gracefully rather than crashing.
func IsFlightRecorderEnabled() bool {
	return resolveBool("NERD_FLIGHTREC",
		func(f *FeaturesConfig) *bool { return f.FlightRecorder }, false)
}

// IsProvenanceEnabled gates Mangle's DerivationRecorder. Defaults OFF
// because it allocates per-derivation event objects; enable when the
// user wants /explain to give precise proof trees.
func IsProvenanceEnabled() bool {
	return resolveBool("CODENERD_PROVENANCE",
		func(f *FeaturesConfig) *bool { return f.Provenance }, false)
}

// IsSystemShardsEnabled is the master switch for booting the
// autopoiesis/observer background shards. Note: this is independent of
// the legacy NERD_DISABLE_SYSTEM_SHARDS env var (which is parsed at the
// call site as a comma-separated list of per-shard names to disable).
// Setting CODENERD_SYSTEM_SHARDS=0 turns off ALL system shards
// regardless of which ones are in the legacy list. Default ON.
func IsSystemShardsEnabled() bool {
	return resolveBool("CODENERD_SYSTEM_SHARDS",
		func(f *FeaturesConfig) *bool { return f.SystemShards }, true)
}

// IsPerShardFactsEnabled gates the per-shard fact store coordinator
// (ShardFactRouter in internal/core/shard_fact_router.go). The cortex
// builds and installs the router on shard registration when this flag
// is on. Default OFF until the manifest is auto-wired into
// internal/system/factory.go's hard-coded shard list.
func IsPerShardFactsEnabled() bool {
	return resolveBool("CODENERD_PER_SHARD_FACTS",
		func(f *FeaturesConfig) *bool { return f.PerShardFacts }, false)
}

// IsDarkModeEnabled overrides the TUI palette to dark. The config
// `theme` key still controls light/dark when this flag is false.
func IsDarkModeEnabled() bool {
	return resolveBool("CODENERD_DARK_MODE",
		func(f *FeaturesConfig) *bool { return f.DarkMode }, false)
}

// IsOnboardingSkipped lets the chat boot skip the first-run wizard.
func IsOnboardingSkipped() bool {
	return resolveBool("NERD_SKIP_ONBOARDING",
		func(f *FeaturesConfig) *bool { return f.SkipOnboarding }, false)
}

// IsTaxonomyFastEnabled selects the fast codepath in verify_taxonomy.
func IsTaxonomyFastEnabled() bool {
	return resolveBool("CODENERD_TAXONOMY_FAST",
		func(f *FeaturesConfig) *bool { return f.TaxonomyFast }, true)
}

// FastScanWorkers returns the configured worker count, or zero when
// the call site should pick a default. Env var wins over config.
func FastScanWorkers() int {
	if v := os.Getenv("NERD_FAST_SCAN_WORKERS"); v != "" {
		if n, err := parseUint(v); err == nil {
			return n
		}
	}
	if a := active.Load(); a != nil {
		return a.FastScanWorkers
	}
	return 0
}

// FastASTMaxBytes returns the configured size cutoff, or zero for default.
func FastASTMaxBytes() int64 {
	if v := os.Getenv("NERD_FAST_AST_MAX_BYTES"); v != "" {
		if n, err := parseInt64(v); err == nil && n > 0 {
			return n
		}
	}
	if a := active.Load(); a != nil {
		return a.FastASTMaxBytes
	}
	return 0
}

func parseUint(s string) (int, error) {
	var n int
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0, errBadInt
		}
		n = n*10 + int(ch-'0')
	}
	if n <= 0 {
		return 0, errBadInt
	}
	return n, nil
}

func parseInt64(s string) (int64, error) {
	var n int64
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0, errBadInt
		}
		n = n*10 + int64(ch-'0')
	}
	if n <= 0 {
		return 0, errBadInt
	}
	return n, nil
}

var errBadInt = newErr("features: invalid integer override")

type featuresErr string

func (e featuresErr) Error() string { return string(e) }
func newErr(s string) error         { return featuresErr(s) }
