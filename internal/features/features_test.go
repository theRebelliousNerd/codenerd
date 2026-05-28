package features

import (
	"testing"
)

// TestResolveBoolPrecedence locks in the documented precedence:
// env var > active config > compile-time default. The env var is
// accepted only as "1"/"true"/"0"/"false" (case-insensitive forms in
// the implementation); other values fall through.
func TestResolveBoolPrecedence(t *testing.T) {
	t.Run("env wins over active", func(t *testing.T) {
		f := false
		SetActive(&FeaturesConfig{DiffEval: &f})
		t.Cleanup(func() { SetActive(nil) })

		t.Setenv("CODENERD_DIFF_EVAL", "1")
		if got := IsDiffEvalEnabled(); !got {
			t.Fatalf("env=1 should win over active=false; got %v", got)
		}
	})

	t.Run("active wins over default", func(t *testing.T) {
		// Default for DiffEval is true, so we set active=false and
		// verify the active value sticks.
		f := false
		SetActive(&FeaturesConfig{DiffEval: &f})
		t.Cleanup(func() { SetActive(nil) })

		t.Setenv("CODENERD_DIFF_EVAL", "")
		if got := IsDiffEvalEnabled(); got {
			t.Fatalf("active=false should win over default=true; got %v", got)
		}
	})

	t.Run("default kicks in when active nil and env empty", func(t *testing.T) {
		SetActive(nil)
		t.Setenv("CODENERD_DIFF_EVAL", "")
		// DiffEval defaults OFF at compile time (see DefaultFeaturesConfig
		// rationale); .nerd/config.json flips it on in production.
		if got := IsDiffEvalEnabled(); got {
			t.Fatalf("default should be false; got %v", got)
		}
	})

	t.Run("invalid env value falls through to active", func(t *testing.T) {
		f := false
		SetActive(&FeaturesConfig{DiffEval: &f})
		t.Cleanup(func() { SetActive(nil) })

		// "yes" is not in the accepted forms — should NOT override.
		t.Setenv("CODENERD_DIFF_EVAL", "yes")
		if got := IsDiffEvalEnabled(); got {
			t.Fatalf("invalid env should not override active=false; got %v", got)
		}
	})

	t.Run("env 0 forces off even when active true", func(t *testing.T) {
		t1 := true
		SetActive(&FeaturesConfig{DiffEval: &t1})
		t.Cleanup(func() { SetActive(nil) })

		t.Setenv("CODENERD_DIFF_EVAL", "0")
		if got := IsDiffEvalEnabled(); got {
			t.Fatalf("env=0 should force off; got %v", got)
		}
	})
}

// TestPerShardFactsPrecedence: now that Track D's coordinator
// (ShardFactRouter) is wired, the flag resolves through the standard
// env → active → default precedence. Default is OFF since the
// coordinator is not yet auto-wired into the production shard
// factory; explicit opt-in via .nerd/config.json or the env var.
func TestPerShardFactsPrecedence(t *testing.T) {
	SetActive(nil)
	t.Setenv("CODENERD_PER_SHARD_FACTS", "")
	if got := IsPerShardFactsEnabled(); got {
		t.Fatalf("default off; got %v", got)
	}

	t1 := true
	SetActive(&FeaturesConfig{PerShardFacts: &t1})
	t.Cleanup(func() { SetActive(nil) })
	if got := IsPerShardFactsEnabled(); !got {
		t.Fatalf("active=true should win over default; got %v", got)
	}

	t.Setenv("CODENERD_PER_SHARD_FACTS", "0")
	if got := IsPerShardFactsEnabled(); got {
		t.Fatalf("env=0 should override active=true; got %v", got)
	}
}

// TestSystemShardsLegacyEnvIgnored confirms that the legacy
// NERD_DISABLE_SYSTEM_SHARDS env var (which is a per-shard disable
// list, not a master switch) does NOT bleed into the master toggle.
// The master switch is CODENERD_SYSTEM_SHARDS only.
func TestSystemShardsLegacyEnvIgnored(t *testing.T) {
	SetActive(nil)
	t.Cleanup(func() { SetActive(nil) })

	t.Setenv("NERD_DISABLE_SYSTEM_SHARDS", "autopoiesis")
	t.Setenv("CODENERD_SYSTEM_SHARDS", "")

	if got := IsSystemShardsEnabled(); !got {
		t.Fatalf("legacy env should not affect master switch; got %v", got)
	}
}

// TestSetActiveCopySemantics proves SetActive snapshots its argument so
// callers cannot mutate the active pointer's struct after install.
func TestSetActiveCopySemantics(t *testing.T) {
	f := false
	cfg := &FeaturesConfig{DiffEval: &f}
	SetActive(cfg)
	t.Cleanup(func() { SetActive(nil) })

	// Mutate the original after install — this should NOT affect Active.
	tval := true
	cfg.DiffEval = &tval

	a := Active()
	if a == nil || a.DiffEval == nil || *a.DiffEval != false {
		t.Fatalf("SetActive should snapshot; got active=%+v", a)
	}
}

// TestNumericAccessors covers FastScanWorkers / FastASTMaxBytes:
// env wins, then active, then zero (meaning "use default" at the
// call site).
func TestNumericAccessors(t *testing.T) {
	SetActive(nil)
	t.Setenv("NERD_FAST_SCAN_WORKERS", "")
	t.Setenv("NERD_FAST_AST_MAX_BYTES", "")
	if FastScanWorkers() != 0 || FastASTMaxBytes() != 0 {
		t.Fatalf("expected zeros when no source set")
	}

	SetActive(&FeaturesConfig{FastScanWorkers: 17, FastASTMaxBytes: 1024 * 1024})
	t.Cleanup(func() { SetActive(nil) })
	if got := FastScanWorkers(); got != 17 {
		t.Fatalf("config-driven workers; want 17 got %d", got)
	}
	if got := FastASTMaxBytes(); got != 1024*1024 {
		t.Fatalf("config-driven max bytes; want 1MiB got %d", got)
	}

	t.Setenv("NERD_FAST_SCAN_WORKERS", "32")
	if got := FastScanWorkers(); got != 32 {
		t.Fatalf("env should win; want 32 got %d", got)
	}
}
