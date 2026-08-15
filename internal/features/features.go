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
// Env-var naming and the NERD_* → CODENERD_* migration:
//
//	Every flag's canonical variable is CODENERD_ + the config key in
//	upper case (diff_eval → CODENERD_DIFF_EVAL). Four flags predate that
//	rule and shipped under a bare NERD_ prefix: NERD_FLIGHTREC,
//	NERD_SKIP_ONBOARDING, NERD_FAST_SCAN_WORKERS and
//	NERD_FAST_AST_MAX_BYTES. Those legacy names are still READ — an
//	operator's shell profile or a CI job must not break on an upgrade —
//	but the canonical name wins when both are set, and the legacy name is
//	reported as deprecated.
//
//	Deprecations() lists the legacy variables currently in use, in a form
//	a caller can log. This package must not import internal/logging (see
//	the layering rule above), so it reports rather than warns; LoadUserConfig
//	and `nerd features` are the surfaces that make the report visible.
//
//	Removal criterion, so this does not become permanent: the legacy names
//	go away once a release has shipped with Deprecations() surfaced at boot
//	AND the docs under Docs/architecture/{observability,ux}/ that still
//	instruct operators to set NERD_FLIGHTREC / NERD_SKIP_ONBOARDING have
//	been updated to the canonical spelling. Delete legacyEnvVar from
//	boolFlags and the two intFlags entries; TestEnvMigration_* then fails
//	loudly rather than the behavior changing silently.
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
	"strings"
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
	// panic and on /diag flightrec.
	// Env var: CODENERD_FLIGHT_RECORDER (legacy: NERD_FLIGHTREC).
	FlightRecorder *bool `json:"flight_recorder,omitempty"`

	// Provenance enables the Mangle DerivationRecorder so /explain can
	// answer "why was this fact derived?". Costs extra memory per
	// evaluation, so off by default unless explicitly requested.
	// Env var: CODENERD_PROVENANCE.
	Provenance *bool `json:"provenance,omitempty"`

	// SystemShards controls whether the chat session boots its
	// background system shards (autopoiesis, observer, etc).
	// Env var: CODENERD_SYSTEM_SHARDS (0 disables all of them).
	// Distinct from the legacy NERD_DISABLE_SYSTEM_SHARDS, which is parsed
	// at the call site as a comma-separated list of individual shard names.
	SystemShards *bool `json:"system_shards,omitempty"`

	// PerShardFacts gates the per-shard fact-store partition (Track D
	// in the marathon plan). Off by default until the cross-shard join
	// coordinator exists — see FullyEnabledFeaturesConfig for the audit.
	// Env var: CODENERD_PER_SHARD_FACTS.
	// The accessor applies ordinary env → active → default precedence; it
	// does not hard short-circuit, so an operator who sets the flag gets
	// the (incomplete) router rather than a silently ignored setting.
	PerShardFacts *bool `json:"per_shard_facts,omitempty"`

	// DarkMode forces the TUI to load the dark palette regardless of
	// the user's theme preference. Env var: CODENERD_DARK_MODE.
	DarkMode *bool `json:"dark_mode,omitempty"`

	// SkipOnboarding bypasses the first-run wizard in internal/ux.
	// Env var: CODENERD_SKIP_ONBOARDING (legacy: NERD_SKIP_ONBOARDING).
	SkipOnboarding *bool `json:"skip_onboarding,omitempty"`

	// TaxonomyFast switches cmd/tools/verify_taxonomy to its fast
	// codepath, which skips the scenario sweep. Off by default: the fast
	// path is a test accelerator, and a verification tool whose default is
	// "skip verification" verifies nothing.
	// Env var: CODENERD_TAXONOMY_FAST.
	TaxonomyFast *bool `json:"taxonomy_fast,omitempty"`

	// FastScanWorkers overrides the worker count used by
	// internal/world's fast scanner. Zero means "use default".
	// Env var: CODENERD_FAST_SCAN_WORKERS (legacy: NERD_FAST_SCAN_WORKERS).
	FastScanWorkers int `json:"fast_scan_workers,omitempty"`

	// FastASTMaxBytes overrides the size cutoff above which fast AST
	// parsing is skipped. Zero means "use default".
	// Env var: CODENERD_FAST_AST_MAX_BYTES (legacy: NERD_FAST_AST_MAX_BYTES).
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
		TaxonomyFast:   &f, // fast path skips the scenario sweep; opt in explicitly
	}
}

// FullyEnabledFeaturesConfig returns a config where every boolean is
// explicitly true. This is what `nerd init` (or the auth wizard) writes
// to .nerd/config.json on first creation so the user inherits the most
// modern code paths out of the box.
//
// PerShardFacts is intentionally still false even here. Audited 2026-08-15,
// because the stated blocker had in fact been cleared and the flag still must
// not flip:
//
//   - The old blocker — "the manifest is not auto-wired into
//     internal/system/factory.go's hard-coded shard list" — is RESOLVED.
//     defaultKernelShardConfigs now builds every KernelShardConfig from
//     shards.DefaultShardPredicateManifests, and CortexKernel.RegisterShard
//     installs the router and registers each shard's owned predicates.
//   - The real blocker remains: ShardFactRouter is a dispatch table, not a
//     join coordinator. AssertVia/QueryVia/RetractVia route ONE predicate to
//     its owning shard, but rule evaluation happens inside a shard's own
//     *RealKernel over that shard's local facts. A rule body spanning
//     predicates owned by two shards can therefore never derive — e.g. a
//     routing-shard rule joining user_intent (owned by "routing") with
//     project_profile (owned by "world") sees an empty project_profile.
//     Nothing in the tree implements distributed evaluation; grep for
//     "cross-shard" finds only comments.
//
// Flipping the flag would silently delete every cross-domain derivation
// instead of failing loudly, which is the worst available failure mode for an
// executive kernel. It stays opt-in until a join coordinator exists.
//
// `IsPerShardFactsEnabled` does NOT hard short-circuit — it is an ordinary
// resolveBool accessor, so an explicit env or config opt-in is honoured.
// Earlier revisions of this comment claimed a short-circuit that the code
// never had.
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

// Source describes where a resolved flag value came from.
type Source string

const (
	// SourceEnv means the canonical CODENERD_* variable decided the value.
	SourceEnv Source = "env"
	// SourceLegacyEnv means a deprecated NERD_* variable decided the value,
	// because the canonical one was unset. Distinct from SourceEnv so an
	// operator reading `nerd features` can see which line is on borrowed time.
	SourceLegacyEnv Source = "legacy-env"
	// SourceConfig means the active FeaturesConfig decided the value.
	SourceConfig Source = "config"
	// SourceDefault means neither env nor config specified it.
	SourceDefault Source = "default"
)

// Flag is one resolved toggle: the value a caller of the accessor actually
// gets, plus where it came from.
type Flag struct {
	Name         string // JSON/config key, e.g. "diff_eval"
	EnvVar       string // canonical environment variable
	LegacyEnvVar string // deprecated pre-CODENERD_ spelling, "" when there is none
	Value        bool
	Source       Source
	Default      bool
}

// boolFlags is the single source of truth tying each flag's name, env vars,
// accessor and default together, so Summary, Resolved, Deprecations and the CLI
// cannot drift from the accessors.
//
// legacyEnvVar is the pre-migration spelling. It is read only when the
// canonical name is unset; see the package doc for the removal criterion.
var boolFlags = []struct {
	name         string
	envVar       string
	legacyEnvVar string
	get          func(*FeaturesConfig) *bool
	def          bool
}{
	{"diff_eval", "CODENERD_DIFF_EVAL", "", func(f *FeaturesConfig) *bool { return f.DiffEval }, false},
	{"flight_recorder", "CODENERD_FLIGHT_RECORDER", "NERD_FLIGHTREC", func(f *FeaturesConfig) *bool { return f.FlightRecorder }, false},
	{"provenance", "CODENERD_PROVENANCE", "", func(f *FeaturesConfig) *bool { return f.Provenance }, false},
	{"system_shards", "CODENERD_SYSTEM_SHARDS", "", func(f *FeaturesConfig) *bool { return f.SystemShards }, true},
	{"per_shard_facts", "CODENERD_PER_SHARD_FACTS", "", func(f *FeaturesConfig) *bool { return f.PerShardFacts }, false},
	{"dark_mode", "CODENERD_DARK_MODE", "", func(f *FeaturesConfig) *bool { return f.DarkMode }, false},
	{"skip_onboarding", "CODENERD_SKIP_ONBOARDING", "NERD_SKIP_ONBOARDING", func(f *FeaturesConfig) *bool { return f.SkipOnboarding }, false},
	{"taxonomy_fast", "CODENERD_TAXONOMY_FAST", "", func(f *FeaturesConfig) *bool { return f.TaxonomyFast }, false},
}

// intFlags mirrors boolFlags for the two non-boolean overrides, so the
// migration table has exactly one home. FastScanWorkers and FastASTMaxBytes
// read it through envInt.
var intFlags = []struct {
	name         string
	envVar       string
	legacyEnvVar string
}{
	{"fast_scan_workers", "CODENERD_FAST_SCAN_WORKERS", "NERD_FAST_SCAN_WORKERS"},
	{"fast_ast_max_bytes", "CODENERD_FAST_AST_MAX_BYTES", "NERD_FAST_AST_MAX_BYTES"},
}

// Resolved returns every boolean toggle as the accessors actually resolve it,
// with the winning source. This is what an operator needs: the raw config
// says "unset" for a key that an env var is currently forcing on.
func Resolved() []Flag {
	a := active.Load()
	out := make([]Flag, 0, len(boolFlags))
	for _, f := range boolFlags {
		flag := Flag{Name: f.name, EnvVar: f.envVar, LegacyEnvVar: f.legacyEnvVar, Default: f.def}
		flag.Value = resolveBool(f.envVar, f.legacyEnvVar, f.get, f.def)

		switch {
		case envBool(f.envVar) != nil:
			flag.Source = SourceEnv
		case f.legacyEnvVar != "" && envBool(f.legacyEnvVar) != nil:
			flag.Source = SourceLegacyEnv
		case a != nil && f.get(a) != nil:
			flag.Source = SourceConfig
		default:
			flag.Source = SourceDefault
		}
		out = append(out, flag)
	}
	return out
}

// Deprecations reports every legacy NERD_* variable that is currently set to a
// value this package would act on, together with the canonical replacement.
// Empty when nothing legacy is in play.
//
// It returns strings rather than logging because features is a leaf package and
// must not import internal/logging. The caller of SetActive is responsible for
// surfacing these at boot; `nerd features` and `/features` also print them.
//
// A legacy variable that is set but shadowed by the canonical one is still
// reported: it is doing nothing, which is exactly what the operator needs to
// be told before they debug the wrong knob.
func Deprecations() []string {
	var out []string
	report := func(name, canonical, legacy string) {
		if legacy == "" || strings.TrimSpace(os.Getenv(legacy)) == "" {
			return
		}
		if strings.TrimSpace(os.Getenv(canonical)) != "" {
			out = append(out, fmt.Sprintf(
				"%s is deprecated and ignored here because %s is also set (flag %s); remove %s",
				legacy, canonical, name, legacy))
			return
		}
		out = append(out, fmt.Sprintf(
			"%s is deprecated (flag %s); rename it to %s", legacy, name, canonical))
	}
	for _, f := range boolFlags {
		report(f.name, f.envVar, f.legacyEnvVar)
	}
	for _, f := range intFlags {
		report(f.name, f.envVar, f.legacyEnvVar)
	}
	return out
}

// Summary returns a short single-line description of the flags in effect,
// suitable for boot-time logging by the caller of SetActive.
//
// It reports RESOLVED values, not raw config fields. The previous version
// printed the active FeaturesConfig's *bool fields, so a key absent from
// config.json logged as "unset" even when an environment variable was
// forcing it on — precisely the case an operator reads this line to
// diagnose. It also returned "defaults active" whenever no config had been
// installed, hiding env overrides entirely.
func Summary() string {
	parts := make([]string, 0, len(boolFlags)+2)
	for _, f := range Resolved() {
		// Mark non-default sources so a scan of the line shows what was
		// deliberately changed.
		switch f.Source {
		case SourceDefault:
			parts = append(parts, fmt.Sprintf("%s=%t", f.Name, f.Value))
		default:
			parts = append(parts, fmt.Sprintf("%s=%t(%s)", f.Name, f.Value, f.Source))
		}
	}
	parts = append(parts,
		fmt.Sprintf("fast_scan_workers=%d", FastScanWorkers()),
		fmt.Sprintf("fast_ast_max_bytes=%d", FastASTMaxBytes()),
	)
	return "features: " + strings.Join(parts, " ")
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

// resolveBool implements the canonical-env → legacy-env → active → default
// precedence for a boolean toggle. Both env vars are queried via os.Getenv and
// accept "1" / "true" (case-insensitive) for true, "0" / "false" for false.
// Any other non-empty value is treated as "no override" (we don't want a stray
// export to silently flip a bit).
//
// legacyEnvVar may be "". It is read only when the canonical name is absent or
// unparseable, so an operator migrating a shell profile can set the new name
// and delete the old one in either order without a window where both apply.
func resolveBool(envVar, legacyEnvVar string, fromActive func(*FeaturesConfig) *bool, def bool) bool {
	if v := envBool(envVar); v != nil {
		return *v
	}
	if legacyEnvVar != "" {
		if v := envBool(legacyEnvVar); v != nil {
			return *v
		}
	}
	if a := active.Load(); a != nil {
		if p := fromActive(a); p != nil {
			return *p
		}
	}
	return def
}

// envBool parses a boolean environment override, returning nil when the
// variable is absent, empty, or holds an unrecognized value — a stray export
// must not silently flip a bit.
//
// Matching is genuinely case-insensitive. The old switch listed only "true",
// "TRUE" and "True", so a perfectly ordinary `CODENERD_DIFF_EVAL=True` worked
// while `=tRue` silently fell through to the default, contradicting the
// package doc.
func envBool(envVar string) *bool {
	v := strings.TrimSpace(os.Getenv(envVar))
	if v == "" {
		return nil
	}
	t, f := true, false
	switch {
	case v == "1", strings.EqualFold(v, "true"):
		return &t
	case v == "0", strings.EqualFold(v, "false"):
		return &f
	}
	// Anything else ("yes", "maybe", a typo) is deliberately not an override.
	return nil
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
	return resolveBool("CODENERD_DIFF_EVAL", "",
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
	return resolveBool("CODENERD_FLIGHT_RECORDER", "NERD_FLIGHTREC",
		func(f *FeaturesConfig) *bool { return f.FlightRecorder }, false)
}

// IsProvenanceEnabled gates Mangle's DerivationRecorder. Defaults OFF
// because it allocates per-derivation event objects; enable when the
// user wants /explain to give precise proof trees.
func IsProvenanceEnabled() bool {
	return resolveBool("CODENERD_PROVENANCE", "",
		func(f *FeaturesConfig) *bool { return f.Provenance }, false)
}

// IsSystemShardsEnabled is the master switch for booting the
// autopoiesis/observer background shards. Note: this is independent of
// the legacy NERD_DISABLE_SYSTEM_SHARDS env var (which is parsed at the
// call site as a comma-separated list of per-shard names to disable).
// Setting CODENERD_SYSTEM_SHARDS=0 turns off ALL system shards
// regardless of which ones are in the legacy list. Default ON.
func IsSystemShardsEnabled() bool {
	return resolveBool("CODENERD_SYSTEM_SHARDS", "",
		func(f *FeaturesConfig) *bool { return f.SystemShards }, true)
}

// IsPerShardFactsEnabled gates the per-shard fact store coordinator
// (ShardFactRouter in internal/core/shard_fact_router.go). The cortex
// builds and installs the router on shard registration when this flag
// is on. Default OFF: the router dispatches single predicates but does not
// evaluate joins across shards, so cross-domain rules stop deriving when it is
// installed. See FullyEnabledFeaturesConfig for the 2026-08-15 audit.
func IsPerShardFactsEnabled() bool {
	return resolveBool("CODENERD_PER_SHARD_FACTS", "",
		func(f *FeaturesConfig) *bool { return f.PerShardFacts }, false)
}

// IsDarkModeEnabled overrides the TUI palette to dark. The config
// `theme` key still controls light/dark when this flag is false.
func IsDarkModeEnabled() bool {
	return resolveBool("CODENERD_DARK_MODE", "",
		func(f *FeaturesConfig) *bool { return f.DarkMode }, false)
}

// IsOnboardingSkipped lets the chat boot skip the first-run wizard.
func IsOnboardingSkipped() bool {
	return resolveBool("CODENERD_SKIP_ONBOARDING", "NERD_SKIP_ONBOARDING",
		func(f *FeaturesConfig) *bool { return f.SkipOnboarding }, false)
}

// IsTaxonomyFastEnabled selects the fast codepath in verify_taxonomy, which
// skips the scenario sweep entirely.
//
// Default OFF. It used to default ON, which nothing noticed because nothing
// read the flag: verify_taxonomy checked os.Getenv("CODENERD_TAXONOMY_FAST")
// directly and only honoured the literal "1". Wiring the tool to this accessor
// with the old default would have made the verification tool skip verification
// on every ordinary run.
func IsTaxonomyFastEnabled() bool {
	return resolveBool("CODENERD_TAXONOMY_FAST", "",
		func(f *FeaturesConfig) *bool { return f.TaxonomyFast }, false)
}

// FastScanWorkers returns the configured worker count, or zero when
// the call site should pick a default. Env var wins over config, with the
// canonical CODENERD_ name taking precedence over the legacy NERD_ one.
func FastScanWorkers() int {
	if v, ok := envInt(intFlags[0].envVar, intFlags[0].legacyEnvVar); ok {
		return int(v)
	}
	if a := active.Load(); a != nil {
		return a.FastScanWorkers
	}
	return 0
}

// FastASTMaxBytes returns the configured size cutoff, or zero for default.
func FastASTMaxBytes() int64 {
	if v, ok := envInt(intFlags[1].envVar, intFlags[1].legacyEnvVar); ok {
		return v
	}
	if a := active.Load(); a != nil {
		return a.FastASTMaxBytes
	}
	return 0
}

// envInt reads a positive integer override, preferring the canonical name and
// falling back to the legacy one. A non-numeric or non-positive value is not an
// override — same rule as envBool, so a typo cannot silently change a limit.
func envInt(envVar, legacyEnvVar string) (int64, bool) {
	for _, name := range [2]string{envVar, legacyEnvVar} {
		if name == "" {
			continue
		}
		v := strings.TrimSpace(os.Getenv(name))
		if v == "" {
			continue
		}
		if n, err := parseInt64(v); err == nil && n > 0 {
			return n, true
		}
	}
	return 0, false
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
