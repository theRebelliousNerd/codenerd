package config

import (
	"encoding/json"
	"testing"
)

// unmarshalReflection decodes a reflection JSON fragment through the real
// ReflectionConfig.UnmarshalJSON (which tracks explicitly-set fields), matching
// the production load path.
func unmarshalReflection(t *testing.T, body string) *ReflectionConfig {
	t.Helper()
	var rc ReflectionConfig
	if err := json.Unmarshal([]byte(body), &rc); err != nil {
		t.Fatalf("unmarshal %q: %v", body, err)
	}
	return &rc
}

// TestGetReflectionConfig_ExplicitZeroMinScorePreserved is the regression:
// min_score:0 means "no similarity floor — recall everything" and must NOT be
// clobbered to the 0.70 default. Before the fix, GetReflectionConfig treated
// MinScore==0 as "unset" and forced 0.70, silently dropping most System-2
// recalls (the consumer applies `if weighted < cfg.MinScore { continue }`).
func TestGetReflectionConfig_ExplicitZeroMinScorePreserved(t *testing.T) {
	c := &UserConfig{Reflection: unmarshalReflection(t, `{"min_score": 0}`)}

	got := c.GetReflectionConfig()
	if got.MinScore != 0 {
		t.Errorf("explicit min_score:0 must be preserved, got %v", got.MinScore)
	}
	// Fields the user omitted still receive their defaults.
	if got.TopK != DefaultReflectionConfig().TopK {
		t.Errorf("omitted TopK should default to %d, got %d", DefaultReflectionConfig().TopK, got.TopK)
	}
}

// TestGetReflectionConfig_OmittedMinScoreGetsDefault: when min_score is absent,
// the 0.70 default must still apply (the fix must not disable defaulting).
func TestGetReflectionConfig_OmittedMinScoreGetsDefault(t *testing.T) {
	c := &UserConfig{Reflection: unmarshalReflection(t, `{"top_k": 3}`)}

	got := c.GetReflectionConfig()
	if want := DefaultReflectionConfig().MinScore; got.MinScore != want {
		t.Errorf("omitted min_score should default to %v, got %v", want, got.MinScore)
	}
	if got.TopK != 3 {
		t.Errorf("explicit top_k should be honored, got %d", got.TopK)
	}
}

// TestGetReflectionConfig_ExplicitNonZeroMinScore: an ordinary explicit value
// passes through unchanged.
func TestGetReflectionConfig_ExplicitNonZeroMinScore(t *testing.T) {
	c := &UserConfig{Reflection: unmarshalReflection(t, `{"min_score": 0.42}`)}

	if got := c.GetReflectionConfig().MinScore; got != 0.42 {
		t.Errorf("explicit min_score:0.42 should be honored, got %v", got)
	}
}

// TestGetReflectionConfig_NilReflectionUsesDefaults: no reflection block at all
// yields the full default set.
func TestGetReflectionConfig_NilReflectionUsesDefaults(t *testing.T) {
	c := &UserConfig{}
	got := c.GetReflectionConfig()
	if got != DefaultReflectionConfig() {
		t.Errorf("nil reflection should return defaults, got %+v", got)
	}
}

// TestReflectionUnmarshal_ExplicitZeroEnabledStillWorks guards the sibling
// pattern this fix mirrors: enabled:false is an explicit value, not "unset".
func TestReflectionUnmarshal_ExplicitZeroEnabledStillWorks(t *testing.T) {
	c := &UserConfig{Reflection: unmarshalReflection(t, `{"enabled": false, "min_score": 0}`)}
	got := c.GetReflectionConfig()
	if got.Enabled {
		t.Error("explicit enabled:false must be preserved")
	}
	if got.MinScore != 0 {
		t.Errorf("explicit min_score:0 must be preserved alongside enabled:false, got %v", got.MinScore)
	}
}
