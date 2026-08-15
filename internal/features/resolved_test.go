package features

import (
	"strings"
	"testing"
)

// TestResolved_PrecedenceMatrix is the table-driven precedence check the corpus
// asked for: every boolean accessor, across env / config / default, asserting
// both the value and the source attribution.
func TestResolved_PrecedenceMatrix(t *testing.T) {
	for _, spec := range boolFlags {
		spec := spec
		t.Run(spec.name, func(t *testing.T) {
			find := func() Flag {
				t.Helper()
				for _, f := range Resolved() {
					if f.Name == spec.name {
						return f
					}
				}
				t.Fatalf("Resolved() omitted %q", spec.name)
				return Flag{}
			}

			t.Run("default", func(t *testing.T) {
				SetActive(nil)
				t.Cleanup(func() { SetActive(nil) })

				got := find()
				if got.Source != SourceDefault {
					t.Errorf("Source = %q, want %q", got.Source, SourceDefault)
				}
				if got.Value != spec.def {
					t.Errorf("Value = %v, want default %v", got.Value, spec.def)
				}
			})

			// Config wins over the default, in both directions.
			for _, want := range []bool{true, false} {
				want := want
				t.Run("config_"+boolWord(want), func(t *testing.T) {
					cfg := &FeaturesConfig{}
					setFlag(t, cfg, spec.name, want)
					SetActive(cfg)
					t.Cleanup(func() { SetActive(nil) })

					got := find()
					if got.Source != SourceConfig {
						t.Errorf("Source = %q, want %q", got.Source, SourceConfig)
					}
					if got.Value != want {
						t.Errorf("Value = %v, want %v", got.Value, want)
					}
				})
			}

			// Env wins over config, in both directions.
			for _, tc := range []struct {
				env  string
				want bool
			}{{"1", true}, {"true", true}, {"TRUE", true}, {"tRuE", true},
				{"0", false}, {"false", false}, {"FALSE", false}, {"fAlSe", false}} {
				tc := tc
				t.Run("env_"+tc.env, func(t *testing.T) {
					// Install the OPPOSITE value in config so a passing test
					// proves env actually won rather than coinciding.
					cfg := &FeaturesConfig{}
					setFlag(t, cfg, spec.name, !tc.want)
					SetActive(cfg)
					t.Cleanup(func() { SetActive(nil) })
					t.Setenv(spec.envVar, tc.env)

					got := find()
					if got.Source != SourceEnv {
						t.Errorf("Source = %q, want %q", got.Source, SourceEnv)
					}
					if got.Value != tc.want {
						t.Errorf("Value = %v, want %v", got.Value, tc.want)
					}
				})
			}

			// An unrecognized env value must not override anything.
			t.Run("env_garbage_falls_through", func(t *testing.T) {
				cfg := &FeaturesConfig{}
				setFlag(t, cfg, spec.name, !spec.def)
				SetActive(cfg)
				t.Cleanup(func() { SetActive(nil) })
				t.Setenv(spec.envVar, "maybe")

				got := find()
				if got.Source != SourceConfig {
					t.Errorf("Source = %q, want %q (garbage env must not override)", got.Source, SourceConfig)
				}
				if got.Value != !spec.def {
					t.Errorf("Value = %v, want %v", got.Value, !spec.def)
				}
			})
		})
	}
}

func boolWord(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// setFlag writes one named flag on cfg. Kept explicit rather than reflective so
// a renamed field fails to compile instead of silently skipping coverage.
func setFlag(t *testing.T, cfg *FeaturesConfig, name string, v bool) {
	t.Helper()
	p := &v
	switch name {
	case "diff_eval":
		cfg.DiffEval = p
	case "flight_recorder":
		cfg.FlightRecorder = p
	case "provenance":
		cfg.Provenance = p
	case "system_shards":
		cfg.SystemShards = p
	case "per_shard_facts":
		cfg.PerShardFacts = p
	case "dark_mode":
		cfg.DarkMode = p
	case "skip_onboarding":
		cfg.SkipOnboarding = p
	case "taxonomy_fast":
		cfg.TaxonomyFast = p
	default:
		t.Fatalf("setFlag has no case for %q — add it when adding a flag", name)
	}
}

// TestResolved_ShouldCoverEveryAccessor guards the boolFlags table against
// drifting from the public accessors it is supposed to mirror.
func TestResolved_ShouldMatchAccessors(t *testing.T) {
	SetActive(nil)
	t.Cleanup(func() { SetActive(nil) })

	accessors := map[string]func() bool{
		"diff_eval":       IsDiffEvalEnabled,
		"flight_recorder": IsFlightRecorderEnabled,
		"provenance":      IsProvenanceEnabled,
		"system_shards":   IsSystemShardsEnabled,
		"per_shard_facts": IsPerShardFactsEnabled,
		"dark_mode":       IsDarkModeEnabled,
		"skip_onboarding": IsOnboardingSkipped,
		"taxonomy_fast":   IsTaxonomyFastEnabled,
	}

	resolved := Resolved()
	if len(resolved) != len(accessors) {
		t.Fatalf("Resolved() has %d flags, accessors map has %d", len(resolved), len(accessors))
	}
	for _, f := range resolved {
		fn, ok := accessors[f.Name]
		if !ok {
			t.Errorf("Resolved() reports %q with no matching accessor", f.Name)
			continue
		}
		if got := fn(); got != f.Value {
			t.Errorf("%s: accessor returned %v, Resolved() reported %v", f.Name, got, f.Value)
		}
	}
}

// TestSummary_ShouldNotLeakPointers pins the format contract.
func TestSummary_ShouldBeSingleLineAndPointerFree(t *testing.T) {
	SetActive(&FeaturesConfig{})
	t.Cleanup(func() { SetActive(nil) })

	got := Summary()
	if strings.Contains(got, "\n") {
		t.Errorf("Summary must be a single line: %q", got)
	}
	if strings.Contains(got, "0x") {
		t.Errorf("Summary leaked a pointer address: %q", got)
	}
	if !strings.HasPrefix(got, "features: ") {
		t.Errorf("Summary lost its prefix: %q", got)
	}
}

// TestSetActive_ConcurrentWithReads exercises the atomic pointer under -race.
func TestSetActive_ConcurrentWithReads(t *testing.T) {
	t.Cleanup(func() { SetActive(nil) })

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 500; i++ {
			cfg := FullyEnabledFeaturesConfig()
			SetActive(&cfg)
			SetActive(nil)
		}
	}()

	for i := 0; i < 500; i++ {
		_ = Resolved()
		_ = Summary()
		_ = IsDiffEvalEnabled()
		_ = FastScanWorkers()
	}
	<-done
}

// TestSetActive_ShouldCopyTheConfig ensures a caller mutating its struct after
// SetActive cannot change what the registry reports.
func TestSetActive_ShouldCopyTheConfig(t *testing.T) {
	v := true
	cfg := &FeaturesConfig{DiffEval: &v}
	SetActive(cfg)
	t.Cleanup(func() { SetActive(nil) })

	if !IsDiffEvalEnabled() {
		t.Fatal("precondition: diff_eval should be on")
	}
	cfg.DiffEval = nil // caller mutates its own struct afterwards
	if !IsDiffEvalEnabled() {
		t.Error("SetActive did not copy the config; caller mutation leaked in")
	}
}
